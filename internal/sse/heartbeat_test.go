package sse

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestHeartbeatManager_StartAndStop(t *testing.T) {
	c, _ := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	hm := NewHeartbeatManager(50*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := hm.Start(ctx, writer, "test_msg")

	// Wait long enough for at least one heartbeat tick
	time.Sleep(120 * time.Millisecond)

	// Stop the heartbeat
	hm.Stop(stop)

	// Give the goroutine time to exit
	time.Sleep(20 * time.Millisecond)
}

func TestHeartbeatManager_ContextCancel(t *testing.T) {
	c, _ := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	hm := NewHeartbeatManager(50*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())

	stop := hm.Start(ctx, writer, "test_msg")

	// Cancel context - heartbeat goroutine should exit
	cancel()

	// Stop should be a no-op since context is already cancelled
	hm.Stop(stop)

	// Give the goroutine time to exit
	time.Sleep(20 * time.Millisecond)
}

func TestHeartbeatManager_StopIdempotent(t *testing.T) {
	c, _ := newTestContext(t)
	logger := zap.NewNop()
	writer := NewSSEWriter(c, logger)
	defer writer.Close()

	hm := NewHeartbeatManager(50*time.Millisecond, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := hm.Start(ctx, writer, "test_msg")

	// Stop multiple times should not panic
	hm.Stop(stop)
	hm.Stop(stop)
	hm.Stop(stop)

	time.Sleep(20 * time.Millisecond)
}
