// Package store persists events, sessions, watched paths, and daemon status
// to SQLite. It is the single point of truth the CLI queries.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fmsh/internal/events"
)

// tsLayout is the timestamp format stored in TEXT columns (RFC3339 with
// nanoseconds, sortable lexicographically when zones match; we always store
// in the local zone with offset).
const tsLayout = time.RFC3339Nano

// Store is a SQLite-backed event store.
type Store struct {
	db *sql.DB
}

// New wraps an open *sql.DB.
func New(db *sql.DB) *Store { return &Store{db: db} }

// DB exposes the underlying handle for health checks.
func (s *Store) DB() *sql.DB { return s.db }

func formatTS(t time.Time) string { return t.Format(tsLayout) }

func parseTS(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	t, err := time.Parse(tsLayout, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

// InsertEvent persists a single event and returns its assigned ID.
func (s *Store) InsertEvent(ev *events.Event) error {
	var meta string
	if len(ev.Metadata) > 0 {
		b, err := json.Marshal(ev.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		meta = string(b)
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	if ev.Severity == "" {
		ev.Severity = events.SeverityInfo
	}
	res, err := s.db.Exec(`INSERT INTO events
		(ts, type, category, source, severity, subject, path, pid, process_name, cmdline, repo_path, session_id, metadata_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		formatTS(ev.Timestamp), ev.Type, ev.Category, ev.Source, ev.Severity,
		nullStr(ev.Subject), nullStr(ev.Path), nullInt(ev.PID), nullStr(ev.ProcessName),
		nullStr(ev.Cmdline), nullStr(ev.RepoPath), nullStr(ev.SessionID), nullStr(meta))
	if err != nil {
		return err
	}
	ev.ID, _ = res.LastInsertId()
	return nil
}

// QueryEvents returns events matching the filter, newest first.
func (s *Store) QueryEvents(f events.EventFilter) ([]events.Event, error) {
	var where []string
	var args []any
	if !f.Since.IsZero() {
		where = append(where, "ts >= ?")
		args = append(args, formatTS(f.Since))
	}
	if !f.Until.IsZero() {
		where = append(where, "ts <= ?")
		args = append(args, formatTS(f.Until))
	}
	if f.Type != "" {
		where = append(where, "type = ?")
		args = append(args, f.Type)
	}
	if f.Category != "" {
		where = append(where, "category = ?")
		args = append(args, f.Category)
	}
	if f.Path != "" {
		// match either exact path or basename contained
		where = append(where, "(path = ? OR path LIKE ?)")
		args = append(args, f.Path, "%"+f.Path)
	}
	if f.Severity != "" {
		where = append(where, "severity = ?")
		args = append(args, f.Severity)
	}
	if f.SessionID != "" {
		where = append(where, "session_id = ?")
		args = append(args, f.SessionID)
	}
	if f.RepoPath != "" {
		where = append(where, "repo_path = ?")
		args = append(args, f.RepoPath)
	}

	q := "SELECT id, ts, type, category, source, severity, subject, path, pid, process_name, cmdline, repo_path, session_id, metadata_json FROM events"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY ts ASC, id ASC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []events.Event
	for rows.Next() {
		var ev events.Event
		var ts string
		var subject, path, procName, cmdline, repoPath, sessionID, meta sql.NullString
		var pid sql.NullInt64
		if err := rows.Scan(&ev.ID, &ts, &ev.Type, &ev.Category, &ev.Source, &ev.Severity,
			&subject, &path, &pid, &procName, &cmdline, &repoPath, &sessionID, &meta); err != nil {
			return nil, err
		}
		ev.Timestamp = parseTS(ts)
		ev.Subject = subject.String
		ev.Path = path.String
		ev.PID = int(pid.Int64)
		ev.ProcessName = procName.String
		ev.Cmdline = cmdline.String
		ev.RepoPath = repoPath.String
		ev.SessionID = sessionID.String
		if meta.String != "" {
			_ = json.Unmarshal([]byte(meta.String), &ev.Metadata)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// CountEventsSince returns the number of events since t.
func (s *Store) CountEventsSince(t time.Time) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM events WHERE ts >= ?", formatTS(t)).Scan(&n)
	return n, err
}

// InsertSession persists a new session.
func (s *Store) InsertSession(sess *events.Session) error {
	_, err := s.db.Exec(`INSERT INTO sessions
		(id, started_at, ended_at, detected_agent, confidence, repo_path, status, summary, risk_count, files_changed_count, commands_seen_count, ports_opened_count)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		sess.ID, formatTS(sess.StartedAt), nullTime(sess.EndedAt), sess.DetectedAgent,
		sess.Confidence, sess.RepoPath, sess.Status, sess.Summary,
		sess.RiskCount, sess.FilesChangedCount, sess.CommandsSeenCount, sess.PortsOpenedCount)
	return err
}

// UpdateSession updates a session in place.
func (s *Store) UpdateSession(sess *events.Session) error {
	_, err := s.db.Exec(`UPDATE sessions SET
		ended_at=?, detected_agent=?, confidence=?, repo_path=?, status=?, summary=?,
		risk_count=?, files_changed_count=?, commands_seen_count=?, ports_opened_count=?
		WHERE id=?`,
		nullTime(sess.EndedAt), sess.DetectedAgent, sess.Confidence, sess.RepoPath,
		sess.Status, sess.Summary, sess.RiskCount, sess.FilesChangedCount,
		sess.CommandsSeenCount, sess.PortsOpenedCount, sess.ID)
	return err
}

// QuerySessions returns sessions matching the filter, newest first.
func (s *Store) QuerySessions(f events.SessionFilter) ([]events.Session, error) {
	var where []string
	var args []any
	if f.ID != "" {
		where = append(where, "id = ?")
		args = append(args, f.ID)
	}
	if !f.Since.IsZero() {
		where = append(where, "started_at >= ?")
		args = append(args, formatTS(f.Since))
	}
	if !f.Until.IsZero() {
		where = append(where, "started_at <= ?")
		args = append(args, formatTS(f.Until))
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.RepoPath != "" {
		where = append(where, "repo_path = ?")
		args = append(args, f.RepoPath)
	}
	q := "SELECT id, started_at, ended_at, detected_agent, confidence, repo_path, status, summary, risk_count, files_changed_count, commands_seen_count, ports_opened_count FROM sessions"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY started_at DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []events.Session
	for rows.Next() {
		var sess events.Session
		var started string
		var ended, agent, repo, summary sql.NullString
		var conf sql.NullFloat64
		if err := rows.Scan(&sess.ID, &started, &ended, &agent, &conf, &repo, &sess.Status,
			&summary, &sess.RiskCount, &sess.FilesChangedCount, &sess.CommandsSeenCount, &sess.PortsOpenedCount); err != nil {
			return nil, err
		}
		sess.StartedAt = parseTS(started)
		if ended.Valid && ended.String != "" {
			t := parseTS(ended.String)
			sess.EndedAt = &t
		}
		sess.DetectedAgent = agent.String
		sess.Confidence = conf.Float64
		sess.RepoPath = repo.String
		sess.Summary = summary.String
		out = append(out, sess)
	}
	return out, rows.Err()
}

// GetSession returns a single session by ID (or by unique prefix).
func (s *Store) GetSession(id string) (*events.Session, error) {
	sessions, err := s.QuerySessions(events.SessionFilter{ID: id})
	if err != nil {
		return nil, err
	}
	if len(sessions) == 1 {
		return &sessions[0], nil
	}
	// Fall back to prefix match.
	all, err := s.QuerySessions(events.SessionFilter{})
	if err != nil {
		return nil, err
	}
	var match *events.Session
	for i := range all {
		if strings.HasPrefix(all[i].ID, id) {
			if match != nil {
				return nil, fmt.Errorf("session id %q is ambiguous", id)
			}
			match = &all[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return match, nil
}

// --- watched paths ---

// AddWatchedPath registers a path (idempotent).
func (s *Store) AddWatchedPath(path string) error {
	_, err := s.db.Exec(`INSERT INTO watched_paths (path, created_at, enabled) VALUES (?, ?, 1)
		ON CONFLICT(path) DO UPDATE SET enabled=1`, path, formatTS(time.Now()))
	return err
}

// RemoveWatchedPath deletes a watched path.
func (s *Store) RemoveWatchedPath(path string) error {
	_, err := s.db.Exec("DELETE FROM watched_paths WHERE path = ?", path)
	return err
}

// ListWatchedPaths returns all enabled watched paths.
func (s *Store) ListWatchedPaths() ([]string, error) {
	rows, err := s.db.Query("SELECT path FROM watched_paths WHERE enabled = 1 ORDER BY path")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- daemon status ---

// DaemonStatus mirrors the daemon_status row.
type DaemonStatus struct {
	PID             int
	StartedAt       time.Time
	LastHeartbeatAt time.Time
	Status          string
	Version         string
	Metadata        map[string]any
}

// SetDaemonStatus upserts the singleton daemon status row.
func (s *Store) SetDaemonStatus(st DaemonStatus) error {
	var meta string
	if len(st.Metadata) > 0 {
		b, _ := json.Marshal(st.Metadata)
		meta = string(b)
	}
	_, err := s.db.Exec(`INSERT INTO daemon_status (id, pid, started_at, last_heartbeat_at, status, version, metadata_json)
		VALUES (1, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET pid=excluded.pid, started_at=excluded.started_at,
		last_heartbeat_at=excluded.last_heartbeat_at, status=excluded.status,
		version=excluded.version, metadata_json=excluded.metadata_json`,
		st.PID, formatTS(st.StartedAt), formatTS(st.LastHeartbeatAt), st.Status, st.Version, nullStr(meta))
	return err
}

// Heartbeat updates only the heartbeat timestamp.
func (s *Store) Heartbeat(t time.Time) error {
	_, err := s.db.Exec("UPDATE daemon_status SET last_heartbeat_at = ? WHERE id = 1", formatTS(t))
	return err
}

// GetDaemonStatus reads the singleton daemon status row.
func (s *Store) GetDaemonStatus() (*DaemonStatus, error) {
	var st DaemonStatus
	var started, heartbeat, status, version, meta sql.NullString
	var pid sql.NullInt64
	err := s.db.QueryRow("SELECT pid, started_at, last_heartbeat_at, status, version, metadata_json FROM daemon_status WHERE id = 1").
		Scan(&pid, &started, &heartbeat, &status, &version, &meta)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	st.PID = int(pid.Int64)
	st.StartedAt = parseTS(started.String)
	st.LastHeartbeatAt = parseTS(heartbeat.String)
	st.Status = status.String
	st.Version = version.String
	if meta.String != "" {
		_ = json.Unmarshal([]byte(meta.String), &st.Metadata)
	}
	return &st, nil
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return formatTS(*t)
}
