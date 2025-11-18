package textwidth_test

import (
	"testing"

	"github.com/lucal/lucal/internal/textwidth"
)

func TestStringWidthMixedScripts(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"ascii", "hello", 5},
		{"chinese", "中文", 4},
		{"mixed", "A中", 3},
		{"multiline", "ab\n中文", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := textwidth.StringWidth(tt.in); got != tt.want {
				t.Fatalf("StringWidth(%q)=%d want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestPadRight(t *testing.T) {
	got := textwidth.PadRight("中", 4)
	if textwidth.StringWidth(got) != 4 {
		t.Fatalf("PadRight width=%d want 4", textwidth.StringWidth(got))
	}
	if got == "中" {
		t.Fatalf("PadRight should append spaces")
	}
}

