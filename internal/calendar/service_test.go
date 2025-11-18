package calendar

import (
	"testing"
	"time"
)

func TestMonthGeneratesCompleteWeeks(t *testing.T) {
	now := time.Date(2025, 11, 18, 10, 0, 0, 0, time.Local)
	svc := NewService(WithNow(func() time.Time { return now }))
	view, err := svc.Month(2025, 11)
	if err != nil {
		t.Fatalf("Month returned error: %v", err)
	}
	if view.Month != time.November {
		t.Fatalf("expected November, got %v", view.Month)
	}
	if len(view.Weeks) < 5 {
		t.Fatalf("expected at least 5 weeks, got %d", len(view.Weeks))
	}
	start := view.Weeks[0][0].Date
	if start.Weekday() != time.Sunday {
		t.Fatalf("calendar should start on Sunday, got %v", start.Weekday())
	}
	foundToday := false
	for _, week := range view.Weeks {
		if len(week) != 7 {
			t.Fatalf("week should have 7 days, got %d", len(week))
		}
		for _, day := range week {
			if day.IsToday {
				foundToday = true
				if day.Date.Day() != 18 {
					t.Fatalf("expected IsToday on 18th, got %d", day.Date.Day())
				}
			}
		}
	}
	if !foundToday {
		t.Fatalf("expected to flag current day")
	}
}

func TestYearLoadsAllMonths(t *testing.T) {
	svc := NewService()
	months, err := svc.Year(2024)
	if err != nil {
		t.Fatalf("Year returned error: %v", err)
	}
	if len(months) != 12 {
		t.Fatalf("expected 12 months, got %d", len(months))
	}
}

func TestInvalidMonth(t *testing.T) {
	svc := NewService()
	if _, err := svc.Month(2024, 13); err == nil {
		t.Fatalf("expected error for invalid month")
	}
}

