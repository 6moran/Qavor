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
	closeMu sync.Once
	closed  bool
	tasks   int
	done    chan struct{}
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
		done:       make(chan struct{}),
	}
	go drainParserStderr(stderr)

	var ready parserReady
	if err := w.decoder.Decode(&ready); err != nil {
		w.forceClose()
		return nil, parserReadError(err)
	}
	if err := validateParserReady(ready); err != nil {
		w.forceClose()
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			w.forceClose()
		case <-w.done:
		}
	}()
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
	go func() {
		var response parserResponse
		responseCh <- parserRead{response: response, err: w.decoder.Decode(&response)}
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

func (w *PythonWorker) Close() error {
	var closeErr error
	w.closeMu.Do(func() {
		w.markClosed()
		if w.stdin != nil {
			if err := w.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				closeErr = err
			}
		}
		if w.cmd != nil && w.cmd.Process != nil {
			if err := w.cmd.Wait(); err != nil && !isExpectedProcessExit(err) && closeErr == nil {
				closeErr = err
			}
		}
		if w.controller != nil {
			if err := w.controller.close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		close(w.done)
	})
	return closeErr
}

func (w *PythonWorker) forceClose() {
	w.closeMu.Do(func() {
		w.markClosed()
		if w.stdin != nil {
			_ = w.stdin.Close()
		}
		if w.controller != nil {
			_ = w.controller.terminate()
			_ = w.controller.close()
		} else if w.cmd != nil && w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
		if w.cmd != nil && w.cmd.Process != nil {
			_ = w.cmd.Wait()
		}
		close(w.done)
	})
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
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 4*1024), 1024*1024)
	for scanner.Scan() {
		logWarn("Python parser worker", zap.String("stderr", scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		logWarn("读取 Python parser stderr 失败", zap.Error(err))
	}
}
