package holidays

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/progress"
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
	err      error
}

type downloadModel struct {
	progress   progress.Model
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
	progressCh chan downloadProgressMsg
	completeCh chan downloadCompleteMsg
}

func newDownloadModel(url, destPath string) downloadModel {
	p := progress.New(progress.WithScaledGradient("#FF6B6B", "#4ECDC4"))
	p.Width = 60
	return downloadModel{
		progress:   p,
		url:        url,
		destPath:   destPath,
		progressCh: make(chan downloadProgressMsg, 10),
		completeCh: make(chan downloadCompleteMsg, 1),
	}
}

func (m downloadModel) Init() tea.Cmd {
	return tea.Batch(
		m.progress.Init(),
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
			for {
				select {
				case <-ticker.C:
					currentBytes := atomic.LoadInt64(&downloaded)
					if currentBytes > 0 {
						elapsed := time.Since(startTime).Seconds()
						speed := float64(currentBytes) / elapsed
						m.progressCh <- downloadProgressMsg{
							bytesDownloaded: currentBytes,
							totalBytes:      totalBytes,
							speed:           speed,
						}
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

		m.completeCh <- downloadCompleteMsg{
			fileSize: info.Size(),
			modTime:  info.ModTime(),
			filePath: m.destPath,
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
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case downloadCompleteMsg:
		m.done = true
		m.err = msg.err
		m.fileSize = msg.fileSize
		m.modTime = msg.modTime
		m.filePath = msg.filePath
		return m, tea.Quit
	case downloadProgressMsg:
		m.downloaded = msg.bytesDownloaded
		m.total = msg.totalBytes
		m.speed = msg.speed
		if m.total > 0 {
			percent := float64(m.downloaded) / float64(m.total)
			m.progress.SetPercent(percent)
		}
		return m, m.listenProgress
	}

	var cmd tea.Cmd
	updated, cmd := m.progress.Update(msg)
	if p, ok := updated.(progress.Model); ok {
		m.progress = p
	}
	return m, cmd
}

func (m downloadModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("❌ 下载失败: %v\n按任意键退出...\n", m.err)
		}
		sizeStr := formatBytes(m.fileSize)
		timeStr := m.modTime.Format("2006-01-02 15:04:05")
		return fmt.Sprintf("✅ 下载成功!\n文件大小: %s\n更新时间: %s\n下载成功，保存到缓存 %s\n按任意键退出...\n", sizeStr, timeStr, m.filePath)
	}

	var progressView string
	if m.total > 0 {
		percent := float64(m.downloaded) / float64(m.total)
		m.progress.SetPercent(percent)
		progressView = m.progress.View()
		speedStr := formatSpeed(m.speed)
		downloadedStr := formatBytes(m.downloaded)
		totalStr := formatBytes(m.total)
		return fmt.Sprintf("正在下载节假日数据...\n\n%s\n%s / %s  %s\n\n按 Ctrl+C 取消\n", progressView, downloadedStr, totalStr, speedStr)
	}
	return fmt.Sprintf("正在下载节假日数据...\n\n%s\n已下载: %s\n\n按 Ctrl+C 取消\n", progressView, formatBytes(m.downloaded))
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
