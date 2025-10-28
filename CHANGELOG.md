## 0.2.0

### Bug fixes and improvements

#### Race condition fix
- **Critical fix**: Resolved race condition in goroutine synchronization by replacing `sync.WaitGroup` with local variable in `Process` function
- Fixed issue with accessing `t.wg.Done()` from different goroutines that could lead to panic
- Improved task completion safety through proper synchronization

#### Error handling improvements
- Added panic and error collection to `Result` structure
- Implemented `collectPanicError()` method for safe error collection from goroutines
- Improved panic handling in handlers and `onFinish` callback functions
- Added informative error messages with worker number indication

#### API changes
- Changed `WithOnFinish` signature - now accepts `func(Result[State])` instead of `func(State)`
- Added new fields to `Result` structure: `IsFinished`, `IsCompleted`, and `Errors`
- Changed `ProcessedCount` type from `int` to `int64` for atomic operations
- `Wait()` method now returns `Result[State]` instead of `*Result[State]`
- Added `WithLock()` method for safe concurrent state modifications
- Added `ProcessedCount()` method for real-time progress tracking

#### New features
- Added `ProcessedCount()` method for getting current count of processed elements
- Implemented new `NewProcessor()` constructor for creating processors with preset configurations
- Added `WithStateFunc()` option for dynamic state initialization

#### Technical improvements
- Replaced global `sync.WaitGroup` with local variable to prevent race conditions
- Implemented atomic operations for `ProcessedCount` using `sync/atomic`
- Added comprehensive error collection system with panic recovery
- Improved memory management and resource cleanup
- Enhanced thread safety with proper mutex usage
- Optimized goroutine synchronization patterns

#### Testing improvements
- Added tests for panic handling in handlers and callback functions
- Extended tests for checking correct behavior with empty arrays
- Added tests for edge cases (negative concurrency, nil handlers)
- Improved tests for checking atomicity of operations
- Added comprehensive tests for `NewProcessor` functionality
- Added tests for `WithStateFunc` dynamic state initialization
- Added tests for concurrent access to task state
- Added tests for error collection from multiple panics

#### Documentation and examples
- Complete rewrite of README.md with comprehensive English documentation
- Added detailed usage examples with best practices
- Added advanced usage patterns and performance optimization tips
- Created new example: `async_letter_counter` - parallel letter counting in files
- Added example demonstrating panic handling and error collection
- Added example showing cancellation and timeout patterns

## 0.1.0

### Initial release
