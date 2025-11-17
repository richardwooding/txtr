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

## Code Patterns

- Standard Go Project Layout for clear code organization
- Error handling: Errors written to stderr with GNU strings-compatible format
- Dependency injection: Print function passed to extractor for testability
- Package separation: CLI, extraction logic, and output formatting are independent
