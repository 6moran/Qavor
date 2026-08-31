package ingestion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func startTestPythonWorker(t *testing.T) *PythonWorker {
	return startTestPythonWorkerWithOptions(t, context.Background(), PythonWorkerOptions{})
}

func startTestPythonWorkerWithOptions(t *testing.T, ctx context.Context, options PythonWorkerOptions) *PythonWorker {
	t.Helper()
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable: ", err)
	}
	options.PythonPath = python
	options.ScriptPath = filepath.Join("testdata", "stdio_worker.py")
	worker, err := startPythonWorker(ctx, options)
	if err != nil {
		t.Fatalf("start worker: %v", err)
	}
	return worker
}

func TestPythonWorkerCompletesReadyHandshake(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.Close()

	result, err := worker.Parse(context.Background(), ParseInput{Filename: "ready.pdf", Content: []byte("ready")})
	if err != nil {
		t.Fatalf("parse after ready handshake: %v", err)
	}
	if result.Markdown == "" {
		t.Fatal("worker returned an empty result")
	}
}

func TestPythonWorkerReusesProcess(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.Close()

	first, err := worker.Parse(context.Background(), ParseInput{Filename: "one.pdf", Content: []byte("one")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := worker.Parse(context.Background(), ParseInput{Filename: "two.pdf", Content: []byte("two")})
	if err != nil {
		t.Fatal(err)
	}
	if pidFromMarkdown(first.Markdown) != pidFromMarkdown(second.Markdown) {
		t.Fatalf("worker process was not reused: %q / %q", first.Markdown, second.Markdown)
	}
}

func TestPythonWorkerDrainsStderrWithoutCorruptingResult(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.Close()

	result, err := worker.Parse(context.Background(), ParseInput{Filename: "stderr.pdf", Content: []byte("stderr")})
	if err != nil {
		t.Fatalf("parse with worker stderr output: %v", err)
	}
	if pidFromMarkdown(result.Markdown) == "" {
		t.Fatalf("unexpected result: %q", result.Markdown)
	}
}

func TestPythonWorkerDrainsStderrLongerThanScannerToken(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.forceClose()

	parseDone := make(chan error, 1)
	go func() {
		_, err := worker.Parse(context.Background(), ParseInput{Filename: "long-stderr.pdf", Content: []byte("stderr")})
		parseDone <- err
	}()

	select {
	case err := <-parseDone:
		if err != nil {
			t.Fatalf("parse with a long stderr record: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("long stderr output blocked the worker")
	}
}

func TestPythonWorkerStartCancellationInterruptsReadyHandshake(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable: ", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	marker := filepath.Join(t.TempDir(), "waiting")
	startDone := make(chan error, 1)
	go func() {
		_, err := startPythonWorker(ctx, PythonWorkerOptions{
			PythonPath: python,
			ScriptPath: filepath.Join("testdata", "stdio_worker.py"),
			ExtraEnv:   []string{"QAVOR_STDIO_MODE=no-ready", "QAVOR_TEST_READY_MARKER=" + marker, "QAVOR_TEST_EXIT_AFTER=3"},
		})
		startDone <- err
	}()

	waitForTestMarker(t, marker)
	cancel()
	select {
	case err := <-startDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("start error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not interrupt the ready handshake")
	}
}

func TestPythonWorkerStartContextCancellationUnblocksCloseAndActiveParse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	marker := filepath.Join(t.TempDir(), "blocked")
	worker := startTestPythonWorkerWithOptions(t, ctx, PythonWorkerOptions{
		ExtraEnv: []string{"QAVOR_TEST_BLOCK_MARKER=" + marker},
	})
	t.Cleanup(func() {
		cancel()
		if worker.cmd != nil && worker.cmd.Process != nil {
			_ = worker.cmd.Process.Kill()
		}
		worker.forceClose()
	})

	parseDone := make(chan error, 1)
	go func() {
		_, err := worker.Parse(context.Background(), ParseInput{Filename: "block.pdf", Content: []byte("block")})
		parseDone <- err
	}()
	waitForTestMarker(t, marker)

	closeDone := make(chan error, 1)
	go func() { closeDone <- worker.Close() }()
	waitForWorkerClosed(t, worker)
	cancel()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("close error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context cancellation did not unblock Close")
	}
	select {
	case err := <-parseDone:
		if !errors.Is(err, ErrPythonWorkerCrashed) {
			t.Fatalf("parse error = %v, want ErrPythonWorkerCrashed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context cancellation did not unblock Parse")
	}
}

func TestPythonWorkerConcurrentCloseAfterResponseStarts(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		marker := filepath.Join(t.TempDir(), "response-started")
		worker := startTestPythonWorkerWithOptions(t, context.Background(), PythonWorkerOptions{
			ExtraEnv: []string{"QAVOR_TEST_CLOSE_RACE_MARKER=" + marker},
		})

		parseDone := make(chan error, 1)
		go func() {
			result, err := worker.Parse(context.Background(), ParseInput{Filename: "close-race.pdf", Content: []byte("race")})
			if err == nil && len(result.Markdown) != 4*1024*1024 {
				err = errors.New("response was truncated during Close")
			}
			parseDone <- err
		}()
		waitForTestMarker(t, marker)

		closeDone := make(chan error, 1)
		go func() { closeDone <- worker.Close() }()
		select {
		case err := <-parseDone:
			if err != nil {
				t.Fatalf("attempt %d parse: %v", attempt, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("attempt %d Parse did not finish", attempt)
		}
		select {
		case err := <-closeDone:
			if err != nil {
				t.Fatalf("attempt %d Close: %v", attempt, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("attempt %d Close did not finish", attempt)
		}
	}
}

func TestPythonWorkerCancellationWaitsForRegisteredStdoutReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := startTestPythonWorkerWithOptions(t, ctx, PythonWorkerOptions{})
	registered := worker.registerStdoutReader()
	if !registered {
		t.Fatal("worker rejected a reader before shutdown")
	}
	finished := false
	t.Cleanup(func() {
		if !finished {
			worker.finishStdoutReader()
		}
		cancel()
		worker.forceClose()
	})

	cancel()
	select {
	case <-worker.waitDone:
		t.Fatal("Cmd.Wait completed before the registered reader was released")
	case <-time.After(100 * time.Millisecond):
	}

	worker.finishStdoutReader()
	finished = true
	select {
	case <-worker.waitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Cmd.Wait did not resume after the registered reader finished")
	}
	if worker.registerStdoutReader() {
		t.Fatal("shutdown allowed a new stdout reader registration")
	}
}

func TestPythonWorkerNormalCloseWaitsForParseBeforeSealingReaders(t *testing.T) {
	worker := startTestPythonWorker(t)
	worker.parseMu.Lock()
	parseLocked := true
	registered := false
	t.Cleanup(func() {
		if registered {
			worker.finishStdoutReader()
		}
		if parseLocked {
			worker.parseMu.Unlock()
		}
		worker.forceClose()
	})

	closeDone := make(chan error, 1)
	go func() { closeDone <- worker.Close() }()
	waitForWorkerClosed(t, worker)
	if !worker.registerStdoutReader() {
		t.Fatal("normal Close sealed stdout readers before the active Parse could register its response")
	}
	registered = true
	worker.finishStdoutReader()
	registered = false
	worker.parseMu.Unlock()
	parseLocked = false

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not finish after Parse registration completed")
	}
}

func TestPythonWorkerNormalCloseWaitsForRequestReaderRegistration(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "request-received")
	worker := startTestPythonWorkerWithOptions(t, context.Background(), PythonWorkerOptions{
		ExtraEnv: []string{"QAVOR_TEST_NORMAL_CLOSE_MARKER=" + marker},
	})
	worker.readerMu.Lock()
	locked := true
	t.Cleanup(func() {
		if locked {
			worker.readerMu.Unlock()
		}
		worker.forceClose()
	})

	parseDone := make(chan error, 1)
	go func() {
		result, err := worker.Parse(context.Background(), ParseInput{Filename: "normal-close-race.pdf", Content: []byte("race")})
		if err == nil && len(result.Markdown) != 2*1024*1024 {
			err = errors.New("response was truncated during normal Close")
		}
		parseDone <- err
	}()
	waitForTestMarker(t, marker)

	closeDone := make(chan error, 1)
	go func() { closeDone <- worker.Close() }()
	// Give Close time to queue behind readerMu before releasing Parse's
	// registration path. The worker has already received the request marker.
	time.Sleep(200 * time.Millisecond)
	worker.readerMu.Unlock()
	locked = false

	select {
	case err := <-parseDone:
		if err != nil {
			t.Fatalf("Parse error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Parse did not finish after normal Close")
	}
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("normal Close blocked after rejecting an unwatched response")
	}
}

func TestPythonWorkerRejectsMismatchedRequestID(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.Close()

	_, err := worker.Parse(context.Background(), ParseInput{Filename: "wrong-request-id.pdf", Content: []byte("id")})
	if !errors.Is(err, ErrPythonProtocol) {
		t.Fatalf("error = %v, want ErrPythonProtocol", err)
	}
}

func TestPythonWorkerRejectsInvalidJSON(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.Close()

	_, err := worker.Parse(context.Background(), ParseInput{Filename: "bad-json.pdf", Content: []byte("bad")})
	if !errors.Is(err, ErrPythonProtocol) {
		t.Fatalf("error = %v, want ErrPythonProtocol", err)
	}
}

func TestPythonWorkerReportsProcessExit(t *testing.T) {
	worker := startTestPythonWorker(t)
	defer worker.Close()

	_, err := worker.Parse(context.Background(), ParseInput{Filename: "crash.pdf", Content: []byte("crash")})
	if !errors.Is(err, ErrPythonWorkerCrashed) {
		t.Fatalf("error = %v, want ErrPythonWorkerCrashed", err)
	}
}

func TestPythonWorkerCloseRejectsNewRequests(t *testing.T) {
	worker := startTestPythonWorker(t)
	if err := worker.Close(); err != nil {
		t.Fatalf("close worker: %v", err)
	}

	_, err := worker.Parse(context.Background(), ParseInput{Filename: "after-close.pdf", Content: []byte("closed")})
	if !errors.Is(err, ErrPythonWorkerCrashed) {
		t.Fatalf("error = %v, want ErrPythonWorkerCrashed", err)
	}
}

func TestPythonWorkerTracksCompletedTasksForPoolRotation(t *testing.T) {
	worker := startTestPythonWorkerWithOptions(t, context.Background(), PythonWorkerOptions{MaxTasks: 2})
	defer worker.Close()

	if worker.reachedMaxTasks() {
		t.Fatal("new worker should not be ready for rotation")
	}
	if _, err := worker.Parse(context.Background(), ParseInput{Filename: "one.pdf", Content: []byte("one")}); err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if worker.reachedMaxTasks() {
		t.Fatal("worker should not rotate before MaxTasks responses")
	}
	if _, err := worker.Parse(context.Background(), ParseInput{Filename: "two.pdf", Content: []byte("two")}); err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if !worker.reachedMaxTasks() {
		t.Fatal("worker should be ready for pool-managed rotation after MaxTasks responses")
	}
}

func waitForTestMarker(t *testing.T, marker string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker did not create marker %q", marker)
}

func waitForWorkerClosed(t *testing.T, worker *PythonWorker) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if worker.isClosed() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Close did not begin")
}

func pidFromMarkdown(markdown string) string {
	match := regexp.MustCompile(`pid=(\d+)`).FindStringSubmatch(markdown)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}
