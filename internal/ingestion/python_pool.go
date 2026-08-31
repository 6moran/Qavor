package ingestion

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// PythonPoolOptions configures a fixed-size set of long-lived parser workers.
type PythonPoolOptions struct {
	Worker PythonWorkerOptions
	Size   int
}

// ParserHealth is a point-in-time view of the running parser pool.
type ParserHealth struct {
	ConfiguredWorkers int
	AvailableWorkers  int
	Capabilities      map[string]bool
}

// PythonWorkerPool lends one worker for each Parse call. The idle channel
// supplies bounded backpressure while the registry lets Health and Close take
// a coherent snapshot without exposing mutable worker state.
type PythonWorkerPool struct {
	ctx  context.Context
	opts PythonPoolOptions
	idle chan *PythonWorker
	done chan struct{}

	mu      sync.Mutex
	workers map[*PythonWorker]struct{}
	leased  map[*PythonWorker]struct{}
	closed  bool
}

// NewPythonWorkerPool starts the configured workers. A partial startup keeps
// the usable workers alive so callers can report a degraded, rather than
// unavailable, parser service.
func NewPythonWorkerPool(ctx context.Context, opts PythonPoolOptions) (*PythonWorkerPool, error) {
	if opts.Size <= 0 {
		return nil, fmt.Errorf("python worker pool size must be positive")
	}
	if opts.Worker.MaxTasks <= 0 {
		return nil, fmt.Errorf("python worker max tasks must be positive")
	}

	p := &PythonWorkerPool{
		ctx:     ctx,
		opts:    opts,
		idle:    make(chan *PythonWorker, opts.Size),
		done:    make(chan struct{}),
		workers: make(map[*PythonWorker]struct{}, opts.Size),
		leased:  make(map[*PythonWorker]struct{}, opts.Size),
	}
	var lastErr error
	for range opts.Size {
		if err := p.startAndRegister(); err != nil {
			lastErr = err
		}
	}
	if p.workerCount() > 0 {
		return p, nil
	}
	if lastErr == nil {
		lastErr = ErrPythonWorkerCrashed
	}
	return nil, fmt.Errorf("start python worker pool: %w", lastErr)
}

// Parse retries the current request exactly once, and only after infrastructure
// failures that make the borrowed worker untrustworthy.
func (p *PythonWorkerPool) Parse(ctx context.Context, input ParseInput) (ParseResult, error) {
	var result ParseResult
	var parseErr error
	for attempt := 0; attempt < 2; attempt++ {
		worker, err := p.borrow(ctx)
		if err != nil {
			return ParseResult{}, err
		}
		result, parseErr = worker.Parse(ctx, input)
		if !isPythonWorkerInfrastructureError(parseErr) {
			p.returnWorker(worker)
			return result, parseErr
		}

		if err := p.retireWorker(worker); err != nil {
			return result, fmt.Errorf("%w: replacement worker start failed: %v", parseErr, err)
		}
		if attempt == 1 {
			return result, parseErr
		}
	}
	return result, parseErr
}

// Health copies all mutable data so callers cannot change the pool state.
func (p *PythonWorkerPool) Health() ParserHealth {
	p.mu.Lock()
	defer p.mu.Unlock()
	health := ParserHealth{
		ConfiguredWorkers: p.opts.Size,
		AvailableWorkers:  len(p.workers),
		Capabilities: map[string]bool{
			"docling":  false,
			"rapidocr": false,
			"api_ocr":  false,
		},
	}
	for worker := range p.workers {
		for capability, available := range worker.capabilitySnapshot() {
			health.Capabilities[capability] = health.Capabilities[capability] || available
		}
	}
	return health
}

// Close rejects new borrowers and shuts down every idle worker. A worker that
// is currently leased completes its release path and is then closed there.
func (p *PythonWorkerPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.done)
	workers := p.drainIdleLocked()
	p.mu.Unlock()

	var closeErr error
	for _, worker := range workers {
		if err := worker.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (p *PythonWorkerPool) borrow(ctx context.Context) (*PythonWorker, error) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, ErrPythonWorkerCrashed
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.done:
		return nil, ErrPythonWorkerCrashed
	case worker := <-p.idle:
		p.mu.Lock()
		if p.closed {
			delete(p.workers, worker)
			p.mu.Unlock()
			_ = worker.Close()
			return nil, ErrPythonWorkerCrashed
		}
		if _, registered := p.workers[worker]; !registered {
			p.mu.Unlock()
			return nil, ErrPythonWorkerCrashed
		}
		p.leased[worker] = struct{}{}
		p.mu.Unlock()
		return worker, nil
	}
}

func (p *PythonWorkerPool) returnWorker(worker *PythonWorker) {
	p.mu.Lock()
	delete(p.leased, worker)
	_, registered := p.workers[worker]
	retire := registered && (p.closed || worker.isClosed() || worker.reachedMaxTasks())
	if retire {
		delete(p.workers, worker)
	} else if registered {
		p.idle <- worker
	}
	closed := p.closed
	p.mu.Unlock()

	if !retire {
		return
	}
	_ = worker.Close()
	if !closed {
		_ = p.startAndRegister()
	}
}

func (p *PythonWorkerPool) retireWorker(worker *PythonWorker) error {
	p.mu.Lock()
	delete(p.leased, worker)
	_, registered := p.workers[worker]
	if registered {
		delete(p.workers, worker)
	}
	closed := p.closed
	p.mu.Unlock()

	if !registered {
		return nil
	}
	_ = worker.Close()
	if !closed {
		return p.startAndRegister()
	}
	return nil
}

func (p *PythonWorkerPool) startAndRegister() error {
	worker, err := startPythonWorker(p.ctx, p.opts.Worker)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		_ = worker.Close()
		return ErrPythonWorkerCrashed
	}
	p.workers[worker] = struct{}{}
	p.idle <- worker
	p.mu.Unlock()
	return nil
}

func (p *PythonWorkerPool) workerCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.workers)
}

func (p *PythonWorkerPool) drainIdleLocked() []*PythonWorker {
	workers := make([]*PythonWorker, 0, len(p.idle))
	for {
		select {
		case worker := <-p.idle:
			delete(p.workers, worker)
			workers = append(workers, worker)
		default:
			return workers
		}
	}
}

func isPythonWorkerInfrastructureError(err error) bool {
	return errors.Is(err, ErrPythonWorkerCrashed) || errors.Is(err, ErrPythonProtocol)
}
