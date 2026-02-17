package stats

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

// Test data generation helpers

// createBenchmarkStrings generates a set of test strings with varying characteristics
func createBenchmarkStrings(count int) [][]byte {
	strings := make([][]byte, 0, count)

	patterns := [][]byte{
		[]byte("short"),
		[]byte("medium length string"),
		[]byte("This is a much longer string that should be categorized differently"),
		[]byte("test@example.com"),
		[]byte("https://www.example.com/path"),
		[]byte("Error: something went wrong"),
		[]byte("192.168.1.1"),
	}

	for i := range count {
		strings = append(strings, patterns[i%len(patterns)])
	}

	return strings
}

// Benchmark: Statistics collection

func BenchmarkStatistics_Add_Small(b *testing.B) {
	benchmarkStatisticsAdd(b, 10)
}

func BenchmarkStatistics_Add_Medium(b *testing.B) {
	benchmarkStatisticsAdd(b, 100)
}

func BenchmarkStatistics_Add_Large(b *testing.B) {
	benchmarkStatisticsAdd(b, 1000)
}

func benchmarkStatisticsAdd(b *testing.B, count int) {
	strings := createBenchmarkStrings(count)
	config := extractor.Config{MinLength: 4}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := New(4)
		for j, str := range strings {
			stats.Add(str, "test.bin", int64(j*100), config)
		}
	}
}

// Benchmark: Statistics formatting

func BenchmarkStatistics_Format_Small(b *testing.B) {
	benchmarkStatisticsFormat(b, 10)
}

func BenchmarkStatistics_Format_Medium(b *testing.B) {
	benchmarkStatisticsFormat(b, 100)
}

func BenchmarkStatistics_Format_Large(b *testing.B) {
	benchmarkStatisticsFormat(b, 1000)
}

func benchmarkStatisticsFormat(b *testing.B, count int) {
	stats := New(4)
	strings := createBenchmarkStrings(count)
	config := extractor.Config{MinLength: 4}

	for j, str := range strings {
		stats.Add(str, "test.bin", int64(j*100), config)
	}

	var buf bytes.Buffer
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		stats.Format(&buf, extractor.ColorNever)
	}
}

// Benchmark: Statistics merge (parallel aggregation)

func BenchmarkStatistics_Merge_Small(b *testing.B) {
	benchmarkStatisticsMerge(b, 10)
}

func BenchmarkStatistics_Merge_Medium(b *testing.B) {
	benchmarkStatisticsMerge(b, 100)
}

func BenchmarkStatistics_Merge_Large(b *testing.B) {
	benchmarkStatisticsMerge(b, 1000)
}

func benchmarkStatisticsMerge(b *testing.B, count int) {
	// Create two statistics instances with data
	stats1 := New(4)
	stats2 := New(4)
	strings := createBenchmarkStrings(count)
	config := extractor.Config{MinLength: 4}

	for j, str := range strings {
		stats1.Add(str, "file1.bin", int64(j*100), config)
		stats2.Add(str, "file2.bin", int64(j*100), config)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a fresh copy for merging
		statsCopy := New(4)
		statsCopy.Merge(stats1)
		statsCopy.Merge(stats2)
	}
}

// Benchmark: Encoding classification overhead

func BenchmarkStatistics_EncodingClassification(b *testing.B) {
	stats := New(4)
	config := extractor.Config{MinLength: 4}

	// Different string types
	testCases := []struct {
		name string
		str  []byte
	}{
		{"ASCII7bit", []byte("Hello World")},
		{"ASCII8bit", []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x80, 0x81}},
		{"UTF8", []byte("Hello 世界")},
		{"UTF16", []byte{0x48, 0x00, 0x65, 0x00, 0x6c, 0x00, 0x6c, 0x00}},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				stats.Add(tc.str, "test.bin", 0, config)
			}
		})
	}
}

// Benchmark: Length bucketing

func BenchmarkStatistics_LengthBucketing(b *testing.B) {
	stats := New(4)
	config := extractor.Config{MinLength: 4}

	// Strings of different lengths
	testCases := []struct {
		name string
		str  []byte
	}{
		{"4-10chars", []byte("short")},
		{"11-50chars", []byte("This is a medium length string")},
		{"51-100chars", []byte("This is a longer string that falls into the fifty-one to one hundred character bucket for testing")},
		{"100+chars", bytes.Repeat([]byte("A"), 150)},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				stats.Add(tc.str, "test.bin", 0, config)
			}
		})
	}
}

// Benchmark: Longest strings tracking

func BenchmarkStatistics_LongestTracking(b *testing.B) {
	config := extractor.Config{MinLength: 4}

	// Pre-generate strings of varying lengths
	strings := make([][]byte, 100)
	for i := range 100 {
		length := 10 + (i * 5) // Increasing lengths
		strings[i] = bytes.Repeat([]byte("A"), length)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := New(4)
		for j, str := range strings {
			stats.Add(str, "test.bin", int64(j*100), config)
		}
	}
}

// Benchmark: Filter tracking overhead

func BenchmarkStatistics_WithFilterTracking(b *testing.B) {
	strings := createBenchmarkStrings(100)
	config := extractor.Config{MinLength: 4}

	b.Run("NoFilterTracking", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats := New(4)
			for j, str := range strings {
				stats.Add(str, "test.bin", int64(j*100), config)
			}
		}
	})

	b.Run("WithFilterTracking", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stats := New(4)
			for j := 0; j < len(strings)*2; j++ {
				stats.AddUnfiltered()
				if j%2 == 0 {
					stats.Add(strings[j/2], "test.bin", int64(j*50), config)
				}
			}
		}
	})
}

// Benchmark: Full pipeline (collect + format)

func BenchmarkStatistics_FullPipeline_Small(b *testing.B) {
	benchmarkStatisticsFullPipeline(b, 10)
}

func BenchmarkStatistics_FullPipeline_Medium(b *testing.B) {
	benchmarkStatisticsFullPipeline(b, 100)
}

func BenchmarkStatistics_FullPipeline_Large(b *testing.B) {
	benchmarkStatisticsFullPipeline(b, 1000)
}

func benchmarkStatisticsFullPipeline(b *testing.B, count int) {
	strings := createBenchmarkStrings(count)
	config := extractor.Config{MinLength: 4}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := New(4)
		for j, str := range strings {
			stats.Add(str, "test.bin", int64(j*100), config)
		}
		stats.Format(io.Discard, extractor.ColorNever)
	}
}

// Benchmark: Parallel aggregation simulation

func BenchmarkStatistics_ParallelAggregation(b *testing.B) {
	workerCounts := []int{2, 4, 8}

	for _, workers := range workerCounts {
		b.Run(formatWorkers(workers), func(b *testing.B) {
			strings := createBenchmarkStrings(1000)
			config := extractor.Config{MinLength: 4}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Simulate parallel workers
				workerStats := make([]*Statistics, workers)
				stringsPerWorker := len(strings) / workers

				for w := range workers {
					workerStats[w] = New(4)
					start := w * stringsPerWorker
					end := start + stringsPerWorker
					if w == workers-1 {
						end = len(strings)
					}

					for j := start; j < end; j++ {
						workerStats[w].Add(strings[j], "test.bin", int64(j*100), config)
					}
				}

				// Merge all worker statistics
				aggregated := New(4)
				for _, ws := range workerStats {
					aggregated.Merge(ws)
				}
			}
		})
	}
}

// Benchmark: Memory allocation patterns

func BenchmarkStatistics_Allocations(b *testing.B) {
	strings := createBenchmarkStrings(100)
	config := extractor.Config{MinLength: 4}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stats := New(4)
		for j, str := range strings {
			stats.Add(str, "test.bin", int64(j*100), config)
		}
	}
}

// Benchmark: Comparison with standard extraction

func BenchmarkComparison_StandardVsStats(b *testing.B) {
	strings := createBenchmarkStrings(100)
	config := extractor.Config{MinLength: 4}

	b.Run("StandardExtraction", func(b *testing.B) {
		// Simulate standard extraction (just print function call overhead)
		printFunc := func(_ []byte, _ string, _ int64, _ extractor.Config) {}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			for j, str := range strings {
				printFunc(str, "test.bin", int64(j*100), config)
			}
		}
	})

	b.Run("StatsCollection", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			stats := New(4)
			for j, str := range strings {
				stats.Add(str, "test.bin", int64(j*100), config)
			}
		}
	})
}

// Helper function to format worker count
func formatWorkers(count int) string {
	switch count {
	case 2:
		return "2workers"
	case 4:
		return "4workers"
	case 8:
		return "8workers"
	default:
		return fmt.Sprintf("%dworkers", count)
	}
}
