// Package daemon runs fmshd: it starts collectors, funnels their events
// through the risk detector and sessionizer, persists everything to SQLite, and
// maintains health/status. Collectors run as independent loops; a failure in
// one never crashes the daemon.
package daemon

import (
	"context"
	"sync"
	"time"

	"fmsh/internal/collector/files"
	gitcol "fmsh/internal/collector/git"
	"fmsh/internal/collector/ports"
	proccol "fmsh/internal/collector/process"
	"fmsh/internal/config"
	"fmsh/internal/events"
	"fmsh/internal/guard"
	"fmsh/internal/risk"
	"fmsh/internal/session"
	"fmsh/internal/snapshot"
	"fmsh/internal/store"
)

// Version is stamped into daemon_status.
const Version = "0.2.0"

// Daemon owns the running collectors and the event pipeline. It implements
// events.EventSink.
type Daemon struct {
	cfg   *config.Config
	store *store.Store
	roots []string
	logf  func(string, ...any)

	sessionizer *session.Sessionizer

	// many-files window tracking
	mu           sync.Mutex
	fileChanges  []time.Time
	lastManyEmit time.Time
	riskDedup    map[string]time.Time // "type|path" -> last emit
	lastSnapshot time.Time            // cooldown for daemon-driven snapshots
	pid          int
	ctx          context.Context
}

// riskDedupWindow collapses identical risk events (same type + path) that arrive
// from multiple collectors (e.g. the file and git collectors both see .env).
const riskDedupWindow = 30 * time.Second

// New builds a Daemon.
func New(cfg *config.Config, st *store.Store, pid int, logf func(string, ...any)) *Daemon {
	roots := cfg.ResolvedWatchPaths()
	// Merge in DB-registered watched paths.
	if extra, err := st.ListWatchedPaths(); err == nil {
		seen := map[string]bool{}
		for _, r := range roots {
			seen[r] = true
		}
		for _, r := range extra {
			if !seen[r] {
				roots = append(roots, r)
				seen[r] = true
			}
		}
	}
	return &Daemon{
		cfg:         cfg,
		store:       st,
		roots:       roots,
		logf:        logf,
		pid:         pid,
		riskDedup:   map[string]time.Time{},
		sessionizer: session.New(st, cfg, roots, logf),
	}
}

// Emit implements events.EventSink. It is safe for concurrent use.
func (d *Daemon) Emit(ev events.Event) error {
	extra := d.sessionizer.Assign(&ev)
	if err := d.store.InsertEvent(&ev); err != nil {
		d.logf("daemon: insert event failed: %v", err)
		return err
	}
	for i := range extra {
		e := extra[i]
		if err := d.store.InsertEvent(&e); err != nil {
			d.logf("daemon: insert session event failed: %v", err)
		}
		if e.Type == events.TypeAgentSessionStarted && d.cfg.Guard.Enabled && d.cfg.Guard.SnapshotOnSession {
			d.maybeCheckpoint(store.TriggerSessionStart, e.RepoPath, "session started", e.SessionID)
		}
	}

	// Per-event risk rules.
	for _, r := range risk.EvaluateEvent(ev, d.cfg.Risk) {
		r.SessionID = ev.SessionID
		if r.RepoPath == "" {
			r.RepoPath = ev.RepoPath
		}
		if d.riskDeduped(r.Type, r.Path) {
			continue
		}
		if err := d.store.InsertEvent(&r); err != nil {
			d.logf("daemon: insert risk failed: %v", err)
			continue
		}
		d.sessionizer.NoteRisk(r.SessionID)
		if r.Severity == events.SeverityHigh && d.cfg.Guard.Enabled && d.cfg.Guard.SnapshotOnHighRisk {
			root := r.RepoPath
			if root == "" {
				root = ev.RepoPath
			}
			d.maybeCheckpoint(store.TriggerHighRisk, root, r.Subject, r.SessionID)
		}
	}

	// Windowed "many files changed" rule.
	if ev.Category == events.CategoryFile || ev.Category == events.CategoryGit {
		d.noteFileChange(ev)
	}
	return nil
}

// maybeCheckpoint takes an APFS snapshot restore point if snapshots are
// available and the cooldown has elapsed. It runs asynchronously so it never
// blocks the event pipeline.
func (d *Daemon) maybeCheckpoint(trigger, root, reason, sessionID string) {
	if !snapshot.Available() {
		return
	}
	minInterval := d.cfg.Guard.MinInterval.Duration
	if minInterval <= 0 {
		minInterval = 2 * time.Minute
	}
	now := time.Now()
	d.mu.Lock()
	if now.Sub(d.lastSnapshot) < minInterval {
		d.mu.Unlock()
		return
	}
	d.lastSnapshot = now
	d.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(d.ctx, 60*time.Second)
		defer cancel()
		cp, err := guard.CreateCheckpoint(ctx, d.store, trigger, root, "", reason, sessionID)
		if err != nil {
			d.logf("daemon: checkpoint (%s) failed: %v", trigger, err)
			return
		}
		if cp.Status == store.CheckpointSaved {
			d.logf("daemon: checkpoint %s taken (%s, snapshot %s)", cp.ID, trigger, cp.SnapshotDate)
		}
	}()
}

// riskDeduped reports whether a risk of this type+path was emitted within the
// dedup window, recording the time otherwise. Safe for concurrent use.
func (d *Daemon) riskDeduped(typ, path string) bool {
	key := typ + "|" + path
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.riskDedup[key]; ok && now.Sub(last) < riskDedupWindow {
		return true
	}
	d.riskDedup[key] = now
	return false
}

func (d *Daemon) noteFileChange(ev events.Event) {
	window := d.cfg.Risk.ManyFilesWindow.Duration
	threshold := d.cfg.Risk.ManyFilesThreshold
	if window <= 0 || threshold <= 0 {
		return
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()

	d.fileChanges = append(d.fileChanges, now)
	cutoff := now.Add(-window)
	kept := d.fileChanges[:0]
	for _, t := range d.fileChanges {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	d.fileChanges = kept

	if len(d.fileChanges) > threshold && now.Sub(d.lastManyEmit) > window {
		d.lastManyEmit = now
		count := len(d.fileChanges)
		r := events.Event{
			Timestamp: now,
			Type:      events.TypeRiskManyFilesChanged,
			Category:  events.CategoryRisk,
			Source:    events.SourceRiskDetector,
			Severity:  events.SeverityMedium,
			Subject:   "many files changed in a short window",
			SessionID: ev.SessionID,
			RepoPath:  ev.RepoPath,
			Metadata: map[string]any{
				"rule":      "many_files_changed",
				"count":     count,
				"window":    window.String(),
				"threshold": threshold,
			},
		}
		if err := d.store.InsertEvent(&r); err != nil {
			d.logf("daemon: insert many-files risk failed: %v", err)
		} else {
			d.sessionizer.NoteRisk(ev.SessionID)
		}
	}
}

// Run starts all collectors and background loops, blocking until ctx is
// cancelled. On return it closes active sessions and marks the daemon stopped.
func (d *Daemon) Run(ctx context.Context) error {
	d.ctx = ctx
	now := time.Now()
	_ = d.store.SetDaemonStatus(store.DaemonStatus{
		PID:             d.pid,
		StartedAt:       now,
		LastHeartbeatAt: now,
		Status:          "running",
		Version:         Version,
		Metadata:        map[string]any{"watch_paths": d.roots},
	})
	d.logf("fmshd %s started (pid %d), watching %d path(s)", Version, d.pid, len(d.roots))

	collectors := []events.Collector{
		files.New(d.cfg, d.roots, d.logf),
		proccol.New(d.cfg, d.logf),
		ports.New(d.cfg, d.logf),
		gitcol.New(d.cfg, d.logf),
	}

	var wg sync.WaitGroup
	for _, col := range collectors {
		wg.Add(1)
		go func(c events.Collector) {
			defer wg.Done()
			if err := c.Run(ctx, d); err != nil && ctx.Err() == nil {
				d.logf("collector %s stopped: %v", c.Name(), err)
			}
		}(col)
	}

	d.backgroundLoops(ctx)

	wg.Wait()

	// Graceful teardown.
	d.sessionizer.CloseAll(time.Now())
	_ = d.store.SetDaemonStatus(store.DaemonStatus{
		PID:             d.pid,
		StartedAt:       now,
		LastHeartbeatAt: time.Now(),
		Status:          "stopped",
		Version:         Version,
	})
	d.logf("fmshd stopped")
	return nil
}

// backgroundLoops runs heartbeat and session sweep until ctx is cancelled.
func (d *Daemon) backgroundLoops(ctx context.Context) {
	hb := d.cfg.Daemon.HeartbeatInterval.Duration
	if hb <= 0 {
		hb = 10 * time.Second
	}
	heartbeat := time.NewTicker(hb)
	defer heartbeat.Stop()
	sweep := time.NewTicker(30 * time.Second)
	defer sweep.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_ = d.store.Heartbeat(time.Now())
		case <-sweep.C:
			for _, e := range d.sessionizer.Sweep(time.Now()) {
				ev := e
				_ = d.store.InsertEvent(&ev)
			}
		}
	}
}
