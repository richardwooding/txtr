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

### Linting
```bash
# Run linters locally (matches CI environment)
set -o pipefail; golangci-lint run --timeout=5m

# Verify linter configuration
set -o pipefail; golangci-lint config verify

# Run specific linters
set -o pipefail; golangci-lint run --enable-only=govet,staticcheck
```

**Linter Configuration** (.golangci.yml):
- Tool: golangci-lint v2.6.2
- Format: v2 configuration schema
- Enabled linters: govet, staticcheck, errcheck, ineffassign, unused, misspell, revive
- Formatter: gofmt
- Integration: Runs automatically in CI on every PR

### Build Release-Matching Binaries
```bash
# Build static binary matching production releases
CGO_ENABLED=0 go build \
  -ldflags="-s -w -extldflags '-static' -X main.version=dev -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(git log -1 --format=%cd --date=iso8601)" \
  -trimpath \
  -tags netgo,osusergo \
  -o txtr ./cmd/txtr

# Verify it's static (Linux)
ldd txtr  # Should show "not a dynamic executable"

# Verify version info
./txtr --version
```

### Test Release Process
```bash
# Validate GoReleaser configuration
goreleaser check

# Test build process without releasing (creates dist/ directory)
goreleaser build --snapshot --clean

# Test full release process without publishing
goreleaser release --snapshot --clean

# Clean up test artifacts
rm -rf dist/
```

### Fuzzing
```bash
# Run a single fuzz target for 1 minute
set -o pipefail; go test -fuzz=FuzzExtractASCII -fuzztime=1m ./internal/extractor

# Run all fuzz tests with seed corpus only (fast)
set -o pipefail; go test ./internal/extractor ./internal/binary

# Run all fuzz targets in parallel (for comprehensive testing)
for target in FuzzExtractASCII FuzzExtractUTF8Aware FuzzExtractUTF16 FuzzExtractUTF32; do
  go test -fuzz=$target -fuzztime=10m ./internal/extractor &
done
wait

# View fuzz corpus
ls -R $HOME/.cache/go-build/fuzz/

# Clean corpus (if needed)
rm -rf $HOME/.cache/go-build/fuzz/
```

## CI/CD Pipeline

The project uses GitHub Actions for continuous integration and automated releases.

### Continuous Integration (.github/workflows/ci.yml)

**Triggers:** Push to main branch, all pull requests

**Jobs:**

1. **Test** - Run tests with race detection and coverage
   ```bash
   go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
   ```
   - Uploads coverage to Codecov (optional)

2. **Build** - Verify binary compiles and works
   ```bash
   go build -v ./cmd/txtr
   ./txtr --version
   ```

3. **Lint** - Run golangci-lint v2.6.2
   ```bash
   golangci-lint run --timeout=5m
   ```
   - Uses golangci-lint-action@v7
   - Configured via .golangci.yml

4. **GoReleaser Check** - Validate release configuration
   ```bash
   goreleaser check
   goreleaser build --snapshot --clean
   ```
   - Ensures release process will work
   - Uploads snapshot artifacts for main branch commits

### Release Automation (.github/workflows/release.yml)

**Triggers:** Git tags matching `v*` pattern (e.g., v2.0.1)

**Process:**
1. Checkout code with full git history (`fetch-depth: 0`)
2. Set up Go 1.25 with caching
3. Set up QEMU for multi-platform container builds
4. Set up Docker Buildx
5. Login to GitHub Container Registry (ghcr.io)
6. Run GoReleaser v2 with `--clean` flag

**Produces:**
- Pre-built binaries for 10+ OS/arch combinations
- Container images at ghcr.io/richardwooding/txtr
- GitHub Release with changelog
- Checksums (SHA256)
- SBOM (Software Bill of Materials)

**Permissions Required:**
- `contents: write` - Create GitHub Releases
- `packages: write` - Push to ghcr.io
- `id-token: write` - OIDC token for signing (future)

### Fuzzing Automation (.github/workflows/fuzz.yml)

**Triggers:**
- **Pull Requests**: Runs on code changes to Go files
- **Scheduled**: Daily at 2 AM UTC for continuous testing
- **Manual Dispatch**: On-demand with configurable fuzz time

**Jobs:**

1. **fuzz-string-extraction** - Matrix of 4 targets
   - FuzzExtractASCII
   - FuzzExtractUTF8Aware
   - FuzzExtractUTF16
   - FuzzExtractUTF32

2. **fuzz-binary-parsers** - Matrix of 4 targets
   - FuzzParseELF
   - FuzzParsePE
   - FuzzParseMachO
   - FuzzDetectFormat

3. **fuzz-summary** - Aggregates results

**Fuzz Time by Trigger:**
- Pull Requests: 2 minutes per target (quick smoke test)
- Scheduled: 1 hour per target (deep testing)
- Manual: Configurable (default 10 minutes)

**Corpus Management:**
- Cached at `~/.cache/go-build/fuzz`
- Keyed by target + commit SHA
- Restored from previous runs (incremental growth)
- Separate cache per fuzz target

**Artifacts:**
- Fuzz logs uploaded on all runs
- 7-day retention for debugging
- Separate artifacts per target

**Parallelization:**
- All 8 fuzz targets run in parallel
- Total wall time ≈ longest single target
- fail-fast: false (all targets run even if one fails)

## Release Process

### Creating a New Release

1. **Ensure CI passes** on the main branch
   - All tests passing
   - Linting clean
   - GoReleaser check successful

2. **Create and push a git tag**:
   ```bash
   # Create annotated tag with semantic version
   git tag -a v2.0.2 -m "Release v2.0.2

   ## What's New
   - Feature A
   - Bug fix B
   "

   # Push tag to trigger release workflow
   git push origin v2.0.2
   ```

3. **GitHub Actions automatically**:
   - Runs tests
   - Builds binaries for all platforms
   - Creates GitHub Release with generated changelog
   - Publishes container images to ghcr.io/richardwooding/txtr
   - Tags images as: latest, v2.0.2, v2.0, v2

4. **Monitor the release**:
   - Visit https://github.com/richardwooding/txtr/actions
   - Check for "Release" workflow completion
   - Verify artifacts on https://github.com/richardwooding/txtr/releases

5. **Verify container images**:
   ```bash
   docker pull ghcr.io/richardwooding/txtr:v2.0.2
   docker run --rm ghcr.io/richardwooding/txtr:v2.0.2 --version
   ```

### Release Versioning

Follow semantic versioning (semver):
- **Major** (v2.0.0): Breaking changes
- **Minor** (v2.1.0): New features, backward compatible
- **Patch** (v2.0.1): Bug fixes, backward compatible

### Rollback

To rollback a release:
1. Delete the git tag: `git tag -d v2.0.2 && git push origin :refs/tags/v2.0.2`
2. Delete the GitHub Release via UI
3. Delete container images via GitHub Packages UI (if needed)

## GoReleaser Configuration (.goreleaser.yaml)

The project uses GoReleaser v2.12.7 for automated builds and releases.

### Key Configuration

**Builds:**
- Main: `./cmd/txtr`
- Binary: `txtr`
- CGO: Disabled (`CGO_ENABLED=0`)
- Platforms: linux, darwin, windows, freebsd
- Architectures: amd64, arm64, armv6, armv7
- Static linking: `-extldflags "-static"`
- Stripping: `-s -w` (remove debug info)
- Reproducible: `-trimpath` flag
- Tags: `netgo`, `osusergo` (pure Go networking)
- Version injection:
  - `-X main.version={{.Version}}`
  - `-X main.commit={{.ShortCommit}}`
  - `-X main.date={{.CommitDate}}`
  - `-X main.builtBy=goreleaser`

**Archives:**
- Format: tar.gz for Unix, zip for Windows
- Name: `txtr_{{.Version}}_{{.Os}}_{{.Arch}}`
- Includes: LICENSE, README.md, CLAUDE.md

**Container Images (Ko):**
- Base: `cgr.dev/chainguard/static` (minimal ~2MB image)
- Registry: `ghcr.io/richardwooding/txtr`
- Platforms: linux/amd64, linux/arm64
- Tags: latest, version, major.minor, major
- SBOM: SPDX format included
- OCI labels: Full metadata for GitHub integration
- Build method: Ko (no Dockerfile needed)

**Changelog:**
- Generated from conventional commits
- Groups: New Features, Bug Fixes, Performance, Other
- Excludes: docs, test, chore commits

**GitHub Release:**
- Draft: false (published immediately)
- Prerelease: auto-detected from version
- Mode: append (adds to existing release if re-run)
- Header: Installation instructions
- Footer: Full changelog link

### Static Binary Compilation

All release binaries are fully static with zero runtime dependencies:

**Configuration:**
- `CGO_ENABLED=0`: No C dependencies
- `-extldflags "-static"`: Force static linking
- `-trimpath`: Remove build paths for reproducibility
- Tags: `netgo`, `osusergo` (pure Go networking and user lookups)

**Benefits:**
- Works on any system without libc or other dependencies
- Consistent behavior across distributions
- Smaller attack surface (no dynamic library vulnerabilities)
- Single-file deployment
- Binary size: ~3.8MB (stripped)

**Verification:**
```bash
# Check if binary is static (Linux)
ldd dist/txtr_linux_amd64/txtr
# Expected: "not a dynamic executable"

# Check if binary is static (macOS)
otool -L dist/txtr_darwin_arm64/txtr
# Expected: only system libraries (libSystem, libresolv)
```

## Container Images

Container images are built automatically via Ko integration with GoReleaser.

### Configuration

**Base Image:** `cgr.dev/chainguard/static`
- Minimal distroless image (~2MB)
- No shell, no package manager
- Security-focused (minimal attack surface)
- Perfect for static Go binaries

**Registry:** ghcr.io/richardwooding/txtr

**Platforms:**
- linux/amd64
- linux/arm64

**Tags:**
- `latest` - Most recent release
- `v2.0.1` - Specific version
- `v2.0` - Minor version (updated with patches)
- `v2` - Major version (updated with minors)

**Metadata:**
- SBOM: SPDX format included in image
- OCI Labels: GitHub integration metadata
  - org.opencontainers.image.source
  - org.opencontainers.image.version
  - org.opencontainers.image.revision
  - org.opencontainers.image.created
  - org.opencontainers.image.licenses

### Using Ko

Ko is embedded in GoReleaser and requires no separate installation. It builds container images without Dockerfiles:

**Advantages:**
- No Dockerfile needed
- Fast builds (uses Go build cache)
- Produces minimal images
- Multi-platform support built-in
- SBOM generation automatic

**How It Works:**
1. Ko builds the Go binary with same flags as regular builds
2. Creates minimal container with binary as entrypoint
3. Adds metadata and labels
4. Pushes to registry

## Version Information

Build metadata is embedded at compile time via ldflags.

### Variables (cmd/txtr/main.go)

```go
var (
    version = "dev"      // Set by: -X main.version={{.Version}}
    commit  = "none"     // Set by: -X main.commit={{.ShortCommit}}
    date    = "unknown"  // Set by: -X main.date={{.CommitDate}}
    builtBy = "unknown"  // Set by: -X main.builtBy=goreleaser
)
```

### Output Format

```bash
$ txtr --version
txtr v2.0.1
  commit: cec6729
  built: 2025-11-17T12:00:00Z
  built by: goreleaser
GNU strings compatible utility written in Go
```

### Development Builds

When building locally without ldflags, defaults are used:
```bash
$ go build -o txtr ./cmd/txtr
$ ./txtr --version
txtr dev
  commit: none
  built: unknown
  built by: unknown
GNU strings compatible utility written in Go
```

## Statistics Mode

The `--stats` flag outputs aggregated statistics instead of individual strings, useful for quick file analysis and triage.

### Usage

```bash
# Basic statistics
txtr --stats binary.exe

# Per-file statistics for multiple files
txtr --stats --stats-per-file file1.bin file2.bin

# Statistics with pattern filtering
txtr --stats -m '\S+@\S+' malware.exe

# Works with binary parsing
txtr --stats -d binary.exe
```

### Architecture

**Package**: `internal/stats`

**Core Types:**
- `Statistics` struct: Aggregates string metrics during extraction
  - Count fields: TotalStrings, TotalBytes, MinLength, MaxLength
  - Distribution maps: EncodingCounts, LengthBuckets
  - File metadata: Filename, BinaryFormat, Sections
  - Filter tracking: UnfilteredCount, FilteredCount
  - Longest strings: Top 5 with offsets

**Key Methods:**
- `New(minLength int)`: Creates initialized Statistics instance
- `Add(str []byte, filename string, offset int64, config Config)`: Collects string (matches printFunc signature)
- `AddUnfiltered()`: Tracks strings before filtering
- `Format(w io.Writer)`: Human-readable output with commas and percentages
- `ToJSON()`: Structured JSON output (future feature)
- `Merge(other *Statistics)`: Combines statistics for parallel aggregation

**Integration Points:**
- `cmd/txtr/main.go`:
  - `processWithStats()`: Routes to statistics mode
  - `makeFilterTrackingFunc()`: Wraps Add() to track filter statistics
  - `processFileWithStatsAndBinaryParsing()`: Handles binary format detection with statistics
- `internal/extractor`: Statistics.Add() passed as printFunc callback
- Parallel processing: Worker pool aggregates per-file statistics via Merge()

**Output Features:**
- Human-readable formatting with thousand separators
- Percentage calculations with 1 decimal place
- Encoding classification: 7-bit ASCII, 8-bit, UTF-8, UTF-16, UTF-32
- Length buckets: 4-10, 11-50, 51-100, 100+ characters
- Top 5 longest strings with offset and preview
- Filter statistics: Shows extraction count and match percentage

**Testing:**
- Unit tests: 18 test functions covering all methods
- Edge cases: Empty files, single string, zero division
- Format verification: Number formatting, percentages, encoding names
- Merge testing: Parallel aggregation correctness

**Use Cases:**
- Quick triage: "Is this file interesting?"
- Binary comparison: "How do these firmware versions differ?"
- Pattern analysis: "How many URLs are embedded?"
- Encoding distribution: "Is this file ASCII or Unicode?"
- Per-file analysis: Compare multiple files side-by-side

## Architecture

This project follows the Standard Go Project Layout with clear separation of concerns:

### Directory Structure

```
txtr/
├── cmd/txtr/           # Application entry point
│   └── main.go         # CLI using Kong library
├── internal/           # Private application code
│   ├── binary/         # Binary format parsing
│   │   ├── parser.go
│   │   ├── parser_test.go
│   │   └── fuzz_test.go    # Fuzzing for binary parsers
│   ├── extractor/      # String extraction logic
│   │   ├── extractor.go
│   │   ├── extractor_test.go
│   │   ├── encoding_test.go
│   │   └── fuzz_test.go    # Fuzzing for string extraction
│   └── printer/        # Output formatting logic
│       ├── printer.go
│       ├── printer_test.go
│       ├── color.go         # Color detection and ANSI codes
│       ├── color_test.go
│       ├── json.go          # JSON output format
│       └── json_test.go
├── testdata/           # Test data and fuzz corpus
│   └── fuzz/           # Seed corpus for fuzzing
│       ├── FuzzExtractASCII/
│       ├── FuzzExtractUTF8Aware/
│       ├── FuzzExtractUTF16/
│       └── FuzzExtractUTF32/
├── .github/
│   └── workflows/
│       ├── ci.yml      # Continuous integration
│       ├── release.yml # Automated releases
│       └── fuzz.yml    # Fuzzing automation
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
- `ColorMode` type: Enum for color output mode (Auto/Always/Never)
- `Config` struct: Configuration for extraction options (MinLength, PrintFileName, Radix, PrintOffset, Encoding, Unicode, OutputSeparator, IncludeAllWhitespace, ScanAll, ColorMode)
- `ExtractStrings()`: Router function that dispatches to encoding-specific extractors
- `extractASCII()`: Extracts 7-bit or 8-bit ASCII strings (delegates to UTF-8 aware version if Unicode mode set)
- `extractUTF8Aware()`: UTF-8 aware extraction with multibyte sequence validation and special display modes
- `extractUTF16()`: Extracts UTF-16 encoded strings (handles surrogate pairs)
- `extractUTF32()`: Extracts UTF-32 encoded strings
- `IsPrintable()`: Determines if a byte is in printable ASCII range (32-126)
- `isPrintableASCII()`: Enhanced printable check with 8-bit and whitespace support
- `isPrintableRune()`: Checks if a Unicode rune is printable

**internal/printer/printer.go** - Output formatting:
- `PrintString()`: Formats and outputs strings with optional filename prefix, offset, custom separator, and colors

**internal/printer/color.go** - Color detection and ANSI codes:
- `ColorMode`: Type definition (Auto/Always/Never) moved to extractor.Config to avoid circular imports
- `ShouldUseColor()`: Determines if colors should be used based on mode, NO_COLOR env var, and TTY detection
- `isTerminal()`: Checks if output is a TTY using os.ModeCharDevice (cross-platform)
- ANSI color code constants: Cyan, Yellow, Green, Magenta, Dim, Bold, Reset
- `ColorString()`: Wraps strings with ANSI color codes when enabled

**internal/printer/json.go** - JSON output formatting:
- `JSONPrinter`: Collects strings and outputs in structured JSON format
- `StringResult`: Represents a single extracted string with metadata
- `JSONOutput`: Complete output structure with files and summary
- `NewJSONPrinter()`: Creates a new JSON printer with configuration
- `SetFileInfo()`: Sets file, format, and section metadata
- `PrintString()`: Collects string results (implements printFunc signature)
- `Flush()`: Outputs all collected results as JSON with summary statistics

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

**internal/printer/color_test.go**:
- `TestShouldUseColor`: Color mode behavior and NO_COLOR environment variable
- `TestColorString`: ANSI code wrapping logic
- `TestIsTerminal`: TTY detection with nil and regular files
- `TestColorModeConstants`: ColorMode enum value validation
- `TestANSIColorCodes`: ANSI code constant verification

**internal/printer/json_test.go**:
- `TestJSONPrinter`: Tests basic JSON output with various configurations
- `TestJSONPrinterWithFileInfo`: Tests JSON output with file metadata
- `TestGetEncodingName`: Tests encoding name conversion
- `TestJSONPrinterNilWriter`: Tests default writer behavior
- `TestJSONOutputValid`: Tests JSON structure and validity

### Fuzzing

The project uses Go's native fuzzing (introduced in Go 1.18) for property-based security testing.

**Fuzz Tests Implemented:**

**internal/extractor/fuzz_test.go** - String extraction fuzzing (330 lines):
- `FuzzExtractASCII`: Tests 7-bit and 8-bit ASCII extraction
  - Seed corpus: 5 files covering boundary chars, control chars, high bytes
  - Invariants: no panics, minimum length enforcement, printable validation, deterministic behavior
  - Execution rate: ~3.8M execs/10s

- `FuzzExtractUTF8Aware`: Tests UTF-8 multibyte character handling
  - Seed corpus: 6 files with Chinese, Russian, emoji, invalid UTF-8, overlong encodings, surrogates
  - Timeout protection: 1 second to catch infinite loops
  - Invariants: valid UTF-8 output in locale mode
  - CVE coverage: CVE-2023-26302 (invalid UTF-8), CVE-2024-2689 (overlong encodings)
  - Execution rate: ~2.3M execs/10s

- `FuzzExtractUTF16`: Tests UTF-16 BE/LE extraction
  - Seed corpus: 5 binary files with valid UTF-16, surrogate pairs, incomplete sequences
  - Timeout protection: 1 second for CVE-2020-14040 (infinite loop in UTF-16 decoder)
  - Invariants: valid UTF-8 output, valid runes, no crashes
  - Execution rate: ~2.9M execs/10s

- `FuzzExtractUTF32`: Tests UTF-32 BE/LE extraction
  - Seed corpus: 6 binary files with valid UTF-32, invalid runes (>0x10FFFF), surrogates
  - Invariants: valid UTF-8 output, valid runes, no surrogate range (0xD800-0xDFFF)
  - Execution rate: ~4.8M execs/10s

**internal/binary/fuzz_test.go** - Binary parser fuzzing (267 lines):
- `FuzzParseELF`: Tests ELF (Linux/Unix) binary parsing
  - Seed corpus: 6 entries with valid ELF headers, short headers, invalid data
  - Invariants: no panics, valid section data, non-negative offsets
  - Execution rate: ~805/sec

- `FuzzParsePE`: Tests PE (Windows) binary parsing
  - Seed corpus: 6 entries with DOS stubs, MZ headers, PE signatures
  - Invariants: no panics, only .data/.rdata sections returned
  - Execution rate: ~539/sec

- `FuzzParseMachO`: Tests Mach-O (macOS/iOS) binary parsing
  - Seed corpus: 8 entries with 32/64-bit BE/LE magics, universal binary
  - Invariants: no panics, only known data sections returned
  - Execution rate: ~1019/sec

- `FuzzDetectFormat`: Tests binary format auto-detection
  - Seed corpus: 9 entries with all magic signatures (ELF, PE, Mach-O)
  - Invariants: deterministic detection, valid format values
  - Execution rate: ~324/sec

**Seed Corpus Structure:**
```
testdata/fuzz/
├── FuzzExtractASCII/       # 5 files: ASCII patterns, boundary cases
├── FuzzExtractUTF8Aware/   # 6 files: UTF-8, invalid sequences
├── FuzzExtractUTF16/       # 5 files: UTF-16 BE/LE binary data
└── FuzzExtractUTF32/       # 6 files: UTF-32 BE/LE binary data
```

**Key Features:**
- Property-based invariant checking (all outputs must satisfy constraints)
- Timeout protection (1s) for infinite loop detection
- Input size limits (10MB) to prevent resource exhaustion
- Comprehensive panic recovery with input dumping
- CVE regression testing (CVE-2020-14040, CVE-2023-26302, CVE-2024-2689)
- Deterministic behavior validation
- Corpus caching in CI/CD for incremental growth

**Security Benefits:**
- Discovers edge cases that manual tests miss
- Growing corpus improves coverage over time
- Continuous fuzzing in CI catches regressions
- Tests against known vulnerabilities in similar tools

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
- `-j` / `--json`: Output results in JSON format for automation
- `--color`: When to use colored output (auto/always/never, default: auto)

**Performance Flags:**
- `-P` / `--parallel`: Number of parallel workers (default: 0)
  - `0`: Auto-detect CPUs (default, enables automatic parallelism)
  - `1`: Sequential processing (disables parallelism)
  - `N`: Use N parallel workers

**Utility Flags:**
- `-a` / `--all`: Scan entire file (always enabled)
- `-v`, `-V` / `--version`: Display version information
- `-h` / `--help`: Show help message

Kong struct tags define flags with types, defaults, enums, and help text. Special handling for `-o` flag as alias for `-t o` using post-parse logic. Full backward compatibility with GNU strings.

### Dependencies

**Runtime:**
- **Kong v1.7.0** (`github.com/alecthomas/kong`): Command-line parser (only external dependency)
- Go 1.25 standard library

**Development/Build:**
- **GoReleaser v2.12.7**: Automated build and release management
- **Ko** (embedded in GoReleaser): Container image builder without Dockerfiles
- **golangci-lint v2.6.2**: Code linting and static analysis
- **Docker/Podman**: For building and testing container images
- **QEMU**: For multi-platform container builds (CI only)

**Key Points:**
- Zero CGO dependencies - fully static binaries
- No libc or other dynamic dependencies required at runtime
- Pure Go implementation enables cross-compilation to 10+ platforms

## JSON Output Format

The `--json` flag enables structured JSON output for automation and tool integration.

### Architecture

**Design Pattern:**
- Collector pattern: JSONPrinter collects all strings before outputting
- Implements same `printFunc` signature as regular PrintString
- Buffers results in memory, then outputs complete JSON structure on Flush()

**Output Structure:**
```json
{
  "files": [
    {
      "file": "binary.exe",
      "format": "PE",
      "sections": [".data", ".rdata"],
      "strings": [
        {
          "value": "Hello World",
          "offset": 1024,
          "offset_hex": "0x400",
          "length": 11,
          "encoding": "ascii-7bit"
        }
      ]
    }
  ],
  "summary": {
    "total_strings": 42,
    "total_bytes": 1234,
    "min_length": 4,
    "encoding": "ascii-7bit"
  }
}
```

### Usage Examples

```bash
# Basic JSON output
txtr --json file.bin

# With jq filtering - strings longer than 20 chars
txtr --json file.bin | jq '.files[0].strings[] | select(.length > 20)'

# Extract offsets
txtr --json file.bin | jq '.files[0].strings[].offset_hex'

# Count total strings
txtr --json file.bin | jq '.summary.total_strings'

# Analyze binary format
txtr --json -d binary.exe | jq '.files[0].format'

# Get section information
txtr --json -d binary.exe | jq '.files[0].sections[]'
```

### Implementation Details

**File Processing:**
- Currently processes only the first file for JSON output
- Stdin input supported (file field omitted from output)
- Works with all encodings (ASCII, UTF-16, UTF-32)
- Works with `-d` flag to include format and section metadata

**Encoding Names:**
- `s` → `ascii-7bit`
- `S` → `ascii-8bit`
- `b` → `utf-16be`
- `l` → `utf-16le`
- `B` → `utf-32be`
- `L` → `utf-32le`

## Colored Output

The `--color` flag enables ANSI colored output for improved terminal readability.

### Color Modes

- **`auto`** (default): Automatically detects if stdout is a TTY
  - Enables colors when output is to a terminal
  - Disables colors when piped or redirected
  - Respects `NO_COLOR` environment variable

- **`always`**: Forces colored output regardless of TTY detection
  - Useful for piping to `less -R` or similar pagers
  - Still respects `NO_COLOR` environment variable

- **`never`**: Disables colored output completely

### Color Scheme

| Element | Color | ANSI Code | Applied To |
|---------|-------|-----------|------------|
| Filename | Bold Cyan | `\x1b[1m\x1b[36m` | When `-f` flag used |
| Offset | Yellow | `\x1b[33m` | When `-t` flag used |
| 7-bit ASCII | Default | None | Standard ASCII strings |
| 8-bit ASCII | Magenta | `\x1b[35m` | High-byte strings (`-e S`) |
| UTF-8/UTF-16/UTF-32 | Green | `\x1b[32m` | Unicode strings |
| Custom separator | Dim | `\x1b[2m` | Non-newline separators (`-s`) |

### Implementation

**Color Detection (`internal/printer/color.go`):**
1. `ShouldUseColor()` checks three conditions in order:
   - NO_COLOR environment variable (https://no-color.org/) - overrides everything
   - ColorMode setting (never/always/auto)
   - TTY detection via `isTerminal()` (only for auto mode)

2. `isTerminal()` uses `os.ModeCharDevice`:
   - Cross-platform (works on Unix, Linux, macOS, Windows)
   - Checks if file mode has ModeCharDevice bit set
   - Returns false for pipes, redirects, and regular files

3. `ColorString()` wraps text with ANSI codes:
   - Only when colors are enabled
   - Automatically adds reset code at the end
   - No-op when colors are disabled or string is empty

**Color Application (`internal/printer/printer.go`):**
- `PrintString()` applies colors conditionally based on `config.ColorMode`
- Colors are encoding-aware:
  - Detects 8-bit ASCII via `config.Encoding == "S"`
  - Detects UTF-8 mode via `config.Unicode` setting
  - Detects UTF-16/UTF-32 via encoding flags
- Filename and offset always get their respective colors when present
- Custom separators are dimmed if not newline

### Usage Examples

```bash
# Auto-detect terminal (default)
txtr file.bin
txtr --color=auto file.bin

# Force colors for piping to pager
txtr --color=always file.bin | less -R

# Disable colors
txtr --color=never file.bin

# Respect NO_COLOR environment variable
NO_COLOR=1 txtr file.bin

# Colored output with all metadata
txtr -f -t x --color=always file.bin
```

### Testing

**Color Tests (`internal/printer/color_test.go`):**
- `TestShouldUseColor`: Tests all three color modes and NO_COLOR behavior
  - Uses `t.Setenv()` for safe environment variable manipulation
  - Prevents race conditions in parallel test execution
  - Skips TTY detection test with `t.Skip()` (not reliably testable)

- `TestColorString`: Verifies ANSI code wrapping logic
- `TestIsTerminal`: Tests TTY detection with nil and regular files
- `TestColorModeConstants`: Validates ColorMode enum values
- `TestANSIColorCodes`: Ensures ANSI codes are correct

**Key Testing Practices:**
- Use `t.Setenv()` instead of manual `os.Setenv/Unsetenv`
  - Automatic cleanup after test
  - Prevents parallel execution when env vars are modified
  - Cleaner and safer code
- Use `t.Skip()` for untestable scenarios (like TTY detection)
  - Makes test output clearer
  - Documents why certain cases aren't fully tested

## Pattern Filtering

The `-m/--match`, `-M/--exclude`, and `-i/--ignore-case` flags enable regex-based string filtering.

### Architecture

**Design Pattern:**
- Compile patterns once at startup (fail fast on invalid regex)
- Filter strings inline during extraction (before calling printFunc)
- Exclude patterns take precedence over match patterns

**Key Components:**

1. **filter.go** (internal/extractor/filter.go):
   - `CompilePatterns(patterns []string, ignoreCase bool) ([]*regexp.Regexp, error)`
     - Compiles regex patterns with optional case-insensitive flag ((?i) prefix)
     - Returns error with pattern number and original pattern for debugging
   - `ShouldPrintString(str []byte, config Config) bool`
     - Checks exclude patterns first (any match = false)
     - Checks match patterns (if any defined, at least one must match)
     - Returns true if no patterns defined (no filtering)

2. **Config fields** (internal/extractor/extractor.go):
   - `MatchPatterns []*regexp.Regexp` - Inclusion filter
   - `ExcludePatterns []*regexp.Regexp` - Exclusion filter (blacklist)

3. **CLI flags** (cmd/txtr/main.go):
   - `MatchPatterns []string` - Can be specified multiple times
   - `ExcludePatterns []string` - Can be specified multiple times
   - `IgnoreCase bool` - Applies to both match and exclude

4. **Pattern compilation** (cmd/txtr/main.go):
   - Compiles patterns before config creation
   - Exits with error on invalid regex (clear error message with pattern number)

5. **Integration points** (internal/extractor/extractor.go):
   - `extractASCII()` - 2 call sites
   - `extractUTF8Aware()` - 4 call sites
   - `extractUTF16()` - 2 call sites
   - `extractUTF32()` - 2 call sites
   - `extractASCIIFromBytes()` - 2 call sites
   - `extractUTF16FromBytes()` - 2 call sites
   - `extractUTF32FromBytes()` - 2 call sites

### Testing

**Unit Tests** (internal/extractor/filter_test.go):
- `TestCompilePatterns` - 8 test cases covering valid/invalid patterns
- `TestCompilePatternsIgnoreCase` - Case-sensitive vs case-insensitive behavior
- `TestShouldPrintString` - 14 test cases covering all filtering logic combinations
- `TestShouldPrintStringSpecialPatterns` - 6 common patterns (URL, email, IP, etc.)

**Fuzz Test** (internal/extractor/fuzz_test.go):
- `FuzzFilterPatterns` - Tests random inputs with ReDoS protection
- Seed corpus: 5 files with common patterns (email, URL, IP, error, exclude)
- In-code seed data: 10 f.Add() calls covering additional edge cases
- Invariants:
  - No panics during filtering
  - Deterministic behavior (same input = same result)
  - Exclude always overrides match
  - 1-second timeout protection against ReDoS attacks
- Execution rate: ~150K-280K execs/sec

**CI/CD Integration** (.github/workflows/fuzz.yml):
- Added `FuzzFilterPatterns` to matrix (9 total fuzz targets)
- Runs on PRs (2min), daily (1hr), and manual dispatch

### Usage Examples

```bash
# Extract email addresses
txtr -m '\S+@\S+\.\S+' file.bin

# Extract URLs
txtr -m 'https?://\S+' malware.exe

# Find error messages (case-insensitive)
txtr -m -i 'error|warning|fatal' app.log

# Exclude debug symbols
txtr -M 'debug_.*|__.*' binary.exe

# Multiple patterns (OR logic)
txtr -m '\S+@\S+' -m 'https?://\S+' file.bin

# Combine match and exclude
txtr -m '\S+@\S+' -M 'spam.*' file.bin
```

### Common Patterns

- Email: `\S+@\S+\.\S+`
- URL: `https?://\S+`
- IP address: `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`
- Error messages: `(?i)(error|warning|fatal)`
- Hex addresses: `0x[0-9a-fA-F]+`
- UUID: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

### Implementation Notes

- Filtering happens after minimum length check (more efficient)
- Patterns are compiled once at startup (not per-string)
- Timeout protection in fuzz tests prevents ReDoS in CI/CD
- Exclude precedence ensures security (can't bypass exclusions)
- Case-insensitive flag adds `(?i)` prefix to pattern

## Parallel Processing

The `-P/--parallel` flag enables concurrent file processing using a worker pool pattern.

### Architecture

**Design Pattern:**
- Worker pool with goroutines and channels
- Job distribution via buffered channel
- Result collection with index-based ordering
- Per-file error handling (one failure doesn't stop batch)

**Key Components:**

1. **job struct** (cmd/txtr/main.go:48-51):
   - Contains filename and index for ordering
   - Sent to workers via jobs channel

2. **result struct** (cmd/txtr/main.go:54-58):
   - Contains index, output string, and error
   - Collected from workers via results channel

3. **processFilesParallel()** (cmd/txtr/main.go:358-427):
   - Creates worker pool with configurable size
   - Each worker processes jobs from channel
   - Captures output to bytes.Buffer per file
   - Uses PrintStringToWriter to collect formatted output
   - Results collected in indexed array for ordering
   - Prints results in original file order

4. **processFileWithBinaryParsingToWriter()** (cmd/txtr/main.go:429-499):
   - Helper for binary parsing in parallel mode
   - Writes output to provided buffer instead of stdout
   - Supports -d flag with parallel processing

### Behavior

**When parallel mode is used:**
- Multiple files (`len(files) > 1`) AND workers > 1
- Default workers: runtime.NumCPU() (auto-detect)
- Single file: Always sequential (no parallelism overhead)
- stdin: Always sequential

**Output ordering:**
- Files appear in same order as input arguments
- Achieved via indexed result collection
- No interleaving of file outputs

**Error handling:**
- Errors printed to stderr per file
- Other files continue processing
- Exit code reflects overall success

**JSON mode:**
- Supports parallel processing for multiple files
- Uses `processFilesParallelJSON()` for concurrent extraction
- Results collected and ordered before final JSON output
- Failed files included in output with error field and empty strings array

### Performance

**Expected speedup:**
- 2 cores: ~1.8x
- 4 cores: ~3.5x
- 8 cores: ~6-7x

**Factors affecting speedup:**
- File I/O vs CPU ratio
- Filesystem caching
- File sizes (small files = less benefit)
- Number of files vs number of cores

### Testing

**Test coverage** (cmd/txtr/parallel_test.go):
- `TestParallelProcessingOrder`: Verifies output ordering
- `TestParallelProcessingErrorHandling`: Tests per-file error handling
- `TestSequentialVsParallel`: Ensures output consistency

### Usage Examples

```bash
# Automatic parallelism (default, uses all CPUs)
txtr -f *.bin

# Control worker count
txtr -P 4 -f *.bin

# Force sequential
txtr -P 1 -f *.bin

# With all flags
txtr -P 8 -f -t x --color=always *.exe
```

## Code Patterns

- Standard Go Project Layout for clear code organization
- Error handling: Errors written to stderr with GNU strings-compatible format
- Dependency injection: Print function passed to extractor for testability
- Package separation: CLI, extraction logic, and output formatting are independent
- Collector pattern: JSONPrinter buffers results before output for structured format
- Worker pool pattern: Parallel file processing with ordered output and per-file error handling
