package calendar

import (
	"errors"
	"fmt"
	"time"

	calendarlib "github.com/Lofanmi/chinese-calendar-golang/calendar"
	"github.com/lululau/lucal/internal/holidays"
)

// Supported Gregorian year range enforced by the upstream library.
const (
	MinSupportedYear = 1900
	MaxSupportedYear = 3000
)

// ViewMode indicates whether we display a single month or an entire year.
type ViewMode int

const (
	ModeMonth ViewMode = iota
	ModeYear
)

// Request captures the initial year/month/mode that should be rendered.
type Request struct {
	Year  int
	Month int
	Mode  ViewMode
}

// Normalize keeps the month within 1..12 by rolling the year value.
func (r Request) Normalize() Request {
	for r.Month > 12 {
		r.Month -= 12
		r.Year++
	}
	for r.Month < 1 {
		r.Month += 12
		r.Year--
	}
	return r
}

// NextMonth moves the request to the following month.
func (r Request) NextMonth() Request {
	r.Month++
	return r.Normalize()
}

// PreviousMonth moves the request to the preceding month.
func (r Request) PreviousMonth() Request {
	r.Month--
	return r.Normalize()
}

// NextYear moves to the following year.
func (r Request) NextYear() Request {
	r.Year++
	return r
}

// PreviousYear moves to the preceding year.
func (r Request) PreviousYear() Request {
	r.Year--
	return r
}

// Day represents a single Gregorian day with lunar metadata.
type Day struct {
	Date            time.Time
	InMonth         bool
	LunarDayAlias   string
	LunarMonthAlias string
	SolarTerm       string
	IsToday         bool
	hasLunarData    bool
	HolidayInfo     *holidays.HolidayInfo
}

// SecondaryLabel selects the string that should be rendered beneath the
// Gregorian date. Solar terms take precedence, followed by lunar month names
// whenever it is the first day of a lunar month.
func (d Day) SecondaryLabel() string {
	if d.SolarTerm != "" {
		return d.SolarTerm
	}
	if d.LunarDayAlias == "初一" && d.LunarMonthAlias != "" {
		return d.LunarMonthAlias
	}
	return d.LunarDayAlias
}

// HasLunarData reports whether lunar metadata was successfully calculated.
func (d Day) HasLunarData() bool {
	return d.hasLunarData
}

// MonthView describes a month laid out into ISO weeks.
type MonthView struct {
	Year  int
	Month time.Month
	Title string
	Weeks [][]Day
}

// Service materialises month/year views using the upstream lunar calendar.
type Service struct {
	now         func() time.Time
	holidayData map[string]map[string]*holidays.HolidayEntry
}

// Option configures the Service.
type Option func(*Service)

// WithNow overrides the clock, which is useful for tests.
func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

// WithHolidays sets the holiday data for the service.
func WithHolidays(data map[string]map[string]*holidays.HolidayEntry) Option {
	return func(s *Service) {
		s.holidayData = data
	}
}

// NewService constructs a Service.
func NewService(opts ...Option) *Service {
	s := &Service{
		now: time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var (
	// ErrYearOutOfRange indicates the requested year is unsupported.
	ErrYearOutOfRange = fmt.Errorf("year must be between %d and %d", MinSupportedYear, MaxSupportedYear)
	// ErrInvalidMonth indicates the month is not in the 1..12 range.
	ErrInvalidMonth = errors.New("month must be between 1 and 12")
)

// Month builds a MonthView.
func (s *Service) Month(year, month int) (MonthView, error) {
	if year < MinSupportedYear || year > MaxSupportedYear {
		return MonthView{}, ErrYearOutOfRange
	}
	if month < 1 || month > 12 {
		return MonthView{}, ErrInvalidMonth
	}

	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	start := firstDay.AddDate(0, 0, -int(firstDay.Weekday()))
	end := firstDay.AddDate(0, 1, 0)
	now := s.now()

	weeks := make([][]Day, 0, 6)
	cursor := start
	for {
		week := make([]Day, 7)
		for i := 0; i < 7; i++ {
			week[i] = s.buildDay(cursor, firstDay.Month(), now)
			cursor = cursor.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)

		if (cursor.Equal(end) || cursor.After(end)) && cursor.Weekday() == time.Sunday {
			break
		}
		// Safety to avoid infinite loops.
		if len(weeks) >= 6 && cursor.After(end.AddDate(0, 0, 7)) {
			break
		}
	}

	view := MonthView{
		Year:  year,
		Month: firstDay.Month(),
		Title: fmt.Sprintf("%d 年 %d 月", year, month),
		Weeks: weeks,
	}
	return view, nil
}

// Year returns the MonthView list for an entire year.
func (s *Service) Year(year int) ([]MonthView, error) {
	if year < MinSupportedYear || year > MaxSupportedYear {
		return nil, ErrYearOutOfRange
	}
	months := make([]MonthView, 0, 12)
	for m := 1; m <= 12; m++ {
		view, err := s.Month(year, m)
		if err != nil {
			return nil, err
		}
		months = append(months, view)
	}
	return months, nil
}

func (s *Service) buildDay(day time.Time, currentMonth time.Month, now time.Time) Day {
	inMonth := day.Month() == currentMonth
	isToday := sameDay(day, now)

	if day.Year() < MinSupportedYear || day.Year() > MaxSupportedYear {
		return Day{
			Date:    day,
			InMonth: inMonth,
			IsToday: isToday,
		}
	}

	cal := calendarlib.BySolar(
		int64(day.Year()),
		int64(day.Month()),
		int64(day.Day()),
		12, 0, 0,
	)
	dayData := Day{
		Date:            day,
		InMonth:         inMonth,
		LunarDayAlias:   cal.Lunar.DayAlias(),
		LunarMonthAlias: cal.Lunar.MonthAlias(),
		IsToday:         isToday,
		hasLunarData:    true,
	}
	if solarterm := cal.Solar.CurrentSolarterm; solarterm != nil {
		if solarterm.IsInDay(&day) {
			dayData.SolarTerm = solarterm.Alias()
		}
	}
	// Add holiday information if available
	if s.holidayData != nil {
		dayData.HolidayInfo = holidays.GetHolidayForDate(s.holidayData, day.Year(), int(day.Month()), day.Day())
	}
	return dayData
}

func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
