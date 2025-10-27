package concslice

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcess(t *testing.T) {
	type testCase[T any] struct {
		name    string
		ctx     context.Context
		items   []T
		handler func(pc *Task[T], item T)
		check   func(t *testing.T, got *Result[T])
		options []Option[T]
	}
	tests := []testCase[int64]{
		{
			name:  "sum of items on one worker",
			ctx:   context.Background(),
			items: []int64{1, 2, 3},
			handler: func(pc *Task[int64], item int64) {
				pc.State += item
			},
			check: func(t *testing.T, got *Result[int64]) {
				require.Equal(t, int64(6), got.State, "Expected state to be 6")
			},
			options: []Option[int64]{
				WithMaxWorkers[int64](1),
			},
		},
		{
			name:  "sum of items on more workers",
			ctx:   context.Background(),
			items: slices.Repeat([]int64{1}, 1000),
			handler: func(pc *Task[int64], item int64) {
				atomic.AddInt64(&pc.State, item)
			},
			check: func(t *testing.T, got *Result[int64]) {
				require.Equal(t, int64(1000), got.State, "Expected state to be 1000")
			},
		},
		{
			name:  "items less than concurrency",
			ctx:   context.Background(),
			items: []int64{1, 2, 3},
			handler: func(pc *Task[int64], item int64) {
				atomic.AddInt64(&pc.State, item)
			},
			check: func(t *testing.T, got *Result[int64]) {
				require.Equal(t, int64(6), got.State, "Expected state to be 6")
			},
		},
		{
			name:  "empty items slice",
			ctx:   context.Background(),
			items: []int64{},
			handler: func(pc *Task[int64], item int64) {
				atomic.AddInt64(&pc.State, item)
			},
			check: func(t *testing.T, got *Result[int64]) {
				require.Equal(t, int64(0), got.State, "Expected state to be 0")
			},
		},
		{
			name:  "zero concurrency",
			ctx:   context.Background(),
			items: []int64{1, 2, 3},
			handler: func(pc *Task[int64], item int64) {
				atomic.AddInt64(&pc.State, item)
			},
			check: func(t *testing.T, got *Result[int64]) {
				require.Equal(t, int64(6), got.State, "Expected state to be 6")
			},
			options: []Option[int64]{
				WithMaxWorkers[int64](0),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Process(tt.ctx, tt.items, tt.handler, tt.options...).Wait()
			tt.check(t, got)
		})
	}
}

func TestProcessWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	items := slices.Repeat([]int64{1}, 1000)
	processedCount := int64(0)

	got := Process(
		ctx,
		items,
		func(pc *Task[int64], item int64) {
			atomic.AddInt64(&processedCount, 1)
			if atomic.AddInt64(&pc.State, item) > 100 {
				cancel()
			}
		},
	).Wait()

	assert.Equal(t, int(processedCount), got.ProcessedCount, "Expected processed count to be equal to processed count")
	assert.NotEqual(t, int64(0), processedCount, "Expected some items to be processed before cancelation")
	assert.NotEqual(t, int64(len(items)), processedCount, "Expected processing to be canceled before all items were processed")
	assert.GreaterOrEqual(t, got.State, int64(100), "Expected state to be greater than or equal to 100")
	assert.Less(t, got.State, int64(1000), "Expected state to be less than 1000")
}

func TestProcessAbort(t *testing.T) {
	items := slices.Repeat([]int64{1}, 1000)
	processedCount := int64(0)
	got := Process(
		context.Background(),
		items,
		func(pc *Task[int64], item int64) {
			if atomic.AddInt64(&processedCount, 1) > 50 {
				pc.Cancel()
			}
			atomic.AddInt64(&pc.State, item)
			time.Sleep(1 * time.Millisecond)
		},
	).Wait()

	assert.Equal(t, int(processedCount), got.ProcessedCount, "Expected processed count to be 50")
	assert.NotEqual(t, int64(0), processedCount, "Expected some items to be processed before abort")
	assert.NotEqual(t, int64(len(items)), processedCount, "Expected processing to be aborted before all items were processed")
	assert.GreaterOrEqual(t, got.State, int64(50), "Expected state to be greater than or equal to 50")
	assert.Less(t, got.State, int64(1000), "Expected state to be less than 1000")
}

func TestProcessConcurrentlyWithOnCompleteHook(t *testing.T) {
	type testStateWithOnCompleteHook struct {
		Sum       int64
		Completed bool
	}
	got := Process(
		context.Background(),
		[]int{1, 2, 3, 4, 5},
		func(pc *Task[*testStateWithOnCompleteHook], item int) {
			atomic.AddInt64(&pc.State.Sum, int64(item))
		},
		WithState(&testStateWithOnCompleteHook{}),
		WithOnFinish(func(state *testStateWithOnCompleteHook) {
			state.Completed = true
		}),
	).Wait()

	assert.True(t, got.State.Completed, "Expected completed to be true")
	assert.Equal(t, int64(15), got.State.Sum, "Expected sum to be 15")
	assert.Equal(t, 5, got.ProcessedCount, "Expected count to be 5")
}

func TestProcessConcurrentlyWithDifferentTypes(t *testing.T) {
	type Person struct {
		Name string
		Age  int64
	}

	type Stats struct {
		TotalAge int64
	}

	people := []Person{
		{"Alice", 25},
		{"Bob", 30},
		{"Charlie", 35},
	}

	got := Process(
		context.Background(),
		people,
		func(pc *Task[Stats], person Person) {
			atomic.AddInt64(&pc.State.TotalAge, person.Age)
		},
	).Wait()

	require.Equal(t, int64(90), got.State.TotalAge, "Expected total age to be 90")
	require.Equal(t, 3, got.ProcessedCount, "Expected count to be 3")
}

func TestProcessConcurrentlyWithStringItems(t *testing.T) {
	type StringProcessor struct {
		result string
		mu     sync.Mutex
	}

	items := []string{"hello", "world", "test", "concurrent"}

	got := Process(
		context.Background(),
		items,
		func(pc *Task[StringProcessor], item string) {
			pc.State.mu.Lock()
			defer pc.State.mu.Unlock()
			pc.State.result += item + " "
		},
	).Wait()

	assert.Equal(t, 4, got.ProcessedCount, "Expected count to be 4")
	assert.Contains(t, got.State.result, "hello", "Expected 'hello' to be in result")
	assert.Contains(t, got.State.result, "world", "Expected 'world' to be in result")
	assert.Contains(t, got.State.result, "test", "Expected 'test' to be in result")
	assert.Contains(t, got.State.result, "concurrent", "Expected 'concurrent' to be in result")
}

func TestProcessConcurrentlyWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	items := slices.Repeat([]int64{1}, 1000)
	got := Process(
		ctx,
		items,
		func(pc *Task[int64], item int64) {
			atomic.AddInt64(&pc.State, item)
			time.Sleep(2 * time.Millisecond) // Simulate work
		},
		WithMaxWorkers[int64](3),
	).Wait()

	assert.NotEqual(t, 0, got.ProcessedCount, "Expected some items to be processed before timeout")
	assert.NotEqual(t, len(items), got.ProcessedCount, "Expected processing to be canceled due to timeout")
	assert.Greater(t, got.State, int64(0), "Expected state to be greater than 0")
	assert.Less(t, got.State, int64(1000), "Expected state to be less than 1000")
}
