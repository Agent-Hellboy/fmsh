package store

import (
	"path/filepath"
	"testing"
	"time"

	"fmsh/internal/db"
	"fmsh/internal/events"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	if wal, err := db.WALEnabled(sqlDB); err != nil || !wal {
		t.Fatalf("expected WAL mode, got %v (err %v)", wal, err)
	}
	return New(sqlDB)
}

func TestInsertAndQueryEvents(t *testing.T) {
	st := newTestStore(t)
	base := time.Now()

	evs := []events.Event{
		{Timestamp: base, Type: events.TypeFileWrite, Category: events.CategoryFile, Source: "t", Path: "/repo/a.go"},
		{Timestamp: base.Add(time.Second), Type: events.TypeProcessStart, Category: events.CategoryProcess, Source: "t", ProcessName: "claude"},
		{Timestamp: base.Add(2 * time.Second), Type: events.TypeRiskSecretTouched, Category: events.CategoryRisk, Source: "t", Severity: events.SeverityHigh, Path: "/repo/.env"},
	}
	for i := range evs {
		if err := st.InsertEvent(&evs[i]); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if evs[i].ID == 0 {
			t.Fatal("expected non-zero ID after insert")
		}
	}

	all, err := st.QueryEvents(events.EventFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 events, got %d", len(all))
	}
	// Ascending order.
	if !all[0].Timestamp.Before(all[1].Timestamp) {
		t.Error("events not in ascending time order")
	}

	byCat, _ := st.QueryEvents(events.EventFilter{Category: events.CategoryRisk})
	if len(byCat) != 1 || byCat[0].Type != events.TypeRiskSecretTouched {
		t.Errorf("category filter failed: %+v", byCat)
	}

	byPath, _ := st.QueryEvents(events.EventFilter{Path: "a.go"})
	if len(byPath) != 1 {
		t.Errorf("path suffix filter failed, got %d", len(byPath))
	}
}

func TestEventMetadataRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ev := events.Event{
		Timestamp: time.Now(), Type: events.TypeProcessStart, Category: events.CategoryProcess, Source: "t",
		Metadata: map[string]any{"detected_tool": "claude", "confidence": 0.8},
	}
	if err := st.InsertEvent(&ev); err != nil {
		t.Fatal(err)
	}
	out, _ := st.QueryEvents(events.EventFilter{})
	if out[0].Metadata["detected_tool"] != "claude" {
		t.Errorf("metadata round-trip failed: %+v", out[0].Metadata)
	}
}

func TestSessions(t *testing.T) {
	st := newTestStore(t)
	sess := &events.Session{
		ID: "abc123", StartedAt: time.Now(), Status: events.SessionActive,
		DetectedAgent: "claude", Confidence: 0.8, RepoPath: "/repo",
	}
	if err := st.InsertSession(sess); err != nil {
		t.Fatal(err)
	}
	sess.RiskCount = 3
	sess.Status = events.SessionEnded
	end := time.Now()
	sess.EndedAt = &end
	if err := st.UpdateSession(sess); err != nil {
		t.Fatal(err)
	}

	got, err := st.GetSession("abc")
	if err != nil {
		t.Fatalf("prefix lookup: %v", err)
	}
	if got.RiskCount != 3 || got.Status != events.SessionEnded {
		t.Errorf("session update not persisted: %+v", got)
	}
	if got.EndedAt == nil {
		t.Error("ended_at not persisted")
	}
}

func TestWatchedPaths(t *testing.T) {
	st := newTestStore(t)
	if err := st.AddWatchedPath("/a"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddWatchedPath("/a"); err != nil {
		t.Fatalf("idempotent add failed: %v", err)
	}
	st.AddWatchedPath("/b")
	paths, _ := st.ListWatchedPaths()
	if len(paths) != 2 {
		t.Fatalf("want 2 paths, got %d: %v", len(paths), paths)
	}
	st.RemoveWatchedPath("/a")
	paths, _ = st.ListWatchedPaths()
	if len(paths) != 1 || paths[0] != "/b" {
		t.Errorf("remove failed: %v", paths)
	}
}

func TestDaemonStatus(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	if err := st.SetDaemonStatus(DaemonStatus{PID: 42, StartedAt: now, LastHeartbeatAt: now, Status: "running", Version: "test"}); err != nil {
		t.Fatal(err)
	}
	// Upsert must not create a second row.
	st.SetDaemonStatus(DaemonStatus{PID: 43, StartedAt: now, LastHeartbeatAt: now, Status: "running", Version: "test"})
	ds, err := st.GetDaemonStatus()
	if err != nil || ds == nil {
		t.Fatalf("get status: %v", err)
	}
	if ds.PID != 43 {
		t.Errorf("upsert failed, PID = %d, want 43", ds.PID)
	}
}
