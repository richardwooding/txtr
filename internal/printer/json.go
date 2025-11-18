// Package printer provides functions for formatting and printing extracted strings
// with optional filename and offset prefixes.
package printer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/richardwooding/txtr/internal/extractor"
)

// StringResult represents a single extracted string in JSON format
type StringResult struct {
	File      string `json:"file,omitempty"`
	Value     string `json:"value"`
	Offset    int64  `json:"offset"`
	OffsetHex string `json:"offset_hex"`
	Length    int    `json:"length"`
	Encoding  string `json:"encoding"`
	Section   string `json:"section,omitempty"`
}

// JSONOutput represents the complete JSON output structure
type JSONOutput struct {
	Files   []FileResult `json:"files"`
	Summary Summary      `json:"summary"`
}

// FileResult represents results for a single file
type FileResult struct {
	File     string         `json:"file,omitempty"`
	Format   string         `json:"format,omitempty"`
	Sections []string       `json:"sections,omitempty"`
	Strings  []StringResult `json:"strings"`
	Error    string         `json:"error,omitempty"`
}

// Summary contains metadata about the extraction
type Summary struct {
	TotalStrings int   `json:"total_strings"`
	TotalBytes   int64 `json:"total_bytes"`
	MinLength    int   `json:"min_length"`
	Encoding     string `json:"encoding"`
}

// JSONPrinter collects and outputs strings in JSON format
type JSONPrinter struct {
	FileResults []FileResult // Exported for access in parallel processing
	config      extractor.Config
	writer      io.Writer
	// Current file being processed
	currentFile    string
	currentFormat  string
	currentSections []string
	currentStrings  []StringResult
}

// NewJSONPrinter creates a new JSON printer
func NewJSONPrinter(config extractor.Config, writer io.Writer) *JSONPrinter {
	if writer == nil {
		writer = os.Stdout
	}
	return &JSONPrinter{
		FileResults:    make([]FileResult, 0),
		currentStrings: make([]StringResult, 0),
		config:         config,
		writer:         writer,
	}
}

// SetFileInfo sets the current file and format information
// If there's a current file being processed, it finalizes it first
func (jp *JSONPrinter) SetFileInfo(filename, format string, sections []string) {
	// Finalize previous file if exists
	if jp.currentFile != "" || len(jp.currentStrings) > 0 {
		jp.FinalizeCurrentFile()
	}

	// Start new file
	jp.currentFile = filename
	jp.currentFormat = format
	jp.currentSections = sections
	jp.currentStrings = make([]StringResult, 0)
}

// PrintString collects a string result (implements the printFunc signature)
func (jp *JSONPrinter) PrintString(str []byte, filename string, offset int64, config extractor.Config) {
	result := StringResult{
		Value:     string(str),
		Offset:    offset,
		OffsetHex: fmt.Sprintf("0x%x", offset),
		Length:    len(str),
		Encoding:  getEncodingName(config.Encoding),
	}

	// Only include filename if PrintFileName is enabled or it's different from stdin
	if config.PrintFileName && filename != "" {
		result.File = filename
	}

	jp.currentStrings = append(jp.currentStrings, result)
}

// FinalizeCurrentFile adds the current file's results to the fileResults list
func (jp *JSONPrinter) FinalizeCurrentFile() {
	fileResult := FileResult{
		File:     jp.currentFile,
		Format:   jp.currentFormat,
		Sections: jp.currentSections,
		Strings:  jp.currentStrings,
	}

	jp.FileResults = append(jp.FileResults, fileResult)

	// Reset current file state
	jp.currentFile = ""
	jp.currentFormat = ""
	jp.currentSections = nil
	jp.currentStrings = make([]StringResult, 0)
}

// AddFileResult adds a file result (useful for adding error results from parallel processing)
func (jp *JSONPrinter) AddFileResult(filename, format string, sections []string, strings []StringResult, err error) {
	// Ensure strings is never nil (use empty array instead)
	if strings == nil {
		strings = make([]StringResult, 0)
	}

	fileResult := FileResult{
		File:     filename,
		Format:   format,
		Sections: sections,
		Strings:  strings,
	}

	if err != nil {
		fileResult.Error = err.Error()
	}

	jp.FileResults = append(jp.FileResults, fileResult)
}

// Flush outputs all collected results as JSON
func (jp *JSONPrinter) Flush() error {
	// Finalize any remaining current file
	if jp.currentFile != "" || len(jp.currentStrings) > 0 {
		jp.FinalizeCurrentFile()
	}

	// Calculate summary across all files
	totalStrings := 0
	totalBytes := int64(0)
	for _, fileResult := range jp.FileResults {
		for _, result := range fileResult.Strings {
			totalStrings++
			totalBytes += int64(result.Length)
		}
	}

	summary := Summary{
		TotalStrings: totalStrings,
		TotalBytes:   totalBytes,
		MinLength:    jp.config.MinLength,
		Encoding:     getEncodingName(jp.config.Encoding),
	}

	// Build output structure
	output := JSONOutput{
		Files:   jp.FileResults,
		Summary: summary,
	}

	// Encode and output
	encoder := json.NewEncoder(jp.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// getEncodingName returns a human-readable encoding name
func getEncodingName(encoding string) string {
	switch encoding {
	case "s":
		return "ascii-7bit"
	case "S":
		return "ascii-8bit"
	case "b":
		return "utf-16be"
	case "l":
		return "utf-16le"
	case "B":
		return "utf-32be"
	case "L":
		return "utf-32le"
	default:
		return "ascii-7bit"
	}
}
