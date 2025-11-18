package binary

import (
	"os"
	"path/filepath"
	"testing"
)

// Test data generation helpers

// createELFBenchmarkFile creates a minimal valid ELF file for benchmarking
func createELFBenchmarkFile(t testing.TB) string {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.elf")

	// Minimal ELF header (64-bit, little-endian)
	elfData := []byte{
		// ELF Magic
		0x7f, 0x45, 0x4c, 0x46, // Magic: 0x7f, 'E', 'L', 'F'
		0x02,                   // Class: 64-bit
		0x01,                   // Data: Little-endian
		0x01,                   // Version: Current
		0x00,                   // OS/ABI: System V
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Padding

		// ELF Header
		0x02, 0x00, // Type: Executable
		0x3e, 0x00, // Machine: x86-64
		0x01, 0x00, 0x00, 0x00, // Version
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Entry point
		0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Program header offset
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Section header offset
		0x00, 0x00, 0x00, 0x00, // Flags
		0x40, 0x00, // ELF header size
		0x38, 0x00, // Program header size
		0x00, 0x00, // Program header count
		0x40, 0x00, // Section header size
		0x00, 0x00, // Section header count
		0x00, 0x00, // Section name string table index
	}

	if err := os.WriteFile(path, elfData, 0644); err != nil {
		t.Fatalf("Failed to create ELF file: %v", err)
	}

	return path
}

// createPEBenchmarkFile creates a minimal valid PE file for benchmarking
func createPEBenchmarkFile(t testing.TB) string {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.exe")

	// Minimal PE file structure
	peData := []byte{
		// DOS Header
		0x4d, 0x5a, // Magic: 'M', 'Z'
		0x90, 0x00, // Bytes on last page
		0x03, 0x00, // Pages in file
		0x00, 0x00, // Relocations
		0x04, 0x00, // Size of header in paragraphs
		0x00, 0x00, // Minimum extra paragraphs
		0xff, 0xff, // Maximum extra paragraphs
		0x00, 0x00, // Initial SS
		0xb8, 0x00, // Initial SP
		0x00, 0x00, // Checksum
		0x00, 0x00, // Initial IP
		0x00, 0x00, // Initial CS
		0x40, 0x00, // File address of relocation table
		0x00, 0x00, // Overlay number
		0x00, 0x00, 0x00, 0x00, // Reserved
		0x00, 0x00, 0x00, 0x00, // Reserved
		0x00, 0x00, 0x00, 0x00, // Reserved
		0x00, 0x00, 0x00, 0x00, // Reserved
		0x00, 0x00, // OEM identifier
		0x00, 0x00, // OEM information
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Reserved
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Reserved
		0x80, 0x00, 0x00, 0x00, // PE header offset (at 0x80)
	}

	// Pad to PE header offset
	for len(peData) < 0x80 {
		peData = append(peData, 0x00)
	}

	// PE Signature
	peData = append(peData, []byte{
		0x50, 0x45, 0x00, 0x00, // 'P', 'E', 0, 0
	}...)

	if err := os.WriteFile(path, peData, 0644); err != nil {
		t.Fatalf("Failed to create PE file: %v", err)
	}

	return path
}

// createMachOBenchmarkFile creates a minimal valid Mach-O file for benchmarking
func createMachOBenchmarkFile(t testing.TB) string {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.macho")

	// Minimal Mach-O header (64-bit)
	machoData := []byte{
		// Mach-O Magic (64-bit)
		0xcf, 0xfa, 0xed, 0xfe, // Magic
		0x07, 0x00, 0x00, 0x01, // CPU type: x86_64
		0x03, 0x00, 0x00, 0x00, // CPU subtype
		0x02, 0x00, 0x00, 0x00, // File type: Executable
		0x00, 0x00, 0x00, 0x00, // Number of load commands
		0x00, 0x00, 0x00, 0x00, // Size of load commands
		0x00, 0x00, 0x00, 0x00, // Flags
		0x00, 0x00, 0x00, 0x00, // Reserved
	}

	if err := os.WriteFile(path, machoData, 0644); err != nil {
		t.Fatalf("Failed to create Mach-O file: %v", err)
	}

	return path
}

// createRandomBinaryFile creates a file with random data (no specific format)
func createRandomBinaryFile(t testing.TB, size int) string {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "random.bin")

	// Create pseudo-random data
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to create random binary file: %v", err)
	}

	return path
}

// Benchmark: Format detection

func BenchmarkDetectFormat_ELF(b *testing.B) {
	path := createELFBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DetectFormat(path)
	}
}

func BenchmarkDetectFormat_PE(b *testing.B) {
	path := createPEBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DetectFormat(path)
	}
}

func BenchmarkDetectFormat_MachO(b *testing.B) {
	path := createMachOBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DetectFormat(path)
	}
}

func BenchmarkDetectFormat_Unknown(b *testing.B) {
	path := createRandomBinaryFile(b, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DetectFormat(path)
	}
}

// Benchmark: ELF parsing

func BenchmarkParseELF(b *testing.B) {
	path := createELFBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseELF(path)
	}
}

// Benchmark: PE parsing

func BenchmarkParsePE(b *testing.B) {
	path := createPEBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParsePE(path)
	}
}

// Benchmark: Mach-O parsing

func BenchmarkParseMachO(b *testing.B) {
	path := createMachOBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseMachO(path)
	}
}

// Benchmark: Full binary parsing pipeline

func BenchmarkParseBinary_ELF(b *testing.B) {
	path := createELFBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseBinary(path, FormatELF)
	}
}

func BenchmarkParseBinary_PE(b *testing.B) {
	path := createPEBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseBinary(path, FormatPE)
	}
}

func BenchmarkParseBinary_MachO(b *testing.B) {
	path := createMachOBenchmarkFile(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseBinary(path, FormatMachO)
	}
}

func BenchmarkParseBinary_Unknown(b *testing.B) {
	path := createRandomBinaryFile(b, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseBinary(path, FormatRaw)
	}
}

// Benchmark: File size impact on parsing

func BenchmarkParseBinary_FileSize(b *testing.B) {
	sizes := []int{
		1 * 1024,       // 1KB
		10 * 1024,      // 10KB
		100 * 1024,     // 100KB
		1024 * 1024,    // 1MB
		10 * 1024 * 1024, // 10MB
	}

	for _, size := range sizes {
		b.Run(formatBytes(size), func(b *testing.B) {
			path := createRandomBinaryFile(b, size)
			b.SetBytes(int64(size))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = ParseBinary(path, FormatRaw)
			}
		})
	}
}

// Benchmark: Format comparison

func BenchmarkFormatComparison(b *testing.B) {
	testCases := []struct {
		name       string
		createFunc func(testing.TB) string
		format     Format
	}{
		{"ELF", createELFBenchmarkFile, FormatELF},
		{"PE", createPEBenchmarkFile, FormatPE},
		{"MachO", createMachOBenchmarkFile, FormatMachO},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			path := tc.createFunc(b)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = ParseBinary(path, tc.format)
			}
		})
	}
}

// Helper function to format byte sizes
func formatBytes(bytes int) string {
	if bytes >= 1024*1024 {
		return "10MB"
	} else if bytes >= 1024 {
		kb := bytes / 1024
		if kb >= 1000 {
			return "1MB"
		}
		return "100KB"
	}
	return "1KB"
}
