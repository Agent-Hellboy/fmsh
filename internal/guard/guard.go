// Package guard implements fmsh's pre-command safety net. It creates APFS local
// snapshots as restore points — from the shell preexec hook before a
// destructive command, and from the daemon at session start / before high-risk
// events — and restores files from them with `fmsh restore`.
//
// Snapshots are instant, copy-on-write, and stored by macOS: fmsh keeps only a
// reference, never your file contents. Creating a snapshot needs no sudo;
// restoring mounts it read-only, which needs a one-time elevation.
package guard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/events"
	"fmsh/internal/risk"
	"fmsh/internal/snapshot"
	"fmsh/internal/store"
)

// IsDestructive reports whether a command matches a destructive pattern, so
// callers can short-circuit before doing any work.
func IsDestructive(command string) (string, bool) {
	return risk.IsDestructiveCommand(command)
}

// Result summarizes a guard invocation.
type Result struct {
	Triggered  bool
	Matched    string
	Checkpoint *store.Checkpoint
}

// Guard is the shell-hook entry point. If command is destructive it takes an
// APFS snapshot and records a checkpoint plus a risk event. It is best-effort;
// callers must never fail the user's command because of it.
func Guard(ctx context.Context, st *store.Store, root, command string) (*Result, error) {
	matched, ok := risk.IsDestructiveCommand(command)
	if !ok {
		return &Result{Triggered: false}, nil
	}
	res := &Result{Triggered: true, Matched: matched}

	root = absRoot(root)
	// Record the destructive command as a risk event regardless of snapshot
	// success.
	_ = st.InsertEvent(&events.Event{
		Timestamp: time.Now(), Type: events.TypeRiskDestructiveCommand, Category: events.CategoryRisk,
		Source: events.SourceGuard, Severity: events.SeverityHigh,
		Subject: "destructive command: " + truncate(command, 80), Path: root, RepoPath: repoOf(root),
		Cmdline:  command,
		Metadata: map[string]any{"rule": "destructive_command", "matched": matched},
	})

	cp, err := CreateCheckpoint(ctx, st, store.TriggerDestructive, root, command,
		"destructive command: "+matched, "")
	res.Checkpoint = cp
	return res, err
}

// CreateCheckpoint takes a snapshot and records it. It is shared by the shell
// guard and the daemon. If snapshots are unavailable it records a skipped
// checkpoint so the attempt is still auditable.
func CreateCheckpoint(ctx context.Context, st *store.Store, trigger, root, command, reason, sessionID string) (*store.Checkpoint, error) {
	cp := &store.Checkpoint{
		ID:        newID(),
		CreatedAt: time.Now(),
		Trigger:   trigger,
		Root:      root,
		Command:   command,
		Reason:    reason,
		SessionID: sessionID,
		Status:    store.CheckpointSaved,
	}

	if !snapshot.Available() {
		cp.Status = store.CheckpointSkipped
		cp.Reason = "APFS local snapshots unavailable on this system"
		_ = st.InsertCheckpoint(cp)
		return cp, nil
	}

	snap, err := snapshot.Create(ctx)
	if err != nil {
		cp.Status = store.CheckpointSkipped
		cp.Reason = "snapshot failed: " + err.Error()
		_ = st.InsertCheckpoint(cp)
		return cp, err
	}
	cp.SnapshotName = snap.Name
	cp.SnapshotDate = snap.Date
	if err := st.InsertCheckpoint(cp); err != nil {
		return cp, err
	}

	_ = st.InsertEvent(&events.Event{
		Timestamp: cp.CreatedAt, Type: events.TypeCheckpointCreated, Category: events.CategoryGuard,
		Source: events.SourceGuard, Severity: events.SeverityInfo,
		Subject: fmt.Sprintf("checkpoint %s (%s) — snapshot %s", cp.ID, trigger, snap.Date),
		Path:    root, RepoPath: repoOf(root), SessionID: sessionID,
		Metadata: map[string]any{"checkpoint_id": cp.ID, "trigger": trigger, "snapshot": snap.Name},
	})
	return cp, nil
}

// RestorePlan describes what a restore will do, for --print / --dry-run.
type RestorePlan struct {
	MountCmd   []string
	CopyFrom   string
	CopyTo     string
	UnmountCmd []string
}

// Restore recovers absPath (a file or directory) from the checkpoint's
// snapshot. It mounts the snapshot read-only (via sudo), copies the path back,
// and unmounts. With print=true it only returns the plan without executing.
func Restore(ctx context.Context, st *store.Store, cp *store.Checkpoint, absPath string, print bool) (*RestorePlan, []string, error) {
	if cp.Status == store.CheckpointSkipped || cp.SnapshotName == "" {
		return nil, nil, fmt.Errorf("checkpoint %s has no snapshot to restore (%s)", cp.ID, cp.Reason)
	}
	if absPath == "" {
		return nil, nil, fmt.Errorf("--path is required: specify the file or directory to recover")
	}
	absPath = config.Expand(absPath)
	if !filepath.IsAbs(absPath) {
		if a, err := filepath.Abs(absPath); err == nil {
			absPath = a
		}
	}

	if !snapshot.Available() {
		return nil, nil, fmt.Errorf("APFS snapshots unavailable on this system")
	}
	if !snapshot.Exists(ctx, cp.SnapshotName) {
		return nil, nil, fmt.Errorf("snapshot %s has been purged by macOS and can no longer be restored", cp.SnapshotDate)
	}
	device, err := snapshot.DataDevice(ctx)
	if err != nil {
		return nil, nil, err
	}

	mnt := filepath.Join(os.TempDir(), "fmsh-restore-"+cp.ID)
	src := filepath.Join(mnt, absPath)
	plan := &RestorePlan{
		MountCmd:   append([]string{"sudo"}, snapshot.MountArgs(cp.SnapshotName, device, mnt)...),
		CopyFrom:   src,
		CopyTo:     absPath,
		UnmountCmd: append([]string{"sudo"}, snapshot.UnmountArgs(mnt)...),
	}
	if print {
		return plan, nil, nil
	}

	if err := os.MkdirAll(mnt, 0o755); err != nil {
		return plan, nil, err
	}
	defer os.RemoveAll(mnt)

	if err := runCmd(ctx, plan.MountCmd); err != nil {
		return plan, nil, fmt.Errorf("mount snapshot (needs sudo): %w", err)
	}
	defer runCmd(ctx, plan.UnmountCmd)

	if _, err := os.Stat(src); err != nil {
		return plan, nil, fmt.Errorf("%s was not present in the snapshot", absPath)
	}
	restored, err := copyPath(src, absPath)
	if err != nil {
		return plan, restored, err
	}

	now := time.Now()
	_ = st.MarkCheckpointRestored(cp.ID, now)
	_ = st.InsertEvent(&events.Event{
		Timestamp: now, Type: events.TypeCheckpointRestored, Category: events.CategoryGuard,
		Source: events.SourceGuard, Severity: events.SeverityInfo,
		Subject: fmt.Sprintf("restored %d file(s) from checkpoint %s", len(restored), cp.ID),
		Path:    absPath, RepoPath: repoOf(absPath),
		Metadata: map[string]any{"checkpoint_id": cp.ID, "file_count": len(restored)},
	})
	return plan, restored, nil
}

// copyPath copies a file or directory tree from src to dst, returning the list
// of destination files written.
func copyPath(src, dst string) ([]string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := copyFile(src, dst); err != nil {
			return nil, err
		}
		return []string{dst}, nil
	}
	var written []string
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if err := copyFile(path, out); err != nil {
			return err
		}
		written = append(written, out)
		return nil
	})
	return written, err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func runCmd(ctx context.Context, argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr // sudo prompts / diagnostics to stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func absRoot(root string) string {
	root = config.Expand(root)
	if a, err := filepath.Abs(root); err == nil {
		return a
	}
	return root
}

func repoOf(root string) string {
	cur := root
	for i := 0; i < 40 && cur != "/" && cur != "."; i++ {
		if info, err := os.Stat(filepath.Join(cur, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return ""
}

func newID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
