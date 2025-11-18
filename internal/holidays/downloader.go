package holidays

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	holidaysURL = "https://raw.githubusercontent.com/lululau/lucal/main/holidays.json"
)

type downloadProgressMsg struct {
	bytesDownloaded int64
	totalBytes      int64
	speed           float64
}

type downloadCompleteMsg struct {
	fileSize int64
	modTime  time.Time
	filePath string
	yearInfo *YearInfo // Information about years in the downloaded data
	err      error
}

// YearInfo contains information about the years in the holiday data
type YearInfo struct {
	MinYear int // Earliest year
	MaxYear int // Latest year
	Count   int // Total number of years
}

type downloadModel struct {
	url        string
	destPath   string
	downloaded int64
	total      int64
	speed      float64
	done       bool
	err        error
	fileSize   int64
	modTime    time.Time
	filePath   string
	yearInfo   *YearInfo
	progressCh chan downloadProgressMsg
	completeCh chan downloadCompleteMsg
	waitingKey bool // Whether we're waiting for user to press a key after completion
}

func newDownloadModel(url, destPath string) downloadModel {
	return downloadModel{
		url:        url,
		destPath:   destPath,
		progressCh: make(chan downloadProgressMsg, 10),
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
	// Create directory if it doesn't exist
	dir := filepath.Dir(m.destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to create directory: %w", err)}
		return nil
	}

	// Start download in goroutine
	go func() {
		// Start HTTP request
		resp, err := http.Get(m.url)
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to start download: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)}
			return
		}

		totalBytes := resp.ContentLength

		// Create destination file
		file, err := os.Create(m.destPath)
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to create file: %w", err)}
			return
		}
		defer file.Close()

		// Track download progress
		var downloaded int64
		startTime := time.Now()

		// Use TeeReader to track bytes
		reader := io.TeeReader(resp.Body, &progressWriter{
			onWrite: func(n int) {
				atomic.AddInt64(&downloaded, int64(n))
			},
		})

		// Send progress updates periodically
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				currentBytes := atomic.LoadInt64(&downloaded)
				if currentBytes > 0 {
					elapsed := time.Since(startTime).Seconds()
					speed := float64(currentBytes) / elapsed
					select {
					case m.progressCh <- downloadProgressMsg{
						bytesDownloaded: currentBytes,
						totalBytes:      totalBytes,
						speed:           speed,
					}:
					default:
						// Channel is full, skip this update
					}
				}
			}
		}()

		// Copy data
		_, err = io.Copy(file, reader)
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to write file: %w", err)}
			return
		}

		// Get file info
		info, err := os.Stat(m.destPath)
		if err != nil {
			m.completeCh <- downloadCompleteMsg{err: fmt.Errorf("failed to stat file: %w", err)}
			return
		}

		// Parse the downloaded file to extract year information
		yearInfo, err := extractYearInfo(m.destPath)
		if err != nil {
			// If we can't parse year info, continue anyway (non-fatal)
			yearInfo = nil
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

type progressWriter struct {
	onWrite func(int)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	if pw.onWrite != nil {
		pw.onWrite(len(p))
	}
	return len(p), nil
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
	case downloadProgressMsg:
		m.downloaded = msg.bytesDownloaded
		m.total = msg.totalBytes
		m.speed = msg.speed
		return m, m.listenProgress
	}

	return m, nil
}

func (m downloadModel) View() string {
	if m.done {
		if m.err != nil {
			cachePath := m.destPath
			errorMsg := fmt.Sprintf("❌ 下载失败\n\n错误详情: %v\n\n", m.err)
			errorMsg += "您可以手动下载节假日数据文件：\n"
			errorMsg += fmt.Sprintf("1. 访问: %s\n", holidaysURL)
			errorMsg += fmt.Sprintf("2. 下载文件并保存到: %s\n", cachePath)
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
	var progressInfo string

	if m.total > 0 {
		percent = float64(m.downloaded) / float64(m.total)
		if percent > 1.0 {
			percent = 1.0
		}
		filled := int(percent * barWidth)
		empty := barWidth - filled
		progressBar = strings.Repeat("█", filled) + strings.Repeat("░", empty)
		speedStr := formatSpeed(m.speed)
		downloadedStr := formatBytes(m.downloaded)
		totalStr := formatBytes(m.total)
		progressInfo = fmt.Sprintf("%s / %s  %s  %.1f%%", downloadedStr, totalStr, speedStr, percent*100)
	} else {
		// Unknown total size
		progressBar = strings.Repeat("░", barWidth)
		downloadedStr := formatBytes(m.downloaded)
		if m.speed > 0 {
			speedStr := formatSpeed(m.speed)
			progressInfo = fmt.Sprintf("%s  %s", downloadedStr, speedStr)
		} else {
			progressInfo = downloadedStr
		}
	}

	return fmt.Sprintf("正在下载节假日数据...\n\n[%s]\n%s\n\n按 Ctrl+C 取消\n", progressBar, progressInfo)
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

func formatSpeed(speed float64) string {
	return fmt.Sprintf("%s/s", formatBytes(int64(speed)))
}

// extractYearInfo parses the holiday JSON file and extracts year information
func extractYearInfo(filePath string) (*YearInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var holidayData HolidayData
	if err := json.Unmarshal(data, &holidayData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(holidayData) == 0 {
		return nil, fmt.Errorf("no year data found")
	}

	// Extract all years and convert to integers
	years := make([]int, 0, len(holidayData))
	for _, yearData := range holidayData {
		year, err := strconv.Atoi(yearData.Year)
		if err != nil {
			continue // Skip invalid years
		}
		years = append(years, year)
	}

	if len(years) == 0 {
		return nil, fmt.Errorf("no valid years found")
	}

	// Find min and max years
	minYear := years[0]
	maxYear := years[0]
	for _, year := range years {
		if year < minYear {
			minYear = year
		}
		if year > maxYear {
			maxYear = year
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

	m := newDownloadModel(holidaysURL, cachePath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	if m.err != nil {
		return m.err
	}

	return nil
}
