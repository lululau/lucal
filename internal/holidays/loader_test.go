package holidays

import (
	"os"
	"path/filepath"
	"testing"
)

const mergedFixture = `[
  {
    "year": 2025,
    "days": [
      {"name": "元旦", "date": "2025-01-01", "isOffDay": true},
      {"name": "国庆节", "date": "2025-09-28", "isOffDay": false},
      {"name": "国庆节", "date": "2025-10-01", "isOffDay": true}
    ]
  },
  {
    "year": 2026,
    "days": [
      {"name": "元旦", "date": "2026-01-01", "isOffDay": true},
      {"name": "元旦", "date": "2026-01-04", "isOffDay": false}
    ]
  }
]`

const singleYearFixture = `{
  "year": 2026,
  "papers": ["https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm"],
  "days": [
    {"name": "元旦", "date": "2026-01-01", "isOffDay": true},
    {"name": "元旦", "date": "2026-01-04", "isOffDay": false}
  ]
}`

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "holidays.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFromFileMergedArray(t *testing.T) {
	index, err := LoadFromFile(writeTempFile(t, mergedFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 5 {
		t.Fatalf("expected 5 indexed days, got %d", len(index))
	}
}

func TestLoadFromFileSingleYear(t *testing.T) {
	index, err := LoadFromFile(writeTempFile(t, singleYearFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 2 {
		t.Fatalf("expected 2 indexed days, got %d", len(index))
	}
}

func TestGetHolidayForDate(t *testing.T) {
	index, err := LoadFromFile(writeTempFile(t, mergedFixture))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		year      int
		month     int
		day       int
		wantFound bool
		wantOff   bool
		wantName  string
	}{
		{"holiday", 2026, 1, 1, true, true, "元旦"},
		{"makeup workday", 2026, 1, 4, true, false, "元旦"},
		{"other year", 2025, 10, 1, true, true, "国庆节"},
		{"cross-year boundary", 2026, 10, 1, false, false, ""},
		{"unknown date", 2026, 3, 15, false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetHolidayForDate(index, tt.year, tt.month, tt.day)
			if !tt.wantFound {
				if info != nil {
					t.Fatalf("expected no info, got %+v", info)
				}
				return
			}
			if info == nil {
				t.Fatal("expected info, got nil")
			}
			if info.IsHoliday != tt.wantOff {
				t.Errorf("IsHoliday = %v, want %v", info.IsHoliday, tt.wantOff)
			}
			if info.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", info.Name, tt.wantName)
			}
		})
	}
}

func TestGetHolidayForDateNilIndex(t *testing.T) {
	if info := GetHolidayForDate(nil, 2026, 1, 1); info != nil {
		t.Fatalf("expected nil for nil index, got %+v", info)
	}
}

// The pre-holiday-cn cache format must be rejected so it can be re-downloaded.
func TestLoadFromFileRejectsLegacyFormat(t *testing.T) {
	legacy := `[{"year": "2013", "holiday": {"01-01": {"holiday": true, "name": "元旦", "wage": 3, "date": "2013-01-01"}}}]`
	if _, err := LoadFromFile(writeTempFile(t, legacy)); err == nil {
		t.Fatal("expected legacy format to be rejected")
	}
}
