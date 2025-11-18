package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lucal/lucal/internal/calendar"
	"github.com/lucal/lucal/internal/render"
)

var (
	noColorMode bool // Global flag to disable all color output
)

// SetNoColor sets the global no-color flag
func SetNoColor(disable bool) {
	noColorMode = disable
}

type inputMode int

const (
	inputNone inputMode = iota
	inputYear
	inputMonth
)

// Run starts the interactive Bubble Tea UI.
func Run(svc *calendar.Service, req calendar.Request, holidayCacheValid bool) error {
	if svc == nil {
		svc = calendar.NewService()
	}
	m := newModel(svc, req.Normalize(), holidayCacheValid)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err := prog.Run()
	return err
}

type model struct {
	svc               *calendar.Service
	request           calendar.Request
	width             int
	inputMode         inputMode
	input             textinput.Model
	statusMsg         string
	holidayCacheValid bool
}

func newModel(svc *calendar.Service, req calendar.Request, holidayCacheValid bool) model {
	ti := textinput.New()
	ti.Placeholder = "数字"
	ti.CharLimit = 16
	ti.Prompt = "> "
	return model{
		svc:               svc,
		request:           req,
		input:             ti,
		holidayCacheValid: holidayCacheValid,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		if m.inputMode != inputNone {
			return m.handleInputKey(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "k", "[":
			m.request = m.request.PreviousMonth()
			m.statusMsg = ""
		case "j", "]":
			m.request = m.request.NextMonth()
			m.statusMsg = ""
		case "K", "{":
			m.request = m.request.PreviousYear()
			m.statusMsg = ""
		case "J", "}":
			m.request = m.request.NextYear()
			m.statusMsg = ""
		case "y":
			m.activateInput(inputYear, "")
		case "m":
			m.activateInput(inputMonth, "")
		case ".":
			now := time.Now()
			m.request.Year = now.Year()
			m.request.Month = int(now.Month())
			m.request.Mode = calendar.ModeMonth
			m.statusMsg = ""
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.inputMode != inputNone {
		return m.inputView()
	}

	body, err := m.renderCalendar()
	status := m.statusMsg
	if err != nil {
		status = err.Error()
	}

	help := render.HelpLine()
	sb := strings.Builder{}
	sb.WriteString(body)
	sb.WriteString("\n\n")
	sb.WriteString(help)
	if status != "" {
		sb.WriteString("\n")
		if noColorMode {
			sb.WriteString(status)
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Render(status))
		}
	}
	if !m.holidayCacheValid {
		sb.WriteString("\n")
		warningMsg := "\n尚未下载节假日数据或节假日数据超过 6 个月未更新，运行  lucal -u 获取最新数据"
		if noColorMode {
			sb.WriteString(warningMsg)
		} else {
			warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
			sb.WriteString(warningStyle.Render(warningMsg))
		}
	}
	return sb.String()
}

func (m model) renderCalendar() (string, error) {
	views, err := m.fetchViews()
	if err != nil {
		return "", err
	}
	blocks, err := render.BuildBlocks(views)
	if err != nil {
		return "", err
	}
	width := m.width
	if width <= 0 {
		width = 100
	}
	return render.Layout(blocks, width), nil
}

func (m model) fetchViews() ([]calendar.MonthView, error) {
	month, err := m.svc.Month(m.request.Year, m.request.Month)
	if err != nil {
		return nil, err
	}
	return []calendar.MonthView{month}, nil
}

func (m model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.inputMode = inputNone
		m.statusMsg = ""
		return m, nil
	case tea.KeyEnter:
		m.applyInput()
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) activateInput(mode inputMode, placeholder string) {
	m.inputMode = mode
	m.input.SetValue("")
	m.input.Placeholder = placeholder
	m.input.CursorEnd()
	m.input.Focus()
	m.statusMsg = ""
}

func (m *model) applyInput() {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		m.statusMsg = "请输入数字"
		return
	}
	switch m.inputMode {
	case inputYear:
		fields := strings.Fields(value)
		if len(fields) == 0 || len(fields) > 2 {
			m.statusMsg = "格式应为: 年 或 年 月"
			return
		}
		year, err := strconv.Atoi(fields[0])
		if err != nil {
			m.statusMsg = "无效的年份"
			return
		}
		m.request.Year = year
		if len(fields) == 2 {
			month, err := strconv.Atoi(fields[1])
			if err != nil || month < 1 || month > 12 {
				m.statusMsg = "月份需在 1-12 之间"
				return
			}
			m.request.Month = month
		}
		m.request.Mode = calendar.ModeMonth
	case inputMonth:
		num, err := strconv.Atoi(value)
		if err != nil {
			m.statusMsg = "无效的月份"
			return
		}
		if num < 1 || num > 12 {
			m.statusMsg = "月份需在 1-12 之间"
			return
		}
		m.request.Month = num
		m.request.Mode = calendar.ModeMonth
	}
	m.request = m.request.Normalize()
	m.statusMsg = ""
	m.inputMode = inputNone
	m.input.Blur()
}

func (m model) inputView() string {
	var label string
	switch m.inputMode {
	case inputYear:
		label = "输入年份 (回车确认 / Esc 取消)"
	case inputMonth:
		label = "输入月份 1-12 (回车确认 / Esc 取消)"
	default:
		return ""
	}
	if noColorMode {
		return label + "\n\n" + m.input.View()
	}
	return lipgloss.NewStyle().
		Bold(true).
		Render(label) + "\n\n" + m.input.View()
}
