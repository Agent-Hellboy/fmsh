// Package timeparse parses the human-friendly time expressions accepted by the
// CLI: durations like "1h" for --since, and clock/day expressions like
// "5pm" or "yesterday 6pm" for --from/--to. All parsing uses the local zone.
package timeparse

import (
	"fmt"
	"strings"
	"time"
)

// Now is overridable in tests.
var Now = time.Now

// ParseSince interprets a --since value. It accepts a bare duration ("30m",
// "1h", "24h"), an optional "since " prefix, or an absolute time expression
// understood by ParseMoment. It returns the resulting instant.
func ParseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "since ")
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty --since value")
	}
	if d, err := parseDuration(s); err == nil {
		return Now().Add(-d), nil
	}
	return ParseMoment(s)
}

// ParseMoment interprets an absolute time expression for --from/--to.
// Supported forms: "5pm", "17:00", "5:30pm", "today 5pm", "yesterday 6pm",
// and bare durations (interpreted as "ago").
func ParseMoment(s string) (time.Time, error) {
	orig := s
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time value")
	}

	// Bare duration -> ago.
	if d, err := parseDuration(s); err == nil {
		return Now().Add(-d), nil
	}

	now := Now()
	base := now
	if strings.HasPrefix(s, "today ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "today "))
	} else if strings.HasPrefix(s, "yesterday ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "yesterday "))
		base = now.AddDate(0, 0, -1)
	} else if s == "today" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	} else if s == "yesterday" {
		y := now.AddDate(0, 0, -1)
		return time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, now.Location()), nil
	}

	hh, mm, ok := parseClock(s)
	if !ok {
		return time.Time{}, fmt.Errorf("could not parse time %q", orig)
	}
	return time.Date(base.Year(), base.Month(), base.Day(), hh, mm, 0, 0, now.Location()), nil
}

// parseDuration extends time.ParseDuration with day support ("2d") and rejects
// clock-like strings.
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		var days float64
		if _, err := fmt.Sscanf(s, "%fd", &days); err == nil {
			return time.Duration(days * float64(24*time.Hour)), nil
		}
	}
	return time.ParseDuration(s)
}

// parseClock parses "5pm", "5:30pm", "17:00", "5" (hour).
func parseClock(s string) (hour, min int, ok bool) {
	s = strings.TrimSpace(s)
	pm, am := false, false
	if strings.HasSuffix(s, "pm") {
		pm = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "pm"))
	} else if strings.HasSuffix(s, "am") {
		am = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "am"))
	}

	var h, m int
	if strings.Contains(s, ":") {
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
			return 0, 0, false
		}
	} else {
		if _, err := fmt.Sscanf(s, "%d", &h); err != nil {
			return 0, 0, false
		}
	}

	if pm && h < 12 {
		h += 12
	}
	if am && h == 12 {
		h = 0
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}
