package extractor

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
	"unicode/utf8"
)

// FuzzExtractASCII tests the extractASCII function with random inputs
func FuzzExtractASCII(f *testing.F) {
	// Seed corpus with known patterns
	f.Add([]byte("hello\x00world"), 4, false)
	f.Add([]byte("\x00\x00test\x01\x02data"), 3, true)
	f.Add([]byte(""), 4, false)
	f.Add([]byte("\xff\xfe\xfd\xfc"), 1, true)
	f.Add([]byte("printable"), 4, false)
	f.Add([]byte("\x1f\x20\x7e\x7f"), 1, false) // Boundary chars
	f.Add([]byte("long"+string(make([]byte, 1000))), 10, false)

	f.Fuzz(func(t *testing.T, data []byte, minLen int, allow8bit bool) {
		// Constrain minLen to reasonable range (1-100)
		if minLen <= 0 || minLen > 100 {
			minLen = (minLen%100) + 1
		}

		// Skip extremely large inputs to prevent resource exhaustion
		if len(data) > 10*1024*1024 { // 10MB limit
			t.Skip("Input too large")
		}

		// Collect extracted strings
		var results [][]byte
		printFunc := func(str []byte, _ string, _ int64, _ Config) {
			// Make a copy to avoid slice aliasing issues
			results = append(results, append([]byte(nil), str...))
		}

		config := Config{MinLength: minLen}
		reader := bytes.NewReader(data)

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panic: %v\nInput: %q\nMinLen: %d, Allow8bit: %v",
					r, data, minLen, allow8bit)
			}
		}()

		// Execute extraction
		extractASCII(reader, "", config, printFunc, allow8bit)

		// Invariant 1: All results meet minimum length
		for i, result := range results {
			if len(result) < minLen {
				t.Errorf("String #%d length %d < min %d: %q",
					i, len(result), minLen, result)
			}
		}

		// Invariant 2: All bytes are printable according to rules
		for i, result := range results {
			for j, b := range result {
				if !isPrintableASCII(b, allow8bit, false) {
					t.Errorf("String #%d byte #%d (0x%02x) is not printable: %q",
						i, j, b, result)
					break
				}
			}
		}

		// Invariant 3: Deterministic behavior (same input = same output)
		var results2 [][]byte
		printFunc2 := func(str []byte, _ string, _ int64, _ Config) {
			results2 = append(results2, append([]byte(nil), str...))
		}
		reader2 := bytes.NewReader(data)
		extractASCII(reader2, "", config, printFunc2, allow8bit)

		if len(results) != len(results2) {
			t.Errorf("Non-deterministic: got %d strings, second run got %d",
				len(results), len(results2))
		}
	})
}

// FuzzExtractUTF8Aware tests UTF-8 multibyte sequence handling
func FuzzExtractUTF8Aware(f *testing.F) {
	// Seed corpus with UTF-8 examples
	f.Add([]byte("Hello 世界"), 4)
	f.Add([]byte("Привет мир"), 4)
	f.Add([]byte("🌍🌎🌏"), 4)
	f.Add([]byte("\xc3\x28"), 4)           // Invalid UTF-8
	f.Add([]byte("\xf0\x28\x8c\xbc"), 4)   // Invalid
	f.Add([]byte("test\xc0\xaf/"), 4)      // Overlong encoding
	f.Add([]byte("\xed\xa0\x80"), 4)       // Surrogate
	f.Add([]byte(""), 4)                    // Empty
	f.Add([]byte("normal text"), 6)

	f.Fuzz(func(t *testing.T, data []byte, minLen int) {
		// Constrain minLen
		if minLen <= 0 || minLen > 100 {
			minLen = (minLen%100) + 1
		}

		// Skip large inputs
		if len(data) > 10*1024*1024 {
			t.Skip("Input too large")
		}

		// Determine mode from first byte of data
		modes := []string{"default", "invalid", "locale", "escape", "hex", "highlight"}
		mode := "locale"
		if len(data) > 0 {
			mode = modes[int(data[0])%len(modes)]
		}

		var results [][]byte
		printFunc := func(str []byte, _ string, _ int64, _ Config) {
			results = append(results, append([]byte(nil), str...))
		}

		config := Config{MinLength: minLen, Unicode: mode}
		reader := bytes.NewReader(data)

		// Test with timeout to catch infinite loops
		done := make(chan bool, 1)
		var panicked interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = r
				}
				done <- true
			}()
			extractUTF8Aware(reader, "", config, printFunc)
		}()

		select {
		case <-done:
			if panicked != nil {
				t.Fatalf("Panic: %v\nInput: %q\nMode: %s", panicked, data, mode)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout (possible infinite loop)\nInput: %q\nMode: %s", data, mode)
		}

		// Invariant: For locale mode, output must be valid UTF-8
		if mode == "locale" || mode == "default" || mode == "invalid" {
			for i, result := range results {
				if !utf8.Valid(result) {
					t.Errorf("String #%d contains invalid UTF-8 in %s mode: %q",
						i, mode, result)
				}
			}
		}

		// Invariant: All results meet minimum length
		for i, result := range results {
			if len(result) < minLen {
				t.Errorf("String #%d length %d < min %d: %q",
					i, len(result), minLen, result)
			}
		}
	})
}

// FuzzExtractUTF16 tests UTF-16 extraction with both byte orders
func FuzzExtractUTF16(f *testing.F) {
	// UTF-16LE "hello"
	f.Add([]byte{0x68, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F, 0x00}, 4, true)
	// UTF-16BE "hello"
	f.Add([]byte{0x00, 0x68, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F}, 4, false)
	// Incomplete sequence (odd byte)
	f.Add([]byte{0x68}, 1, true)
	// Surrogate pair (😀 in UTF-16LE)
	f.Add([]byte{0x3d, 0xd8, 0x0c, 0xdc}, 1, true)
	// Empty
	f.Add([]byte{}, 4, false)

	f.Fuzz(func(t *testing.T, data []byte, minLen int, littleEndian bool) {
		// Constrain minLen
		if minLen <= 0 || minLen > 100 {
			minLen = (minLen%100) + 1
		}

		// Skip large inputs
		if len(data) > 10*1024*1024 {
			t.Skip("Input too large")
		}

		var results []string
		printFunc := func(str []byte, _ string, _ int64, _ Config) {
			results = append(results, string(str))
		}

		var byteOrder binary.ByteOrder = binary.BigEndian
		if littleEndian {
			byteOrder = binary.LittleEndian
		}

		config := Config{MinLength: minLen}
		reader := bytes.NewReader(data)

		// Test with timeout (CVE-2020-14040: infinite loop in UTF-16 decoder)
		done := make(chan bool, 1)
		var panicked interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = r
				}
				done <- true
			}()
			extractUTF16(reader, "", config, printFunc, byteOrder)
		}()

		select {
		case <-done:
			if panicked != nil {
				t.Fatalf("Panic: %v\nInput: %q\nLE: %v", panicked, data, littleEndian)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout (possible infinite loop - CVE-2020-14040?)\nInput: %q", data)
		}

		// Invariant: All outputs are valid UTF-8 strings
		for i, result := range results {
			if !utf8.ValidString(result) {
				t.Errorf("String #%d contains invalid UTF-8: %q", i, result)
			}
		}

		// Invariant: All runes in output are valid
		for i, result := range results {
			for _, r := range result {
				if !utf8.ValidRune(r) {
					t.Errorf("String #%d contains invalid rune 0x%X", i, r)
					break
				}
			}
		}
	})
}

// FuzzExtractUTF32 tests UTF-32 extraction with both byte orders
func FuzzExtractUTF32(f *testing.F) {
	// UTF-32LE "test"
	f.Add([]byte{
		0x74, 0x00, 0x00, 0x00, // t
		0x65, 0x00, 0x00, 0x00, // e
		0x73, 0x00, 0x00, 0x00, // s
		0x74, 0x00, 0x00, 0x00, // t
	}, 4, true)
	// UTF-32BE "test"
	f.Add([]byte{
		0x00, 0x00, 0x00, 0x74, // t
		0x00, 0x00, 0x00, 0x65, // e
		0x00, 0x00, 0x00, 0x73, // s
		0x00, 0x00, 0x00, 0x74, // t
	}, 4, false)
	// Invalid rune (> 0x10FFFF)
	f.Add([]byte{0xFF, 0xFF, 0x11, 0x00}, 1, true)
	// Incomplete sequence (3 bytes)
	f.Add([]byte{0x74, 0x00, 0x00}, 1, true)
	// Empty
	f.Add([]byte{}, 4, false)
	// Surrogate range (invalid)
	f.Add([]byte{0x00, 0xD8, 0x00, 0x00}, 1, false)

	f.Fuzz(func(t *testing.T, data []byte, minLen int, littleEndian bool) {
		// Constrain minLen
		if minLen <= 0 || minLen > 100 {
			minLen = (minLen%100) + 1
		}

		// Skip large inputs
		if len(data) > 10*1024*1024 {
			t.Skip("Input too large")
		}

		var results []string
		printFunc := func(str []byte, _ string, _ int64, _ Config) {
			results = append(results, string(str))
		}

		var byteOrder binary.ByteOrder = binary.BigEndian
		if littleEndian {
			byteOrder = binary.LittleEndian
		}

		config := Config{MinLength: minLen}
		reader := bytes.NewReader(data)

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panic: %v\nInput: %q\nLE: %v", r, data, littleEndian)
			}
		}()

		extractUTF32(reader, "", config, printFunc, byteOrder)

		// Invariant: All outputs are valid UTF-8
		for i, result := range results {
			if !utf8.ValidString(result) {
				t.Errorf("String #%d contains invalid UTF-8: %q", i, result)
			}
		}

		// Invariant: All runes are valid (< 0x10FFFF, not surrogates)
		for i, result := range results {
			for _, r := range result {
				if !utf8.ValidRune(r) {
					t.Errorf("String #%d contains invalid rune 0x%X (> 0x10FFFF or surrogate)",
						i, r)
					break
				}
				// Additional check: no surrogates (0xD800-0xDFFF)
				if r >= 0xD800 && r <= 0xDFFF {
					t.Errorf("String #%d contains surrogate rune 0x%X", i, r)
					break
				}
			}
		}
	})
}
