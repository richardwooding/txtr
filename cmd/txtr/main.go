// Package main implements txtr, a GNU strings compatible utility for extracting
// printable strings from binary files.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"sync"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/richardwooding/archives"
	"github.com/richardwooding/txtr/internal/binary"
	"github.com/richardwooding/txtr/internal/extractor"
	"github.com/richardwooding/txtr/internal/printer"
	"github.com/richardwooding/txtr/internal/stats"
)

// reportErr prints a per-file error to stderr unless it is a context
// cancellation (which is reported once, centrally, as "strings: interrupted").
// It returns true when err is a cancellation, signalling callers to stop.
func reportErr(filename string, err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if filename != "" {
		fmt.Fprintf(os.Stderr, "strings: %s: %v\n", filename, err)
	} else {
		fmt.Fprintf(os.Stderr, "strings: %v\n", err)
	}
	return false
}

// printFunc is the callback every output mode supplies to receive extracted
// strings: (string bytes, source path, byte offset within the source, config).
type printFunc = func([]byte, string, int64, extractor.Config)

// extractFile routes a single input file through the correct extraction path
// based on config precedence: recursion (archive walk) > data-only (binary
// sections) > whole-file scan. Extracted strings are delivered to fn. Used by
// every output mode so recursion composes with text/triage/stats/JSON output.
func extractFile(ctx context.Context, filename string, config extractor.Config, fn printFunc) error {
	if config.Recursive {
		opts := archives.Options{MaxDepth: config.RecurseMaxDepth, MaxBytes: config.RecurseMaxBytes}
		return archives.Walk(ctx, filename, opts, func(wctx context.Context, vpath string, r io.Reader) error {
			return extractor.ExtractStrings(wctx, r, vpath, config, fn)
		})
	}
	if config.ScanDataOnly {
		return extractFileWithBinaryParsing(ctx, filename, config, fn)
	}
	return extractor.ExtractStringsFromFile(ctx, filename, config, fn)
}

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
	Triage               bool     `name:"triage" help:"Security triage: tag strings by entropy, classification, and detected secrets"`
	Secrets              bool     `name:"secrets" help:"Only surface detected secrets and high-entropy strings (implies --triage)"`
	MinEntropy           float64  `name:"min-entropy" default:"4.5" help:"Entropy threshold (bits/byte) for high-entropy flagging in triage mode"`
	Recurse              bool     `short:"r" name:"recurse" help:"Recurse into archives/compressed files (zip/apk/tar/gz/bz2/xz/zst/deb), scanning each member"`
	MaxDepth             int      `name:"max-depth" default:"8" help:"Maximum archive nesting depth when recursing"`
	MaxDecompressedSize  int64    `name:"max-decompressed-size" default:"2147483648" help:"Max total decompressed bytes per input when recursing (bomb guard; <0 = unlimited)"`
	DisableMmap          bool     `name:"no-mmap" help:"Disable memory-mapped I/O optimization"`
	MmapThreshold        int64    `name:"mmap-threshold" default:"1048576" help:"Minimum file size (bytes) for using mmap (default: 1MB)"`
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

	// --secrets implies triage (it is a secrets-only filter on triage output)
	if cli.Secrets {
		cli.Triage = true
	}

	// Validate --recurse usage
	if cli.Recurse && len(cli.Files) == 0 {
		fmt.Fprintf(os.Stderr, "error: -r/--recurse requires file arguments (cannot be used with stdin)\n")
		os.Exit(1)
	}
	if cli.Recurse && cli.ScanDataOnly {
		fmt.Fprintf(os.Stderr, "error: -r/--recurse and -d/--data cannot be used together\n")
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
		DisableMmap:          cli.DisableMmap,
		MmapThreshold:        cli.MmapThreshold,
		TriageMode:           cli.Triage,
		SecretsOnly:          cli.Secrets,
		MinEntropy:           cli.MinEntropy,
		Recursive:            cli.Recurse,
		RecurseMaxDepth:      cli.MaxDepth,
		RecurseMaxBytes:      cli.MaxDecompressedSize,
	}

	// Determine number of parallel workers
	workers := cli.Parallel
	if workers == 0 {
		workers = runtime.NumCPU()
	}

	// Cancellable context wired to Ctrl-C (SIGINT) and SIGTERM so long scans,
	// worker pools, and archive walks can be aborted promptly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Process files or stdin
	if cli.Stats {
		// Statistics output mode
		processWithStats(ctx, cli.Files, workers, config, cli.StatsPerFile)
	} else if cli.JSON {
		// JSON output mode
		processWithJSON(ctx, cli.Files, workers, config)
	} else if cli.Triage {
		// Security triage output mode
		processWithTriage(ctx, cli.Files, workers, config)
	} else if len(cli.Files) == 0 {
		// Read from stdin
		if err := extractor.ExtractStrings(ctx, os.Stdin, "", config, printer.PrintString); err != nil {
			reportErr("", err)
		}
	} else if len(cli.Files) > 1 && workers > 1 {
		// Process multiple files in parallel
		processFilesParallel(ctx, cli.Files, workers, config)
	} else {
		// Process each file sequentially (single file or workers=1)
		for _, filename := range cli.Files {
			if err := extractFile(ctx, filename, config, printer.PrintString); err != nil {
				if reportErr(filename, err) {
					break
				}
				continue
			}
		}
	}

	// Single, central cancellation notice + conventional 128+SIGINT exit code.
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "strings: interrupted")
		os.Exit(130)
	}
}

// processWithJSON processes files or stdin with JSON output
// Supports parallel processing for multiple files with automatic error handling
func processWithJSON(ctx context.Context, files []string, workers int, config extractor.Config) {
	var jsonPrinter *printer.JSONPrinter

	if len(files) == 0 {
		// Read from stdin
		jsonPrinter = printer.NewJSONPrinter(config, os.Stdout)
		jsonPrinter.SetFileInfo("", "", nil)
		if err := extractor.ExtractStrings(ctx, os.Stdin, "", config, jsonPrinter.PrintString); err != nil {
			reportErr("", err)
		}
	} else if len(files) > 1 && workers > 1 {
		// Process multiple files in parallel
		jsonPrinter = processFilesParallelJSON(ctx, files, workers, config)
	} else {
		// Process files sequentially (single file or workers=1)
		jsonPrinter = printer.NewJSONPrinter(config, os.Stdout)

		for _, filename := range files {
			if config.Recursive {
				jsonPrinter.SetFileInfo(filename, "archive", nil)
				if err := extractFile(ctx, filename, config, jsonPrinter.PrintString); err != nil {
					if reportErr(filename, err) {
						break
					}
					jsonPrinter.AddFileResult(filename, "archive", nil, nil, err)
					continue
				}
			} else if config.ScanDataOnly {
				// Parse binary and extract from data sections
				processFileWithBinaryParsingJSON(ctx, filename, config, jsonPrinter)
			} else {
				// Regular full-file scanning with automatic mmap optimization
				jsonPrinter.SetFileInfo(filename, "", nil)
				if err := extractor.ExtractStringsFromFile(ctx, filename, config, jsonPrinter.PrintString); err != nil {
					if reportErr(filename, err) {
						break
					}
					// Add error result to JSON
					jsonPrinter.AddFileResult(filename, "", nil, nil, err)
					continue
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
func processFileWithBinaryParsingJSON(ctx context.Context, filename string, config extractor.Config, jsonPrinter *printer.JSONPrinter) {
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
		if err := extractor.ExtractStrings(ctx, file, filename, config, jsonPrinter.PrintString); err != nil {
			reportErr(filename, err)
		}
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

		if err := extractor.ExtractStrings(ctx, file, filename, config, jsonPrinter.PrintString); err != nil {
			reportErr(filename, err)
		}
		return
	}

	// Extract strings from each data section
	for _, section := range sections {
		if err := extractor.ExtractFromSection(ctx, section.Data, section.Name, section.Offset, filename, config, jsonPrinter.PrintString); err != nil {
			if reportErr(filename, err) {
				break
			}
		}
	}
}

// processFilesParallel processes multiple files in parallel using a worker pool
func processFilesParallel(ctx context.Context, filenames []string, workers int, config extractor.Config) {
	// Create channels for jobs and results
	jobs := make(chan job, len(filenames))
	results := make(chan result, len(filenames))

	// Start worker goroutines
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case j, ok := <-jobs:
					if !ok {
						return
					}
					// Create a buffer to capture output for this file
					var buf bytes.Buffer

					// Create a print function that writes to the buffer
					printToBuf := func(str []byte, filename string, offset int64, cfg extractor.Config) {
						printer.PrintStringToWriter(&buf, str, filename, offset, cfg)
					}

					// Process the file (recursion/data-only/whole-file)
					err := extractFile(ctx, j.filename, config, printToBuf)

					// Send result
					select {
					case results <- result{index: j.index, output: buf.String(), err: err}:
					case <-ctx.Done():
						return
					}
				}
			}
		})
	}

	// Send jobs
	for i, filename := range filenames {
		select {
		case jobs <- job{filename: filename, index: i}:
		case <-ctx.Done():
		}
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
			reportErr(filenames[r.index], r.err)
			continue
		}
		fmt.Print(r.output)
	}
}

// processFilesParallelJSON processes multiple files in parallel for JSON output
func processFilesParallelJSON(ctx context.Context, filenames []string, workers int, config extractor.Config) *printer.JSONPrinter {
	// Create channels for jobs and results
	jobs := make(chan job, len(filenames))
	results := make(chan jsonFileResult, len(filenames))

	// Start worker goroutines
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				var j job
				var ok bool
				select {
				case <-ctx.Done():
					return
				case j, ok = <-jobs:
					if !ok {
						return
					}
				}

				// Create a temporary JSON printer for this file
				var buf bytes.Buffer
				tempPrinter := printer.NewJSONPrinter(config, &buf)

				var format string
				var sections []string
				var strings []printer.StringResult
				var err error

				if config.Recursive {
					tempPrinter.SetFileInfo(j.filename, "archive", nil)
					err = extractFile(ctx, j.filename, config, tempPrinter.PrintString)
					if err != nil {
						sendJSONResult(ctx, results, jsonFileResult{index: j.index, filename: j.filename, err: err})
						continue
					}
					tempPrinter.FinalizeCurrentFile()
					if len(tempPrinter.FileResults) > 0 {
						fileRes := tempPrinter.FileResults[0]
						strings = fileRes.Strings
						format = fileRes.Format
						sections = fileRes.Sections
					}
				} else if config.ScanDataOnly {
					// Process with binary parsing
					format, sections, strings, err = processFileForJSON(ctx, j.filename, config)
				} else {
					// Regular full-file scanning with automatic mmap optimization
					tempPrinter.SetFileInfo(j.filename, "", nil)
					err = extractor.ExtractStringsFromFile(ctx, j.filename, config, tempPrinter.PrintString)
					if err != nil {
						sendJSONResult(ctx, results, jsonFileResult{
							index:    j.index,
							filename: j.filename,
							err:      err,
						})
						continue
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
				sendJSONResult(ctx, results, jsonFileResult{
					index:    j.index,
					filename: j.filename,
					format:   format,
					sections: sections,
					strings:  strings,
					err:      err,
				})
			}
		})
	}

	// Send jobs
	for i, filename := range filenames {
		select {
		case jobs <- job{filename: filename, index: i}:
		case <-ctx.Done():
		}
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
		// Skip jobs a cancelled worker never produced (zero-value entries).
		if r.filename == "" {
			continue
		}
		if r.err != nil {
			// Suppress cancellation noise / JSON error entries; report real errors.
			if reportErr(r.filename, r.err) {
				continue
			}
		}
		// Add file result (with error if present)
		jsonPrinter.AddFileResult(r.filename, r.format, r.sections, r.strings, r.err)
	}

	return jsonPrinter
}

// sendJSONResult delivers a result unless the context is cancelled first,
// preventing a worker from blocking forever once the collector has stopped.
func sendJSONResult(ctx context.Context, results chan<- jsonFileResult, r jsonFileResult) {
	select {
	case results <- r:
	case <-ctx.Done():
	}
}

// processFileForJSON processes a single file with binary parsing for JSON output
func processFileForJSON(ctx context.Context, filename string, config extractor.Config) (string, []string, []printer.StringResult, error) {
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
		if err := extractor.ExtractStrings(ctx, file, filename, config, tempPrinter.PrintString); err != nil {
			return "", nil, nil, err
		}
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
		if err := extractor.ExtractStrings(ctx, file, filename, config, tempPrinter.PrintString); err != nil {
			return "", nil, nil, err
		}
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
		if err := extractor.ExtractFromSection(ctx, section.Data, section.Name, section.Offset, filename, config, tempPrinter.PrintString); err != nil {
			return "", nil, nil, err
		}
	}

	tempPrinter.FinalizeCurrentFile()
	if len(tempPrinter.FileResults) > 0 {
		fileRes := tempPrinter.FileResults[0]
		return fileRes.Format, fileRes.Sections, fileRes.Strings, nil
	}

	return format.String(), sectionNames, nil, nil
}

// processWithStats processes files or stdin with statistics output
func processWithStats(ctx context.Context, files []string, workers int, config extractor.Config, perFile bool) {
	// stdin case
	if len(files) == 0 {
		s := stats.New(config.MinLength)

		// Create wrapper function for filter tracking if needed
		collectFunc := s.Add
		if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
			collectFunc = makeFilterTrackingFunc(s, config)
		}

		if err := extractor.ExtractStrings(ctx, os.Stdin, "", config, collectFunc); err != nil {
			reportErr("", err)
		}
		s.Format(os.Stdout, config.ColorMode)
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
			if config.Recursive {
				if err := extractFile(ctx, filename, config, collectFunc); err != nil {
					if reportErr(filename, err) {
						return
					}
					continue
				}
			} else if config.ScanDataOnly {
				if err := processFileWithStatsAndBinaryParsing(ctx, filename, config, s); err != nil {
					if reportErr(filename, err) {
						return
					}
					continue
				}
			} else {
				// Use ExtractStringsFromFile with automatic mmap optimization
				s.SetFileInfo(filename, "", nil)
				if err := extractor.ExtractStringsFromFile(ctx, filename, config, collectFunc); err != nil {
					if reportErr(filename, err) {
						return
					}
					continue
				}
			}

			// Output statistics for this file
			s.Format(os.Stdout, config.ColorMode)
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
			if config.Recursive {
				if err := extractFile(ctx, filename, config, collectFunc); err != nil {
					if reportErr(filename, err) {
						break
					}
					continue
				}
			} else if config.ScanDataOnly {
				if err := processFileWithStatsAndBinaryParsing(ctx, filename, config, aggregated); err != nil {
					if reportErr(filename, err) {
						break
					}
					continue
				}
			} else {
				// Use ExtractStringsFromFile with automatic mmap optimization
				if err := extractor.ExtractStringsFromFile(ctx, filename, config, collectFunc); err != nil {
					if reportErr(filename, err) {
						break
					}
					continue
				}
			}
		}
	} else {
		// Parallel processing
		jobs := make(chan job, len(files))
		results := make(chan *stats.Statistics, len(files))
		var wg sync.WaitGroup

		// Start workers
		for range workers {
			wg.Go(func() {
				for {
					var j job
					var ok bool
					select {
					case <-ctx.Done():
						return
					case j, ok = <-jobs:
						if !ok {
							return
						}
					}

					s := stats.New(config.MinLength)

					// Create wrapper function for filter tracking if needed
					localCollectFunc := s.Add
					if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
						localCollectFunc = makeFilterTrackingFunc(s, config)
					}

					res := s
					if config.Recursive {
						if err := extractFile(ctx, j.filename, config, localCollectFunc); err != nil {
							reportErr(j.filename, err)
							res = nil
						}
					} else if config.ScanDataOnly {
						if err := processFileWithStatsAndBinaryParsing(ctx, j.filename, config, s); err != nil {
							reportErr(j.filename, err)
							res = nil
						}
					} else {
						// Use ExtractStringsFromFile with automatic mmap optimization
						if err := extractor.ExtractStringsFromFile(ctx, j.filename, config, localCollectFunc); err != nil {
							reportErr(j.filename, err)
							res = nil
						}
					}

					select {
					case results <- res:
					case <-ctx.Done():
						return
					}
				}
			})
		}

		// Send jobs
		for _, filename := range files {
			select {
			case jobs <- job{filename: filename}:
			case <-ctx.Done():
			}
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
	aggregated.Format(os.Stdout, config.ColorMode)
}

// processWithTriage processes files or stdin with security-triage output.
func processWithTriage(ctx context.Context, files []string, workers int, config extractor.Config) {
	// stdin case
	if len(files) == 0 {
		tp := printer.NewTriagePrinter(config, os.Stdout)
		if err := extractor.ExtractStrings(ctx, os.Stdin, "", config, tp.PrintString); err != nil {
			reportErr("", err)
		}
		return
	}

	// Parallel processing for multiple files (output buffered per file, in order)
	if len(files) > 1 && workers > 1 {
		jobs := make(chan job, len(files))
		results := make(chan result, len(files))

		var wg sync.WaitGroup
		for range workers {
			wg.Go(func() {
				for {
					var j job
					var ok bool
					select {
					case <-ctx.Done():
						return
					case j, ok = <-jobs:
						if !ok {
							return
						}
					}

					var buf bytes.Buffer
					tp := printer.NewTriagePrinter(config, &buf)
					err := extractFile(ctx, j.filename, config, tp.PrintString)
					select {
					case results <- result{index: j.index, output: buf.String(), err: err}:
					case <-ctx.Done():
						return
					}
				}
			})
		}

		for i, filename := range files {
			select {
			case jobs <- job{filename: filename, index: i}:
			case <-ctx.Done():
			}
		}
		close(jobs)

		go func() {
			wg.Wait()
			close(results)
		}()

		outputs := make([]result, len(files))
		for r := range results {
			outputs[r.index] = r
		}

		for _, r := range outputs {
			if r.err != nil {
				reportErr(files[r.index], r.err)
				continue
			}
			fmt.Print(r.output)
		}
		return
	}

	// Sequential (single file or workers=1)
	tp := printer.NewTriagePrinter(config, os.Stdout)
	for _, filename := range files {
		if err := extractFile(ctx, filename, config, tp.PrintString); err != nil {
			if reportErr(filename, err) {
				break
			}
			continue
		}
	}
}

// extractFileWithBinaryParsing detects the binary format, extracts strings from
// its data sections (falling back to a full scan when parsing fails or no
// sections are found), and routes each string through printFunc.
func extractFileWithBinaryParsing(ctx context.Context, filename string, config extractor.Config, printFunc func([]byte, string, int64, extractor.Config)) error {
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

	sections, err := binary.ParseBinary(filename, format)
	if err != nil {
		// Fall back to regular scanning if parsing fails
		return scanWholeFile(ctx, filename, config, printFunc)
	}

	// If no sections found (raw binary), scan the whole file
	if len(sections) == 0 {
		return scanWholeFile(ctx, filename, config, printFunc)
	}

	// Extract strings from each data section
	for _, section := range sections {
		if err := extractor.ExtractFromSection(ctx, section.Data, section.Name, section.Offset, filename, config, printFunc); err != nil {
			return err
		}
	}
	return nil
}

// scanWholeFile opens filename and extracts strings from its entire contents.
func scanWholeFile(ctx context.Context, filename string, config extractor.Config, printFunc func([]byte, string, int64, extractor.Config)) error {
	file, openErr := os.Open(filename)
	if openErr != nil {
		return openErr
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "strings: %s: error closing file: %v\n", filename, closeErr)
		}
	}()

	return extractor.ExtractStrings(ctx, file, filename, config, printFunc)
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
func processFileWithStatsAndBinaryParsing(ctx context.Context, filename string, config extractor.Config, s *stats.Statistics) error {
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

		return extractor.ExtractStrings(ctx, file, filename, config, collectFunc)
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

		return extractor.ExtractStrings(ctx, file, filename, config, collectFunc)
	}

	// Create wrapper function for filter tracking if needed
	collectFunc := s.Add
	if len(config.MatchPatterns) > 0 || len(config.ExcludePatterns) > 0 {
		collectFunc = makeFilterTrackingFunc(s, config)
	}

	// Extract strings from data sections
	for _, section := range sections {
		if err := extractor.ExtractFromSection(ctx, section.Data, section.Name, section.Offset, filename, config, collectFunc); err != nil {
			return err
		}
	}

	return nil
}
