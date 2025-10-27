package concslice

import (
	"context"
)

type (
	Processor[Item, State any] struct {
		handler func(p *Task[State], i Item)
		options []Option[State]
	}
	Option[State any] func(*Task[State])
)

func NewProcessor[Item, State any](handler func(p *Task[State], i Item), options ...Option[State]) *Processor[Item, State] {
	return &Processor[Item, State]{
		handler: handler,
		options: options,
	}
}

func (p *Processor[Item, State]) Process(ctx context.Context, items []Item) *Task[State] {
	return Process(ctx, items, p.handler, p.options...)
}

func Process[Item any, State any](ctx context.Context, items []Item, handler func(p *Task[State], i Item), options ...Option[State]) *Task[State] {
	t := &Task[State]{
		itemsCount: len(items),
	}
	if t.itemsCount == 0 {
		return t
	}
	t.initialize(ctx, options)
	for i := range t.maxWorkers {
		go func(index int) {
			defer func() {
				if err := recover(); err != nil {
					// TODO: collect panic error
				}
				t.wg.Done()
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
	go t.wait()
	return t
}

func WithMaxWorkers[State any](concurrency int) Option[State] {
	return func(p *Task[State]) {
		p.maxWorkers = concurrency
	}
}

func WithOnFinish[State any](onFinish func(State)) Option[State] {
	return func(p *Task[State]) {
		p.onFinish = onFinish
	}
}

func WithState[State any](state State) Option[State] {
	return func(p *Task[State]) {
		p.State = state
	}
}

func WithStateFunc[State any](fn func(*Task[State]) State) Option[State] {
	return func(p *Task[State]) {
		p.State = fn(p)
	}
}
