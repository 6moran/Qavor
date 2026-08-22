package trace

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"Qavor/internal/model/entity"
	"Qavor/pkg/logger"

	"go.uber.org/zap"
)

// WriterConfig Writer 异步写入队列配置
// 同步写会拖慢agent业务
type WriterConfig struct {
	BufferSize int // channel 缓冲区大小（默认 2048），队列满时 Start 事件丢弃，End 事件等待 100ms
}

// writeEventKind 事件类型
type writeEventKind int

const (
	kindCreateTrace writeEventKind = iota // 创建整条Trace总记录
	kindUpdateTrace                       // 补充trace的业务元数据
	kindStartSpan                         // 创建一个span
	kindEndSpan                           // 结束一个span
)

// writeEvent 写入队列中的单个事件（值拷贝，避免外部指针被修改）
// 所有事件使用同一个 FIFO channel，保证同一 Span 的 Start 排在 End 前（顺序一致性）
type writeEvent struct {
	kind    writeEventKind     // 事件类型（CreateTrace / UpdateTrace / StartSpan / EndSpan）
	record  entity.TraceRecord // kindCreateTrace：TraceRecord 实体
	traceID string             // kindUpdateTrace：Trace ID
	meta    TraceMetadata      // kindUpdateTrace：补全的元数据
	span    entity.TraceSpan   // kindStartSpan：Span 实体
	spanID  string             // kindEndSpan：Span ID
	end     SpanEnd            // kindEndSpan：结束数据
}

// Writer 有界异步写入队列，实现 SpanWriter 接口
// 核心设计：
//   - 所有事件使用同一个 FIFO channel，保证同一 Span 的 Start 排在 End 前（顺序一致性）
//   - 单消费者 goroutine 顺序处理，避免并发写 DB 的锁竞争
//   - 背压策略：Start 事件队列满时直接丢弃（+1 dropped），End 事件最多等待 100ms
//   - 优雅关闭：Close() 置标志 → drain 剩余事件 → 关闭 done channel
type Writer struct {
	repo    TraceRepository    // 数据访问接口（PostgreSQL）
	events  chan writeEvent    // 有界缓冲 channel（默认 2048）
	dropped atomic.Uint64      // 丢弃计数（监控指标，暴露给 /metrics）
	closed  atomic.Bool        // 关闭标志（Close 时置 true，防止新事件入队）
	stopCh  chan struct{}      // 停止信号（Close 时关闭）
	done    chan struct{}      // 完成信号（goroutine 退出时关闭）
	flushCh chan chan struct{} // flush 请求 channel（阻塞等待队列清空）
	once    sync.Once          // 保证 Close 只执行一次
}

// NewWriter 创建异步写入器并启动后台 goroutine
func NewWriter(repo TraceRepository, cfg WriterConfig) *Writer {
	size := cfg.BufferSize
	if size <= 0 {
		size = 2048
	}
	w := &Writer{
		repo:    repo,
		events:  make(chan writeEvent, size),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
		flushCh: make(chan chan struct{}, 16),
	}
	go w.run()
	return w
}

// run 后台 goroutine：从 channel 读取事件并写入 repo
func (w *Writer) run() {
	defer close(w.done)
	for {
		// 单次事件处理用匿名函数包裹以捕获 panic 并继续循环；
		// stopCh 分支返回 true 时由外层退出 run()，否则 Close 永远等不到 done。
		stopped := func() bool {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("trace writer: process panic recovered",
						zap.Any("recover", r))
				}
			}()
			select {
			case <-w.stopCh:
				w.drain()             // 处理队列中已经存在的所有事件
				w.ackPendingFlushes() // 关闭所有等待中的flush，避免调用方一直等待
				return true
			case event := <-w.events:
				w.process(event) // 根据不同的时间类型调用
			case ack := <-w.flushCh:
				w.drain()
				close(ack)
			}
			return false
		}()
		if stopped {
			return
		}
	}
}

// drain 排空 channel 中所有待处理事件
func (w *Writer) drain() {
	for {
		select {
		case event := <-w.events:
			w.process(event)
		default:
			return
		}
	}
}

// ackPendingFlushes 排空所有待响应的 flush 请求
func (w *Writer) ackPendingFlushes() {
	for {
		select {
		case ack := <-w.flushCh:
			close(ack)
		default:
			return
		}
	}
}

// process 将单个事件写入 repo（错误仅记录日志，不返回调用方）
func (w *Writer) process(event writeEvent) {
	ctx := context.Background()
	switch event.kind {
	case kindCreateTrace:
		if err := w.repo.CreateTrace(ctx, &event.record); err != nil {
			logger.Warn("trace writer: CreateTrace 失败", zap.Error(err))
		}
	case kindUpdateTrace:
		if err := w.repo.UpdateTraceMetadata(ctx, event.traceID, event.meta); err != nil {
			logger.Warn("trace writer: UpdateTraceMetadata 失败", zap.Error(err))
		}
	case kindStartSpan:
		if err := w.repo.StartSpan(ctx, &event.span); err != nil {
			logger.Warn("trace writer: StartSpan 失败", zap.Error(err))
		}
	case kindEndSpan:
		if err := w.repo.EndSpan(ctx, event.spanID, event.end); err != nil {
			logger.Warn("trace writer: EndSpan 失败", zap.Error(err))
		}
	}
}

// UpdateTraceMetadata 入队 TraceRecord 元数据补全事件（队列满时丢弃）
func (w *Writer) UpdateTraceMetadata(_ context.Context, traceID string, meta TraceMetadata) error {
	if w.closed.Load() {
		return nil
	}
	select {
	case w.events <- writeEvent{kind: kindUpdateTrace, traceID: traceID, meta: meta}:
	default:
		w.dropped.Add(1)
	}
	return nil
}

// CreateTrace 入队 TraceRecord 创建事件（队列满时丢弃）
func (w *Writer) CreateTrace(_ context.Context, record *entity.TraceRecord) error {
	if w.closed.Load() {
		return nil
	}
	select {
	case w.events <- writeEvent{kind: kindCreateTrace, record: *record}:
	default:
		w.dropped.Add(1)
	}
	return nil
}

// StartSpan 入队 Span 创建事件（队列满时丢弃）
func (w *Writer) StartSpan(_ context.Context, span *entity.TraceSpan) error {
	if w.closed.Load() {
		return nil
	}
	select {
	case w.events <- writeEvent{kind: kindStartSpan, span: *span}:
	default:
		w.dropped.Add(1)
	}
	return nil
}

// EndSpan 入队 Span 结束事件（最多等待 100ms，超时丢弃）
func (w *Writer) EndSpan(_ context.Context, spanID string, end SpanEnd) error {
	if w.closed.Load() {
		return nil
	}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case w.events <- writeEvent{kind: kindEndSpan, spanID: spanID, end: end}:
	case <-timer.C:
		w.dropped.Add(1)
	case <-w.stopCh:
		// Writer 正在关闭，丢弃
		w.dropped.Add(1)
	}
	return nil
}

// Flush 等待所有已入队事件写入完成
func (w *Writer) Flush(ctx context.Context) error {
	if w.closed.Load() {
		return nil
	}
	ack := make(chan struct{})
	select {
	case w.flushCh <- ack:
		select {
		case <-ack:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-w.done:
			return nil
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-w.done:
		return nil
	}
}

// Close 停止后台 goroutine，排空剩余事件后返回
func (w *Writer) Close(ctx context.Context) error {
	w.once.Do(func() {
		w.closed.Store(true)
		close(w.stopCh)
	})
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dropped 返回因队列满被丢弃的事件数
func (w *Writer) Dropped() uint64 {
	return w.dropped.Load()
}
