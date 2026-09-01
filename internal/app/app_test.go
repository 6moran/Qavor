package app

import (
	"testing"
	"time"
)

func TestShutdownDocumentWorkersWaitsForConsumersBeforeClosingParserPool(t *testing.T) {
	workerDone := make(chan struct{})
	poolClosed := make(chan struct{})
	application := &App{
		workerStop: func() {},
		workerDone: workerDone,
		closeParserPool: func() error {
			close(poolClosed)
			return nil
		},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		application.shutdownDocumentWorkers(time.Millisecond)
	}()

	select {
	case <-poolClosed:
		t.Fatal("parser pool closed while a document worker was still running")
	case <-time.After(25 * time.Millisecond):
	}

	close(workerDone)
	select {
	case <-poolClosed:
	case <-time.After(time.Second):
		t.Fatal("parser pool was not closed after document workers exited")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("document worker shutdown did not finish")
	}
}

func TestShutdownDocumentWorkersKeepsParserPoolOpenAfterTimeout(t *testing.T) {
	workerDone := make(chan struct{})
	poolClosed := make(chan struct{})
	application := &App{
		workerStop: func() {},
		workerDone: workerDone,
		closeParserPool: func() error {
			close(poolClosed)
			return nil
		},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		application.shutdownDocumentWorkers(time.Millisecond)
	}()

	select {
	case <-poolClosed:
		t.Fatal("parser pool closed after worker shutdown timeout")
	case <-time.After(25 * time.Millisecond):
	}
	select {
	case <-done:
		t.Fatal("document worker shutdown returned before the nonresponsive worker exited")
	default:
	}

	close(workerDone)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("document worker shutdown did not finish after worker exit")
	}
}
