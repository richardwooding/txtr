package printer

import (
	"bytes"
	"io"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

// Benchmark: Basic string printing

func BenchmarkPrintString_Simple(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength: 4,
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

// Benchmark: With filename prefix

func BenchmarkPrintString_WithFilename(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:     4,
		PrintFileName: true,
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

// Benchmark: With offset in different radixes

func BenchmarkPrintString_OffsetOctal(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:   4,
		PrintOffset: true,
		Radix:       "o",
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

func BenchmarkPrintString_OffsetDecimal(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:   4,
		PrintOffset: true,
		Radix:       "d",
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

func BenchmarkPrintString_OffsetHex(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:   4,
		PrintOffset: true,
		Radix:       "x",
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

// Benchmark: With all metadata

func BenchmarkPrintString_AllMetadata(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:     4,
		PrintFileName: true,
		PrintOffset:   true,
		Radix:         "x",
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

// Benchmark: Color mode overhead

func BenchmarkPrintString_ColorNever(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:     4,
		PrintFileName: true,
		PrintOffset:   true,
		Radix:         "x",
		ColorMode:     extractor.ColorNever,
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

func BenchmarkPrintString_ColorAlways(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:     4,
		PrintFileName: true,
		PrintOffset:   true,
		Radix:         "x",
		ColorMode:     extractor.ColorAlways,
		Encoding:      "s",
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

// Benchmark: Different string lengths

func BenchmarkPrintString_Length(b *testing.B) {
	lengths := []int{4, 16, 64, 256, 1024}
	config := extractor.Config{
		MinLength: 4,
	}
	writer := io.Discard

	for _, length := range lengths {
		b.Run(formatLength(length), func(b *testing.B) {
			str := bytes.Repeat([]byte("A"), length)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				PrintStringToWriter(writer, str, "", 0, config)
			}
		})
	}
}

// Benchmark: Custom output separator

func BenchmarkPrintString_CustomSeparator(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	config := extractor.Config{
		MinLength:        4,
		OutputSeparator:  " | ",
	}
	writer := io.Discard

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PrintStringToWriter(writer, str, filename, offset, config)
	}
}

// Benchmark: JSON output

func BenchmarkJSONPrinter_Collect(b *testing.B) {
	config := extractor.Config{MinLength: 4, Encoding: "s"}
	printer := NewJSONPrinter(config, io.Discard)
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printer.PrintString(str, filename, offset, config)
	}
}

func BenchmarkJSONPrinter_Flush(b *testing.B) {
	config := extractor.Config{MinLength: 4, Encoding: "s"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		printer := NewJSONPrinter(config, io.Discard)
		// Collect 100 strings
		for j := 0; j < 100; j++ {
			printer.PrintString([]byte("Test String"), "test.bin", int64(j*10), config)
		}
		b.StartTimer()

		printer.Flush()
	}
}

func BenchmarkJSONPrinter_SmallDataset(b *testing.B) {
	benchmarkJSONPrinterWithCount(b, 10)
}

func BenchmarkJSONPrinter_MediumDataset(b *testing.B) {
	benchmarkJSONPrinterWithCount(b, 100)
}

func BenchmarkJSONPrinter_LargeDataset(b *testing.B) {
	benchmarkJSONPrinterWithCount(b, 1000)
}

func benchmarkJSONPrinterWithCount(b *testing.B, count int) {
	config := extractor.Config{MinLength: 4, Encoding: "s"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printer := NewJSONPrinter(config, io.Discard)
		for j := 0; j < count; j++ {
			printer.PrintString([]byte("Test String"), "test.bin", int64(j*10), config)
		}
		printer.Flush()
	}
}

// Benchmark: Output writer types

func BenchmarkPrintString_WriterTypes(b *testing.B) {
	str := []byte("Hello, World!")
	config := extractor.Config{MinLength: 4}

	b.Run("Discard", func(b *testing.B) {
		writer := io.Discard
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			PrintStringToWriter(writer, str, "", 0, config)
		}
	})

	b.Run("Buffer", func(b *testing.B) {
		var buf bytes.Buffer
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf.Reset()
			PrintStringToWriter(&buf, str, "", 0, config)
		}
	})
}

// Benchmark: Color functions

func BenchmarkShouldUseColor_Never(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldUseColor(extractor.ColorNever)
	}
}

func BenchmarkShouldUseColor_Always(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldUseColor(extractor.ColorAlways)
	}
}

func BenchmarkShouldUseColor_Auto(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ShouldUseColor(extractor.ColorAuto)
	}
}

func BenchmarkColorString(b *testing.B) {
	str := "Hello, World!"
	color := AnsiCyan

	b.Run("WithColor", func(b *testing.B) {
		useColor := true
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ColorString(str, color, useColor)
		}
	})

	b.Run("WithoutColor", func(b *testing.B) {
		useColor := false
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ColorString(str, color, useColor)
		}
	})
}

// Benchmark: Configuration comparison

func BenchmarkPrintString_ConfigComparison(b *testing.B) {
	str := []byte("Hello, World!")
	filename := "test.bin"
	offset := int64(1024)
	writer := io.Discard

	configs := map[string]extractor.Config{
		"Minimal": {
			MinLength: 4,
		},
		"WithFilename": {
			MinLength:     4,
			PrintFileName: true,
		},
		"WithOffset": {
			MinLength:   4,
			PrintOffset: true,
			Radix:       "x",
		},
		"WithColor": {
			MinLength: 4,
			ColorMode: extractor.ColorAlways,
			Encoding:  "s",
		},
		"WithAll": {
			MinLength:     4,
			PrintFileName: true,
			PrintOffset:   true,
			Radix:         "x",
			ColorMode:     extractor.ColorAlways,
			Encoding:      "s",
		},
	}

	for name, config := range configs {
		b.Run(name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				PrintStringToWriter(writer, str, filename, offset, config)
			}
		})
	}
}

// Helper function to format length
func formatLength(length int) string {
	if length >= 1024 {
		return "1KB"
	}
	return "Short"
}
