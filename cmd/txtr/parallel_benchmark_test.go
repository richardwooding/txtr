package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

// Test data generation helpers

// createBenchmarkFile creates a temporary file with test data
func createBenchmarkFile(t testing.TB, size int) string {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")

	// Create data with mixed printable and non-printable bytes
	data := make([]byte, size)
	pattern := []byte("BenchmarkString123")
	separator := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xFF}

	pos := 0
	for pos < size {
		// Add pattern
		for i := 0; i < len(pattern) && pos < size; i++ {
			data[pos] = pattern[i]
			pos++
		}
		// Add separator
		for i := 0; i < len(separator) && pos < size; i++ {
			data[pos] = separator[i]
			pos++
		}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return path
}

// createBenchmarkFiles creates multiple temporary files for parallel testing
func createBenchmarkFiles(t testing.TB, count int, sizePerFile int) []string {
	files := make([]string, count)
	for i := 0; i < count; i++ {
		files[i] = createBenchmarkFile(t, sizePerFile)
	}
	return files
}

// Benchmark: Sequential vs parallel processing

func BenchmarkProcessing_Sequential_1File(b *testing.B) {
	files := createBenchmarkFiles(b, 1, 1*1024*1024) // 1x 1MB
	benchmarkSequential(b, files)
}

func BenchmarkProcessing_Sequential_4Files(b *testing.B) {
	files := createBenchmarkFiles(b, 4, 1*1024*1024) // 4x 1MB
	benchmarkSequential(b, files)
}

func BenchmarkProcessing_Sequential_8Files(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024) // 8x 1MB
	benchmarkSequential(b, files)
}

func BenchmarkProcessing_Sequential_16Files(b *testing.B) {
	files := createBenchmarkFiles(b, 16, 1*1024*1024) // 16x 1MB
	benchmarkSequential(b, files)
}

func benchmarkSequential(b *testing.B, files []string) {
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}
	// Use no-op print function for benchmarking
	printFunc := func(_ []byte, _ string, _ int64, _ extractor.Config) {}

	totalSize := int64(len(files) * 1024 * 1024)
	b.SetBytes(totalSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, filename := range files {
			_ = extractor.ExtractStringsFromFile(filename, config, printFunc)
		}
	}

	throughput := float64(totalSize) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: Parallel processing with different worker counts

func BenchmarkProcessing_Parallel_2Workers_4Files(b *testing.B) {
	files := createBenchmarkFiles(b, 4, 1*1024*1024)
	benchmarkParallel(b, files, 2)
}

func BenchmarkProcessing_Parallel_4Workers_4Files(b *testing.B) {
	files := createBenchmarkFiles(b, 4, 1*1024*1024)
	benchmarkParallel(b, files, 4)
}

func BenchmarkProcessing_Parallel_8Workers_8Files(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	benchmarkParallel(b, files, 8)
}

func BenchmarkProcessing_Parallel_2Workers_8Files(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	benchmarkParallel(b, files, 2)
}

func BenchmarkProcessing_Parallel_4Workers_8Files(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	benchmarkParallel(b, files, 4)
}

func BenchmarkProcessing_Parallel_8Workers_16Files(b *testing.B) {
	files := createBenchmarkFiles(b, 16, 1*1024*1024)
	benchmarkParallel(b, files, 8)
}

func benchmarkParallel(b *testing.B, files []string, workers int) {
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}

	totalSize := int64(len(files) * 1024 * 1024)
	b.SetBytes(totalSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		processFilesParallel(files, workers, config)
	}

	throughput := float64(totalSize) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: Speedup validation (compare sequential vs parallel)

func BenchmarkSpeedup_4Files(b *testing.B) {
	files := createBenchmarkFiles(b, 4, 1*1024*1024)
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ extractor.Config) {}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, filename := range files {
				_ = extractor.ExtractStringsFromFile(filename, config, printFunc)
			}
		}
	})

	b.Run("Parallel-2cores", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processFilesParallel(files, 2, config)
		}
	})

	b.Run("Parallel-4cores", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processFilesParallel(files, 4, config)
		}
	})
}

func BenchmarkSpeedup_8Files(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ extractor.Config) {}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, filename := range files {
				_ = extractor.ExtractStringsFromFile(filename, config, printFunc)
			}
		}
	})

	b.Run("Parallel-2cores", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processFilesParallel(files, 2, config)
		}
	})

	b.Run("Parallel-4cores", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processFilesParallel(files, 4, config)
		}
	})

	b.Run("Parallel-8cores", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processFilesParallel(files, 8, config)
		}
	})
}

// Benchmark: Auto-detect worker count

func BenchmarkProcessing_AutoWorkers(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}

	workers := runtime.NumCPU()
	b.Logf("Auto-detected %d CPUs", workers)

	totalSize := int64(len(files) * 1024 * 1024)
	b.SetBytes(totalSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		processFilesParallel(files, workers, config)
	}

	throughput := float64(totalSize) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: Different file sizes

func BenchmarkParallel_SmallFiles(b *testing.B) {
	// 16 files x 100KB each = 1.6MB total
	files := createBenchmarkFiles(b, 16, 100*1024)
	benchmarkParallel(b, files, 4)
}

func BenchmarkParallel_MediumFiles(b *testing.B) {
	// 8 files x 1MB each = 8MB total
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	benchmarkParallel(b, files, 4)
}

func BenchmarkParallel_LargeFiles(b *testing.B) {
	// 4 files x 10MB each = 40MB total
	files := createBenchmarkFiles(b, 4, 10*1024*1024)
	benchmarkParallel(b, files, 4)
}

// Benchmark: Worker pool overhead

func BenchmarkParallelOverhead(b *testing.B) {
	files := createBenchmarkFiles(b, 4, 1*1024*1024)
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}

	workerCounts := []int{1, 2, 4, 8, 16}

	for _, workers := range workerCounts {
		b.Run(formatWorkers(workers), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				processFilesParallel(files, workers, config)
			}
		})
	}
}

// Benchmark: File count vs worker count balance

func BenchmarkFileWorkerBalance(b *testing.B) {
	testCases := []struct {
		files   int
		workers int
	}{
		{2, 2},   // 1 file per worker
		{4, 2},   // 2 files per worker
		{8, 2},   // 4 files per worker
		{8, 4},   // 2 files per worker
		{16, 4},  // 4 files per worker
		{16, 8},  // 2 files per worker
		{32, 8},  // 4 files per worker
	}

	for _, tc := range testCases {
		name := formatBalance(tc.files, tc.workers)
		b.Run(name, func(b *testing.B) {
			files := createBenchmarkFiles(b, tc.files, 100*1024) // Small files for quick testing
			config := extractor.Config{
				MinLength: 4,
				Encoding:  "s",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				processFilesParallel(files, tc.workers, config)
			}
		})
	}
}

// Benchmark: Memory allocation in parallel mode

func BenchmarkParallel_Allocations(b *testing.B) {
	files := createBenchmarkFiles(b, 8, 1*1024*1024)
	config := extractor.Config{
		MinLength: 4,
		Encoding:  "s",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		processFilesParallel(files, 4, config)
	}
}

// Helper functions

func formatWorkers(count int) string {
	switch count {
	case 1:
		return "1worker"
	case 2:
		return "2workers"
	case 4:
		return "4workers"
	case 8:
		return "8workers"
	case 16:
		return "16workers"
	default:
		return fmt.Sprintf("%dworkers", count)
	}
}

func formatBalance(files, workers int) string {
	perWorker := files / workers
	return formatWorkers(workers) + "_" + formatFiles(files) + "_" + formatRatio(perWorker)
}

func formatFiles(count int) string {
	switch count {
	case 2:
		return "2files"
	case 4:
		return "4files"
	case 8:
		return "8files"
	case 16:
		return "16files"
	case 32:
		return "32files"
	default:
		return fmt.Sprintf("%dfiles", count)
	}
}

func formatRatio(perWorker int) string {
	switch perWorker {
	case 1:
		return "1each"
	case 2:
		return "2each"
	case 4:
		return "4each"
	default:
		return fmt.Sprintf("%deach", perWorker)
	}
}
