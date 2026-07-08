package files

import (
	"testing"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

func testCollector() *Collector {
	cfg := config.Default()
	return New(cfg, []string{"/watch"}, func(string, ...any) {})
}

func TestDedupCollapsesRapidDuplicates(t *testing.T) {
	c := testCollector()
	if c.deduped(events.TypeFileWrite, "/watch/a.go") {
		t.Fatal("first event should not be deduped")
	}
	if !c.deduped(events.TypeFileWrite, "/watch/a.go") {
		t.Error("immediate duplicate should be deduped")
	}
	// Different type is a distinct event.
	if c.deduped(events.TypeFileChmod, "/watch/a.go") {
		t.Error("different type should not be deduped")
	}
	// Different path is distinct.
	if c.deduped(events.TypeFileWrite, "/watch/b.go") {
		t.Error("different path should not be deduped")
	}
}

func TestIgnoredDirs(t *testing.T) {
	c := testCollector()
	cases := map[string]bool{
		"/watch/proj/node_modules/x.js": true,
		"/watch/proj/.git/index":        true,
		"/watch/proj/src/main.go":       false,
		"/watch/proj/dist/bundle.js":    true,
	}
	for path, want := range cases {
		if got := c.ignored(path); got != want {
			t.Errorf("ignored(%q) = %v, want %v", path, got, want)
		}
	}
}
