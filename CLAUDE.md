# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

`txtr` is a GNU strings clone written in Go 1.26. Extracts printable strings from binaries with Standard Go Project Layout.

## Development Commands

```bash
# Build
go build -o txtr ./cmd/txtr

# Test
set -o pipefail; go test -v ./...

# Benchmark
set -o pipefail; go test -bench=. -benchmem -run=^$ ./...

# Lint
set -o pipefail; golangci-lint run --timeout=5m

# Fuzz
set -o pipefail; go test -fuzz=FuzzExtractASCII -fuzztime=1m ./internal/extractor

# Release (test)
goreleaser check
goreleaser build --snapshot --clean
```

## CI/CD

**Workflows:**
- `.github/workflows/ci.yml`: Test, build, lint, benchmark on PRs/main
- `.github/workflows/release.yml`: Automated releases on `v*` tags
- `.github/workflows/fuzz.yml`: Fuzzing (PRs: 3min, daily: 1hr)

**Release Process:**
```bash
git tag -a v2.0.2 -m "Release v2.0.2"
git push origin v2.0.2
```
Produces binaries (10+ platforms), container images (ghcr.io/richardwooding/txtr), checksums, SBOM.

## Architecture

```
txtr/
├── cmd/txtr/main.go        # CLI entry (Kong parser)
├── internal/
│   ├── binary/             # ELF/PE/Mach-O parsing
│   ├── extractor/          # String extraction (ASCII/UTF-8/UTF-16/UTF-32)
│   ├── printer/            # Output (text/JSON/color/triage)
│   ├── stats/              # Statistics mode
│   ├── triage/             # Entropy + classification + secret detection
│   └── archive/            # Recursive archive walking (zip/tar/gz/bz2/xz/zst/ar)
├── testdata/fuzz/          # Fuzz corpus
└── .github/workflows/      # CI/CD
```

**Core Components:**
- `extractor.ExtractStrings()`: Dispatches to encoding-specific extractors
- `printer.PrintString()`: Formats output with colors/offsets
- `printer.JSONPrinter`: Collector pattern for structured output
- `printer.TriagePrinter`: Annotated triage output (tags each string by secret/entropy/category)
- `stats.Statistics`: Aggregates metrics for `--stats` mode (incl. triage rollup)
- `triage.Classify()`: Pure analysis (entropy + classification + secret rules); no internal deps, so `printer` and `stats` both reuse it without import cycles
- `archive.Walk()`: Recurses archives/compressed containers, emitting each leaf file as a stream (`!/`-separated virtual paths) with depth + decompressed-size bomb guards; depends only on stdlib + pure-Go xz/zstd
- `extractFile()` (cmd/txtr): single routing seam used by every mode — recursion > data-only > whole-file — so `--recurse` composes with text/triage/stats/JSON

**Key Patterns:**
- Dependency injection: printFunc callback for testability
- Worker pool: Parallel file processing with ordered output
- Dual I/O: Auto mmap optimization (2-3x faster) with buffered fallback

## CLI Flags

**Encodings:** `-e s/S/b/l/B/L` (7-bit/8-bit ASCII, UTF-16BE/LE, UTF-32BE/LE)
**UTF-8 modes:** `-U locale/escape/hex/highlight`
**Filtering:** `-m <pattern>`, `-M <exclude>`, `-i` (case-insensitive)
**Output:** `-f` (filename), `-t o/d/x` (offset), `--color auto/always/never`, `-j` (JSON), `--stats`
**Triage:** `--triage` (entropy/classification/secret tags), `--secrets` (secrets+high-entropy only), `--min-entropy <float>` (default 4.5)
**Recursion:** `-r`/`--recurse` (descend into zip/apk/tar/gz/bz2/xz/zst/deb), `--max-depth` (default 8), `--max-decompressed-size` (default 2 GiB)
**Parallel:** `-P N` (0=auto CPUs, 1=sequential)
**Performance:** `--no-mmap`, `--mmap-threshold` (default: 1MB)

## Key Features

**mmap Optimization:** 2x faster for files ≥1MB (automatic, transparent fallback)
**Parallel Processing:** Auto-detects CPUs, maintains output order
**Pattern Filtering:** Regex match/exclude with ReDoS protection
**Statistics Mode:** `--stats` for quick triage (encoding dist, length buckets, top-5 longest)
**Security Triage:** `--triage`/`--secrets` for entropy scoring, content classification, and secret detection (composes with `-j` and `--stats`)
**Archive Recursion:** `--recurse` descends into zip/apk/tar/gz/bz2/xz/zst/deb containers, scanning each member with `!/` virtual paths and decompression-bomb guards
**JSON Output:** `--json` for automation (works with jq)
**Color Output:** Auto TTY detection, respects NO_COLOR env var
**Fuzzing:** 11 fuzz targets (string extraction, binary parsing, filtering, triage classification, archive walking) with CVE coverage

## Testing

**Unit tests:** 18 test files covering all packages
**Fuzz tests:** 11 targets (3min on PR, 1hr daily, manual dispatch)
**Benchmarks:** 130+ benchmarks across all components, incl. triage entropy/classify/secret-scan and a plain-vs-triage overhead comparison (CI tracked, 30-day retention)

**Coverage:**
- CVE-2020-14040 (UTF-16 infinite loop)
- CVE-2023-26302 (invalid UTF-8)
- CVE-2024-2689 (overlong encodings)

## Performance Targets

- ASCII extraction: ~400-430 MB/s
- UTF-16 extraction: ~130 MB/s
- mmap speedup: 2-3x for large files
- Parallel speedup: ~1.8x (2 cores), ~6-7x (8 cores)
- Binary format detection: <10µs
- Triage entropy: ~470 MB/s (256B) to ~2.5 GB/s (4KB); zero-alloc
- Triage per-string overhead: ~2.4µs/string on a realistic mix (~3.9x plain output); short strings (<12B) skip the secret-rule scan, so the cost concentrates on long/interesting strings

## Dependencies

**Runtime:** Kong v1.14.0, golang.org/x/exp/mmap, ulikunitz/xz + klauspost/compress (pure-Go xz/zstd for `--recurse`), Go 1.26 stdlib
**Build:** GoReleaser v2.12.7, Ko (containerized), golangci-lint v2.9.0
**Key:** Zero CGO, fully static binaries (~7MB; grew from ~3.8MB with the pure-Go zstd/xz decoders added for `--recurse`)

## Build Configuration

**Static binaries:** `CGO_ENABLED=0`, `-extldflags "-static"`, `-trimpath`, tags: `netgo,osusergo`
**Version injection:** `-X main.version/commit/date/builtBy` via ldflags
**Containers:** Ko builds images (no Dockerfile), base: `cgr.dev/chainguard/static` (~2MB)
**Platforms:** linux/darwin/windows/freebsd × amd64/arm64/armv6/armv7

## Development Tips

- Use `--stats` for quick file triage before full extraction
- Pattern filtering (`-m/-M`) happens after min-length check (efficient)
- Parallel mode (`-P 0`) enabled by default for multiple files
- mmap auto-activates for files ≥1MB (disable with `--no-mmap`)
- JSON mode (`-j`) buffers all strings before output (memory consideration)
- Fuzz corpus grows incrementally in CI (cached by target+commit)

## Common Patterns

```bash
# Extract emails
txtr -m '\S+@\S+\.\S+' file.bin

# Quick triage
txtr --stats malware.exe

# Security triage: secrets, high-entropy blobs, classification
txtr --triage firmware.bin
txtr --secrets firmware.bin
txtr --triage -j firmware.bin | jq '.files[0].strings[] | select((.secrets|length)>0)'

# Archive recursion: descend into containers, then triage every member
txtr -r --secrets app.apk
txtr -r --triage pkg.deb          # ar -> data.tar.xz -> ELF binaries
txtr -r -j firmware.tar.gz | jq '.files[0].strings[] | {file, value}'

# Parallel processing with metadata
txtr -P 8 -f -t x --color=always *.bin

# JSON + jq filtering
txtr -j file.bin | jq '.files[0].strings[] | select(.length > 20)'

# Binary sections with stats
txtr -d --stats binary.exe
```
