// Package session groups related activity events into sessions. A session
// represents a burst of AI/dev activity in a repo or watched path. The model
// is intentionally generic: it does not overfit to any single AI tool, and it
// reports low confidence when the driving agent is unclear.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/events"
	"fmsh/internal/store"
)

// Sessionizer assigns events to sessions and closes idle ones.
type Sessionizer struct {
	store   *store.Store
	cfg     *config.Config
	roots   []string
	timeout time.Duration
	logf    func(string, ...any)

	mu       sync.Mutex
	active   map[string]*state // root -> active session state
	repoRoot map[string]string // dir -> enclosing repo root (cache)
}

type state struct {
	sess     *events.Session
	lastSeen time.Time
}

// New builds a Sessionizer.
func New(st *store.Store, cfg *config.Config, roots []string, logf func(string, ...any)) *Sessionizer {
	to := cfg.Session.InactivityTimeout.Duration
	if to <= 0 {
		to = 10 * time.Minute
	}
	return &Sessionizer{
		store:    st,
		cfg:      cfg,
		roots:    roots,
		timeout:  to,
		logf:     logf,
		active:   map[string]*state{},
		repoRoot: map[string]string{},
	}
}

// Assign mutates ev.SessionID (and ev.RepoPath when a repo is detected),
// creating or extending sessions as appropriate. It returns any events that
// should also be persisted (e.g. agent.session_started).
func (s *Sessionizer) Assign(ev *events.Event) []events.Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	root, repo := s.resolveRoot(ev)
	if repo != "" {
		ev.RepoPath = repo
	}

	now := ev.Timestamp
	if now.IsZero() {
		now = time.Now()
	}

	var extra []events.Event

	// Events without a root (ambient process/port activity) attach to the most
	// recently active session but never create one.
	if root == "" {
		st := s.mostRecentActive(now)
		if st == nil {
			return nil
		}
		ev.SessionID = st.sess.ID
		st.lastSeen = now
		s.count(st.sess, ev)
		_ = s.store.UpdateSession(st.sess)
		return nil
	}

	st := s.active[root]
	if st == nil {
		sess := &events.Session{
			ID:            newID(),
			StartedAt:     now,
			RepoPath:      repo,
			Status:        events.SessionActive,
			DetectedAgent: "unknown_dev_activity",
			Confidence:    0.4,
		}
		if err := s.store.InsertSession(sess); err != nil {
			s.logf("session: insert failed: %v", err)
		}
		st = &state{sess: sess, lastSeen: now}
		s.active[root] = st
		extra = append(extra, events.Event{
			Timestamp: now,
			Type:      events.TypeAgentSessionStarted,
			Category:  events.CategoryAgent,
			Source:    events.SourceSessionizer,
			Severity:  events.SeverityInfo,
			Subject:   filepath.Base(root),
			RepoPath:  repo,
			SessionID: sess.ID,
		})
	}

	ev.SessionID = st.sess.ID
	st.lastSeen = now
	s.applyAgentDetection(st.sess, ev)
	s.count(st.sess, ev)
	if err := s.store.UpdateSession(st.sess); err != nil {
		s.logf("session: update failed: %v", err)
	}
	return extra
}

// NoteRisk increments the risk counter of the session an event belongs to.
func (s *Sessionizer) NoteRisk(sessionID string) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.active {
		if st.sess.ID == sessionID {
			st.sess.RiskCount++
			_ = s.store.UpdateSession(st.sess)
			return
		}
	}
}

// Sweep closes sessions idle longer than the inactivity timeout. It returns
// agent.session_ended events to persist.
func (s *Sessionizer) Sweep(now time.Time) []events.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ended []events.Event
	for root, st := range s.active {
		if now.Sub(st.lastSeen) < s.timeout {
			continue
		}
		end := st.lastSeen
		st.sess.EndedAt = &end
		st.sess.Status = events.SessionEnded
		st.sess.Summary = summarize(st.sess)
		if err := s.store.UpdateSession(st.sess); err != nil {
			s.logf("session: close failed: %v", err)
		}
		ended = append(ended, events.Event{
			Timestamp: now,
			Type:      events.TypeAgentSessionEnded,
			Category:  events.CategoryAgent,
			Source:    events.SourceSessionizer,
			Severity:  events.SeverityInfo,
			Subject:   filepath.Base(root),
			RepoPath:  st.sess.RepoPath,
			SessionID: st.sess.ID,
		})
		delete(s.active, root)
	}
	return ended
}

// CloseAll ends all active sessions (used on graceful shutdown).
func (s *Sessionizer) CloseAll(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.active {
		end := now
		st.sess.EndedAt = &end
		st.sess.Status = events.SessionEnded
		st.sess.Summary = summarize(st.sess)
		_ = s.store.UpdateSession(st.sess)
	}
	s.active = map[string]*state{}
}

func (s *Sessionizer) count(sess *events.Session, ev *events.Event) {
	switch {
	case ev.Category == events.CategoryFile || ev.Category == events.CategoryGit:
		sess.FilesChangedCount++
	case ev.Type == events.TypeProcessStart:
		sess.CommandsSeenCount++
	case ev.Type == events.TypePortOpen:
		sess.PortsOpenedCount++
	}
}

// applyAgentDetection upgrades the session's detected agent when a process
// event carries a higher-confidence tool match.
func (s *Sessionizer) applyAgentDetection(sess *events.Session, ev *events.Event) {
	if ev.Type != events.TypeProcessStart || ev.Metadata == nil {
		return
	}
	tool, _ := ev.Metadata["detected_tool"].(string)
	ttype, _ := ev.Metadata["tool_type"].(string)
	conf, _ := ev.Metadata["confidence"].(float64)
	if tool == "" {
		return
	}
	// Only meaningful drivers upgrade the label.
	if ttype != "ai_agent" && ttype != "editor" {
		return
	}
	if conf > sess.Confidence {
		sess.DetectedAgent = tool
		sess.Confidence = conf
	}
}

func (s *Sessionizer) mostRecentActive(now time.Time) *state {
	var best *state
	for _, st := range s.active {
		if now.Sub(st.lastSeen) > s.timeout {
			continue
		}
		if best == nil || st.lastSeen.After(best.lastSeen) {
			best = st
		}
	}
	return best
}

// resolveRoot determines the grouping root and repo path for an event.
func (s *Sessionizer) resolveRoot(ev *events.Event) (root, repo string) {
	if ev.RepoPath != "" {
		return ev.RepoPath, ev.RepoPath
	}
	if ev.Category == events.CategoryFile && ev.Path != "" {
		if r := s.findRepoRoot(filepath.Dir(ev.Path)); r != "" {
			return r, r
		}
		if wr := s.watchRootFor(ev.Path); wr != "" {
			return wr, ""
		}
	}
	return "", ""
}

// findRepoRoot walks up from dir to find an enclosing Git repo, caching results.
func (s *Sessionizer) findRepoRoot(dir string) string {
	if r, ok := s.repoRoot[dir]; ok {
		return r
	}
	cur := dir
	for i := 0; i < 40 && cur != "/" && cur != "."; i++ {
		if info, err := os.Stat(filepath.Join(cur, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			s.repoRoot[dir] = cur
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	s.repoRoot[dir] = ""
	return ""
}

func (s *Sessionizer) watchRootFor(path string) string {
	for _, r := range s.roots {
		if strings.HasPrefix(path, r) {
			return r
		}
	}
	return ""
}

func summarize(sess *events.Session) string {
	return fmt.Sprintf("%d files changed, %d commands, %d ports, %d risks",
		sess.FilesChangedCount, sess.CommandsSeenCount, sess.PortsOpenedCount, sess.RiskCount)
}

func newID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
