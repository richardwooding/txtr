package extractor

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// createBenchmarkFile creates a test file with the specified size containing printable strings
func createBenchmarkFile(t testing.TB, size int) string {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "benchmark.bin")

	// Create content with mix of strings and non-printable bytes
	var buf bytes.Buffer
	stringPattern := []byte("BenchmarkString123")
	nonPrintablePattern := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xFF}

	for buf.Len() < size {
		buf.Write(stringPattern)
		buf.Write(nonPrintablePattern)
	}

	data := buf.Bytes()[:size]
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to create benchmark file: %v", err)
	}

	return testFile
}

// Benchmark mmap vs buffered I/O for 1MB file
func BenchmarkExtract_1MB_BufferedIO(b *testing.B) {
	testFile := createBenchmarkFile(b, 1*1024*1024)
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   true, // Force buffered I/O
		MmapThreshold: 100 * 1024 * 1024,
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

func BenchmarkExtract_1MB_Mmap(b *testing.B) {
	testFile := createBenchmarkFile(b, 1*1024*1024)
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

// Benchmark mmap vs buffered I/O for 10MB file
func BenchmarkExtract_10MB_BufferedIO(b *testing.B) {
	testFile := createBenchmarkFile(b, 10*1024*1024)
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   true, // Force buffered I/O
		MmapThreshold: 100 * 1024 * 1024,
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

func BenchmarkExtract_10MB_Mmap(b *testing.B) {
	testFile := createBenchmarkFile(b, 10*1024*1024)
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

// Benchmark mmap vs buffered I/O for 100MB file
func BenchmarkExtract_100MB_BufferedIO(b *testing.B) {
	testFile := createBenchmarkFile(b, 100*1024*1024)
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   true, // Force buffered I/O
		MmapThreshold: 1000 * 1024 * 1024,
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

func BenchmarkExtract_100MB_Mmap(b *testing.B) {
	testFile := createBenchmarkFile(b, 100*1024*1024)
	config := Config{
		MinLength:     4,
		Encoding:      "s",
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

// Benchmark UTF-16 extraction with mmap
func BenchmarkExtractUTF16_10MB_BufferedIO(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "utf16_bench.bin")

	// Create UTF-16LE test data
	var buf bytes.Buffer
	for buf.Len() < 10*1024*1024 {
		// "Test" in UTF-16LE
		buf.Write([]byte{0x54, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, 0x00})
		// null
		buf.Write([]byte{0x00, 0x00})
	}

	if err := os.WriteFile(testFile, buf.Bytes(), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "l", // UTF-16LE
		DisableMmap:   true,
		MmapThreshold: 100 * 1024 * 1024,
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

func BenchmarkExtractUTF16_10MB_Mmap(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "utf16_bench.bin")

	// Create UTF-16LE test data
	var buf bytes.Buffer
	for buf.Len() < 10*1024*1024 {
		// "Test" in UTF-16LE
		buf.Write([]byte{0x54, 0x00, 0x65, 0x00, 0x73, 0x00, 0x74, 0x00})
		// null
		buf.Write([]byte{0x00, 0x00})
	}

	if err := os.WriteFile(testFile, buf.Bytes(), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "l", // UTF-16LE
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

// Benchmark 8-bit ASCII extraction
func BenchmarkExtract8BitASCII_10MB_BufferedIO(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "8bit_bench.bin")

	// Create data with high-byte characters
	var buf bytes.Buffer
	for buf.Len() < 10*1024*1024 {
		buf.WriteString("Test\x80\x81\x82\xFF")
		buf.WriteByte(0x00)
	}

	if err := os.WriteFile(testFile, buf.Bytes(), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "S", // 8-bit ASCII
		DisableMmap:   true,
		MmapThreshold: 100 * 1024 * 1024,
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}

func BenchmarkExtract8BitASCII_10MB_Mmap(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "8bit_bench.bin")

	// Create data with high-byte characters
	var buf bytes.Buffer
	for buf.Len() < 10*1024*1024 {
		buf.WriteString("Test\x80\x81\x82\xFF")
		buf.WriteByte(0x00)
	}

	if err := os.WriteFile(testFile, buf.Bytes(), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	config := Config{
		MinLength:     4,
		Encoding:      "S", // 8-bit ASCII
		DisableMmap:   false,
		MmapThreshold: 1, // Force mmap
	}

	printFunc := func(_ []byte, _ string, _ int64, _ Config) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractStringsFromFile(testFile, config, printFunc)
	}
}
