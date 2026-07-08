package store

import (
	"testing"
	"time"
)

func TestCheckpointInsertListGet(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	cps := []*Checkpoint{
		{ID: "aaa111", CreatedAt: now.Add(-time.Hour), Trigger: TriggerSessionStart, Root: "/repo", SnapshotName: "com.apple.TimeMachine.d1.local", SnapshotDate: "d1", Status: CheckpointSaved},
		{ID: "bbb222", CreatedAt: now, Trigger: TriggerDestructive, Root: "/repo", Command: "rm -rf x", SnapshotName: "com.apple.TimeMachine.d2.local", SnapshotDate: "d2", Status: CheckpointSaved},
	}
	for _, c := range cps {
		if err := st.InsertCheckpoint(c); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	list, err := st.ListCheckpoints(time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 checkpoints, got %d", len(list))
	}
	// Newest first.
	if list[0].ID != "bbb222" {
		t.Errorf("expected newest first, got %s", list[0].ID)
	}

	got, err := st.GetCheckpoint("bbb")
	if err != nil {
		t.Fatalf("prefix get: %v", err)
	}
	if got.Command != "rm -rf x" || got.Trigger != TriggerDestructive {
		t.Errorf("checkpoint fields wrong: %+v", got)
	}
}

func TestMarkCheckpointRestored(t *testing.T) {
	st := newTestStore(t)
	c := &Checkpoint{ID: "ccc333", CreatedAt: time.Now(), Trigger: TriggerManual, SnapshotName: "com.apple.TimeMachine.d3.local", Status: CheckpointSaved}
	if err := st.InsertCheckpoint(c); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkCheckpointRestored("ccc333", time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetCheckpoint("ccc333")
	if got.Status != CheckpointRestored || got.RestoredAt == nil {
		t.Errorf("restore not recorded: %+v", got)
	}
}
