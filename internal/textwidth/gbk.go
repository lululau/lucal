package textwidth

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StringWidth returns the maximum visual width (in monospace columns) of the
// provided string. It treats a single Chinese character as occupying two
// columns by encoding the string as GBK per the project requirements.
func StringWidth(s string) int {
	if s == "" {
		return 0
	}
	maxWidth := 0
	for _, line := range strings.Split(s, "\n") {
		width := lineWidth(line)
		if width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

// PadRight appends ASCII spaces until the rendered width matches target.
func PadRight(s string, width int) string {
	diff := width - StringWidth(s)
	if diff <= 0 {
		return s
	}
	return s + strings.Repeat(" ", diff)
}

func lineWidth(s string) int {
	if s == "" {
		return 0
	}
	clean := stripANSI(s)
	encoder := simplifiedchinese.GBK.NewEncoder()
	encoded, _, err := transform.String(encoder, clean)
	if err != nil {
		return fallbackWidth(clean)
	}
	return len(encoded)
}

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func fallbackWidth(s string) int {
	width := 0
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		if r <= unicode.MaxASCII {
			width++
		} else {
			width += 2
		}
	}
	return width
}

