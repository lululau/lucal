package holidays

import (
	"encoding/json"
)

// HolidayEntry represents a single holiday entry in the JSON data.
type HolidayEntry struct {
	Holiday bool   `json:"holiday"`
	Name    string `json:"name"`
	Wage    int    `json:"wage"`
	Date    string `json:"date"`
	// Optional fields
	After  *bool  `json:"after,omitempty"`
	Target string `json:"target,omitempty"`
	Rest   *int   `json:"rest,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling to handle holiday field
// that can be either a boolean or a string (for compatibility with malformed JSON).
func (h *HolidayEntry) UnmarshalJSON(data []byte) error {
	// Use a temporary struct with flexible holiday field
	type Alias HolidayEntry
	aux := &struct {
		Holiday interface{} `json:"holiday"`
		*Alias
	}{
		Alias: (*Alias)(h),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle holiday field: can be bool or string
	switch v := aux.Holiday.(type) {
	case bool:
		h.Holiday = v
	case string:
		// If it's a non-empty string, treat it as true (holiday)
		h.Holiday = v != ""
	default:
		// Default to false if it's neither bool nor string
		h.Holiday = false
	}

	return nil
}

// HolidayData represents the structure of the holidays JSON file.
// It's a map from year string to a map of date strings (MM-DD) to HolidayEntry.
type HolidayData []struct {
	Year    string                           `json:"year"`
	Holiday map[string]*HolidayEntry `json:"holiday"`
}

// HolidayInfo contains information about a holiday for a specific date.
type HolidayInfo struct {
	IsHoliday bool   // true if it's a holiday, false if it's a workday (调休)
	Name      string // Name of the holiday
}

