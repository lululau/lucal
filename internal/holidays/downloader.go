package holidays

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// holiday-cn publishes one JSON file per year, sourced from official
	// State Council announcements (see the "papers" field in each file).
	holidayCnBaseURL = "https://raw.githubusercontent.com/NateScarlet/holiday-cn/master"
	// Earliest year available in holiday-cn.
	firstHolidayYear = 2007
	// Mirror of the merged dataset in this repository, offered as a manual
	// recovery path when the automated download fails.
	mirrorURL = "https://raw.githubusercontent.com/lululau/lucal/main/holidays.json"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type yearProgressMsg struct {
	year    int
	fetched int
	total   int
}

type downloadCompleteMsg struct {
	fileSize int64
	modTime  time.Time
	filePath string
	yearInfo *YearInfo
	err      error
}

// YearInfo contains information about the years in the holiday data
type YearInfo struct {
	MinYear int
	MaxYear int
	Count   int
}

type downloadModel struct {
	destPath     string
	firstYear    int
	lastYear     int
	currentYear  int
	fetched      int
	total        int
	done         bool
	err          error
	fileSize     int64
	modTime      time.Time
	filePath     string
	yearInfo     *YearInfo
	progressCh   chan yearProgressMsg
	completeCh   chan downloadCompleteMsg
	waitingKey   bool
}

func newDownloadModel(destPath string) downloadModel {
	now := time.Now()
	return downloadModel{
		destPath:   destPath,
		firstYear:  firstHolidayYear,
		lastYear:   now.Year() + 1, // next year's schedule is announced late in the year
		progressCh: make(chan yearProgressMsg, 32),
		completeCh: make(chan downloadCompleteMsg, 1),
	}
}

func (m downloadModel) Init() tea.Cmd {
	return tea.Batch(
		m.startDownload,
		m.listenProgress,
	)
}

func (m downloadModel) listenProgress() tea.Msg {
	select {
	case msg := <-m.progressCh:
		return msg
	case msg := <-m.completeCh:
		return msg
	}
}

func (m downloadModel) startDownload() tea.Msg {
	go func() {
		years, err := m.fetchAllYears()
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: err}
			return
		}

		data, err := json.MarshalIndent(years, "", "  ")
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to encode holidays JSON: %w", err)}
			return
		}

		if err := os.MkdirAll(filepath.Dir(m.destPath), 0755); err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to create directory: %w", err)}
			return
		}
		// Write only after every year has been fetched, so a failed download
		// never destroys an existing valid cache.
		if err := os.WriteFile(m.destPath, data, 0644); err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to write file: %w", err)}
			return
		}

		info, err := os.Stat(m.destPath)
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to stat file: %w", err)}
			return
		}

		// Parse the downloaded file to extract year information
		yearInfo, err := extractYearInfo(years)
		if err != nil {
			yearInfo = nil // non-fatal
		}

		m.completeCh <- downloadCompleteMsg{
			fileSize: info.Size(),
			modTime:  info.ModTime(),
			filePath: m.destPath,
			yearInfo: yearInfo,
		}
	}()

	return nil
}

// fetchAllYears downloads every yearly file from holiday-cn. A 404 is skipped
// (the year file does not exist yet); any other failure aborts the download.
func (m downloadModel) fetchAllYears() ([]YearData, error) {
	var years []YearData
	for year := m.firstYear; year <= m.lastYear; year++ {
		yearData, skipped, err := fetchYear(fmt.Sprintf("%s/%d.json", holidayCnBaseURL, year))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %d: %w", year, err)
		}
		if skipped {
			continue
		}
		years = append(years, *yearData)

		m.progressCh <- yearProgressMsg{year: year, fetched: len(years), total: m.lastYear - m.firstYear + 1}
	}

	if len(years) == 0 {
		return nil, fmt.Errorf("no year data available from %s", holidayCnBaseURL)
	}

	sort.Slice(years, func(i, j int) bool { return years[i].Year < years[j].Year })
	return years, nil
}

func fetchYear(url string) (*YearData, bool, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, false, fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response: %w", err)
	}

	var yearData YearData
	if err := json.Unmarshal(body, &yearData); err != nil {
		return nil, false, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &yearData, false, nil
}

func (m downloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.waitingKey {
			// After completion, any key press will quit
			return m, tea.Quit
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case downloadCompleteMsg:
		m.done = true
		m.err = msg.err
		m.fileSize = msg.fileSize
		m.modTime = msg.modTime
		m.filePath = msg.filePath
		m.yearInfo = msg.yearInfo
		m.waitingKey = true
		// Don't quit immediately, wait for user to see the message and press a key
		return m, nil
	case yearProgressMsg:
		m.currentYear = msg.year
		m.fetched = msg.fetched
		m.total = msg.total
		return m, m.listenProgress
	}

	return m, nil
}

func (m downloadModel) View() string {
	if m.done {
		if m.err != nil {
			errorMsg := fmt.Sprintf("❌ 下载失败\n\n错误详情: %v\n\n", m.err)
			errorMsg += "您可以手动下载节假日数据文件：\n"
			errorMsg += fmt.Sprintf("1. 访问: %s\n", mirrorURL)
			errorMsg += fmt.Sprintf("2. 下载文件并保存到: %s\n", m.destPath)
			errorMsg += "3. 确保目录存在（如果不存在，请先创建目录）\n\n"
			errorMsg += "按任意键退出...\n"
			return errorMsg
		}
		sizeStr := formatBytes(m.fileSize)
		timeStr := m.modTime.Format("2006-01-02 15:04:05")
		successMsg := fmt.Sprintf("✅ 下载成功!\n\n文件大小: %s\n更新时间: %s\n保存位置: %s\n", sizeStr, timeStr, m.filePath)

		// Add year information if available
		if m.yearInfo != nil {
			successMsg += fmt.Sprintf("\n数据年份范围: %d 年 - %d 年\n", m.yearInfo.MinYear, m.yearInfo.MaxYear)
			successMsg += fmt.Sprintf("最新数据年份: %d 年\n", m.yearInfo.MaxYear)
			successMsg += fmt.Sprintf("总共包含 %d 年的数据\n", m.yearInfo.Count)
		}

		successMsg += "\n按任意键退出...\n"
		return successMsg
	}

	// Custom progress bar
	const barWidth = 50
	var progressBar string
	var percent float64

	if m.total > 0 {
		percent = float64(m.fetched) / float64(m.total)
		if percent > 1.0 {
			percent = 1.0
		}
		filled := int(percent * barWidth)
		empty := barWidth - filled
		progressBar = strings.Repeat("█", filled) + strings.Repeat("░", empty)
	} else {
		progressBar = strings.Repeat("░", barWidth)
	}

	return fmt.Sprintf("正在从 holiday-cn 下载节假日数据...\n\n[%s]\n%d / %d 年  %.1f%%\n\n按 Ctrl+C 取消\n",
		progressBar, m.fetched, m.total, percent*100)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// extractYearInfo summarizes the year coverage of the downloaded dataset
func extractYearInfo(years []YearData) (*YearInfo, error) {
	if len(years) == 0 {
		return nil, fmt.Errorf("no year data found")
	}

	minYear := years[0].Year
	maxYear := years[0].Year
	for _, yearData := range years {
		if yearData.Year < minYear {
			minYear = yearData.Year
		}
		if yearData.Year > maxYear {
			maxYear = yearData.Year
		}
	}

	return &YearInfo{
		MinYear: minYear,
		MaxYear: maxYear,
		Count:   len(years),
	}, nil
}

// DownloadHolidays downloads the holidays JSON file and saves it to the cache directory.
func DownloadHolidays() error {
	cachePath, err := GetCachePath()
	if err != nil {
		return err
	}

	m := newDownloadModel(cachePath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	if m.err != nil {
		return m.err
	}

	return nil
}
