package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/MagistIO/go-concslice"
)

// fileStats holds statistics for a single file
// including letter counts and any processing errors
type fileStats struct {
	file   string
	counts map[rune]int
	error  error
}

// letterStats represents overall letter statistics
type letterStats struct {
	counts  map[rune]int
	files   int
	letters int
}

// binaryExts contains file extensions that should be skipped
// as they are likely binary files or compressed archives
var binaryExts = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".bin": true, ".obj": true, ".o": true, ".a": true,
	".zip": true, ".tar": true, ".gz": true, ".rar": true,
	".7z": true, ".bz2": true, ".xz": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".ico": true,
	".mp3": true, ".wav": true, ".flac": true, ".aac": true,
	".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true,
	".xlsx": true, ".ppt": true, ".pptx": true,
}

// fileProcessor is a concurrent processor for handling file paths
// it uses a channel to collect results from multiple workers
var fileProcessor = concslice.NewProcessor(
	handleFilePath,
	concslice.WithStateFunc(func(t *concslice.Task[chan fileStats]) chan fileStats {
		return make(chan fileStats, t.MaxWorkers()*2)
	}),
	concslice.WithOnFinish(func(result concslice.Result[chan fileStats]) {
		close(result.State)
	}),
)

// handleFilePath processes a single file path and sends results to the state channel
// This function is called by each worker goroutine
func handleFilePath(task *concslice.Task[chan fileStats], filePath string) {
	stats := fileStats{file: filePath}
	stats.counts, stats.error = countLettersInFile(filePath)
	task.State <- stats
}

// handleFileStats aggregates results from all processed files
// and returns comprehensive letter statistics
func handleFileStats(stats chan fileStats) letterStats {
	// Initialize result structure with empty counts map
	result := letterStats{
		counts: make(map[rune]int),
	}
	// Process each file's statistics
	for stat := range stats {
		// Increment file counter
		result.files++
		// Skip files with errors
		if stat.error != nil {
			fmt.Printf("❌ Error processing file %s: %v\n", stat.file, stat.error)
			continue
		}
		// Aggregate letter counts from this file
		for letter, count := range stat.counts {
			// Add to total letter count
			result.letters += count
			// Add to letter frequency map
			result.counts[letter] += count
		}
		fmt.Printf("✅ File %s processed successfully. Letters processed: %d\n", stat.file, len(stat.counts))
	}

	return result
}

// main is the entry point of the application
// It sets up context, processes files concurrently, and displays results
func main() {
	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle interrupt signals gracefully
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Get list of files to process
	files, err := getAllFiles(ctx)
	if err != nil {
		fmt.Printf("❌ Error getting file list: %v\n", err)
		return
	}

	fmt.Println("📊 Starting letter counting in files...")
	// Start concurrent processing of files
	task := fileProcessor.Process(ctx, files)

	// Collect and aggregate results from all workers
	letterStats := handleFileStats(task.State)

	// Wait for processing to complete
	result := task.Wait()

	// Check the result
	// Check if processing was interrupted
	if result.IsCanceled {
		fmt.Println("❌ Processing was canceled")
		return
	}

	// Report any errors that occurred during processing
	if len(result.Errors) > 0 {
		fmt.Printf("❌ Errors occurred during processing: %d\n", len(result.Errors))
		// Print each error for debugging
		for _, err := range result.Errors {
			fmt.Printf("   - %v\n", err)
		}
	}

	// Print results
	printResults(letterStats)
}

// getAllFiles recursively gets a list of all files in the directory
func getAllFiles(ctx context.Context) ([]string, error) {
	// Get directory for analysis from command line arguments
	// Set default directory to current directory
	dir := "."
	// Use command line argument if provided
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	fmt.Printf("🔍 Analyzing files in directory: %s\n", dir)

	var files []string

	// Walk through directory tree recursively
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip binary files and files without extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" || binaryExts[ext] {
			return nil
		}

		files = append(files, path)
		return nil
	})

	// Check for errors during directory traversal
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	// Ensure we found at least one file
	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in directory %s", dir)
	}

	fmt.Printf("📁 Found files: %d\n", len(files))

	return files, nil
}

// countLettersInFile opens a file and counts all letters
func countLettersInFile(filePath string) (map[rune]int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Initialize letter count map
	counts := make(map[rune]int)
	// Create scanner for efficient line-by-line reading
	scanner := bufio.NewScanner(file)

	// Read file line by line for proper UTF-8 processing
	for scanner.Scan() {
		line := scanner.Text()
		// Process each character in the line
		for _, char := range line {
			// Count only letters (both uppercase and lowercase)
			if unicode.IsLetter(char) {
				counts[char]++
			}
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return counts, nil
}

// printResults prints the letter counting results
func printResults(stats letterStats) {
	fmt.Println("\n📊 LETTER COUNTING RESULTS:")
	fmt.Println(strings.Repeat("=", 50))

	// Sort letters by frequency of use
	type letterCount struct {
		letter rune
		count  int
	}

	// Create slice for sorting letters by frequency
	var sortedLetters []letterCount
	// Convert map to slice for sorting
	for letter, count := range stats.counts {
		sortedLetters = append(sortedLetters, letterCount{letter, count})
	}

	// Sort by count in descending order
	sort.Slice(sortedLetters, func(i, j int) bool {
		return sortedLetters[i].count > sortedLetters[j].count
	})

	// Print overall statistics
	fmt.Println("\n🔤 OVERALL LETTER STATISTICS:")
	fmt.Println(strings.Repeat("-", 30))
	// Display each letter with its count and percentage
	for _, letterCount := range sortedLetters {
		fmt.Printf("%c: %d (%.2f%%)\n", letterCount.letter, letterCount.count,
			float64(letterCount.count)/float64(stats.letters)*100)
	}

	fmt.Printf("\n📈 Total letters: %d\n", stats.letters)
	fmt.Printf("📁 Files processed: %d\n", stats.files)
}
