package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
	"github.com/richardwooding/txtr/internal/printer"
)

// TestParallelProcessingOrder tests that parallel processing maintains file order
func TestParallelProcessingOrder(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()

	files := []struct {
		name    string
		content string
	}{
		{"file1.bin", "File1String\x00\x00"},
		{"file2.bin", "File2String\x00\x00"},
		{"file3.bin", "File3String\x00\x00"},
		{"file4.bin", "File4String\x00\x00"},
	}

	var filePaths []string
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f.name, err)
		}
		filePaths = append(filePaths, path)
	}

	// Test with parallel processing
	config := extractor.Config{
		MinLength:       4,
		PrintFileName:   true,
		Encoding:        "s",
		OutputSeparator: "\n",
	}

	// Capture output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Run parallel processing
	processFilesParallel(filePaths, 2, config)

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}
	output := buf.String()

	// Verify order: files should appear in the same order as input
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// Check that each file appears in order
	for i, line := range lines {
		expectedFile := filepath.Join(tmpDir, files[i].name)
		if !strings.HasPrefix(line, expectedFile+":") {
			t.Errorf("Line %d: expected file %s, got: %s", i+1, expectedFile, line)
		}
	}
}

// TestParallelProcessingErrorHandling tests that errors in one file don't stop processing others
func TestParallelProcessingErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some valid files and reference a nonexistent file
	file1 := filepath.Join(tmpDir, "file1.bin")
	file2 := filepath.Join(tmpDir, "nonexistent.bin")
	file3 := filepath.Join(tmpDir, "file3.bin")

	if err := os.WriteFile(file1, []byte("TestString1\x00"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file3, []byte("TestString3\x00"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := extractor.Config{
		MinLength:       4,
		PrintFileName:   true,
		Encoding:        "s",
		OutputSeparator: "\n",
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stderr = w

	// Run parallel processing
	processFilesParallel([]string{file1, file2, file3}, 2, config)

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe writer: %v", err)
	}
	os.Stderr = oldStderr
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}
	errOutput := buf.String()

	// Verify that error was reported for nonexistent file
	if !strings.Contains(errOutput, "nonexistent.bin") {
		t.Errorf("Expected error message for nonexistent.bin, got: %s", errOutput)
	}
}

// TestSequentialVsParallel tests that sequential and parallel modes produce the same output
func TestSequentialVsParallel(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []struct {
		name    string
		content string
	}{
		{"a.bin", "StringA\x00\x01\x02AnotherA\x00"},
		{"b.bin", "StringB\x00\x01\x02AnotherB\x00"},
	}

	var filePaths []string
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f.name, err)
		}
		filePaths = append(filePaths, path)
	}

	config := extractor.Config{
		MinLength:       4,
		PrintFileName:   true,
		Encoding:        "s",
		OutputSeparator: "\n",
	}

	// Run sequential processing
	var seqBuf bytes.Buffer
	for _, filename := range filePaths {
		file, err := os.Open(filename)
		if err != nil {
			t.Fatalf("Failed to open file %s: %v", filename, err)
		}
		extractor.ExtractStrings(file, filename, config, func(str []byte, fname string, _ int64, cfg extractor.Config) {
			if cfg.PrintFileName && fname != "" {
				seqBuf.WriteString(fname + ": ")
			}
			seqBuf.Write(str)
			seqBuf.WriteString("\n")
		})
		if err := file.Close(); err != nil {
			t.Fatalf("Failed to close file: %v", err)
		}
	}

	// Run parallel processing
	var parBuf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w

	processFilesParallel(filePaths, 2, config)

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout
	if _, err := parBuf.ReadFrom(r); err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}

	// Both outputs should be identical
	if seqBuf.String() != parBuf.String() {
		t.Errorf("Sequential and parallel outputs differ:\nSequential:\n%s\nParallel:\n%s",
			seqBuf.String(), parBuf.String())
	}
}

// TestJSONMultipleFiles tests JSON output with multiple files
func TestJSONMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []struct {
		name    string
		content string
	}{
		{"file1.bin", "TestString1\x00\x00"},
		{"file2.bin", "TestString2\x00\x00"},
		{"file3.bin", "TestString3\x00\x00"},
	}

	var filePaths []string
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f.name, err)
		}
		filePaths = append(filePaths, path)
	}

	config := extractor.Config{
		MinLength:       4,
		PrintFileName:   false,
		Encoding:        "s",
		OutputSeparator: "\n",
	}

	// Process files with JSON output
	jsonPrinter := processFilesParallelJSON(filePaths, 2, config)

	// Verify we have 3 file results
	if len(jsonPrinter.FileResults) != 3 {
		t.Errorf("Expected 3 file results, got %d", len(jsonPrinter.FileResults))
	}

	// Verify files are in order
	for i, fileRes := range jsonPrinter.FileResults {
		expectedPath := filePaths[i]
		if fileRes.File != expectedPath {
			t.Errorf("File %d: expected %s, got %s", i, expectedPath, fileRes.File)
		}

		// Verify each file has strings
		if len(fileRes.Strings) == 0 {
			t.Errorf("File %d (%s): expected strings, got none", i, fileRes.File)
		}

		// Verify no errors
		if fileRes.Error != "" {
			t.Errorf("File %d (%s): unexpected error: %s", i, fileRes.File, fileRes.Error)
		}
	}
}

// TestJSONWithErrors tests JSON output with some files failing
func TestJSONWithErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	file1 := filepath.Join(tmpDir, "good1.bin")
	file2 := filepath.Join(tmpDir, "nonexistent.bin") // This one doesn't exist
	file3 := filepath.Join(tmpDir, "good2.bin")

	if err := os.WriteFile(file1, []byte("GoodString1\x00"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file3, []byte("GoodString2\x00"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := extractor.Config{
		MinLength:       4,
		Encoding:        "s",
		OutputSeparator: "\n",
	}

	// Process files (including nonexistent one)
	jsonPrinter := processFilesParallelJSON([]string{file1, file2, file3}, 2, config)

	// Verify we have 3 file results
	if len(jsonPrinter.FileResults) != 3 {
		t.Errorf("Expected 3 file results, got %d", len(jsonPrinter.FileResults))
	}

	// Verify first file succeeded
	if jsonPrinter.FileResults[0].Error != "" {
		t.Errorf("File 1: unexpected error: %s", jsonPrinter.FileResults[0].Error)
	}
	if len(jsonPrinter.FileResults[0].Strings) == 0 {
		t.Error("File 1: expected strings, got none")
	}

	// Verify second file failed
	if jsonPrinter.FileResults[1].Error == "" {
		t.Error("File 2: expected error, got none")
	}
	if !strings.Contains(jsonPrinter.FileResults[1].Error, "no such file") {
		t.Errorf("File 2: expected 'no such file' error, got: %s", jsonPrinter.FileResults[1].Error)
	}
	if len(jsonPrinter.FileResults[1].Strings) != 0 {
		t.Errorf("File 2: expected empty strings array, got %d strings", len(jsonPrinter.FileResults[1].Strings))
	}

	// Verify third file succeeded
	if jsonPrinter.FileResults[2].Error != "" {
		t.Errorf("File 3: unexpected error: %s", jsonPrinter.FileResults[2].Error)
	}
	if len(jsonPrinter.FileResults[2].Strings) == 0 {
		t.Error("File 3: expected strings, got none")
	}
}

// TestJSONOutputStructure tests that JSON output is valid and well-formed
func TestJSONOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.bin")
	if err := os.WriteFile(testFile, []byte("TestString\x00\x00More"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := extractor.Config{
		MinLength:       4,
		Encoding:        "s",
		OutputSeparator: "\n",
	}

	// Process file
	var buf bytes.Buffer
	tempPrinter := printer.NewJSONPrinter(config, &buf)
	tempPrinter.SetFileInfo(testFile, "", nil)

	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("Failed to close file: %v", err)
		}
	}()

	extractor.ExtractStrings(file, testFile, config, tempPrinter.PrintString)

	if err := tempPrinter.Flush(); err != nil {
		t.Fatalf("Failed to flush JSON: %v", err)
	}

	// Parse JSON to verify it's valid
	var output printer.JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Invalid JSON output: %v\nOutput:\n%s", err, buf.String())
	}

	// Verify structure
	if len(output.Files) != 1 {
		t.Errorf("Expected 1 file in output, got %d", len(output.Files))
	}

	if output.Summary.TotalStrings == 0 {
		t.Error("Expected summary with total strings")
	}

	if output.Summary.MinLength != 4 {
		t.Errorf("Expected min_length=4, got %d", output.Summary.MinLength)
	}
}
