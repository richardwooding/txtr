package printer

import (
	"fmt"
	"io"
	"os"

	"github.com/richardwooding/triage"
	"github.com/richardwooding/txtr/internal/extractor"
)

// triagePreviewLimit caps how many bytes of a string are shown in the preview
// column of triage output.
const triagePreviewLimit = 80

// TriagePrinter formats extracted strings as annotated security-triage lines,
// tagging each with its highest-priority signal (secret, high-entropy,
// content category, or plain text). Its PrintString method matches the
// extractor printFunc signature so it plugs into the standard pipeline.
type TriagePrinter struct {
	config   extractor.Config
	writer   io.Writer
	useColor bool
}

// NewTriagePrinter creates a triage printer writing to w (defaulting to stdout).
func NewTriagePrinter(config extractor.Config, w io.Writer) *TriagePrinter {
	if w == nil {
		w = os.Stdout
	}
	return &TriagePrinter{
		config:   config,
		writer:   w,
		useColor: ShouldUseColor(config.ColorMode),
	}
}

// PrintString classifies a string and emits one annotated line per secret
// finding, or a single line tagged by entropy/category/text. In SecretsOnly
// mode, uninteresting strings are suppressed.
func (tp *TriagePrinter) PrintString(str []byte, filename string, offset int64, config extractor.Config) {
	res := triage.Classify(str, config.MinEntropy)

	if config.SecretsOnly && !res.Interesting() {
		return
	}

	offsetStr := tp.formatOffset(offset)
	prefix := tp.formatFilename(filename)
	preview := previewString(str)

	if len(res.Secrets) > 0 {
		for _, s := range res.Secrets {
			tag := tp.colorTag("SECRET", severityColor(s.Severity))
			tp.writeLine(prefix, tag, offsetStr, s.Rule, preview)
		}
		return
	}

	switch {
	case res.HighEntropy:
		tag := tp.colorTag("HIGH-ENT", AnsiMagenta)
		tp.writeLine(prefix, tag, offsetStr, fmt.Sprintf("entropy=%.1f", res.Entropy), preview)
	case len(res.Categories) > 0:
		cat := string(res.Categories[0])
		tag := tp.colorTag(cat, AnsiCyan)
		tp.writeLine(prefix, tag, offsetStr, "", preview)
	default:
		tag := tp.colorTag("TEXT", AnsiDim)
		tp.writeLine(prefix, tag, offsetStr, "", preview)
	}
}

// writeLine emits a single formatted triage line. The detail column is padded
// to a fixed width but always followed by a separating gap, so an over-long
// detail label can never run into the preview.
func (tp *TriagePrinter) writeLine(filenamePrefix, tag, offset, detail, preview string) {
	line := fmt.Sprintf("%-10s %s", tag, offset)
	if detail != "" {
		line += fmt.Sprintf("  %-20s", detail)
	}
	line += "  " + preview
	if _, err := fmt.Fprintf(tp.writer, "%s%s\n", filenamePrefix, line); err != nil {
		return
	}
}

// formatOffset renders the byte offset, honoring an explicit radix and
// defaulting to hex (offsets are central to triage's locate-it workflow).
func (tp *TriagePrinter) formatOffset(offset int64) string {
	var s string
	switch tp.config.Radix {
	case "o":
		s = fmt.Sprintf("%8o", offset)
	case "d":
		s = fmt.Sprintf("%8d", offset)
	default: // "x" or unset
		s = fmt.Sprintf("0x%06x", offset)
	}
	if tp.useColor {
		s = ColorString(s, AnsiYellow, true)
	}
	return s
}

// formatFilename renders the optional filename prefix.
func (tp *TriagePrinter) formatFilename(filename string) string {
	if !tp.config.PrintFileName || filename == "" {
		return ""
	}
	if tp.useColor {
		return ColorString(filename, AnsiBold+AnsiCyan, true) + ": "
	}
	return filename + ": "
}

// colorTag wraps a bracketed tag in the given color when colors are enabled.
func (tp *TriagePrinter) colorTag(label, color string) string {
	tag := "[" + label + "]"
	if tp.useColor {
		return ColorString(tag, color, true)
	}
	return tag
}

// severityColor maps a secret severity to an ANSI color.
func severityColor(sev triage.Severity) string {
	switch sev {
	case triage.SeverityHigh:
		return AnsiBold + AnsiRed
	case triage.SeverityMedium:
		return AnsiYellow
	default:
		return AnsiCyan
	}
}

// previewString returns a single-line, length-capped preview of a string.
func previewString(str []byte) string {
	if len(str) <= triagePreviewLimit {
		return string(str)
	}
	return string(str[:triagePreviewLimit]) + "..."
}
