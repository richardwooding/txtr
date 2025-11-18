package printer

import (
	"os"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

func TestShouldUseColor(t *testing.T) {
	tests := []struct {
		name        string
		mode        extractor.ColorMode
		noColorEnv  string
		expected    bool
		description string
	}{
		{
			name:        "ColorNever always returns false",
			mode:        extractor.ColorNever,
			noColorEnv:  "",
			expected:    false,
			description: "ColorNever should disable colors regardless of environment",
		},
		{
			name:        "ColorAlways returns true without NO_COLOR",
			mode:        extractor.ColorAlways,
			noColorEnv:  "",
			expected:    true,
			description: "ColorAlways should enable colors when NO_COLOR is not set",
		},
		{
			name:        "ColorAlways respects NO_COLOR",
			mode:        extractor.ColorAlways,
			noColorEnv:  "1",
			expected:    false,
			description: "NO_COLOR environment variable should override ColorAlways",
		},
		{
			name:        "ColorAuto respects NO_COLOR",
			mode:        extractor.ColorAuto,
			noColorEnv:  "1",
			expected:    false,
			description: "NO_COLOR environment variable should disable ColorAuto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set NO_COLOR for this test using t.Setenv, which handles cleanup and prevents parallel execution
			if tt.noColorEnv != "" {
				t.Setenv("NO_COLOR", tt.noColorEnv)
			}

			got := ShouldUseColor(tt.mode)

			// For ColorAuto, we can't reliably test TTY detection in unit tests,
			// so we only verify NO_COLOR behavior
			if tt.mode == extractor.ColorAuto && tt.noColorEnv == "" {
				t.Skip("TTY detection not testable in unit tests")
			}

			if got != tt.expected {
				t.Errorf("ShouldUseColor(%v) with NO_COLOR=%q = %v, want %v\n%s",
					tt.mode, tt.noColorEnv, got, tt.expected, tt.description)
			}
		})
	}
}

func TestColorString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		colorCode string
		enabled   bool
		expected  string
	}{
		{
			name:      "Color enabled",
			input:     "Hello",
			colorCode: AnsiCyan,
			enabled:   true,
			expected:  "\x1b[36mHello\x1b[0m",
		},
		{
			name:      "Color disabled",
			input:     "Hello",
			colorCode: AnsiCyan,
			enabled:   false,
			expected:  "Hello",
		},
		{
			name:      "Empty string with color enabled",
			input:     "",
			colorCode: AnsiCyan,
			enabled:   true,
			expected:  "",
		},
		{
			name:      "Multiple ANSI codes",
			input:     "Bold Cyan",
			colorCode: AnsiBold + AnsiCyan,
			enabled:   true,
			expected:  "\x1b[1m\x1b[36mBold Cyan\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorString(tt.input, tt.colorCode, tt.enabled)
			if got != tt.expected {
				t.Errorf("ColorString(%q, %q, %v) = %q, want %q",
					tt.input, tt.colorCode, tt.enabled, got, tt.expected)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	// Test with nil file
	if isTerminal(nil) {
		t.Error("isTerminal(nil) should return false")
	}

	// Test with a regular file (not a TTY)
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()
	defer func() {
		_ = tmpFile.Close()
	}()

	if isTerminal(tmpFile) {
		t.Error("isTerminal(regular file) should return false")
	}

	// Note: We can't easily test TTY detection without a real terminal
	// In CI/CD, stdin/stdout/stderr may or may not be TTYs
}

func TestColorModeConstants(t *testing.T) {
	// Verify the ColorMode constants have expected values
	if extractor.ColorAuto != 0 {
		t.Errorf("ColorAuto = %d, want 0", extractor.ColorAuto)
	}
	if extractor.ColorAlways != 1 {
		t.Errorf("ColorAlways = %d, want 1", extractor.ColorAlways)
	}
	if extractor.ColorNever != 2 {
		t.Errorf("ColorNever = %d, want 2", extractor.ColorNever)
	}
}

func TestANSIColorCodes(t *testing.T) {
	// Verify ANSI codes are correct
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{"Reset", AnsiReset, "\x1b[0m"},
		{"Cyan", AnsiCyan, "\x1b[36m"},
		{"Yellow", AnsiYellow, "\x1b[33m"},
		{"Green", AnsiGreen, "\x1b[32m"},
		{"Magenta", AnsiMagenta, "\x1b[35m"},
		{"Dim", AnsiDim, "\x1b[2m"},
		{"Bold", AnsiBold, "\x1b[1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.code, tt.expected)
			}
		})
	}
}
