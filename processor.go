package concslice

import (
	"context"
	"fmt"
	"sync"
)

type (
	// Processor provides a reusable way to process slices with pre-configured settings.
	// It encapsulates a handler function and options for consistent processing.
	Processor[Item, State any] struct {
		handler func(p *Task[State], i Item)
		options []Option[State]
	}

	// Option is a function that configures a Task with specific settings.
	Option[State any] func(*Task[State])
)

// NewProcessor creates a new Processor with the given handler and options.
// The processor can be reused multiple times with different slices.
func NewProcessor[Item, State any](handler func(p *Task[State], i Item), options ...Option[State]) *Processor[Item, State] {
	return &Processor[Item, State]{
		handler: handler,
		options: options,
	}
}

// Process starts processing the given items using the processor's handler and options.
// Returns a Task that can be used to monitor progress and get results.
func (p *Processor[Item, State]) Process(ctx context.Context, items []Item) *Task[State] {
	return Process(ctx, items, p.handler, p.options...)
}

// Process concurrently processes a slice of items using the provided handler function.
// It creates multiple worker goroutines and distributes items among them for parallel processing.
// Returns a Task that can be used to monitor progress, cancel processing, and get results.
func Process[Item any, State any](ctx context.Context, items []Item, handler func(p *Task[State], i Item), options ...Option[State]) *Task[State] {
	t := &Task[State]{
		itemsCount: len(items),
	}
	if t.itemsCount == 0 {
		return t
	}
	t.initialize(ctx, options)
	wg := sync.WaitGroup{}
	wg.Add(t.maxWorkers)
	for i := range t.maxWorkers {
		go func(index int) {
			defer func() {
				if err := recover(); err != nil {
					t.collectPanicError(fmt.Errorf("panic in worker %d: %v", index, err))
				}
				wg.Done()
			}()
			for i := index; i < t.itemsCount; i += t.maxWorkers {
				if err := t.ctx.Err(); err != nil {
					break
				}
				handler(t, items[i])
				t.incrementCounter()
			}
		}(i)
	}
	go func() {
		wg.Wait()
		t.finish()
	}()
	return t
}

// WithMaxWorkers sets the maximum number of concurrent workers for processing.
// If not specified, the number of workers defaults to runtime.GOMAXPROCS(0).
// The actual number of workers will be min(itemsCount, concurrency).
func WithMaxWorkers[State any](concurrency int) Option[State] {
	return func(p *Task[State]) {
		p.maxWorkers = concurrency
	}
}

// WithOnFinish sets a callback function that will be called when processing is complete.
// The callback receives the final Result and can be used for cleanup or logging.
// If the callback panics, the panic will be caught and added to the result's errors.
func WithOnFinish[State any](onFinish func(Result[State])) Option[State] {
	return func(p *Task[State]) {
		p.onFinish = onFinish
	}
}

// WithState sets the initial state for the task.
// This state will be available to all worker goroutines and can be modified concurrently.
func WithState[State any](state State) Option[State] {
	return func(p *Task[State]) {
		p.State = state
	}
}

// WithStateFunc sets the initial state using a function that receives the task.
// This allows for dynamic state initialization based on task properties.
func WithStateFunc[State any](fn func(*Task[State]) State) Option[State] {
	return func(p *Task[State]) {
		p.State = fn(p)
	}
}
