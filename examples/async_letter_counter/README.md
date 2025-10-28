# Async Letter Counter

An example of using the `go-concslice` library for parallel letter counting in files.

## Description

The program recursively scans a specified directory, finds all text files, and counts the number of each letter in them in parallel. Uses the `go-concslice` library for efficient parallel processing.

## Features

- 🔍 **Recursive scanning** - automatically finds all files in directory and subdirectories
- ⚡ **Parallel processing** - uses multiple workers for fast file processing
- 🔤 **Multi-alphabet support** - counts letters from Latin and Cyrillic alphabets
- 📊 **Detailed statistics** - shows overall statistics and statistics for each file
- 🛡️ **File filtering** - automatically skips binary files and hidden files
- ⏱️ **Timeouts** - protection against hanging when processing large files

## Usage

```bash
# Analyze current directory
go run main.go

# Analyze specific directory
go run main.go /path/to/directory

# Example output
go run main.go examples/
```

## Example Output

```
🔍 Analyzing files in directory: examples/
📊 Starting letter counting in files...
📁 Found files: 15
✅ Processing completed. Files processed: 15

📊 LETTER COUNTING RESULTS:
==================================================

🔤 OVERALL LETTER STATISTICS:
------------------------------
е: 1250 (15.23%)
а: 980 (12.45%)
о: 920 (11.68%)
...

📈 Total letters: 8234
📁 Files processed: 15
```

## Architecture

1. **Recursive scanning** - `getAllFiles()` traverses the directory and collects a list of text files
2. **Parallel processing** - the `go-concslice` library creates workers to process files
3. **Letter counting** - each worker opens a file and counts letters
4. **Result aggregation** - results are combined into overall statistics
5. **Result output** - beautifully formatted report with sorting by frequency

## Technical Details

- Processing timeout: 30 seconds
- Supports files up to 1GB in size (buffered reading)
- Thread-safe result aggregation
- Automatic binary file filtering