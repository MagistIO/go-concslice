package concslice

import (
	"context"
	"runtime"
	"sync"
)

type Task[State any] struct {
	ctx            context.Context
	cancel         func()
	wg             *sync.WaitGroup
	processedCount int
	itemsCount     int
	maxWorkers     int
	onFinish       func(State)
	done           chan struct{}
	mu             sync.Mutex

	State State
}

type Result[State any] struct {
	State          State
	ProcessedCount int
	IsCanceled     bool
}

func (t *Task[State]) Context() context.Context {
	return t.ctx
}

func (t *Task[State]) Cancel() {
	t.cancel()
}

func (t *Task[State]) Wait() *Result[State] {
	if t.wg == nil {
		isCanceled := false
		if t.ctx != nil && t.ctx.Err() != nil {
			isCanceled = true
		}
		return &Result[State]{
			State:          t.State,
			ProcessedCount: t.processedCount,
			IsCanceled:     isCanceled,
		}
	}
	t.wg.Wait()
	t.mu.Lock()
	done := t.done
	defer t.mu.Unlock()
	if done != nil {
		<-done
	}
	return &Result[State]{
		State:          t.State,
		ProcessedCount: t.processedCount,
		IsCanceled:     t.ctx.Err() != nil,
	}
}

func (t *Task[State]) MaxWorkers() int {
	return t.maxWorkers
}

func (t *Task[State]) initialize(ctx context.Context, options []Option[State]) {
	for _, option := range options {
		option(t)
	}
	if t.maxWorkers == 0 {
		t.maxWorkers = runtime.GOMAXPROCS(0)
	}
	t.maxWorkers = min(t.itemsCount, t.maxWorkers)
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.wg = &sync.WaitGroup{}
	t.wg.Add(t.maxWorkers)
	t.done = make(chan struct{})
}

func (t *Task[State]) wait() {
	t.wg.Wait()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done != nil {
		defer func() {
			close(t.done)
			t.done = nil
		}()
	}
	t.cancel()
	t.finish()
	t.wg = nil
	t.cancel = nil
}

func (t *Task[State]) finish() {
	if t.onFinish != nil {
		defer func() {
			if err := recover(); err != nil {
				// TODO: collect panic error
			}
		}()
		t.onFinish(t.State)
		t.onFinish = nil
	}
}

func (t *Task[State]) incrementCounter() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.processedCount++
}
