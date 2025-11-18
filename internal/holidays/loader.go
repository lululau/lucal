package holidays

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LoadFromFile loads holiday data from a JSON file.
func LoadFromFile(path string) (map[string]map[string]*HolidayEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read holidays file: %w", err)
	}

	var holidayData HolidayData
	if err := json.Unmarshal(data, &holidayData); err != nil {
		return nil, fmt.Errorf("failed to parse holidays JSON: %w", err)
	}

	// Convert array format to map format for easier lookup
	result := make(map[string]map[string]*HolidayEntry)
	for _, yearData := range holidayData {
		result[yearData.Year] = yearData.Holiday
	}

	return result, nil
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
func LoadFromCache() (map[string]map[string]*HolidayEntry, error) {
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
func GetHolidayForDate(data map[string]map[string]*HolidayEntry, year int, month int, day int) *HolidayInfo {
	if data == nil {
		return nil
	}

	yearStr := fmt.Sprintf("%d", year)
	dateStr := fmt.Sprintf("%02d-%02d", month, day)

	yearData, exists := data[yearStr]
	if !exists {
		return nil
	}

	entry, exists := yearData[dateStr]
	if !exists {
		return nil
	}

	return &HolidayInfo{
		IsHoliday: entry.Holiday,
		Name:      entry.Name,
	}
}

