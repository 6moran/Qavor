package ingestion

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PythonWorker owns one long-lived local JSONL parser process. Calls to Parse
// are serialized because its stdin/stdout protocol has one response per request.
type PythonWorker struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	decoder    *json.Decoder
	encoder    *json.Encoder
	controller parserProcessController
	opts       PythonWorkerOptions

	parseMu sync.Mutex
	stateMu sync.Mutex
	closed  bool
	tasks   int

	// readerMu gates Decode registration before any goroutine starts. Closing
	// seals registrations, then waits for every registered reader to finish
	// before the single Cmd.Wait can close StdoutPipe.
	readerMu      sync.Mutex
	readerCond    *sync.Cond
	readerClosed  bool
	activeReaders int

	stdinOnce sync.Once
	stdinErr  error
	waitOnce  sync.Once
	waitMu    sync.Mutex
	waitErr   error
	waitDone  chan struct{}
	forceOnce sync.Once

	controllerMu     sync.Mutex
	controllerClosed bool
}

func startPythonWorker(ctx context.Context, opts PythonWorkerOptions) (*PythonWorker, error) {
	if opts.PythonPath == "" || opts.ScriptPath == "" {
		return nil, fmt.Errorf("python path and parser script path are required")
	}

	cmd := exec.Command(opts.PythonPath, opts.ScriptPath, "--serve-stdio")
	cmd.Env = append(os.Environ(), opts.ExtraEnv...)
	if err := configureParserProcess(cmd); err != nil {
		return nil, fmt.Errorf("configure parser process: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open parser stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open parser stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open parser stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start parser process: %w", err)
	}
	controller, err := attachParserProcess(cmd)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("attach parser process: %w", err)
	}

	w := &PythonWorker{
		cmd:        cmd,
		stdin:      stdin,
		decoder:    json.NewDecoder(bufio.NewReader(stdout)),
		encoder:    json.NewEncoder(stdin),
		controller: controller,
		opts:       opts,
		waitDone:   make(chan struct{}),
	}
	w.readerCond = sync.NewCond(&w.readerMu)
	go drainParserStderr(stderr)

	readyCh := make(chan parserReadyRead, 1)
	if !w.registerStdoutReader() {
		w.forceClose()
		return nil, ErrPythonWorkerCrashed
	}
	w.watchContext(ctx)
	go func() {
		var ready parserReady
		err := w.decoder.Decode(&ready)
		w.finishStdoutReader()
		readyCh <- parserReadyRead{ready: ready, err: err}
	}()
	select {
	case <-ctx.Done():
		w.forceClose()
		return nil, ctx.Err()
	case read := <-readyCh:
		if read.err != nil {
			w.forceClose()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, parserReadError(read.err)
		}
		if err := validateParserReady(read.ready); err != nil {
			w.forceClose()
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		w.forceClose()
		return nil, err
	}
	return w, nil
}

func (w *PythonWorker) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	w.parseMu.Lock()
	defer w.parseMu.Unlock()
	if err := ctx.Err(); err != nil {
		w.forceClose()
		return ParseResult{}, err
	}
	if w.isClosed() {
		return ParseResult{}, ErrPythonWorkerCrashed
	}

	tmpDir, err := os.MkdirTemp("", "qavor-parse-*")
	if err != nil {
		return ParseResult{}, fmt.Errorf("create parser temporary directory: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil {
			logWarn("清理 Python Worker 临时目录失败", zap.String("dir", tmpDir), zap.Error(rmErr))
		}
	}()

	filename := filepath.Base(input.Filename)
	if filename == "." || filename == string(filepath.Separator) {
		return ParseResult{}, fmt.Errorf("parser filename is required")
	}
	localPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(localPath, input.Content, 0o600); err != nil {
		return ParseResult{}, fmt.Errorf("write parser input: %w", err)
	}

	requestID := uuid.NewString()
	request := parserRequest{
		Type:        "parse",
		Version:     parserProtocolVersion,
		RequestID:   requestID,
		InputPath:   localPath,
		Filename:    filename,
		OCREngine:   w.opts.OCR.Engine,
		OCRAPIURL:   w.opts.OCR.APIURL,
		OCRAPIKey:   w.opts.OCR.APIKey,
		OCRAPIModel: w.opts.OCR.APIModel,
	}
	if err := w.encoder.Encode(request); err != nil {
		w.forceClose()
		return ParseResult{}, fmt.Errorf("%w: write request: %v", ErrPythonWorkerCrashed, err)
	}

	responseCh := make(chan parserRead, 1)
	if !w.registerStdoutReader() {
		return ParseResult{}, ErrPythonWorkerCrashed
	}
	go func() {
		var response parserResponse
		err := w.decoder.Decode(&response)
		w.finishStdoutReader()
		responseCh <- parserRead{response: response, err: err}
	}()

	select {
	case <-ctx.Done():
		w.forceClose()
		return ParseResult{}, ctx.Err()
	case read := <-responseCh:
		if read.err != nil {
			w.forceClose()
			return ParseResult{}, parserReadError(read.err)
		}
		if err := validateParserResponse(read.response, requestID); err != nil {
			w.forceClose()
			return ParseResult{}, err
		}
		w.incrementTasks()
		if !read.response.OK {
			return ParseResult{}, &ParserError{Code: read.response.Error.Code, Message: read.response.Error.Message}
		}
		result := *read.response.Result
		result.Markdown = ReplaceImageLinks(result.Markdown, result.PicturePaths, DeriveImageFolder(input.Path), w.opts.Images)
		return result, nil
	}
}

type parserRead struct {
	response parserResponse
	err      error
}

type parserReadyRead struct {
	ready parserReady
	err   error
}

func (w *PythonWorker) Close() error {
	w.markClosed()
	w.parseMu.Lock()
	defer w.parseMu.Unlock()
	// A normal close lets an already-started Parse finish registering and
	// draining its response before sealing new stdout readers. Forced shutdown
	// uses forceClose instead, which seals immediately and terminates first.
	w.sealStdoutReaders()
	closeErr := w.closeStdin()
	if waitErr := w.wait(); waitErr != nil && !isExpectedProcessExit(waitErr) && closeErr == nil {
		return waitErr
	}
	return closeErr
}

func (w *PythonWorker) forceClose() {
	w.markClosed()
	w.sealStdoutReaders()
	_ = w.closeStdin()
	w.forceOnce.Do(func() { w.terminateProcess() })
	_ = w.wait()
}

func (w *PythonWorker) watchContext(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			w.forceClose()
		case <-w.waitDone:
		}
	}()
}

func (w *PythonWorker) closeStdin() error {
	w.stdinOnce.Do(func() {
		if w.stdin != nil {
			if err := w.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				w.stdinErr = err
			}
		}
	})
	return w.stdinErr
}

func (w *PythonWorker) terminateProcess() {
	w.controllerMu.Lock()
	defer w.controllerMu.Unlock()
	if w.controller != nil && !w.controllerClosed {
		_ = w.controller.terminate()
		return
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
}

func (w *PythonWorker) wait() error {
	w.waitOnce.Do(func() {
		go func() {
			w.waitForStdoutReaders()
			err := w.cmd.Wait()
			w.controllerMu.Lock()
			if w.controller != nil && !w.controllerClosed {
				_ = w.controller.close()
				w.controllerClosed = true
			}
			w.controllerMu.Unlock()
			w.waitMu.Lock()
			w.waitErr = err
			w.waitMu.Unlock()
			close(w.waitDone)
		}()
	})
	<-w.waitDone
	w.waitMu.Lock()
	defer w.waitMu.Unlock()
	return w.waitErr
}

func (w *PythonWorker) registerStdoutReader() bool {
	w.readerMu.Lock()
	defer w.readerMu.Unlock()
	if w.readerClosed {
		return false
	}
	w.activeReaders++
	return true
}

func (w *PythonWorker) finishStdoutReader() {
	w.readerMu.Lock()
	w.activeReaders--
	if w.activeReaders == 0 {
		w.readerCond.Broadcast()
	}
	w.readerMu.Unlock()
}

func (w *PythonWorker) sealStdoutReaders() {
	w.readerMu.Lock()
	w.readerClosed = true
	w.readerMu.Unlock()
}

func (w *PythonWorker) waitForStdoutReaders() {
	w.readerMu.Lock()
	for w.activeReaders > 0 {
		w.readerCond.Wait()
	}
	w.readerMu.Unlock()
}

func (w *PythonWorker) isClosed() bool {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	return w.closed
}

func (w *PythonWorker) markClosed() {
	w.stateMu.Lock()
	w.closed = true
	w.stateMu.Unlock()
}

func (w *PythonWorker) incrementTasks() {
	w.stateMu.Lock()
	w.tasks++
	w.stateMu.Unlock()
}

func (w *PythonWorker) reachedMaxTasks() bool {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	return w.opts.MaxTasks > 0 && w.tasks >= w.opts.MaxTasks
}

func parserReadError(err error) error {
	if errors.Is(err, io.EOF) {
		return ErrPythonWorkerCrashed
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Errorf("%w: invalid JSON", ErrPythonProtocol)
	}
	return fmt.Errorf("%w: read response: %v", ErrPythonWorkerCrashed, err)
}

func isExpectedProcessExit(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func drainParserStderr(stderr io.Reader) {
	buffer := make([]byte, 32*1024)
	for {
		count, err := stderr.Read(buffer)
		if count > 0 {
			logWarn("Python parser worker", zap.ByteString("stderr", buffer[:count]))
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			logWarn("读取 Python parser stderr 失败", zap.Error(err))
		}
		return
	}
}
