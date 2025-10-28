# go-concslice
Go library for parallel slice processing with context support, state management, and configurable concurrency. Easy worker control.

## Key Features

### Parallel Processing
- **Generic slice processing**: Support for any data types through Go generics
- **Automatic worker management**: Automatic determination of optimal number of workers (GOMAXPROCS)

### State Management
- **Built-in state**: Support for custom state with type safety
- **Callback functions**: Ability to execute code upon completion of processing

### Context Support
- **Operation cancellation**: Full integration with Go context for cancellation
- **Timeouts**: Support for context timeouts
- **Graceful shutdown**: Proper worker termination

## API Functions

### Core Functions
- `Process[Item, State]()` - starts parallel processing of slice elements
- `NewProcessor[Item, State]()` - creates a processor with pre-configured settings

### Task Management
- `Wait()` - waits for processing completion and returns Result
- `Cancel()` - cancels current processing
- `Context()` - gets the task context

### State Information
- `ProcessedCount` - number of processed elements (via Result)
- `MaxWorkers()` - maximum number of workers
- `IsCanceled` - cancellation status (via Result)

### Configuration Options
- `WithMaxWorkers[State](concurrency int)` - sets the number of workers
- `WithOnFinish[State](onFinish func(*Result[State]))` - callback on completion
- `WithState[State](state State)` - sets initial state
- `WithStateFunc[State](fn func(*Task[State]) State)` - sets state via function

## Technical Details

### Data Structures
- **`Task[State]`** - main task structure with state
- **`Result[State]`** - processing result with state and metadata
- **`Processor[Item, State]`** - processor with pre-configured settings

### Error Handling
- **Panic recovery**: Built-in panic handling in workers
- **Graceful degradation**: Proper handling of empty slices
- **Context errors**: Handling of context errors (cancellation, timeout)


## Usage Examples

### Basic Usage

```go
// Simple processing with summation
result := Process(ctx, items, func(task *Task[int64], item int64) {
    atomic.AddInt64(&task.State, item)
}).Wait()

// With completion callback
result := Process(ctx, items, handler, 
    WithOnFinish(func(result *Result[int64]) {
        if result.IsCompleted {
            fmt.Printf("Processed elements: %d\n", result.State)
        } else {
            fmt.Printf("Processing was canceled or timed out\n")
        }
    })).Wait()

// Processing custom types
type Person struct { Name string; Age int }
type Stats struct { TotalAge int64 }

people := []Person{{"Alice", 25}, {"Bob", 30}}
result := Process(ctx, people, func(task *Task[Stats], person Person) {
    atomic.AddInt64(&task.State.TotalAge, int64(person.Age))
}).Wait()
```

### Advanced Usage

```go
// Using Processor for reusable processing
processor := NewProcessor(
    func(task *Task[map[string]int], word string) {
        task.WithLock(func() {
            task.State[word]++
        })
    },
    WithState(map[string]int{}),
)

// Process multiple batches
for _, batch := range batches {
    result := processor.Process(ctx, batch).Wait()
    fmt.Printf("Processed %d words\n", result.ProcessedCount)
}

// Dynamic state initialization
result := Process(ctx, items, handler,
    WithStateFunc(func(t *Task[Counter]) Counter {
        return Counter{
            StartTime: time.Now(),
            MaxWorkers: t.MaxWorkers(),
        }
    }),
)
```

### Panic Handling

```go
result := Process(ctx, items, func(task *Task[int], item int) {
    if item < 0 {
        panic(fmt.Sprintf("negative item: %d", item))
    }
    task.State += item
},
WithOnFinish(func(result *Result[int]) {
    panic("onFinish panic")
}),
).Wait()

if len(result.Errors) > 0 {
    fmt.Printf("Processing completed with %d errors\n", len(result.Errors))
    for _, err := range result.Errors {
        fmt.Printf("Error: %v\n", err)
    }
}
```

### Cancellation and Timeouts

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result := Process(ctx, items, handler).Wait()
if result.IsCanceled {
    fmt.Println("Processing was canceled due to timeout")
}

// Manual cancellation
task := Process(ctx, items, handler)
go func() {
    time.Sleep(2*time.Second)
    task.Cancel()
}()
result := task.Wait()
```

## Best Practices

### State Management
- **Use atomic operations** for simple counters and flags
- **Use mutexes** for complex state modifications
- **Initialize state properly** using `WithState` or `WithStateFunc`

### Error Handling
- **Handle panics gracefully** - they are automatically caught and stored in `Result.Errors`
- **Check for context cancellation** in long-running handlers
- **Validate input data** before processing

### Performance Optimization
- **Choose appropriate worker count** - usually `runtime.GOMAXPROCS(0)` is optimal
- **Use Processor for repeated operations** to avoid reconfiguration overhead
- **Consider memory usage** for large slices and complex state