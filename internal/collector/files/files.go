// Package files watches configured directories for filesystem changes using
// fsnotify (kqueue on macOS) and emits normalized file.* events. It manages
// recursive watches itself, skips configured ignore directories, and collapses
// duplicate events for the same path within a short window.
package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

// dedupWindow collapses identical (path, type) events within this window.
const dedupWindow = time.Second

// Collector watches directories for changes.
type Collector struct {
	cfg   *config.Config
	roots []string
	logf  func(string, ...any)

	watcher *fsnotify.Watcher

	mu       sync.Mutex
	lastSeen map[string]time.Time // "type|path" -> last emit time
}

// New builds a file collector. roots are already ~-expanded watch paths.
func New(cfg *config.Config, roots []string, logf func(string, ...any)) *Collector {
	return &Collector{cfg: cfg, roots: roots, logf: logf, lastSeen: map[string]time.Time{}}
}

// Name implements events.Collector.
func (c *Collector) Name() string { return "files" }

// Run watches until ctx is cancelled.
func (c *Collector) Run(ctx context.Context, sink events.EventSink) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	c.watcher = w
	defer w.Close()

	for _, root := range c.roots {
		c.addRecursive(root)
	}

	// Periodically clean the dedup map so it doesn't grow unbounded.
	gc := time.NewTicker(time.Minute)
	defer gc.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			c.handle(ev, sink)
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			c.logf("files: watcher error: %v", err)
		case <-gc.C:
			c.gcDedup()
		}
	}
}

func (c *Collector) handle(ev fsnotify.Event, sink events.EventSink) {
	if c.ignored(ev.Name) {
		return
	}

	var typ string
	switch {
	case ev.Op&fsnotify.Create != 0:
		typ = events.TypeFileCreate
		// Newly created directory: start watching it.
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			c.addRecursive(ev.Name)
		}
	case ev.Op&fsnotify.Write != 0:
		typ = events.TypeFileWrite
	case ev.Op&fsnotify.Remove != 0:
		typ = events.TypeFileDelete
	case ev.Op&fsnotify.Rename != 0:
		typ = events.TypeFileRename
	case ev.Op&fsnotify.Chmod != 0:
		typ = events.TypeFileChmod
	default:
		return
	}

	if c.deduped(typ, ev.Name) {
		return
	}

	root := c.rootFor(ev.Name)
	e := events.Event{
		Timestamp: time.Now(),
		Type:      typ,
		Category:  events.CategoryFile,
		Source:    events.SourceFSEvents,
		Severity:  events.SeverityInfo,
		Subject:   filepath.Base(ev.Name),
		Path:      ev.Name,
		Metadata: map[string]any{
			"root_path": root,
			"extension": strings.ToLower(filepath.Ext(ev.Name)),
		},
	}
	// Enrich with size/mtime/is_dir when the file still exists.
	if info, err := os.Stat(ev.Name); err == nil {
		e.Metadata["size_bytes"] = info.Size()
		e.Metadata["mtime"] = info.ModTime().Format(time.RFC3339)
		e.Metadata["is_dir"] = info.IsDir()
	}
	if err := sink.Emit(e); err != nil {
		c.logf("files: emit failed: %v", err)
	}
}

// addRecursive walks dir adding a watch for every subdirectory not ignored.
func (c *Collector) addRecursive(dir string) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}
		if c.isIgnoredDir(path) {
			return filepath.SkipDir
		}
		if err := c.watcher.Add(path); err != nil {
			c.logf("files: watch add %s: %v", path, err)
		}
		return nil
	})
}

// ignored reports whether a path (file or dir) should be skipped.
func (c *Collector) ignored(path string) bool {
	for _, seg := range strings.Split(path, string(os.PathSeparator)) {
		for _, ig := range c.cfg.IgnoreDirs {
			base := ig
			if strings.Contains(ig, "/") {
				base = filepath.Base(ig)
			}
			if seg == base {
				return true
			}
		}
	}
	return false
}

// isIgnoredDir reports whether a directory matches an ignore rule.
func (c *Collector) isIgnoredDir(path string) bool {
	base := filepath.Base(path)
	for _, ig := range c.cfg.IgnoreDirs {
		if strings.Contains(ig, "/") {
			if strings.HasSuffix(path, ig) {
				return true
			}
		} else if base == ig {
			return true
		}
	}
	return false
}

func (c *Collector) rootFor(path string) string {
	for _, r := range c.roots {
		if strings.HasPrefix(path, r) {
			return r
		}
	}
	return ""
}

// deduped reports whether this (type, path) was emitted within dedupWindow and
// records the current time otherwise.
func (c *Collector) deduped(typ, path string) bool {
	key := typ + "|" + path
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if last, ok := c.lastSeen[key]; ok && now.Sub(last) < dedupWindow {
		return true
	}
	c.lastSeen[key] = now
	return false
}

func (c *Collector) gcDedup() {
	cutoff := time.Now().Add(-dedupWindow)
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, t := range c.lastSeen {
		if t.Before(cutoff) {
			delete(c.lastSeen, k)
		}
	}
}
