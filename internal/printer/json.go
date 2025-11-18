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
	results    []StringResult
	config     extractor.Config
	writer     io.Writer
	currentFile string
	format      string
	sections    []string
}

// NewJSONPrinter creates a new JSON printer
func NewJSONPrinter(config extractor.Config, writer io.Writer) *JSONPrinter {
	if writer == nil {
		writer = os.Stdout
	}
	return &JSONPrinter{
		results: make([]StringResult, 0),
		config:  config,
		writer:  writer,
	}
}

// SetFileInfo sets the current file and format information
func (jp *JSONPrinter) SetFileInfo(filename, format string, sections []string) {
	jp.currentFile = filename
	jp.format = format
	jp.sections = sections
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

	jp.results = append(jp.results, result)
}

// Flush outputs all collected results as JSON
func (jp *JSONPrinter) Flush() error {
	// Calculate summary
	totalBytes := int64(0)
	for _, result := range jp.results {
		totalBytes += int64(result.Length)
	}

	summary := Summary{
		TotalStrings: len(jp.results),
		TotalBytes:   totalBytes,
		MinLength:    jp.config.MinLength,
		Encoding:     getEncodingName(jp.config.Encoding),
	}

	// Build output structure
	output := JSONOutput{
		Files: []FileResult{
			{
				File:     jp.currentFile,
				Format:   jp.format,
				Sections: jp.sections,
				Strings:  jp.results,
			},
		},
		Summary: summary,
	}

	// If no file is specified (stdin), omit file field
	if jp.currentFile == "" {
		output.Files[0].File = ""
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
