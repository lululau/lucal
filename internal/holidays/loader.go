package holidays

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HolidayIndex maps full dates ("2026-01-04") to their holiday-cn day entry.
type HolidayIndex = map[string]*Day

// LoadFromFile loads holiday data from a JSON file in the holiday-cn format.
// The file may be a merged array of years or a single yearly file.
func LoadFromFile(path string) (HolidayIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read holidays file: %w", err)
	}

	var years []YearData
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var single YearData
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return nil, fmt.Errorf("failed to parse holidays JSON: %w", err)
		}
		years = []YearData{single}
	} else {
		if err := json.Unmarshal(trimmed, &years); err != nil {
			return nil, fmt.Errorf("failed to parse holidays JSON: %w", err)
		}
	}

	index := make(HolidayIndex)
	for _, yearData := range years {
		for i := range yearData.Days {
			day := &yearData.Days[i]
			index[day.Date] = day
		}
	}

	return index, nil
}

// GetCachePath returns the path to the holidays cache file in XDG cache directory.
func GetCachePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "lucal", "holidays.json"), nil
}

// LoadFromCache loads holiday data from the XDG cache directory.
func LoadFromCache() (HolidayIndex, error) {
	cachePath, err := GetCachePath()
	if err != nil {
		return nil, err
	}
	return LoadFromFile(cachePath)
}

// IsCacheValid checks if the cache file exists and is not older than 6 months.
func IsCacheValid(cachePath string) (bool, error) {
	info, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// Check if file is older than 6 months (180 days)
	sixMonthsAgo := time.Now().AddDate(0, -6, 0)
	return info.ModTime().After(sixMonthsAgo), nil
}

// GetHolidayForDate retrieves holiday information for a specific date.
func GetHolidayForDate(data HolidayIndex, year int, month int, day int) *HolidayInfo {
	if data == nil {
		return nil
	}

	entry, exists := data[fmt.Sprintf("%04d-%02d-%02d", year, month, day)]
	if !exists {
		return nil
	}

	return &HolidayInfo{
		IsHoliday: entry.IsOffDay,
		Name:      entry.Name,
	}
}
