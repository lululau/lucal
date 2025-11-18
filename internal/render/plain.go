package render

import (
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"

	"github.com/lululau/lucal/internal/calendar"
)

// PlainOptions controls how the non-interactive renderer behaves.
type PlainOptions struct {
	Writer            io.Writer
	Service           *calendar.Service
	Request           calendar.Request
	Width             int
	HolidayCacheValid bool
}

// RunPlain renders the requested view exactly once.
func RunPlain(opts PlainOptions) error {
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.Service == nil {
		opts.Service = calendar.NewService()
	}

	req := opts.Request.Normalize()
	views, err := fetchViews(opts.Service, req)
	if err != nil {
		return err
	}
	blocks, err := BuildBlocks(views)
	if err != nil {
		return err
	}
	width := opts.Width
	if width == 0 {
		width = DetectWidth()
	}
	output := Layout(blocks, width)
	if output == "" {
		return nil
	}
	_, err = fmt.Fprintln(opts.Writer, output)
	if err != nil {
		return err
	}

	// Show color legend if holiday data is available
	if opts.Service != nil && opts.Service.HasHolidayData() {
		legend := ColorLegend()
		// Remove ANSI codes for plain output if needed, but keep the text
		_, err = fmt.Fprintln(opts.Writer, "\n"+legend)
		if err != nil {
			return err
		}
	}

	if !opts.HolidayCacheValid {
		_, err = fmt.Fprintln(opts.Writer, "\n尚未下载节假日数据或节假日数据超过 6 个月未更新，运行  lucal -u 获取最新数据")
	}
	return err
}

// DetectWidth tries to determine the terminal width, falling back to 100 cols.
func DetectWidth() int {
	fd := os.Stdout.Fd()
	if isatty.IsTerminal(fd) {
		if w, _, err := term.GetSize(int(fd)); err == nil {
			return w
		}
	}
	return 100
}

func fetchViews(svc *calendar.Service, req calendar.Request) ([]calendar.MonthView, error) {
	if req.Mode == calendar.ModeYear {
		return svc.Year(req.Year)
	}
	view, err := svc.Month(req.Year, req.Month)
	if err != nil {
		return nil, err
	}
	return []calendar.MonthView{view}, nil
}
