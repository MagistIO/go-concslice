package concslice

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// Task represents a concurrent processing task with state management.
// It provides methods for cancellation, progress tracking, and result retrieval.
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

// Result contains the final state and metadata of a completed task.
type Result[State any] struct {
	State          State   // Final state after processing
	ProcessedCount int     // Number of successfully processed items
	IsCanceled     bool    // Whether the task was canceled
	IsCompleted    bool    // Whether the task completed successfully
	IsFinished     bool    // Whether the task completed normally
	Errors         []error // Any errors that occurred during processing
}

// Context returns the context associated with this task.
// This context can be used to check for cancellation and timeouts.
func (t *Task[State]) Context() context.Context {
	return t.ctx
}

// Cancel cancels the task processing.
// This will cause all workers to stop processing and the task to complete.
func (t *Task[State]) Cancel() {
	t.cancel()
}

// WithLock locks the mutex and executes the given function.
// It is safe to call this method from multiple goroutines.
func (t *Task[State]) WithLock(fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn()
}

// Wait blocks until the task is completed and returns the final result.
// If the task is already completed, it returns the cached result immediately.
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

// MaxWorkers returns the maximum number of workers configured for this task.
func (t *Task[State]) MaxWorkers() int {
	return t.maxWorkers
}

// ProcessedCount returns the current number of processed items.
// This value is updated atomically and can be called concurrently.
func (t *Task[State]) ProcessedCount() int {
	return int(atomic.LoadInt64(&t.processedCount))
}

// initialize sets up the task with the provided context and options.
// It configures the number of workers, creates the context, and initializes the done channel.
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

// finish completes the task and prepares the final result.
// It sets the final state, processes the onFinish callback, and closes the done channel.
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
	t.result.IsCompleted = t.itemsCount == int(t.processedCount)
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

// collectPanicError safely adds a panic error to the result's error list.
// This method is thread-safe and can be called from multiple goroutines.
func (t *Task[State]) collectPanicError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.result.Errors = append(t.result.Errors, err)
}

// incrementCounter atomically increments the processed count.
// This method is thread-safe and can be called from multiple goroutines.
func (t *Task[State]) incrementCounter() {
	atomic.AddInt64(&t.processedCount, 1)
}
