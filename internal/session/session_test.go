package session

import (
	"path/filepath"
	"testing"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/db"
	"fmsh/internal/events"
	"fmsh/internal/store"
)

func newTestSessionizer(t *testing.T, timeout time.Duration) (*Sessionizer, *store.Store) {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	st := store.New(sqlDB)
	cfg := config.Default()
	cfg.Session.InactivityTimeout = config.Duration{Duration: timeout}
	sz := New(st, cfg, []string{"/watch"}, func(string, ...any) {})
	return sz, st
}

func TestSessionCreatedAndReused(t *testing.T) {
	sz, _ := newTestSessionizer(t, 10*time.Minute)
	now := time.Now()

	ev1 := events.Event{Timestamp: now, Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/watch/proj/a.go"}
	extra := sz.Assign(&ev1)
	if ev1.SessionID == "" {
		t.Fatal("first file event should get a session id")
	}
	if len(extra) != 1 || extra[0].Type != events.TypeAgentSessionStarted {
		t.Fatalf("expected a session_started event, got %+v", extra)
	}

	ev2 := events.Event{Timestamp: now.Add(time.Minute), Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/watch/proj/b.go"}
	extra2 := sz.Assign(&ev2)
	if ev2.SessionID != ev1.SessionID {
		t.Error("events in the same root should share a session")
	}
	if len(extra2) != 0 {
		t.Error("second event should not start a new session")
	}
}

func TestSessionSweepClosesIdle(t *testing.T) {
	sz, st := newTestSessionizer(t, time.Minute)
	start := time.Now()
	ev := events.Event{Timestamp: start, Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/watch/proj/a.go"}
	sz.Assign(&ev)

	// Not idle yet.
	if ended := sz.Sweep(start.Add(30 * time.Second)); len(ended) != 0 {
		t.Error("session should not close before timeout")
	}
	// Past the timeout.
	ended := sz.Sweep(start.Add(2 * time.Minute))
	if len(ended) != 1 || ended[0].Type != events.TypeAgentSessionEnded {
		t.Fatalf("expected session_ended, got %+v", ended)
	}
	sessions, _ := st.QuerySessions(events.SessionFilter{Status: events.SessionEnded})
	if len(sessions) != 1 {
		t.Errorf("expected 1 ended session, got %d", len(sessions))
	}
}

func TestAgentDetectionUpgrade(t *testing.T) {
	sz, _ := newTestSessionizer(t, 10*time.Minute)
	now := time.Now()
	// Start with file activity (unknown agent).
	fe := events.Event{Timestamp: now, Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/watch/proj/a.go"}
	sz.Assign(&fe)

	// A high-confidence AI agent process should upgrade the label.
	pe := events.Event{
		Timestamp: now.Add(time.Second), Type: events.TypeProcessStart, Category: events.CategoryProcess,
		RepoPath: "/watch/proj",
		Metadata: map[string]any{"detected_tool": "claude", "tool_type": "ai_agent", "confidence": 0.8},
	}
	sz.Assign(&pe)

	// The sweep-produced session should now be labeled claude.
	sz.mu.Lock()
	var agent string
	for _, stt := range sz.active {
		agent = stt.sess.DetectedAgent
	}
	sz.mu.Unlock()
	if agent != "claude" {
		t.Errorf("expected agent upgraded to claude, got %q", agent)
	}
}
