package concslice

import (
	"context"
	"fmt"
	"slices"
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
		handler func(task *Task[T], item T)
		check   func(t *testing.T, got Result[T])
		options []Option[T]
	}
	tests := []testCase[int64]{
		{
			name:  "sum of items on one worker",
			ctx:   context.Background(),
			items: []int64{1, 2, 3},
			handler: func(task *Task[int64], item int64) {
				task.State += item
			},
			check: func(t *testing.T, got Result[int64]) {
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
			handler: func(task *Task[int64], item int64) {
				atomic.AddInt64(&task.State, item)
			},
			check: func(t *testing.T, got Result[int64]) {
				require.Equal(t, int64(1000), got.State, "Expected state to be 1000")
			},
		},
		{
			name:  "items less than concurrency",
			ctx:   context.Background(),
			items: []int64{1, 2, 3},
			handler: func(task *Task[int64], item int64) {
				atomic.AddInt64(&task.State, item)
			},
			check: func(t *testing.T, got Result[int64]) {
				require.Equal(t, int64(6), got.State, "Expected state to be 6")
			},
		},
		{
			name:  "empty items slice",
			ctx:   context.Background(),
			items: []int64{},
			handler: func(task *Task[int64], item int64) {
				atomic.AddInt64(&task.State, item)
			},
			check: func(t *testing.T, got Result[int64]) {
				require.Equal(t, int64(0), got.State, "Expected state to be 0")
			},
		},
		{
			name:  "zero concurrency",
			ctx:   context.Background(),
			items: []int64{1, 2, 3},
			handler: func(task *Task[int64], item int64) {
				atomic.AddInt64(&task.State, item)
			},
			check: func(t *testing.T, got Result[int64]) {
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
		func(task *Task[int64], item int64) {
			atomic.AddInt64(&processedCount, 1)
			if atomic.AddInt64(&task.State, item) > 100 {
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
		func(task *Task[int64], item int64) {
			if atomic.AddInt64(&processedCount, 1) > 50 {
				task.Cancel()
			}
			atomic.AddInt64(&task.State, item)
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
		func(task *Task[*testStateWithOnCompleteHook], item int) {
			atomic.AddInt64(&task.State.Sum, int64(item))
		},
		WithState(&testStateWithOnCompleteHook{}),
		WithOnFinish(func(state Result[*testStateWithOnCompleteHook]) {
			state.State.Completed = true
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
		func(task *Task[Stats], person Person) {
			atomic.AddInt64(&task.State.TotalAge, person.Age)
		},
	).Wait()

	require.Equal(t, int64(90), got.State.TotalAge, "Expected total age to be 90")
	require.Equal(t, 3, got.ProcessedCount, "Expected count to be 3")
}

func TestProcessConcurrentlyWithStringItems(t *testing.T) {
	items := []string{"hello", "world", "test", "concurrent"}

	got := Process(
		context.Background(),
		items,
		func(task *Task[string], item string) {
			task.WithLock(func() {
				task.State += item + " "
			})
		},
	).Wait()

	assert.Equal(t, 4, got.ProcessedCount, "Expected count to be 4")
	assert.Contains(t, got.State, "hello", "Expected 'hello' to be in result")
	assert.Contains(t, got.State, "world", "Expected 'world' to be in result")
	assert.Contains(t, got.State, "test", "Expected 'test' to be in result")
	assert.Contains(t, got.State, "concurrent", "Expected 'concurrent' to be in result")
}

func TestProcessConcurrentlyWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	items := slices.Repeat([]int64{1}, 1000)
	got := Process(
		ctx,
		items,
		func(task *Task[int64], item int64) {
			atomic.AddInt64(&task.State, item)
			time.Sleep(2 * time.Millisecond) // Simulate work
		},
		WithMaxWorkers[int64](3),
	).Wait()

	assert.NotEqual(t, 0, got.ProcessedCount, "Expected some items to be processed before timeout")
	assert.NotEqual(t, len(items), got.ProcessedCount, "Expected processing to be canceled due to timeout")
	assert.Greater(t, got.State, int64(0), "Expected state to be greater than 0")
	assert.Less(t, got.State, int64(1000), "Expected state to be less than 1000")
}

func TestNewProcessor(t *testing.T) {
	handler := func(task *Task[int64], item int64) {
		atomic.AddInt64(&task.State, item)
	}

	processor := NewProcessor(handler, WithMaxWorkers[int64](2))
	require.NotNil(t, processor, "Expected processor to be created")

	items := []int64{1, 2, 3, 4, 5}
	got := processor.Process(context.Background(), items).Wait()

	assert.Equal(t, int64(15), got.State, "Expected state to be 15")
	assert.Equal(t, 5, got.ProcessedCount, "Expected processed count to be 5")
	assert.True(t, got.IsFinished, "Expected task to be finished")
	assert.False(t, got.IsCanceled, "IsCanceled should have some value")
}

func TestProcessorWithOptions(t *testing.T) {
	type Counter struct {
		Sum int64
	}

	handler := func(task *Task[Counter], item int64) {
		task.WithLock(func() {
			task.State.Sum += item
		})
	}

	var finalSum int64
	processor := NewProcessor(
		handler,
		WithMaxWorkers[Counter](1),
		WithState(Counter{}),
		WithOnFinish(func(result Result[Counter]) {
			finalSum = result.State.Sum * 2 // Store doubled sum
		}),
	)

	items := []int64{1, 2, 3}
	got := processor.Process(context.Background(), items).Wait()

	assert.Equal(t, int64(6), got.State.Sum, "Expected sum to be 6")
	assert.Equal(t, int64(12), finalSum, "Expected doubled sum to be 12")
	assert.Equal(t, 3, got.ProcessedCount, "Expected processed count to be 3")
}

func TestWithStateFunc(t *testing.T) {
	type DynamicState struct {
		Initialized bool
		Value       int64
	}

	handler := func(task *Task[DynamicState], item int64) {
		atomic.AddInt64(&task.State.Value, item)
	}

	got := Process(
		context.Background(),
		[]int64{1, 2, 3},
		handler,
		WithStateFunc(func(t *Task[DynamicState]) DynamicState {
			return DynamicState{
				Initialized: true,
				Value:       100, // Start with 100
			}
		}),
	).Wait()

	assert.True(t, got.State.Initialized, "Expected state to be initialized")
	assert.Equal(t, int64(106), got.State.Value, "Expected value to be 106 (100 + 1 + 2 + 3)")
	assert.Equal(t, 3, got.ProcessedCount, "Expected processed count to be 3")
}

func TestPanicHandlingInHandler(t *testing.T) {
	items := []int64{1, 2, 3}
	got := Process(
		context.Background(),
		items,
		func(task *Task[int64], item int64) {
			if item == 2 {
				panic("test panic")
			}
			atomic.AddInt64(&task.State, item)
		},
	).Wait()

	assert.Equal(t, int64(4), got.State, "Expected state to be 4 (1 + 3)")
	assert.Equal(t, 2, got.ProcessedCount, "Expected processed count to be 2")
	assert.True(t, got.IsFinished, "Expected task to be finished")
	assert.Len(t, got.Errors, 1, "Expected one panic error")
	assert.Contains(t, got.Errors[0].Error(), "panic in worker", "Expected panic error message")
}

func TestPanicHandlingInOnFinish(t *testing.T) {
	type TestState struct {
		Value int64
	}

	got := Process(
		context.Background(),
		[]int64{1, 2, 3},
		func(task *Task[TestState], item int64) {
			task.WithLock(func() {
				task.State.Value += item
			})
		},
		WithState(TestState{}),
		WithOnFinish(func(result Result[TestState]) {
			panic("onFinish panic")
		}),
	).Wait()

	assert.Equal(t, int64(6), got.State.Value, "Expected state value to be 6")
	assert.Equal(t, 3, got.ProcessedCount, "Expected processed count to be 3")
	assert.True(t, got.IsFinished, "Expected task to be finished")
	assert.Len(t, got.Errors, 1, "Expected one panic error")
	assert.Contains(t, got.Errors[0].Error(), "panic in onFinish callback", "Expected onFinish panic error message")
}

func TestTaskMethods(t *testing.T) {
	items := []int64{1, 2, 3, 4, 5}
	task := Process(
		context.Background(),
		items,
		func(task *Task[int64], item int64) {
			atomic.AddInt64(&task.State, item)
			time.Sleep(5 * time.Millisecond) // Longer sleep to ensure cancel works
		},
		WithMaxWorkers[int64](1), // Single worker to make timing more predictable
	)

	// Test Context method
	ctx := task.Context()
	assert.NotNil(t, ctx, "Expected context to be non-nil")

	// Test MaxWorkers method
	assert.Equal(t, 1, task.MaxWorkers(), "Expected max workers to be 1")

	// Test ProcessedCount method during processing
	time.Sleep(2 * time.Millisecond) // Let some processing happen
	count := task.ProcessedCount()
	assert.GreaterOrEqual(t, count, 0, "Expected processed count to be non-negative")
	assert.LessOrEqual(t, count, len(items), "Expected processed count to be less than or equal to items count")

	// Test Cancel method
	task.Cancel()

	// Wait for completion
	result := task.Wait()
	assert.True(t, result.IsFinished, "Expected task to be finished")
	assert.Less(t, result.ProcessedCount, len(items), "Expected some items to be processed before cancel")
}

func TestTaskMethodsWithEmptyItems(t *testing.T) {
	task := Process(
		context.Background(),
		[]int64{},
		func(task *Task[int64], item int64) {
			atomic.AddInt64(&task.State, item)
		},
	)

	// For empty items, task is not initialized and finish() is not called
	result := task.Wait()
	assert.Equal(t, int64(0), result.State, "Expected state to be 0")
	assert.Equal(t, 0, result.ProcessedCount, "Expected processed count to be 0")
	assert.False(t, result.IsFinished, "Expected task not to be finished (no finish() called)")
	assert.False(t, result.IsCanceled, "Expected task not to be canceled")
}

func TestEdgeCases(t *testing.T) {
	t.Run("negative concurrency", func(t *testing.T) {
		// Negative concurrency should cause panic due to WaitGroup.Add with negative value
		assert.Panics(t, func() {
			Process(
				context.Background(),
				[]int64{1, 2, 3},
				func(task *Task[int64], item int64) {
					task.WithLock(func() {
						task.State += item
					})
				},
				WithMaxWorkers[int64](-1),
			).Wait()
		}, "Expected panic with negative concurrency")
	})

	t.Run("concurrency greater than items count", func(t *testing.T) {
		got := Process(
			context.Background(),
			[]int64{1, 2},
			func(task *Task[int64], item int64) {
				task.WithLock(func() {
					task.State += item
				})
			},
			WithMaxWorkers[int64](10),
		).Wait()

		assert.Equal(t, int64(3), got.State, "Expected state to be 3")
		assert.Equal(t, 2, got.ProcessedCount, "Expected processed count to be 2")
	})

	t.Run("nil handler", func(t *testing.T) {
		// This should not panic, but handler won't be called and ProcessedCount won't increment
		got := Process[int64, int64](
			context.Background(),
			[]int64{1, 2, 3},
			nil,
		).Wait()

		assert.Equal(t, int64(0), got.State, "Expected state to be 0")
		assert.Equal(t, 0, got.ProcessedCount, "Expected processed count to be 0 (no incrementCounter calls)")
	})
}

func TestErrorCollection(t *testing.T) {
	items := []int64{1, 2, 3, 4, 5}
	got := Process(
		context.Background(),
		items,
		func(task *Task[int64], item int64) {
			if item == 2 || item == 4 {
				panic(fmt.Sprintf("panic for item %d", item))
			}
			task.WithLock(func() {
				task.State += item
			})
		},
	).Wait()

	assert.Equal(t, int64(9), got.State, "Expected state to be 9 (1 + 3 + 5)")
	assert.Equal(t, 3, got.ProcessedCount, "Expected processed count to be 3")
	assert.Len(t, got.Errors, 2, "Expected two panic errors")

	// Check that both panic errors are present (order is not guaranteed)
	errorMessages := make([]string, len(got.Errors))
	for i, err := range got.Errors {
		errorMessages[i] = err.Error()
	}
	assert.Contains(t, errorMessages, "panic in worker 1: panic for item 2", "Expected panic for item 2")
	assert.Contains(t, errorMessages, "panic in worker 3: panic for item 4", "Expected panic for item 4")
}

func TestConcurrentAccessToTaskState(t *testing.T) {
	type Counter struct {
		Value int
	}

	items := slices.Repeat([]int{1}, 1000)
	got := Process(
		context.Background(),
		items,
		func(task *Task[Counter], item int) {
			task.WithLock(func() {
				task.State.Value += item
			})
		},
		WithMaxWorkers[Counter](10),
	).Wait()

	assert.Equal(t, 1000, got.State.Value, "Expected value to be 1000")
	assert.Equal(t, 1000, got.ProcessedCount, "Expected processed count to be 1000")
	assert.True(t, got.IsFinished, "Expected task to be finished")
	assert.False(t, got.IsCanceled, "IsCanceled should be false")
	assert.Empty(t, got.Errors, "Expected no errors")
}
