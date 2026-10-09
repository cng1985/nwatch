package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/fx"
)

type Pool struct {
	tasks    chan uint
	running  sync.Map
	wg       sync.WaitGroup
	proc     *Processor
	workers  int
	once     sync.Once
	inflight atomic.Int32
}

func NewPool(workers, queueSize int, proc *Processor, lc fx.Lifecycle) *Pool {
	if workers <= 0 {
		workers = 20
	}
	if queueSize <= 0 {
		queueSize = 200
	}
	p := &Pool{
		tasks:   make(chan uint, queueSize),
		proc:    proc,
		workers: workers,
	}
	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				p.Start()
				return nil
			},
			OnStop: func(ctx context.Context) error {
				p.Stop(ctx)
				return nil
			},
		})
	}
	return p
}

func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.loop()
	}
	slog.Info("工作池已启动", "workers", p.workers, "queue", cap(p.tasks))
}

func (p *Pool) Stop(ctx context.Context) {
	p.once.Do(func() { close(p.tasks) })
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		slog.Warn("工作池停止超时", "inflight", p.inflight.Load())
	}
}

func (p *Pool) Workers() int { return p.workers }

func (p *Pool) QueueLen() int { return len(p.tasks) }

func (p *Pool) TryAcquire(id uint) bool {
	_, loaded := p.running.LoadOrStore(id, time.Now())
	return !loaded
}

func (p *Pool) Release(id uint) { p.running.Delete(id) }

func (p *Pool) Enqueue(id uint) bool {
	select {
	case p.tasks <- id:
		return true
	default:
		return false
	}
}

func (p *Pool) loop() {
	defer p.wg.Done()
	for id := range p.tasks {
		p.run(id)
	}
}

func (p *Pool) run(id uint) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("监控任务异常", "monitor", id, "panic", r)
		}
		p.inflight.Add(-1)
		p.Release(id)
	}()
	p.inflight.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := p.proc.Process(ctx, id, false); err != nil {
		slog.Error("监控任务失败", "monitor", id, "err", err.Error())
	}
}
