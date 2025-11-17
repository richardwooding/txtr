# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`txtr` is a clone of the GNU strings utility written in Go 1.25. It extracts printable ASCII strings (characters 32-126) from binary files, with output compatible with GNU strings. The project follows the Standard Go Project Layout conventions.

## Development Commands

### Build
```bash
# Build the binary
go build -o txtr ./cmd/txtr

# Or use go install
go install ./cmd/txtr
```

### Run Tests
```bash
# Run all tests
set -o pipefail; go test -v ./...

# Run tests for a specific package
set -o pipefail; go test -v ./internal/extractor
set -o pipefail; go test -v ./internal/printer

# Run a specific test
set -o pipefail; go test -v -run TestIsPrintable ./internal/extractor
```

### Run the Binary
```bash
# After building
./txtr file.bin

# Or run directly with go
go run ./cmd/txtr file.bin
```

## Architecture

This project follows the Standard Go Project Layout with clear separation of concerns:

### Directory Structure

```
txtr/
├── cmd/txtr/           # Application entry point
│   └── main.go         # CLI using Kong library
├── internal/           # Private application code
│   ├── extractor/      # String extraction logic
│   │   ├── extractor.go
│   │   └── extractor_test.go
│   └── printer/        # Output formatting logic
│       ├── printer.go
│       └── printer_test.go
├── go.mod              # Go module definition (Go 1.25)
├── go.sum              # Dependency checksums
├── README.md           # User documentation
└── CLAUDE.md           # Developer documentation (this file)
```

### Core Components

**cmd/txtr/main.go** - CLI entry point:
- `CLI` struct: Kong-tagged structure for command-line argument parsing
- `main()`: Parses CLI args using Kong, handles stdin vs file input
- Uses Kong library for modern, declarative CLI parsing
- Backward compatible with original GNU strings flags

**internal/extractor/extractor.go** - Core extraction logic:
- `Config` struct: Configuration for extraction options (MinLength, PrintFileName, Radix, PrintOffset, Encoding, Unicode, OutputSeparator, IncludeAllWhitespace, ScanAll)
- `ExtractStrings()`: Router function that dispatches to encoding-specific extractors
- `extractASCII()`: Extracts 7-bit or 8-bit ASCII strings (delegates to UTF-8 aware version if Unicode mode set)
- `extractUTF8Aware()`: UTF-8 aware extraction with multibyte sequence validation and special display modes
- `extractUTF16()`: Extracts UTF-16 encoded strings (handles surrogate pairs)
- `extractUTF32()`: Extracts UTF-32 encoded strings
- `IsPrintable()`: Determines if a byte is in printable ASCII range (32-126)
- `isPrintableASCII()`: Enhanced printable check with 8-bit and whitespace support
- `isPrintableRune()`: Checks if a Unicode rune is printable

**internal/printer/printer.go** - Output formatting:
- `PrintString()`: Formats and outputs strings with optional filename prefix, offset, and custom separator

### String Extraction Algorithm

The extraction logic in `ExtractStrings()` dispatches to encoding-specific extractors:

**ASCII Extraction (7-bit and 8-bit):**
1. Reads input byte-by-byte using bufio.Reader
2. Tracks current offset and string start offset
3. Accumulates consecutive printable bytes into `currentString`
4. When a non-printable byte is encountered, checks if accumulated string meets minimum length
5. Calls the print function for valid strings and resets the accumulator
6. Handles EOF by printing any remaining valid string

**UTF-16 Extraction (Big-Endian and Little-Endian):**
1. Reads 2 bytes at a time to form 16-bit code units
2. Handles UTF-16 surrogate pairs for characters beyond the Basic Multilingual Plane
3. Converts to runes and checks for printability
4. Accumulates printable runes and converts to UTF-8 for output

**UTF-32 Extraction (Big-Endian and Little-Endian):**
1. Reads 4 bytes at a time to form 32-bit code points
2. Validates runes are valid Unicode code points
3. Checks for printability and accumulates valid characters
4. Converts accumulated runes to UTF-8 for output

**UTF-8 Aware Extraction:**
1. Reads byte-by-byte and detects UTF-8 multi-byte sequences
2. Determines sequence length from leading byte (2, 3, or 4 bytes)
3. Validates continuation bytes and UTF-8 encoding correctness
4. Formats output based on Unicode mode:
   - `locale`: Display characters normally
   - `escape`: Format as `\uXXXX` escape sequences
   - `hex`: Format as `<XX>` hex values
   - `highlight`: Add ANSI color codes to escape sequences
5. Treats invalid UTF-8 sequences as non-printable

### Testing

Tests are organized by package:

**internal/extractor/extractor_test.go**:
- `TestIsPrintable`: Boundary testing for printable character detection
- `TestExtractStrings`: Integration testing of extraction logic with various inputs

**internal/extractor/encoding_test.go**:
- `TestExtractUTF8`: UTF-8 extraction with various Unicode characters and display modes
- `TestExtractUTF16LE`: UTF-16 little-endian encoding extraction
- `TestExtractUTF16BE`: UTF-16 big-endian encoding extraction
- `TestExtractUTF32LE`: UTF-32 little-endian encoding extraction
- `TestExtractUTF32BE`: UTF-32 big-endian encoding extraction
- `TestIncludeAllWhitespace`: Whitespace handling flag testing
- `Test8BitASCII`: 7-bit vs 8-bit ASCII extraction testing

**internal/printer/printer_test.go**:
- `TestPrintString`: Output formatting tests (currently skipped - requires stdout capture)

### CLI Flag Handling

The application uses Kong for declarative CLI argument parsing:

**Basic Flags:**
- `-n` / `--bytes`: Minimum string length (default: 4)
- `-f` / `--print-file-name`: Print filename before each string
- `-t` / `--radix`: Print offset in specified radix (o/d/x)
- `-o` / `--octal-offset`: Alias for `-t o` (octal offset)

**Encoding Flags:**
- `-e` / `--encoding`: Character encoding (s/S/b/l/B/L)
  - `s`: 7-bit ASCII (default)
  - `S`: 8-bit ASCII
  - `b`: 16-bit big-endian (UTF-16BE)
  - `l`: 16-bit little-endian (UTF-16LE)
  - `B`: 32-bit big-endian (UTF-32BE)
  - `L`: 32-bit little-endian (UTF-32LE)

- `-U` / `--unicode`: UTF-8 multibyte handling (default/invalid/locale/escape/hex/highlight)
  - `default/invalid`: Treat invalid UTF-8 as non-printable
  - `locale`: Display UTF-8 characters normally
  - `escape`: Show as escape sequences (`\uXXXX`)
  - `hex`: Show as hex sequences (`<XXXX>`)
  - `highlight`: Highlighted escape sequences with ANSI colors

**Output Flags:**
- `-s` / `--output-separator`: Custom output record separator
- `-w` / `--include-all-whitespace`: Include newlines/tabs in strings

**Utility Flags:**
- `-a` / `--all`: Scan entire file (always enabled)
- `-v`, `-V` / `--version`: Display version information
- `-h` / `--help`: Show help message

Kong struct tags define flags with types, defaults, enums, and help text. Special handling for `-o` flag as alias for `-t o` using post-parse logic. Full backward compatibility with GNU strings.

### Dependencies

- **Kong** (`github.com/alecthomas/kong`): Modern, declarative command-line parser
- Go 1.25 standard library

## Code Patterns

- Standard Go Project Layout for clear code organization
- Error handling: Errors written to stderr with GNU strings-compatible format
- Dependency injection: Print function passed to extractor for testability
- Package separation: CLI, extraction logic, and output formatting are independent
