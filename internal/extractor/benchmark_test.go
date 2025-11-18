package extractor

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Test data generation helpers

// createASCIIBenchmarkData generates realistic ASCII test data with mixed printable and non-printable bytes
func createASCIIBenchmarkData(size int) []byte {
	data := make([]byte, 0, size)
	// Realistic pattern: strings interspersed with binary data
	pattern := []byte("BenchmarkString123")
	separator := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xFF}

	for len(data) < size {
		data = append(data, pattern...)
		data = append(data, separator...)
	}

	return data[:size]
}

// create8BitASCIIBenchmarkData generates test data with high-byte (8-bit) ASCII characters
func create8BitASCIIBenchmarkData(size int) []byte {
	data := make([]byte, 0, size)
	// Mix of 7-bit and 8-bit printable characters
	pattern := []byte("Standard ASCII ")
	highBytes := []byte{0x80, 0x81, 0x82, 0x83, 0x84, 0xFF}
	separator := []byte{0x00, 0x01}

	for len(data) < size {
		data = append(data, pattern...)
		data = append(data, highBytes...)
		data = append(data, separator...)
	}

	return data[:size]
}

// createUTF8BenchmarkData generates realistic UTF-8 test data with various Unicode characters
func createUTF8BenchmarkData(size int) []byte {
	data := make([]byte, 0, size)
	// Mix of ASCII, 2-byte, 3-byte, and 4-byte UTF-8 sequences
	patterns := []string{
		"Hello World ",           // ASCII
		"Привет мир ",            // 2-byte UTF-8 (Cyrillic)
		"你好世界 ",                 // 3-byte UTF-8 (Chinese)
		"Hello 🌍 ",              // 4-byte UTF-8 (emoji)
		"Résumé café ",           // Accented characters
		"日本語テキスト ",              // Japanese
	}
	separator := []byte{0x00, 0xFF}

	patternIdx := 0
	for len(data) < size {
		data = append(data, []byte(patterns[patternIdx])...)
		data = append(data, separator...)
		patternIdx = (patternIdx + 1) % len(patterns)
	}

	return data[:size]
}

// createUTF16BenchmarkData generates UTF-16 encoded test data
func createUTF16BenchmarkData(size int, littleEndian bool) []byte {
	data := make([]byte, 0, size)
	// UTF-16 encoding of "Test String 测试"
	var pattern []byte
	if littleEndian {
		// UTF-16LE
		pattern = []byte{
			0x54, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, 0x00, // "Test"
			0x20, 0x00, // " "
			0x53, 0x00, 0x74, 0x00, 0x72, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x67, 0x00, // "String"
			0x20, 0x00, // " "
			0x4B, 0x6D, 0xD5, 0x8B, // "测试" (Chinese)
		}
	} else {
		// UTF-16BE
		pattern = []byte{
			0x00, 0x54, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, // "Test"
			0x00, 0x20, // " "
			0x00, 0x53, 0x00, 0x74, 0x00, 0x72, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x67, // "String"
			0x00, 0x20, // " "
			0x6D, 0x4B, 0x8B, 0xD5, // "测试" (Chinese)
		}
	}
	separator := []byte{0x00, 0x00, 0xFF, 0xFF}

	for len(data) < size {
		data = append(data, pattern...)
		data = append(data, separator...)
	}

	return data[:size]
}

// createUTF32BenchmarkData generates UTF-32 encoded test data
func createUTF32BenchmarkData(size int, littleEndian bool) []byte {
	data := make([]byte, 0, size)
	// UTF-32 encoding of "Test 😀" (includes emoji)
	var pattern []byte
	if littleEndian {
		// UTF-32LE
		pattern = []byte{
			0x54, 0x00, 0x00, 0x00, // 'T'
			0x65, 0x00, 0x00, 0x00, // 'e'
			0x73, 0x00, 0x00, 0x00, // 's'
			0x74, 0x00, 0x00, 0x00, // 't'
			0x20, 0x00, 0x00, 0x00, // ' '
			0x00, 0xF6, 0x01, 0x00, // '😀' (U+1F600)
		}
	} else {
		// UTF-32BE
		pattern = []byte{
			0x00, 0x00, 0x00, 0x54, // 'T'
			0x00, 0x00, 0x00, 0x65, // 'e'
			0x00, 0x00, 0x00, 0x73, // 's'
			0x00, 0x00, 0x00, 0x74, // 't'
			0x00, 0x00, 0x00, 0x20, // ' '
			0x00, 0x01, 0xF6, 0x00, // '😀' (U+1F600)
		}
	}
	separator := []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF}

	for len(data) < size {
		data = append(data, pattern...)
		data = append(data, separator...)
	}

	return data[:size]
}

// Benchmark: ASCII extraction at various file sizes

func BenchmarkExtractASCII_1KB(b *testing.B) {
	benchmarkExtractASCII(b, 1*1024)
}

func BenchmarkExtractASCII_10KB(b *testing.B) {
	benchmarkExtractASCII(b, 10*1024)
}

func BenchmarkExtractASCII_100KB(b *testing.B) {
	benchmarkExtractASCII(b, 100*1024)
}

func BenchmarkExtractASCII_1MB(b *testing.B) {
	benchmarkExtractASCII(b, 1*1024*1024)
}

func BenchmarkExtractASCII_10MB(b *testing.B) {
	benchmarkExtractASCII(b, 10*1024*1024)
}

func BenchmarkExtractASCII_100MB(b *testing.B) {
	benchmarkExtractASCII(b, 100*1024*1024)
}

func benchmarkExtractASCII(b *testing.B, size int) {
	data := createASCIIBenchmarkData(size)
	config := Config{
		MinLength: 4,
		Encoding:  "s",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(size))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	// Calculate and report throughput
	throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: 8-bit ASCII extraction

func BenchmarkExtract8BitASCII_1MB(b *testing.B) {
	benchmarkExtract8BitASCII(b, 1*1024*1024)
}

func BenchmarkExtract8BitASCII_10MB(b *testing.B) {
	benchmarkExtract8BitASCII(b, 10*1024*1024)
}

func BenchmarkExtract8BitASCII_100MB(b *testing.B) {
	benchmarkExtract8BitASCII(b, 100*1024*1024)
}

func benchmarkExtract8BitASCII(b *testing.B, size int) {
	data := create8BitASCIIBenchmarkData(size)
	config := Config{
		MinLength: 4,
		Encoding:  "S",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(size))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: UTF-8 aware extraction with different display modes

func BenchmarkExtractUTF8_1MB(b *testing.B) {
	benchmarkExtractUTF8(b, 1*1024*1024)
}

func BenchmarkExtractUTF8_10MB(b *testing.B) {
	benchmarkExtractUTF8(b, 10*1024*1024)
}

func BenchmarkExtractUTF8_100MB(b *testing.B) {
	benchmarkExtractUTF8(b, 100*1024*1024)
}

func benchmarkExtractUTF8(b *testing.B, size int) {
	data := createUTF8BenchmarkData(size)
	modes := []string{"default", "locale", "escape", "hex"}

	for _, mode := range modes {
		b.Run(mode, func(b *testing.B) {
			config := Config{
				MinLength: 4,
				Encoding:  "s",
				Unicode:   mode,
			}
			printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				reader := bytes.NewReader(data)
				extractUTF8Aware(reader, "", config, printFunc)
			}

			throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
			b.ReportMetric(throughput, "MB/s")
		})
	}
}

// Benchmark: UTF-16 extraction

func BenchmarkExtractUTF16LE_1MB(b *testing.B) {
	benchmarkExtractUTF16(b, 1*1024*1024, true)
}

func BenchmarkExtractUTF16LE_10MB(b *testing.B) {
	benchmarkExtractUTF16(b, 10*1024*1024, true)
}

func BenchmarkExtractUTF16LE_100MB(b *testing.B) {
	benchmarkExtractUTF16(b, 100*1024*1024, true)
}

func BenchmarkExtractUTF16BE_1MB(b *testing.B) {
	benchmarkExtractUTF16(b, 1*1024*1024, false)
}

func BenchmarkExtractUTF16BE_10MB(b *testing.B) {
	benchmarkExtractUTF16(b, 10*1024*1024, false)
}

func BenchmarkExtractUTF16BE_100MB(b *testing.B) {
	benchmarkExtractUTF16(b, 100*1024*1024, false)
}

func benchmarkExtractUTF16(b *testing.B, size int, littleEndian bool) {
	data := createUTF16BenchmarkData(size, littleEndian)
	encoding := "l"
	var byteOrder binary.ByteOrder = binary.LittleEndian
	if !littleEndian {
		encoding = "b"
		byteOrder = binary.BigEndian
	}
	config := Config{
		MinLength: 4,
		Encoding:  encoding,
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(size))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractUTF16(reader, "", config, printFunc, byteOrder)
	}

	throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: UTF-32 extraction

func BenchmarkExtractUTF32LE_1MB(b *testing.B) {
	benchmarkExtractUTF32(b, 1*1024*1024, true)
}

func BenchmarkExtractUTF32LE_10MB(b *testing.B) {
	benchmarkExtractUTF32(b, 10*1024*1024, true)
}

func BenchmarkExtractUTF32LE_100MB(b *testing.B) {
	benchmarkExtractUTF32(b, 100*1024*1024, true)
}

func BenchmarkExtractUTF32BE_1MB(b *testing.B) {
	benchmarkExtractUTF32(b, 1*1024*1024, false)
}

func BenchmarkExtractUTF32BE_10MB(b *testing.B) {
	benchmarkExtractUTF32(b, 10*1024*1024, false)
}

func BenchmarkExtractUTF32BE_100MB(b *testing.B) {
	benchmarkExtractUTF32(b, 100*1024*1024, false)
}

func benchmarkExtractUTF32(b *testing.B, size int, littleEndian bool) {
	data := createUTF32BenchmarkData(size, littleEndian)
	encoding := "L"
	var byteOrder binary.ByteOrder = binary.LittleEndian
	if !littleEndian {
		encoding = "B"
		byteOrder = binary.BigEndian
	}
	config := Config{
		MinLength: 4,
		Encoding:  encoding,
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(size))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractUTF32(reader, "", config, printFunc, byteOrder)
	}

	throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: String density impact

func BenchmarkExtractASCII_SparseDensity(b *testing.B) {
	// Sparse: 10% strings, 90% binary
	data := make([]byte, 0, 1024*1024)
	pattern := []byte("String")
	separator := make([]byte, 54) // 6 bytes string + 54 bytes binary = 10% density

	for len(data) < 1024*1024 {
		data = append(data, pattern...)
		data = append(data, separator...)
	}

	benchmarkWithData(b, data[:1024*1024])
}

func BenchmarkExtractASCII_DenseDensity(b *testing.B) {
	// Dense: 90% strings, 10% binary
	data := make([]byte, 0, 1024*1024)
	pattern := []byte("StringStringStringStringString") // 54 bytes
	separator := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05} // 6 bytes

	for len(data) < 1024*1024 {
		data = append(data, pattern...)
		data = append(data, separator...)
	}

	benchmarkWithData(b, data[:1024*1024])
}

func benchmarkWithData(b *testing.B, data []byte) {
	config := Config{
		MinLength: 4,
		Encoding:  "s",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(len(data)) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: Minimum length impact

func BenchmarkExtractASCII_MinLength4(b *testing.B) {
	benchmarkExtractASCIIWithMinLength(b, 4)
}

func BenchmarkExtractASCII_MinLength8(b *testing.B) {
	benchmarkExtractASCIIWithMinLength(b, 8)
}

func BenchmarkExtractASCII_MinLength16(b *testing.B) {
	benchmarkExtractASCIIWithMinLength(b, 16)
}

func BenchmarkExtractASCII_MinLength32(b *testing.B) {
	benchmarkExtractASCIIWithMinLength(b, 32)
}

func benchmarkExtractASCIIWithMinLength(b *testing.B, minLength int) {
	data := createASCIIBenchmarkData(1 * 1024 * 1024)
	config := Config{
		MinLength: minLength,
		Encoding:  "s",
	}
	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		extractASCII(reader, "", config, printFunc, false)
	}

	throughput := float64(len(data)) * float64(b.N) / b.Elapsed().Seconds() / 1e6
	b.ReportMetric(throughput, "MB/s")
}

// Benchmark: Encoding comparison at same file size

func BenchmarkEncodingComparison_10MB(b *testing.B) {
	size := 10 * 1024 * 1024

	b.Run("ASCII", func(b *testing.B) {
		data := createASCIIBenchmarkData(size)
		config := Config{MinLength: 4, Encoding: "s"}
		printFunc := func(_ []byte, _ string, _ int64, _ Config) {}
		b.SetBytes(int64(size))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			extractASCII(reader, "", config, printFunc, false)
		}
		throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
		b.ReportMetric(throughput, "MB/s")
	})

	b.Run("8BitASCII", func(b *testing.B) {
		data := create8BitASCIIBenchmarkData(size)
		config := Config{MinLength: 4, Encoding: "S"}
		printFunc := func(_ []byte, _ string, _ int64, _ Config) {}
		b.SetBytes(int64(size))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			extractASCII(reader, "", config, printFunc, false)
		}
		throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
		b.ReportMetric(throughput, "MB/s")
	})

	b.Run("UTF8", func(b *testing.B) {
		data := createUTF8BenchmarkData(size)
		config := Config{MinLength: 4, Encoding: "s", Unicode: "locale"}
		printFunc := func(_ []byte, _ string, _ int64, _ Config) {}
		b.SetBytes(int64(size))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			extractUTF8Aware(reader, "", config, printFunc)
		}
		throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
		b.ReportMetric(throughput, "MB/s")
	})

	b.Run("UTF16LE", func(b *testing.B) {
		data := createUTF16BenchmarkData(size, true)
		config := Config{MinLength: 4, Encoding: "l"}
		printFunc := func(_ []byte, _ string, _ int64, _ Config) {}
		b.SetBytes(int64(size))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			extractUTF16(reader, "", config, printFunc, binary.LittleEndian)
		}
		throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
		b.ReportMetric(throughput, "MB/s")
	})

	b.Run("UTF32LE", func(b *testing.B) {
		data := createUTF32BenchmarkData(size, true)
		config := Config{MinLength: 4, Encoding: "L"}
		printFunc := func(_ []byte, _ string, _ int64, _ Config) {}
		b.SetBytes(int64(size))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := bytes.NewReader(data)
			extractUTF32(reader, "", config, printFunc, binary.LittleEndian)
		}
		throughput := float64(size) * float64(b.N) / b.Elapsed().Seconds() / 1e6
		b.ReportMetric(throughput, "MB/s")
	})
}
