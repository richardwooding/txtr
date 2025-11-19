package printer

import (
	"bytes"
	"testing"

	"github.com/richardwooding/txtr/internal/extractor"
)

func TestPrintString(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		filename string
		offset   int64
		config   extractor.Config
		expected string
	}{
		{
			name:     "simple string",
			str:      "hello",
			filename: "",
			offset:   0,
			config:   extractor.Config{},
			expected: "hello\n",
		},
		{
			name:     "with filename",
			str:      "test",
			filename: "file.bin",
			offset:   0,
			config:   extractor.Config{PrintFileName: true},
			expected: "file.bin: test\n",
		},
		{
			name:     "with hex offset",
			str:      "data",
			filename: "",
			offset:   16,
			config:   extractor.Config{PrintOffset: true, Radix: "x"},
			expected: "     10 data\n",
		},
		{
			name:     "with decimal offset",
			str:      "data",
			filename: "",
			offset:   16,
			config:   extractor.Config{PrintOffset: true, Radix: "d"},
			expected: "     16 data\n",
		},
		{
			name:     "with octal offset",
			str:      "data",
			filename: "",
			offset:   16,
			config:   extractor.Config{PrintOffset: true, Radix: "o"},
			expected: "     20 data\n",
		},
		{
			name:     "with filename and offset",
			str:      "test",
			filename: "file.bin",
			offset:   8,
			config:   extractor.Config{PrintFileName: true, PrintOffset: true, Radix: "x"},
			expected: "file.bin:       8 test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintStringToWriter(&buf, []byte(tt.str), tt.filename, tt.offset, tt.config)

			got := buf.String()
			if got != tt.expected {
				t.Errorf("PrintStringToWriter() output mismatch\n  expected: %q\n       got: %q", tt.expected, got)
			}
		})
	}
}

func TestPrintStringWithColors(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		filename string
		offset   int64
		config   extractor.Config
		expected string
	}{
		{
			name:     "7-bit ASCII with color",
			str:      "test",
			filename: "",
			offset:   0,
			config:   extractor.Config{Encoding: "s", ColorMode: extractor.ColorAlways},
			expected: "test\n",
		},
		{
			name:     "8-bit ASCII with magenta color",
			str:      "test",
			filename: "",
			offset:   0,
			config:   extractor.Config{Encoding: "S", ColorMode: extractor.ColorAlways},
			expected: "\x1b[35mtest\x1b[0m\n",
		},
		{
			name:     "UTF-16 with green color",
			str:      "test",
			filename: "",
			offset:   0,
			config:   extractor.Config{Encoding: "l", ColorMode: extractor.ColorAlways},
			expected: "\x1b[32mtest\x1b[0m\n",
		},
		{
			name:     "UTF-8 mode with green color",
			str:      "test",
			filename: "",
			offset:   0,
			config:   extractor.Config{Encoding: "s", Unicode: "locale", ColorMode: extractor.ColorAlways},
			expected: "\x1b[32mtest\x1b[0m\n",
		},
		{
			name:     "colored filename",
			str:      "data",
			filename: "file.bin",
			offset:   0,
			config:   extractor.Config{PrintFileName: true, ColorMode: extractor.ColorAlways},
			expected: "\x1b[1m\x1b[36mfile.bin\x1b[0m: data\n",
		},
		{
			name:     "colored offset",
			str:      "data",
			filename: "",
			offset:   16,
			config:   extractor.Config{PrintOffset: true, Radix: "x", ColorMode: extractor.ColorAlways},
			expected: "\x1b[33m     10\x1b[0m data\n",
		},
		{
			name:     "colored custom separator",
			str:      "test",
			filename: "",
			offset:   0,
			config:   extractor.Config{OutputSeparator: " | ", ColorMode: extractor.ColorAlways},
			expected: "test\x1b[2m | \x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintStringToWriter(&buf, []byte(tt.str), tt.filename, tt.offset, tt.config)

			got := buf.String()
			if got != tt.expected {
				t.Errorf("PrintStringToWriter() output mismatch\n  expected: %q\n       got: %q", tt.expected, got)
			}
		})
	}
}

func TestPrintStringWithCustomSeparator(t *testing.T) {
	tests := []struct {
		name      string
		str       string
		separator string
		useColor  bool
		expected  string
	}{
		{
			name:      "custom separator without color",
			str:       "test",
			separator: " | ",
			useColor:  false,
			expected:  "test | ",
		},
		{
			name:      "newline separator (no dimming)",
			str:       "test",
			separator: "\n",
			useColor:  true,
			expected:  "test\n",
		},
		{
			name:      "empty separator (defaults to newline)",
			str:       "test",
			separator: "",
			useColor:  false,
			expected:  "test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := extractor.Config{OutputSeparator: tt.separator}
			if tt.useColor {
				config.ColorMode = extractor.ColorAlways
			} else {
				config.ColorMode = extractor.ColorNever
			}

			PrintStringToWriter(&buf, []byte(tt.str), "", 0, config)

			got := buf.String()
			if got != tt.expected {
				t.Errorf("PrintStringToWriter() output mismatch\n  expected: %q\n       got: %q", tt.expected, got)
			}
		})
	}
}
