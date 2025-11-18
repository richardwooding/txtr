// Package main implements txtr, a GNU strings compatible utility for extracting
// printable strings from binary files.
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/richardwooding/txtr/internal/binary"
	"github.com/richardwooding/txtr/internal/extractor"
	"github.com/richardwooding/txtr/internal/printer"
	"github.com/richardwooding/txtr/internal/stats"
)

// Build information (set by goreleaser via ldflags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

// CLI defines the command-line interface structure
type CLI struct {
	MinLength            int      `short:"n" name:"bytes" default:"4" help:"Minimum string length"`
	PrintFileName        bool     `short:"f" name:"print-file-name" help:"Print file name before each string"`
	Radix                string   `short:"t" name:"radix" enum:"o,d,x," default:"" help:"Print offset in radix (o=octal, d=decimal, x=hex)"`
	OctalOffset          bool     `short:"o" help:"Print offset in octal (alias for -t o)"`
	Encoding             string   `short:"e" name:"encoding" enum:"s,S,b,l,B,L," default:"s" help:"Character encoding (s=7-bit, S=8-bit, b=16-bit BE, l=16-bit LE, B=32-bit BE, L=32-bit LE)"`
	Unicode              string   `short:"U" name:"unicode" enum:"default,invalid,locale,escape,hex,highlight," default:"default" help:"How to handle UTF-8 sequences (default/invalid/locale/escape/hex/highlight)"`
	OutputSeparator      string   `short:"s" name:"output-separator" default:"\\n" help:"Output record separator (default: newline)"`
	IncludeAllWhitespace bool     `short:"w" name:"include-all-whitespace" help:"Include all whitespace characters in strings"`
	ScanAll              bool     `short:"a" name:"all" help:"Scan entire file"`
	ScanDataOnly         bool     `short:"d" name:"data" help:"Scan only initialized data sections of binary files"`
	TargetFormat         string   `short:"T" name:"target" enum:"elf,pe,macho,binary," default:"" help:"Specify binary format (elf/pe/macho/binary)"`
	JSON                 bool     `short:"j" name:"json" help:"Output results in JSON format for automation"`
	Color                string   `name:"color" enum:"auto,always,never," default:"auto" help:"When to use colored output (auto/always/never)"`
	Parallel             int      `short:"P" name:"parallel" default:"0" help:"Number of parallel workers (0=auto-detect CPUs, 1=sequential)"`
	MatchPatterns        []string `short:"m" name:"match" help:"Only show strings matching pattern (can be specified multiple times)"`
	ExcludePatterns      []string `short:"M" name:"exclude" help:"Exclude strings matching pattern (can be specified multiple times)"`
	IgnoreCase           bool     `short:"i" name:"ignore-case" help:"Case-insensitive pattern matching"`
	Stats                bool     `name:"stats" help:"Output statistics summary instead of strings"`
	StatsPerFile         bool     `name:"stats-per-file" help:"Show per-file statistics instead of aggregated (requires --stats)"`
	Version              bool     `short:"v" name:"version" help:"Display version information"`
	VersionAlt           bool     `short:"V" hidden:"" help:"Display version information (alias)"`
	Files                []string `arg:"" optional:"" name:"file" help:"Files to extract strings from" type:"path"`
}

// job represents a file processing job with its position in the input list
type job struct {
	filename string
	index    int
}

// result represents the output from processing a file
type result struct {
	index  int
	output string
	err    error
}

// jsonFileResult represents the result from processing a file for JSON output
type jsonFileResult struct {
	index    int
	filename string
	format   string
	sections []string
	strings  []printer.StringResult
	err      error
}

func main() {
	var cli CLI

	kong.Parse(&cli,
		kong.Name("txtr"),
		kong.Description("Extract printable strings from binary files. GNU strings compatible."),
		kong.UsageOnError(),
	)

	// Handle version flag
	if cli.Version || cli.VersionAlt {
		fmt.Printf("txtr %s\n", version)
		if commit != "none" {
			fmt.Printf("  commit: %s\n", commit)
		}
		if date != "unknown" {
			fmt.Printf("  built: %s\n", date)
		}
		if builtBy != "unknown" {
			fmt.Printf("  built by: %s\n", builtBy)
		}
		fmt.Println("GNU strings compatible utility written in Go")
		os.Exit(0)
	}

	// Handle -o flag (alias for -t o)
	if cli.OctalOffset {
		cli.Radix = "o"
	}

	// Process output separator escape sequences
	outputSep := cli.OutputSeparator
	switch outputSep {
	case "\\n":
		outputSep = "\n"
	case "\\t":
		outputSep = "\t"
	case "\\r":
		outputSep = "\r"
	}

	// Validate -d flag can only be used with files, not stdin
	if cli.ScanDataOnly && len(cli.Files) == 0 {
		fmt.Fprintf(os.Stderr, "error: -d/--data flag requires file arguments (cannot be used with stdin)\n")
		os.Exit(1)
	}

	// Validate --stats-per-file requires --stats
	if cli.StatsPerFile && !cli.Stats {
		fmt.Fprintf(os.Stderr, "error: --stats-per-file requires --stats flag\n")
		os.Exit(1)
	}

	// Validate --stats and --json cannot be used together (for now)
	if cli.Stats && cli.JSON {
		fmt.Fprintf(os.Stderr, "error: --stats and --json cannot be used together (use one or the other)\n")
		os.Exit(1)
	}

	// Parse color mode
	var colorMode extractor.ColorMode
	switch cli.Color {
	case "always":
		colorMode = extractor.ColorAlways
	case "never":
		colorMode = extractor.ColorNever
	default: // "auto" or empty
		colorMode = extractor.ColorAuto
	}

	// Compile regex patterns
	var matchPatterns, excludePatterns []*regexp.Regexp
	var err error

	if len(cli.MatchPatterns) > 0 {
		matchPatterns, err = extractor.CompilePatterns(cli.MatchPatterns, cli.IgnoreCase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid match pattern: %v\n", err)
			os.Exit(1)
		}
	}

	if len(cli.ExcludePatterns) > 0 {
		excludePatterns, err = extractor.CompilePatterns(cli.ExcludePatterns, cli.IgnoreCase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid exclude pattern: %v\n", err)
			os.Exit(1)
		}
	}

	// Build config from CLI args
	config := extractor.Config{
		MinLength:            cli.MinLength,
		PrintFileName:        cli.PrintFileName,
		Radix:                cli.Radix,
		PrintOffset:          cli.Radix != "",
		Encoding:             cli.Encoding,
		Unicode:              cli.Unicode,
		OutputSeparator:      outputSep,
		IncludeAllWhitespace: cli.IncludeAllWhitespace,
		ScanAll:              cli.ScanAll,
		ScanDataOnly:         cli.ScanDataOnly,
		TargetFormat:         cli.TargetFormat,
		ColorMode:            colorMode,
		MatchPatterns:        matchPatterns,
		ExcludePatterns:      excludePatterns,
	}

	// Determine number of parallel workers
	workers := cli.Parallel
	if workers == 0 {
		workers = runtime.NumCPU()
	}

	// Process files or stdin
	if cli.Stats {
		// Statistics output mode
		processWithStats(cli.Files, workers, config, cli.StatsPerFile)
	} else if cli.JSON {
		// JSON output mode
		processWithJSON(cli.Files, workers, config)
	} else if len(cli.Files) == 0 {
		// Read from stdin
		extractor.ExtractStrings(os.Stdin, "", config, printer.PrintString)
	} else if len(cli.Files) > 1 && workers > 1 {
		// Process multiple files in parallel
		processFilesParallel(cli.Files, workers, config)
	} else {
		// Process each file sequentially (single file or workers=1)
		for _, filename := range cli.Files {
			if config.ScanDataOnly {
				// Parse binary and extract from data sections only
				processFileWithBinaryParsing(filename, config)
			} else {
				// Regular full-file scanning
				file, err := os.Open(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					continue
				}
				extractor.ExtractStrings(file, filename, config, printer.PrintString)
				if err := file.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
				}
			}
		}
	}
}

// processWithJSON processes files or stdin with JSON output
// Supports parallel processing for multiple files with automatic error handling
func processWithJSON(files []string, workers int, config extractor.Config) {
	var jsonPrinter *printer.JSONPrinter

	if len(files) == 0 {
		// Read from stdin
		jsonPrinter = printer.NewJSONPrinter(config, os.Stdout)
		jsonPrinter.SetFileInfo("", "", nil)
		extractor.ExtractStrings(os.Stdin, "", config, jsonPrinter.PrintString)
	} else if len(files) > 1 && workers > 1 {
		// Process multiple files in parallel
		jsonPrinter = processFilesParallelJSON(files, workers, config)
	} else {
		// Process files sequentially (single file or workers=1)
		jsonPrinter = printer.NewJSONPrinter(config, os.Stdout)

		for _, filename := range files {
			if config.ScanDataOnly {
				// Parse binary and extract from data sections
				processFileWithBinaryParsingJSON(filename, config, jsonPrinter)
			} else {
				// Regular full-file scanning
				file, err := os.Open(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					// Add error result to JSON
					jsonPrinter.AddFileResult(filename, "", nil, nil, err)
					continue
				}

				jsonPrinter.SetFileInfo(filename, "", nil)
				extractor.ExtractStrings(file, filename, config, jsonPrinter.PrintString)

				if err := file.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
				}
			}
		}
	}

	// Flush JSON output
	if err := jsonPrinter.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "strings: error writing JSON output: %v\n", err)
		os.Exit(1)
	}
}

// processFileWithBinaryParsingJSON handles binary parsing with JSON output
func processFileWithBinaryParsingJSON(filename string, config extractor.Config, jsonPrinter *printer.JSONPrinter) {
	// Determine format
	var format binary.Format
	var err error

	if config.TargetFormat != "" && config.TargetFormat != "binary" {
		// User specified a format
		switch config.TargetFormat {
		case "elf":
			format = binary.FormatELF
		case "pe":
			format = binary.FormatPE
		case "macho":
			format = binary.FormatMachO
		default:
			format = binary.FormatRaw
		}
	} else {
		// Auto-detect format
		format, err = binary.DetectFormat(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	// Parse binary to get sections
	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning if parsing fails
		fmt.Fprintf(os.Stderr, "strings: %s: warning: cannot parse as %v, falling back to full scan: %v\n",
			filename, format, err)

		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			os.Exit(1)
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		jsonPrinter.SetFileInfo(filename, format.String(), nil)
		extractor.ExtractStrings(file, filename, config, jsonPrinter.PrintString)
		return
	}

	// Collect section names
	sectionNames := make([]string, len(sections))
	for i, section := range sections {
		sectionNames[i] = section.Name
	}

	// Set file info
	jsonPrinter.SetFileInfo(filename, format.String(), sectionNames)

	// If no sections found (raw binary), scan the whole file
	if len(sections) == 0 {
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			os.Exit(1)
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		extractor.ExtractStrings(file, filename, config, jsonPrinter.PrintString)
		return
	}

	// Extract strings from each data section
	for _, section := range sections {
		extractor.ExtractFromSection(section.Data, section.Name, section.Offset, filename, config, jsonPrinter.PrintString)
	}
}

// processFileWithBinaryParsing handles binary format detection and section extraction
func processFileWithBinaryParsing(filename string, config extractor.Config) {
	// Determine format
	var format binary.Format
	var err error

	if config.TargetFormat != "" && config.TargetFormat != "binary" {
		// User specified a format
		switch config.TargetFormat {
		case "elf":
			format = binary.FormatELF
		case "pe":
			format = binary.FormatPE
		case "macho":
			format = binary.FormatMachO
		default:
			format = binary.FormatRaw
		}
	} else {
		// Auto-detect format
		format, err = binary.DetectFormat(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			return
		}
	}

	// Parse binary to get sections
	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning if parsing fails
		fmt.Fprintf(os.Stderr, "strings: %s: warning: cannot parse as %v, falling back to full scan: %v\n",
			filename, format, err)

		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		extractor.ExtractStrings(file, filename, config, printer.PrintString)
		return
	}

	// If no sections found (raw binary), scan the whole file
	if len(sections) == 0 {
		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		extractor.ExtractStrings(file, filename, config, printer.PrintString)
		return
	}

	// Extract strings from each data section
	for _, section := range sections {
		extractor.ExtractFromSection(section.Data, section.Name, section.Offset, filename, config, printer.PrintString)
	}
}

// processFilesParallel processes multiple files in parallel using a worker pool
func processFilesParallel(filenames []string, workers int, config extractor.Config) {
	// Create channels for jobs and results
	jobs := make(chan job, len(filenames))
	results := make(chan result, len(filenames))

	// Start worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				// Create a buffer to capture output for this file
				var buf bytes.Buffer

				// Create a print function that writes to the buffer
				printFunc := func(str []byte, filename string, offset int64, cfg extractor.Config) {
					printer.PrintStringToWriter(&buf, str, filename, offset, cfg)
				}

				// Process the file
				var err error
				if config.ScanDataOnly {
					err = processFileWithBinaryParsingToWriter(&buf, j.filename, config)
				} else {
					file, openErr := os.Open(j.filename)
					if openErr != nil {
						results <- result{index: j.index, output: "", err: openErr}
						continue
					}
					extractor.ExtractStrings(file, j.filename, config, printFunc)
					if closeErr := file.Close(); closeErr != nil {
						fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", j.filename, closeErr)
					}
				}

				// Send result
				results <- result{index: j.index, output: buf.String(), err: err}
			}
		}()
	}

	// Send jobs
	for i, filename := range filenames {
		jobs <- job{filename: filename, index: i}
	}
	close(jobs)

	// Close results channel after all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	outputs := make([]result, len(filenames))
	for r := range results {
		outputs[r.index] = r
	}

	// Print results in order
	for _, r := range outputs {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filenames[r.index], r.err)
			continue
		}
		fmt.Print(r.output)
	}
}

// processFileWithBinaryParsingToWriter handles binary parsing and writes output to a buffer
func processFileWithBinaryParsingToWriter(buf *bytes.Buffer, filename string, config extractor.Config) error {
	// Create a print function that writes to the buffer
	printFunc := func(str []byte, fname string, offset int64, cfg extractor.Config) {
		printer.PrintStringToWriter(buf, str, fname, offset, cfg)
	}

	// Determine format
	var format binary.Format
	var err error

	if config.TargetFormat != "" && config.TargetFormat != "binary" {
		// User specified a format
		switch config.TargetFormat {
		case "elf":
			format = binary.FormatELF
		case "pe":
			format = binary.FormatPE
		case "macho":
			format = binary.FormatMachO
		default:
			format = binary.FormatRaw
		}
	} else {
		// Auto-detect format
		format, err = binary.DetectFormat(filename)
		if err != nil {
			return err
		}
	}

	// Parse binary to get sections
	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning if parsing fails
		file, openErr := os.Open(filename)
		if openErr != nil {
			return openErr
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, closeErr)
			}
		}()

		extractor.ExtractStrings(file, filename, config, printFunc)
		return nil
	}

	// If no sections found (raw binary), scan the whole file
	if len(sections) == 0 {
		file, openErr := os.Open(filename)
		if openErr != nil {
			return openErr
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, closeErr)
			}
		}()

		extractor.ExtractStrings(file, filename, config, printFunc)
		return nil
	}

	// Extract strings from each data section
	for _, section := range sections {
		extractor.ExtractFromSection(section.Data, section.Name, section.Offset, filename, config, printFunc)
	}
	return nil
}

// processFilesParallelJSON processes multiple files in parallel for JSON output
func processFilesParallelJSON(filenames []string, workers int, config extractor.Config) *printer.JSONPrinter {
	// Create channels for jobs and results
	jobs := make(chan job, len(filenames))
	results := make(chan jsonFileResult, len(filenames))

	// Start worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				// Create a temporary JSON printer for this file
				var buf bytes.Buffer
				tempPrinter := printer.NewJSONPrinter(config, &buf)

				var format string
				var sections []string
				var strings []printer.StringResult
				var err error

				if config.ScanDataOnly {
					// Process with binary parsing
					format, sections, strings, err = processFileForJSON(j.filename, config)
				} else {
					// Regular full-file scanning
					file, openErr := os.Open(j.filename)
					if openErr != nil {
						results <- jsonFileResult{
							index:    j.index,
							filename: j.filename,
							err:      openErr,
						}
						continue
					}

					tempPrinter.SetFileInfo(j.filename, "", nil)
					extractor.ExtractStrings(file, j.filename, config, tempPrinter.PrintString)

					if closeErr := file.Close(); closeErr != nil {
						fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", j.filename, closeErr)
					}

					// Get the strings from tempPrinter
					tempPrinter.FinalizeCurrentFile()
					if len(tempPrinter.FileResults) > 0 {
						fileRes := tempPrinter.FileResults[0]
						strings = fileRes.Strings
						format = fileRes.Format
						sections = fileRes.Sections
					}
				}

				// Send result (ensure strings is never nil)
				if strings == nil {
					strings = make([]printer.StringResult, 0)
				}
				results <- jsonFileResult{
					index:    j.index,
					filename: j.filename,
					format:   format,
					sections: sections,
					strings:  strings,
					err:      err,
				}
			}
		}()
	}

	// Send jobs
	for i, filename := range filenames {
		jobs <- job{filename: filename, index: i}
	}
	close(jobs)

	// Close results channel after all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	outputs := make([]jsonFileResult, len(filenames))
	for r := range results {
		outputs[r.index] = r
	}

	// Build final JSON output
	jsonPrinter := printer.NewJSONPrinter(config, os.Stdout)
	for _, r := range outputs {
		if r.err != nil {
			// Print error to stderr as well
			fmt.Fprintf(os.Stderr, "strings: %s: %v\n", r.filename, r.err)
		}
		// Add file result (with error if present)
		jsonPrinter.AddFileResult(r.filename, r.format, r.sections, r.strings, r.err)
	}

	return jsonPrinter
}

// processFileForJSON processes a single file with binary parsing for JSON output
func processFileForJSON(filename string, config extractor.Config) (string, []string, []printer.StringResult, error) {
	// Determine format
	var format binary.Format
	var err error

	if config.TargetFormat != "" && config.TargetFormat != "binary" {
		switch config.TargetFormat {
		case "elf":
			format = binary.FormatELF
		case "pe":
			format = binary.FormatPE
		case "macho":
			format = binary.FormatMachO
		default:
			format = binary.FormatRaw
		}
	} else {
		format, err = binary.DetectFormat(filename)
		if err != nil {
			return "", nil, nil, err
		}
	}

	// Parse binary to get sections
	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning
		file, openErr := os.Open(filename)
		if openErr != nil {
			return "", nil, nil, openErr
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		var buf bytes.Buffer
		tempPrinter := printer.NewJSONPrinter(config, &buf)
		tempPrinter.SetFileInfo(filename, format.String(), nil)
		extractor.ExtractStrings(file, filename, config, tempPrinter.PrintString)
		tempPrinter.FinalizeCurrentFile()

		if len(tempPrinter.FileResults) > 0 {
			fileRes := tempPrinter.FileResults[0]
			return fileRes.Format, fileRes.Sections, fileRes.Strings, nil
		}
		return format.String(), nil, nil, nil
	}

	// Collect section names
	sectionNames := make([]string, len(sections))
	for i, section := range sections {
		sectionNames[i] = section.Name
	}

	// If no sections found, scan whole file
	if len(sections) == 0 {
		file, openErr := os.Open(filename)
		if openErr != nil {
			return "", nil, nil, openErr
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		var buf bytes.Buffer
		tempPrinter := printer.NewJSONPrinter(config, &buf)
		tempPrinter.SetFileInfo(filename, format.String(), sectionNames)
		extractor.ExtractStrings(file, filename, config, tempPrinter.PrintString)
		tempPrinter.FinalizeCurrentFile()

		if len(tempPrinter.FileResults) > 0 {
			fileRes := tempPrinter.FileResults[0]
			return fileRes.Format, fileRes.Sections, fileRes.Strings, nil
		}
		return format.String(), sectionNames, nil, nil
	}

	// Extract strings from data sections
	var buf bytes.Buffer
	tempPrinter := printer.NewJSONPrinter(config, &buf)
	tempPrinter.SetFileInfo(filename, format.String(), sectionNames)

	for _, section := range sections {
		extractor.ExtractFromSection(section.Data, section.Name, section.Offset, filename, config, tempPrinter.PrintString)
	}

	tempPrinter.FinalizeCurrentFile()
	if len(tempPrinter.FileResults) > 0 {
		fileRes := tempPrinter.FileResults[0]
		return fileRes.Format, fileRes.Sections, fileRes.Strings, nil
	}

	return format.String(), sectionNames, nil, nil
}

// processWithStats processes files or stdin with statistics output
func processWithStats(files []string, workers int, config extractor.Config, perFile bool) {
	// stdin case
	if len(files) == 0 {
		s := stats.New(config.MinLength)

		// Create wrapper function for filter tracking if needed
		collectFunc := s.Add
		if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
			collectFunc = makeFilterTrackingFunc(s, config)
		}

		extractor.ExtractStrings(os.Stdin, "", config, collectFunc)
		s.Format(os.Stdout)
		return
	}

	// Per-file statistics mode
	if perFile {
		for _, filename := range files {
			s := stats.New(config.MinLength)

			// Create wrapper function for filter tracking if needed
			collectFunc := s.Add
			if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
				collectFunc = makeFilterTrackingFunc(s, config)
			}

			// Process file with binary parsing if needed
			if config.ScanDataOnly {
				if err := processFileWithStatsAndBinaryParsing(filename, config, s); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					continue
				}
			} else {
				file, err := os.Open(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					continue
				}

				s.SetFileInfo(filename, "", nil)
				extractor.ExtractStrings(file, filename, config, collectFunc)

				if err := file.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
				}
			}

			// Output statistics for this file
			s.Format(os.Stdout)
			if filename != files[len(files)-1] {
				fmt.Println() // Blank line between files
			}
		}
		return
	}

	// Aggregated statistics mode (default)
	aggregated := stats.New(config.MinLength)

	// Create wrapper function for filter tracking if needed
	collectFunc := aggregated.Add
	if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
		collectFunc = makeFilterTrackingFunc(aggregated, config)
	}

	// Sequential processing
	if len(files) == 1 || workers == 1 {
		for _, filename := range files {
			if config.ScanDataOnly {
				if err := processFileWithStatsAndBinaryParsing(filename, config, aggregated); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					continue
				}
			} else {
				file, err := os.Open(filename)
				if err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
					continue
				}

				extractor.ExtractStrings(file, filename, config, collectFunc)

				if err := file.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
				}
			}
		}
	} else {
		// Parallel processing
		jobs := make(chan job, len(files))
		results := make(chan *stats.Statistics, len(files))
		var wg sync.WaitGroup

		// Start workers
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					s := stats.New(config.MinLength)

					// Create wrapper function for filter tracking if needed
					localCollectFunc := s.Add
					if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
						localCollectFunc = makeFilterTrackingFunc(s, config)
					}

					if config.ScanDataOnly {
						if err := processFileWithStatsAndBinaryParsing(j.filename, config, s); err != nil {
							fmt.Fprintf(os.Stderr, "strings: %s: %v\n", j.filename, err)
							results <- nil
							continue
						}
					} else {
						file, err := os.Open(j.filename)
						if err != nil {
							fmt.Fprintf(os.Stderr, "strings: %s: %v\n", j.filename, err)
							results <- nil
							continue
						}

						extractor.ExtractStrings(file, j.filename, config, localCollectFunc)

						if err := file.Close(); err != nil {
							fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", j.filename, err)
						}
					}

					results <- s
				}
			}()
		}

		// Send jobs
		for _, filename := range files {
			jobs <- job{filename: filename}
		}
		close(jobs)

		// Wait for workers to finish
		go func() {
			wg.Wait()
			close(results)
		}()

		// Merge results
		for s := range results {
			if s != nil {
				aggregated.Merge(s)
			}
		}
	}

	// Output aggregated statistics
	aggregated.Format(os.Stdout)
}

// makeFilterTrackingFunc creates a wrapper function that tracks both filtered and unfiltered counts
func makeFilterTrackingFunc(s *stats.Statistics, _ extractor.Config) func([]byte, string, int64, extractor.Config) {
	return func(str []byte, filename string, offset int64, cfg extractor.Config) {
		// Track unfiltered count
		s.AddUnfiltered()

		// Check if string should be included (filtering logic)
		if extractor.ShouldPrintString(str, cfg) {
			// String passed filters, add to statistics
			s.Add(str, filename, offset, cfg)
		}
	}
}

// processFileWithStatsAndBinaryParsing processes a file with binary parsing for statistics
func processFileWithStatsAndBinaryParsing(filename string, config extractor.Config, s *stats.Statistics) error {
	// Determine format
	var format binary.Format
	var err error

	if config.TargetFormat != "" && config.TargetFormat != "binary" {
		switch config.TargetFormat {
		case "elf":
			format = binary.FormatELF
		case "pe":
			format = binary.FormatPE
		case "macho":
			format = binary.FormatMachO
		default:
			format = binary.FormatRaw
		}
	} else {
		format, err = binary.DetectFormat(filename)
		if err != nil {
			return err
		}
	}

	// Parse binary to get sections
	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning
		file, openErr := os.Open(filename)
		if openErr != nil {
			return openErr
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		s.SetFileInfo(filename, format.String(), nil)

		// Create wrapper function for filter tracking if needed
		collectFunc := s.Add
		if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
			collectFunc = makeFilterTrackingFunc(s, config)
		}

		extractor.ExtractStrings(file, filename, config, collectFunc)
		return nil
	}

	// Collect section names
	sectionNames := make([]string, len(sections))
	for i, section := range sections {
		sectionNames[i] = section.Name
	}

	s.SetFileInfo(filename, format.String(), sectionNames)

	// If no sections found, scan whole file
	if len(sections) == 0 {
		file, openErr := os.Open(filename)
		if openErr != nil {
			return openErr
		}
		defer func() {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, err)
			}
		}()

		// Create wrapper function for filter tracking if needed
		collectFunc := s.Add
		if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
			collectFunc = makeFilterTrackingFunc(s, config)
		}

		extractor.ExtractStrings(file, filename, config, collectFunc)
		return nil
	}

	// Create wrapper function for filter tracking if needed
	collectFunc := s.Add
	if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
		collectFunc = makeFilterTrackingFunc(s, config)
	}

	// Extract strings from data sections
	for _, section := range sections {
		extractor.ExtractFromSection(section.Data, section.Name, section.Offset, filename, config, collectFunc)
	}

	return nil
}
