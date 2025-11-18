package render

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/lululau/lucal/internal/calendar"
	"github.com/lululau/lucal/internal/textwidth"
)

const cellPadding = 1

var (
	noColorMode bool // Global flag to disable all color output
)

// SetNoColor sets the global no-color flag
func SetNoColor(disable bool) {
	noColorMode = disable
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FEC260"))
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A5B4FC"))
	cellStyle         = lipgloss.NewStyle()
	dimCellStyle      = cellStyle.Copy().Foreground(lipgloss.Color("#6B7280"))
	todayCellStyle    = cellStyle.Copy().Foreground(lipgloss.Color("#34D399"))
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	tableWrapperStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#475569")).
				Padding(0, 1)
)

var weekdays = []string{"日", "一", "二", "三", "四", "五", "六"}

// MonthBlock packages rendered lines with their visual width/height.
type MonthBlock struct {
	Lines  []string
	Width  int
	Height int
}

// BuildBlocks converts month views into renderable blocks.
func BuildBlocks(views []calendar.MonthView) ([]MonthBlock, error) {
	blocks := make([]MonthBlock, len(views))
	for i, view := range views {
		block, err := buildMonthBlock(view)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}
	return blocks, nil
}

// Layout renders blocks sequentially.
func Layout(blocks []MonthBlock, _ int) string {
	if len(blocks) == 0 {
		return ""
	}
	lines := make([]string, 0, len(blocks)*(blocks[0].Height+1))
	for idx, block := range blocks {
		lines = append(lines, block.Lines...)
		if idx != len(blocks)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func buildMonthBlock(view calendar.MonthView) (MonthBlock, error) {
	colWidth := determineColumnWidth(view) + cellPadding*2
	columns := make([]table.Column, len(weekdays))
	for i, title := range weekdays {
		columns[i] = table.Column{
			Title: title,
			Width: colWidth,
		}
	}

	// Collect dates that need highlighting: today, holidays, and workdays (调休)
	highlights := make(map[int]highlightInfo) // key: day number

	for _, week := range view.Weeks {
		for _, day := range week {
			if !day.InMonth {
				continue
			}
			dayNum := day.Date.Day()
			lunarLabel := day.SecondaryLabel()
			if lunarLabel == "" {
				lunarLabel = "  "
			}

			info := highlightInfo{
				day:        dayNum,
				lunarLabel: lunarLabel,
				isToday:    day.IsToday,
			}

			// Check for holiday/workday
			if day.HolidayInfo != nil {
				info.hasHoliday = true
				info.isHoliday = day.HolidayInfo.IsHoliday
				highlights[dayNum] = info
			} else if day.IsToday {
				// Only highlight today if it's not a holiday/workday
				highlights[dayNum] = info
			}
		}
	}

	rows := make([]table.Row, 0, len(view.Weeks)*3+1)
	rows = append(rows, blankRow(len(weekdays)))
	for weekIdx, week := range view.Weeks {
		gregorianRow := make(table.Row, len(week))
		lunarRow := make(table.Row, len(week))
		for idx, day := range week {
			gregorianRow[idx] = styleDayCell(day, renderGregorianCell(day))
			lunarRow[idx] = styleDayCell(day, renderLunarCell(day))
		}
		rows = append(rows, gregorianRow, lunarRow)
		if weekIdx != len(view.Weeks)-1 {
			rows = append(rows, blankRow(len(week)))
		}
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(len(rows)+2),
	)
	t.SetStyles(tableStyles())
	t.Blur()

	var tableView string
	if noColorMode {
		tableView = strings.TrimRight(t.View(), "\n")
	} else {
		tableView = tableWrapperStyle.Render(strings.TrimRight(t.View(), "\n"))
	}

	// Apply colors after rendering to avoid width calculation issues
	tableView = applyColors(tableView, highlights)
	tableView = applyDimColor(tableView, view)

	var title string
	if noColorMode {
		title = view.Title
	} else {
		title = titleStyle.Render(view.Title)
	}
	lines := append([]string{title, ""}, strings.Split(tableView, "\n")...)

	width := 0
	for _, line := range lines {
		if w := textwidth.StringWidth(line); w > width {
			width = w
		}
	}

	return MonthBlock{
		Lines:  lines,
		Width:  width,
		Height: len(lines),
	}, nil
}

func determineColumnWidth(view calendar.MonthView) int {
	width := 4
	for _, week := range view.Weeks {
		for _, day := range week {
			width = max(width, textwidth.StringWidth(renderGregorianCell(day)))
			width = max(width, textwidth.StringWidth(renderLunarCell(day)))
		}
	}
	return width
}

func renderGregorianCell(day calendar.Day) string {
	if !day.InMonth {
		return ""
	}
	return fmt.Sprintf("%2d", day.Date.Day())
}

func renderLunarCell(day calendar.Day) string {
	if !day.InMonth {
		return ""
	}
	label := day.SecondaryLabel()
	if label == "" {
		label = "  "
	}
	return label
}

func styleDayCell(day calendar.Day, content string) string {
	if content == "" {
		return ""
	}
	// Return raw content without styling - we'll apply colors after table rendering
	// to avoid width calculation issues in bubbles/table
	return content
}

func blankRow(cols int) table.Row {
	row := make(table.Row, cols)
	for i := range row {
		row[i] = ""
	}
	return row
}

func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	if noColorMode {
		styles.Header = lipgloss.NewStyle().Padding(0, 1)
	} else {
		styles.Header = headerStyle.Copy().Padding(0, 1)
	}
	styles.Selected = lipgloss.NewStyle()
	styles.Cell = lipgloss.NewStyle().Padding(0, cellPadding)
	return styles
}

// highlightInfo contains information about a date that needs highlighting
type highlightInfo struct {
	day        int
	lunarLabel string
	hasHoliday bool // true if HolidayInfo is not nil
	isHoliday  bool // true for holiday, false for workday (调休)
	isToday    bool
}

// applyColors adds colors to dates in the rendered table
// Priority: holiday/workday colors > today's green
func applyColors(output string, highlights map[int]highlightInfo) string {
	// If no-color mode is enabled, skip all coloring
	if noColorMode {
		return output
	}

	// Color codes
	holidayStart := "\x1b[38;2;59;130;246m" // Blue for holidays
	workdayStart := "\x1b[38;2;249;115;22m" // Orange for workdays (调休)
	todayStart := "\x1b[38;2;52;211;153m"   // Green for today
	colorEnd := "\x1b[0m"

	// Process each highlighted date
	// Sort by day number in descending order to match two-digit numbers before single-digit ones
	// This prevents partial matches (e.g., matching "1" in "11")
	dayNums := make([]int, 0, len(highlights))
	for dayNum := range highlights {
		dayNums = append(dayNums, dayNum)
	}
	// Sort descending
	for i := 0; i < len(dayNums)-1; i++ {
		for j := i + 1; j < len(dayNums); j++ {
			if dayNums[i] < dayNums[j] {
				dayNums[i], dayNums[j] = dayNums[j], dayNums[i]
			}
		}
	}

	// First pass: highlight all Gregorian dates
	for _, dayNum := range dayNums {
		info := highlights[dayNum]
		dayStr := fmt.Sprintf("%d", dayNum)
		var colorStart string

		// Determine color: holiday/workday takes priority over today
		if info.hasHoliday {
			if info.isHoliday {
				colorStart = holidayStart // Blue for holidays
			} else {
				colorStart = workdayStart // Orange for workdays (调休)
			}
		} else if info.isToday {
			colorStart = todayStart // Green for today (only if not holiday/workday)
		} else {
			continue
		}

		// Highlight the Gregorian date number
		// For single-digit numbers (1-9), match with leading space: " 1", " 2", etc.
		// For two-digit numbers (10-31), match the full number: "10", "11", etc.
		var pattern string
		if dayNum < 10 {
			// Single digit: must have leading space to avoid matching part of two-digit numbers
			pattern = fmt.Sprintf(`(\s+)%s(\s+|│)`, regexp.QuoteMeta(dayStr))
		} else {
			// Two digits: match full number, can have leading space or table border
			pattern = fmt.Sprintf(`(\s|│)%s(\s+|│)`, regexp.QuoteMeta(dayStr))
		}
		replacement := fmt.Sprintf("${1}%s%s%s${2}", colorStart, dayStr, colorEnd)
		re := regexp.MustCompile(pattern)
		output = re.ReplaceAllString(output, replacement)
	}

	// Second pass: highlight lunar labels
	// Use line-based approach to ensure lunar labels are colored in the correct column
	lines := strings.Split(output, "\n")
	coloredLunarLabels := make(map[string]bool) // Track which date's lunar label we've colored

	for _, dayNum := range dayNums {
		info := highlights[dayNum]
		var colorStart string

		// Determine color: holiday/workday takes priority over today
		if info.hasHoliday {
			if info.isHoliday {
				colorStart = holidayStart // Blue for holidays
			} else {
				colorStart = workdayStart // Orange for workdays (调休)
			}
		} else if info.isToday {
			colorStart = todayStart // Green for today (only if not holiday/workday)
		} else {
			continue
		}

		// Create a unique key for this date's lunar label
		lunarKey := fmt.Sprintf("%d:%s", dayNum, info.lunarLabel)
		if coloredLunarLabels[lunarKey] {
			continue // Already colored this specific date's lunar label
		}

		// Find the line containing the colored Gregorian date
		dayStr := fmt.Sprintf("%d", dayNum)
		coloredDatePattern := fmt.Sprintf("%s%s%s", colorStart, dayStr, colorEnd)

		for i := 0; i < len(lines)-1; i++ {
			if strings.Contains(lines[i], coloredDatePattern) {
				// This line contains the colored date, check the next line for lunar labels
				nextLine := lines[i+1]

				// Find all occurrences of this lunar label in the next line
				escapedLunar := regexp.QuoteMeta(info.lunarLabel)
				lunarPattern := fmt.Sprintf(`(\s|│)(%s)(\s+|│)`, escapedLunar)
				lunarRe := regexp.MustCompile(lunarPattern)

				// Replace only the first occurrence (should be in the correct column)
				if loc := lunarRe.FindStringIndex(nextLine); loc != nil {
					before := nextLine[:loc[0]]
					matched := nextLine[loc[0]:loc[1]]
					after := nextLine[loc[1]:]

					// Extract the parts from the matched string
					parts := lunarRe.FindStringSubmatch(matched)
					if len(parts) >= 4 {
						// parts[0] = full match
						// parts[1] = first capture group (\s|│)
						// parts[2] = second capture group (lunar label)
						// parts[3] = third capture group (\s+|│)
						coloredMatch := parts[1] + colorStart + info.lunarLabel + colorEnd + parts[3]
						lines[i+1] = before + coloredMatch + after
						coloredLunarLabels[lunarKey] = true
						break
					}
				}
			}
		}
	}

	output = strings.Join(lines, "\n")

	return output
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// applyDimColor adds gray color to dates outside the current month
func applyDimColor(output string, view calendar.MonthView) string {
	// This is more complex - we'd need to track which dates are outside the month
	// For now, we'll skip this to avoid complexity
	// The dim color can be applied at the cell level if needed
	return output
}

// HelpLine describes the interactive key bindings.
func HelpLine() string {
	helpText := "j/] 下个月  k/[ 上个月  J/} 下一年  K/{ 上一年 . 回到当前月  y 输入年份  m 输入月份  q 退出"
	if noColorMode {
		return helpText
	}
	return helpStyle.Render(helpText)
}

// ColorLegend returns a legend explaining the color coding for holidays.
func ColorLegend() string {
	legend := "\n蓝色=节假日  橙色=调休日"
	if noColorMode {
		return legend
	}
	// Use gray color for the legend
	legendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	return legendStyle.Render(legend)
}
