package ingestion

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

func startTestPythonWorker(t *testing.T) *PythonWorker {
	t.Helper()
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python is unavailable: ", err)
	}
	worker, err := startPythonWorker(context.Background(), PythonWorkerOptions{
		PythonPath: python,
		ScriptPath: filepath.Join("testdata", "stdio_worker.py"),
	})
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

func pidFromMarkdown(markdown string) string {
	match := regexp.MustCompile(`pid=(\d+)`).FindStringSubmatch(markdown)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}
