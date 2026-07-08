// Package output renders events and reports for the terminal. It keeps output
// compact and readable, uses a small symbol vocabulary, and supports --json.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

// Symbols used across reports.
const (
	SymAdded    = "+" // added / started
	SymRemoved  = "-" // removed / exited
	SymModified = "~" // modified
	SymRisk     = "!" // risk / notable
)

// ANSI colors, disabled when not a TTY or NO_COLOR is set.
var colorEnabled = isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("NO_COLOR") == ""

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
)

func colorize(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + reset
}

// Bold, Dim, etc. wrap a string in the corresponding style.
func Bold(s string) string   { return colorize(bold, s) }
func Dim(s string) string    { return colorize(dim, s) }
func Red(s string) string    { return colorize(red, s) }
func Green(s string) string  { return colorize(green, s) }
func Yellow(s string) string { return colorize(yellow, s) }
func Cyan(s string) string   { return colorize(cyan, s) }

// SetColor forces color on/off (used by --no-color).
func SetColor(on bool) { colorEnabled = on }

// Header prints a section title.
func Header(w io.Writer, title string) {
	fmt.Fprintln(w, Bold(title))
}

// Section prints a blank line then a bold subsection label.
func Section(w io.Writer, label string) {
	fmt.Fprintf(w, "\n%s\n", Bold(label))
}

// JSON writes v as indented JSON.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// Clock formats a timestamp as a local wall-clock like "6:01 PM".
func Clock(t time.Time) string {
	return t.Local().Format("3:04 PM")
}

// ClockDate formats a timestamp with the date, used across day boundaries.
func ClockDate(t time.Time) string {
	return t.Local().Format("Jan 2 3:04 PM")
}

// SeveritySymbol returns a colored severity marker.
func SeveritySymbol(sev string) string {
	switch sev {
	case "high":
		return Red(SymRisk)
	case "medium":
		return Yellow(SymRisk)
	default:
		return SymRisk
	}
}

// HumanDuration renders a rounded, human duration like "1h" or "30m".
func HumanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// Pad right-pads s to width n.
func Pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// EmptyNote prints a dim placeholder for empty sections.
func EmptyNote(w io.Writer, note string) {
	fmt.Fprintln(w, Dim("  "+note))
}
