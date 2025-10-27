package concslice

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

type Task[State any] struct {
	ctx            context.Context
	cancel         func()
	processedCount int64
	itemsCount     int
	maxWorkers     int
	onFinish       func(Result[State])
	done           chan struct{}
	mu             sync.Mutex
	result         Result[State]

	State State
}

type Result[State any] struct {
	State          State
	ProcessedCount int
	IsCanceled     bool
	IsFinished     bool
	Errors         []error
}

func (t *Task[State]) Context() context.Context {
	return t.ctx
}

func (t *Task[State]) Cancel() {
	t.cancel()
}

func (t *Task[State]) Wait() Result[State] {
	t.mu.Lock()
	done := t.done
	t.mu.Unlock()
	if done == nil {
		return t.result
	}
	<-done
	return t.result
}

func (t *Task[State]) MaxWorkers() int {
	return t.maxWorkers
}

func (t *Task[State]) ProcessedCount() int {
	return int(atomic.LoadInt64(&t.processedCount))
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
	t.done = make(chan struct{})
}

func (t *Task[State]) finish() {
	t.mu.Lock()
	defer func() {
		close(t.done)
		t.done = nil
		t.mu.Unlock()
	}()
	t.result.State = t.State
	t.result.ProcessedCount = int(t.processedCount)
	t.result.IsCanceled = t.ctx.Err() != nil
	t.result.IsFinished = true
	t.cancel()
	if t.onFinish != nil {
		defer func() {
			if err := recover(); err != nil {
				t.result.Errors = append(t.result.Errors, fmt.Errorf("panic in onFinish callback: %v", err))
			}
		}()
		t.onFinish(t.result)
	}
}

func (t *Task[State]) collectPanicError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.result.Errors = append(t.result.Errors, err)
}

func (t *Task[State]) incrementCounter() {
	atomic.AddInt64(&t.processedCount, 1)
}
