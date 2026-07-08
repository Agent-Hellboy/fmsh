package timeparse

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	// 2026-07-09 15:30:00 local.
	return time.Date(2026, 7, 9, 15, 30, 0, 0, time.Local)
}

func TestParseSinceDurations(t *testing.T) {
	Now = fixedNow
	defer func() { Now = time.Now }()

	cases := map[string]time.Duration{
		"30m":      30 * time.Minute,
		"1h":       time.Hour,
		"24h":      24 * time.Hour,
		"2h":       2 * time.Hour,
		"since 1h": time.Hour,
		"2d":       48 * time.Hour,
	}
	for in, want := range cases {
		got, err := ParseSince(in)
		if err != nil {
			t.Fatalf("ParseSince(%q) error: %v", in, err)
		}
		if !got.Equal(fixedNow().Add(-want)) {
			t.Errorf("ParseSince(%q) = %v, want %v ago", in, got, want)
		}
	}
}

func TestParseMomentClock(t *testing.T) {
	Now = fixedNow
	defer func() { Now = time.Now }()

	cases := map[string]struct{ h, m int }{
		"5pm":       {17, 0},
		"17:00":     {17, 0},
		"5:30pm":    {17, 30},
		"9am":       {9, 0},
		"12am":      {0, 0},
		"12pm":      {12, 0},
		"today 5pm": {17, 0},
	}
	for in, want := range cases {
		got, err := ParseMoment(in)
		if err != nil {
			t.Fatalf("ParseMoment(%q) error: %v", in, err)
		}
		if got.Hour() != want.h || got.Minute() != want.m {
			t.Errorf("ParseMoment(%q) = %02d:%02d, want %02d:%02d", in, got.Hour(), got.Minute(), want.h, want.m)
		}
	}
}

func TestParseMomentYesterday(t *testing.T) {
	Now = fixedNow
	defer func() { Now = time.Now }()

	got, err := ParseMoment("yesterday 6pm")
	if err != nil {
		t.Fatal(err)
	}
	if got.Day() != 8 || got.Hour() != 18 {
		t.Errorf("yesterday 6pm = %v, want Jul 8 18:00", got)
	}
}

func TestParseMomentInvalid(t *testing.T) {
	if _, err := ParseMoment("not a time"); err == nil {
		t.Error("expected error for invalid input")
	}
}
