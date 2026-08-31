package ingestion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestPythonPool(t *testing.T, size, maxTasks int, extraEnv ...string) *PythonWorkerPool {
	t.Helper()
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable: ", err)
	}
	pool, err := NewPythonWorkerPool(context.Background(), PythonPoolOptions{
		Size: size,
		Worker: PythonWorkerOptions{
			PythonPath: python,
			ScriptPath: filepath.Join("testdata", "stdio_worker.py"),
			MaxTasks:   maxTasks,
			ExtraEnv:   extraEnv,
		},
	})
	if err != nil {
		t.Fatalf("start pool: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("close pool: %v", err)
		}
	})
	return pool
}

func TestPythonWorkerPoolUsesFixedCapacityAndWaitsForReturn(t *testing.T) {
	stateDir := t.TempDir()
	release := filepath.Join(stateDir, "release")
	pool := newTestPythonPool(t, 2, 10,
		"QAVOR_TEST_STATE_DIR="+stateDir,
		"QAVOR_TEST_RELEASE_FILE="+release,
	)

	var started sync.WaitGroup
	started.Add(2)
	firstDone := make(chan error, 2)
	for range 2 {
		go func() {
			started.Done()
			_, err := pool.Parse(context.Background(), ParseInput{Filename: "wait.pdf", Content: []byte("wait")})
			firstDone <- err
		}()
	}
	started.Wait()
	waitForPoolTestFiles(t, stateDir, "wait-", 2)

	thirdDone := make(chan error, 1)
	go func() {
		_, err := pool.Parse(context.Background(), ParseInput{Filename: "third.pdf", Content: []byte("third")})
		thirdDone <- err
	}()
	select {
	case err := <-thirdDone:
		t.Fatalf("third request completed before a worker returned: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatalf("release workers: %v", err)
	}
	for range 2 {
		if err := <-firstDone; err != nil {
			t.Fatalf("blocked parse: %v", err)
		}
	}
	if err := <-thirdDone; err != nil {
		t.Fatalf("third parse after worker return: %v", err)
	}

	health := pool.Health()
	if health.ConfiguredWorkers != 2 || health.AvailableWorkers != 2 {
		t.Fatalf("health = %+v, want two registered workers", health)
	}
}

func TestPythonWorkerPoolReusesWorkerAcrossSequentialRequests(t *testing.T) {
	pool := newTestPythonPool(t, 1, 10)
	first, err := pool.Parse(context.Background(), ParseInput{Filename: "one.pdf", Content: []byte("one")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := pool.Parse(context.Background(), ParseInput{Filename: "two.pdf", Content: []byte("two")})
	if err != nil {
		t.Fatal(err)
	}
	if pidFromMarkdown(first.Markdown) != pidFromMarkdown(second.Markdown) {
		t.Fatalf("pool failed to reuse its worker: %q / %q", first.Markdown, second.Markdown)
	}
}

func TestPythonWorkerPoolRotatesIdleWorkerAfterMaxTasks(t *testing.T) {
	pool := newTestPythonPool(t, 1, 1)
	first, err := pool.Parse(context.Background(), ParseInput{Filename: "one.pdf", Content: []byte("one")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := pool.Parse(context.Background(), ParseInput{Filename: "two.pdf", Content: []byte("two")})
	if err != nil {
		t.Fatal(err)
	}
	if pidFromMarkdown(first.Markdown) == pidFromMarkdown(second.Markdown) {
		t.Fatalf("worker was not rotated after MaxTasks: %q / %q", first.Markdown, second.Markdown)
	}
}

func TestPythonWorkerPoolReturnsWhenMaxTaskReplacementCannotStart(t *testing.T) {
	pool := newTestPythonPool(t, 1, 1)
	pool.opts.Worker.ScriptPath = filepath.Join(t.TempDir(), "missing-worker.py")
	if _, err := pool.Parse(context.Background(), ParseInput{Filename: "one.pdf", Content: []byte("one")}); err != nil {
		t.Fatalf("completed request before rotation: %v", err)
	}

	finished := make(chan error, 1)
	go func() {
		_, err := pool.Parse(context.Background(), ParseInput{Filename: "two.pdf", Content: []byte("two")})
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, ErrPythonWorkerCrashed) {
			t.Fatalf("error = %v, want ErrPythonWorkerCrashed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Parse waited for a failed MaxTasks replacement worker")
	}
}

func TestPythonWorkerPoolRetriesOneCrashWithReplacement(t *testing.T) {
	stateDir := t.TempDir()
	pool := newTestPythonPool(t, 1, 10, "QAVOR_TEST_STATE_DIR="+stateDir)
	warm, err := pool.Parse(context.Background(), ParseInput{Filename: "warm.pdf", Content: []byte("warm")})
	if err != nil {
		t.Fatal(err)
	}
	result, err := pool.Parse(context.Background(), ParseInput{Filename: "crash-once.pdf", Content: []byte("crash")})
	if err != nil {
		t.Fatalf("retry after crash: %v", err)
	}
	if pidFromMarkdown(warm.Markdown) == pidFromMarkdown(result.Markdown) {
		t.Fatalf("crashed worker was reused: %q / %q", warm.Markdown, result.Markdown)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "crash-once.pdf")); err != nil {
		t.Fatalf("crash-once state marker: %v", err)
	}
}

func TestPythonWorkerPoolRetriesOneProtocolFailureWithReplacement(t *testing.T) {
	stateDir := t.TempDir()
	pool := newTestPythonPool(t, 1, 10, "QAVOR_TEST_STATE_DIR="+stateDir)
	warm, err := pool.Parse(context.Background(), ParseInput{Filename: "warm.pdf", Content: []byte("warm")})
	if err != nil {
		t.Fatal(err)
	}
	result, err := pool.Parse(context.Background(), ParseInput{Filename: "bad-json-once.pdf", Content: []byte("bad")})
	if err != nil {
		t.Fatalf("retry after protocol failure: %v", err)
	}
	if pidFromMarkdown(warm.Markdown) == pidFromMarkdown(result.Markdown) {
		t.Fatalf("protocol-broken worker was reused: %q / %q", warm.Markdown, result.Markdown)
	}
}

func TestPythonWorkerPoolStopsAfterOneInfrastructureRetry(t *testing.T) {
	stateDir := t.TempDir()
	pool := newTestPythonPool(t, 1, 10, "QAVOR_TEST_STATE_DIR="+stateDir)
	finished := make(chan error, 1)
	go func() {
		_, err := pool.Parse(context.Background(), ParseInput{Filename: "always-crash.pdf", Content: []byte("crash")})
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, ErrPythonWorkerCrashed) {
			t.Fatalf("error = %v, want ErrPythonWorkerCrashed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pool retried an always-crashing worker indefinitely")
	}
	attempts, err := os.ReadFile(filepath.Join(stateDir, "always-crash.pdf"))
	if err != nil {
		t.Fatalf("read attempt count: %v", err)
	}
	if string(attempts) != "2" {
		t.Fatalf("attempts = %q, want exactly 2", attempts)
	}
}

func TestPythonWorkerPoolReturnsWhenReplacementCannotStart(t *testing.T) {
	pool := newTestPythonPool(t, 1, 10)
	pool.opts.Worker.ScriptPath = filepath.Join(t.TempDir(), "missing-worker.py")

	finished := make(chan error, 1)
	go func() {
		_, err := pool.Parse(context.Background(), ParseInput{Filename: "crash.pdf", Content: []byte("crash")})
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, ErrPythonWorkerCrashed) {
			t.Fatalf("error = %v, want ErrPythonWorkerCrashed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Parse waited for an unavailable replacement worker")
	}
}

func TestPythonWorkerPoolDoesNotRetryParserErrors(t *testing.T) {
	pool := newTestPythonPool(t, 1, 10)
	warm, err := pool.Parse(context.Background(), ParseInput{Filename: "warm.pdf", Content: []byte("warm")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Parse(context.Background(), ParseInput{Filename: "parser-error.pdf", Content: []byte("bad input")})
	var parserErr *ParserError
	if !errors.As(err, &parserErr) {
		t.Fatalf("error = %v, want ParserError", err)
	}
	after, err := pool.Parse(context.Background(), ParseInput{Filename: "after.pdf", Content: []byte("after")})
	if err != nil {
		t.Fatal(err)
	}
	if pidFromMarkdown(warm.Markdown) != pidFromMarkdown(after.Markdown) {
		t.Fatal("business parse error replaced a healthy worker")
	}
}

func TestPythonWorkerPoolHealthReturnsIndependentCapabilitySnapshot(t *testing.T) {
	pool := newTestPythonPool(t, 1, 10)
	first := pool.Health()
	if first.ConfiguredWorkers != 1 || first.AvailableWorkers != 1 {
		t.Fatalf("health = %+v", first)
	}
	first.Capabilities["docling"] = true
	second := pool.Health()
	if second.Capabilities["docling"] {
		t.Fatal("Health exposed mutable pool capability state")
	}
}

func TestPythonWorkerPoolCloseRejectsNewRequests(t *testing.T) {
	pool := newTestPythonPool(t, 1, 10)
	if err := pool.Close(); err != nil {
		t.Fatalf("close pool: %v", err)
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("second close pool: %v", err)
	}
	_, err := pool.Parse(context.Background(), ParseInput{Filename: "after-close.pdf", Content: []byte("closed")})
	if !errors.Is(err, ErrPythonWorkerCrashed) {
		t.Fatalf("error = %v, want ErrPythonWorkerCrashed", err)
	}
}

func TestPythonWorkerPoolCloseWakesWaitingBorrower(t *testing.T) {
	stateDir := t.TempDir()
	release := filepath.Join(stateDir, "release")
	pool := newTestPythonPool(t, 1, 10,
		"QAVOR_TEST_STATE_DIR="+stateDir,
		"QAVOR_TEST_RELEASE_FILE="+release,
	)
	firstDone := make(chan error, 1)
	go func() {
		_, err := pool.Parse(context.Background(), ParseInput{Filename: "wait.pdf", Content: []byte("wait")})
		firstDone <- err
	}()
	waitForPoolTestFiles(t, stateDir, "wait-", 1)

	secondDone := make(chan error, 1)
	go func() {
		_, err := pool.Parse(context.Background(), ParseInput{Filename: "second.pdf", Content: []byte("second")})
		secondDone <- err
	}()
	select {
	case err := <-secondDone:
		t.Fatalf("second Parse completed before Close: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := pool.Close(); err != nil {
		t.Fatalf("close pool: %v", err)
	}
	select {
	case err := <-secondDone:
		if !errors.Is(err, ErrPythonWorkerCrashed) {
			t.Fatalf("waiting Parse error = %v, want ErrPythonWorkerCrashed", err)
		}
	case <-time.After(2 * time.Second):
		_ = os.WriteFile(release, []byte("release"), 0o600)
		<-firstDone
		t.Fatal("Close did not wake the waiting borrower")
	}
	if err := os.WriteFile(release, []byte("release"), 0o600); err != nil {
		t.Fatalf("release active parse: %v", err)
	}
	if err := <-firstDone; err != nil {
		t.Fatalf("active Parse after Close: %v", err)
	}
}

func TestNewPythonWorkerPoolRejectsInvalidCapacity(t *testing.T) {
	if _, err := NewPythonWorkerPool(context.Background(), PythonPoolOptions{Size: 0, Worker: PythonWorkerOptions{MaxTasks: 1}}); err == nil {
		t.Fatal("pool accepted zero size")
	}
	if _, err := NewPythonWorkerPool(context.Background(), PythonPoolOptions{Size: 1, Worker: PythonWorkerOptions{MaxTasks: 0}}); err == nil {
		t.Fatal("pool accepted zero MaxTasks")
	}
}

func waitForPoolTestFiles(t *testing.T, directory, prefix string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("read worker state directory: %v", err)
		}
		count := 0
		for _, entry := range entries {
			if len(entry.Name()) >= len(prefix) && entry.Name()[:len(prefix)] == prefix {
				count++
			}
		}
		if count >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("only %d workers reached the blocking parse", want)
}
