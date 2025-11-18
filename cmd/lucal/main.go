package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lucal/lucal/internal/calendar"
	"github.com/lucal/lucal/internal/holidays"
	"github.com/lucal/lucal/internal/render"
	"github.com/lucal/lucal/internal/tui"
)

var (
	yearFlag      = flag.Bool("y", false, "显示全年日历")
	plain         = flag.Bool("n", false, "直接渲染并退出（非交互模式）")
	updateHolidays = flag.Bool("u", false, "下载最新的节假日数据")
	updateHolidaysLong = flag.Bool("update-holidays", false, "下载最新的节假日数据")
	holidaysFile  = flag.String("h", "", "指定节假日数据文件路径（用于调试）")
	holidaysFileLong = flag.String("holidays-file", "", "指定节假日数据文件路径（用于调试）")
	noColor       = flag.Bool("N", false, "禁用所有颜色输出")
	noColorLong   = flag.Bool("no-color", false, "禁用所有颜色输出")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "用法: %s [选项] [year] [month]\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), `
  无参数      展示当前月份
  -y          展示当前年份
  9           展示当年9月份
  1983        展示1983年
  2012 12     展示2012年12月
  -y 9        展示公元9年的全年

选项:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	// Set no-color flag if specified
	if *noColor || *noColorLong {
		render.SetNoColor(true)
		tui.SetNoColor(true)
	}

	// Handle update holidays flag
	if *updateHolidays || *updateHolidaysLong {
		if err := holidays.DownloadHolidays(); err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}
		return
	}

	// Load holiday data
	var holidayData map[string]map[string]*holidays.HolidayEntry
	var cacheValid bool
	var err error

	holidayFilePath := *holidaysFile
	if holidayFilePath == "" {
		holidayFilePath = *holidaysFileLong
	}

	if holidayFilePath != "" {
		// Load from specified file
		holidayData, err = holidays.LoadFromFile(holidayFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 无法加载节假日文件 %s: %v\n", holidayFilePath, err)
		} else {
			cacheValid = true
		}
	} else {
		// Try to load from cache
		cachePath, cacheErr := holidays.GetCachePath()
		if cacheErr == nil {
			valid, validErr := holidays.IsCacheValid(cachePath)
			if validErr == nil {
				cacheValid = valid
				if valid {
					holidayData, err = holidays.LoadFromCache()
					if err != nil {
						// Cache file exists but can't be read, mark as invalid
						cacheValid = false
					}
				}
			}
		}
	}

	req, err := parseRequest(*yearFlag, flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}

	// Create service with holiday data
	service := calendar.NewService()
	if holidayData != nil {
		service = calendar.NewService(calendar.WithHolidays(holidayData))
	}

	nonInteractive := *plain || req.Mode == calendar.ModeYear
	if nonInteractive {
		if err := render.RunPlain(render.PlainOptions{
			Service:          service,
			Request:          req,
			HolidayCacheValid: cacheValid,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}
		return
	}

	if err := tui.Run(service, req, cacheValid); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func parseRequest(showYear bool, args []string) (calendar.Request, error) {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	switch len(args) {
	case 0:
		// defaults
	case 1:
		if showYear {
			val, err := parseNumber(args[0], "year")
			if err != nil {
				return calendar.Request{}, err
			}
			year = val
		} else {
			val, err := parseNumber(args[0], "month/year")
			if err != nil {
				return calendar.Request{}, err
			}
			if val >= 1 && val <= 12 {
				month = val
			} else {
				year = val
				showYear = true
			}
		}
	case 2:
		if showYear {
			return calendar.Request{}, errors.New("使用 -y 时最多只需要指定一个年份参数")
		}
		y, err := parseNumber(args[0], "year")
		if err != nil {
			return calendar.Request{}, err
		}
		m, err := parseNumber(args[1], "month")
		if err != nil {
			return calendar.Request{}, err
		}
		if m < 1 || m > 12 {
			return calendar.Request{}, fmt.Errorf("月份需要在 1-12 之间 (收到 %d)", m)
		}
		year = y
		month = m
	default:
		return calendar.Request{}, errors.New("参数过多，请参考 --help")
	}

	req := calendar.Request{
		Year:  year,
		Month: month,
		Mode:  calendar.ModeMonth,
	}
	if showYear {
		req.Mode = calendar.ModeYear
	}
	return req.Normalize(), nil
}

func parseNumber(value string, field string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("无法将 %q 解析为 %s", value, field)
	}
	return n, nil
}

