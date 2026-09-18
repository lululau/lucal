package holidays

// Day mirrors a single day entry in the NateScarlet/holiday-cn dataset.
type Day struct {
	Name     string `json:"name"`     // holiday name, e.g. "元旦", "春节"
	Date     string `json:"date"`     // full date, e.g. "2026-01-04"
	IsOffDay bool   `json:"isOffDay"` // true = day off, false = makeup workday (调休)
}

// YearData mirrors one yearly file of the NateScarlet/holiday-cn dataset.
type YearData struct {
	Year int   `json:"year"`
	Days []Day `json:"days"`
}

// HolidayInfo contains information about a holiday for a specific date.
type HolidayInfo struct {
	IsHoliday bool   // true if it's a holiday, false if it's a workday (调休)
	Name      string // Name of the holiday
}
