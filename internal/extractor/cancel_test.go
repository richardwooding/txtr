package extractor

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// makeScannable builds an input large enough that the extraction loops cross
// several ctxChunk (64 KiB) cancellation-poll boundaries.
func makeScannable() []byte {
	var b bytes.Buffer
	line := []byte("AAAAAAAAAAAAAAAA\x00") // printable run + delimiter
	for b.Len() < 4*ctxChunk {
		b.Write(line)
	}
	return b.Bytes()
}

func TestExtractStringsCancelledUpfront(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before any work

	cfg := Config{MinLength: 4, Encoding: "s"}
	encodings := []string{"s", "S", "b", "l", "B", "L"}
	for _, enc := range encodings {
		cfg.Encoding = enc
		called := false
		err := ExtractStrings(ctx, bytes.NewReader(makeScannable()), "test", cfg, func([]byte, string, int64, Config) {
			called = true
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("encoding %q: expected context.Canceled, got %v", enc, err)
		}
		if called {
			t.Errorf("encoding %q: printFunc should not run when ctx is cancelled upfront", enc)
		}
	}
}

func TestExtractStringsCancelMidScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := Config{MinLength: 4, Encoding: "s"}
	var count int
	// Cancel after the first delivered string so the loop must observe it
	// at the next 64 KiB poll boundary.
	err := ExtractStrings(ctx, bytes.NewReader(makeScannable()), "test", cfg, func([]byte, string, int64, Config) {
		count++
		cancel()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestExtractFromSectionCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{MinLength: 4, Encoding: "s"}
	err := ExtractFromSection(ctx, makeScannable(), "sec", 0, "test", cfg, func([]byte, string, int64, Config) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestExtractStringsCompletesWithLiveContext(t *testing.T) {
	cfg := Config{MinLength: 4, Encoding: "s"}
	var got int
	err := ExtractStrings(context.Background(), strings.NewReader("hello\x00world\x00"), "test", cfg, func([]byte, string, int64, Config) {
		got++
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected 2 strings, got %d", got)
	}
}
