package render

import (
	"strings"
	"testing"

	"github.com/lululau/lucal/internal/calendar"
)

func TestMonthBlockContainsLunarLabels(t *testing.T) {
	svc := calendar.NewService()
	view, err := svc.Month(2025, 11)
	if err != nil {
		t.Fatalf("Month failed: %v", err)
	}

	blocks, err := BuildBlocks([]calendar.MonthView{view})
	if err != nil {
		t.Fatalf("BuildBlocks failed: %v", err)
	}
	output := Layout(blocks, 120)
	if !strings.Contains(output, "初") && !strings.Contains(output, "廿") {
		t.Fatalf("expected lunar labels in layout, got:\n%s", output)
	}
}
