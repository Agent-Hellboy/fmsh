package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Checkpoint records an APFS local snapshot taken as a restore point before a
// risky moment. fmsh stores only a reference to the OS snapshot — never file
// contents.
type Checkpoint struct {
	ID           string     `json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	Trigger      string     `json:"trigger"` // session_start | destructive_command | high_risk | manual
	Root         string     `json:"root,omitempty"`
	Command      string     `json:"command,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	SnapshotName string     `json:"snapshot_name,omitempty"`
	SnapshotDate string     `json:"snapshot_date,omitempty"`
	Status       string     `json:"status"` // saved | skipped | restored
	SessionID    string     `json:"session_id,omitempty"`
	RestoredAt   *time.Time `json:"restored_at,omitempty"`
}

// Checkpoint statuses.
const (
	CheckpointSaved    = "saved"
	CheckpointSkipped  = "skipped"
	CheckpointRestored = "restored"
)

// Checkpoint triggers.
const (
	TriggerSessionStart = "session_start"
	TriggerDestructive  = "destructive_command"
	TriggerHighRisk     = "high_risk"
	TriggerManual       = "manual"
)

// InsertCheckpoint persists a checkpoint record.
func (s *Store) InsertCheckpoint(c *Checkpoint) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	if c.Status == "" {
		c.Status = CheckpointSaved
	}
	_, err := s.db.Exec(`INSERT INTO checkpoints
		(id, created_at, trigger, root, command, reason, snapshot_name, snapshot_date, status, session_id, restored_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, formatTS(c.CreatedAt), c.Trigger, nullStr(c.Root), nullStr(c.Command), nullStr(c.Reason),
		nullStr(c.SnapshotName), nullStr(c.SnapshotDate), c.Status, nullStr(c.SessionID), nullTime(c.RestoredAt))
	return err
}

// MarkCheckpointRestored records that a checkpoint was restored.
func (s *Store) MarkCheckpointRestored(id string, at time.Time) error {
	_, err := s.db.Exec("UPDATE checkpoints SET status=?, restored_at=? WHERE id=?",
		CheckpointRestored, formatTS(at), id)
	return err
}

// ListCheckpoints returns checkpoints newest first, optionally since a time.
func (s *Store) ListCheckpoints(since time.Time, limit int) ([]Checkpoint, error) {
	q := "SELECT id, created_at, trigger, root, command, reason, snapshot_name, snapshot_date, status, session_id, restored_at FROM checkpoints"
	var args []any
	if !since.IsZero() {
		q += " WHERE created_at >= ?"
		args = append(args, formatTS(since))
	}
	q += " ORDER BY created_at DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		c, err := scanCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCheckpoint returns a checkpoint by id or unique prefix.
func (s *Store) GetCheckpoint(id string) (*Checkpoint, error) {
	rows, err := s.db.Query("SELECT id, created_at, trigger, root, command, reason, snapshot_name, snapshot_date, status, session_id, restored_at FROM checkpoints WHERE id = ? OR id LIKE ?", id, id+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []Checkpoint
	for rows.Next() {
		c, err := scanCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, c)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("checkpoint %q not found", id)
	case 1:
		return &matches[0], nil
	default:
		for i := range matches {
			if matches[i].ID == id {
				return &matches[i], nil
			}
		}
		return nil, fmt.Errorf("checkpoint id %q is ambiguous", id)
	}
}

func scanCheckpoint(rows *sql.Rows) (Checkpoint, error) {
	var c Checkpoint
	var created string
	var root, command, reason, snapName, snapDate, session, restored sql.NullString
	if err := rows.Scan(&c.ID, &created, &c.Trigger, &root, &command, &reason,
		&snapName, &snapDate, &c.Status, &session, &restored); err != nil {
		return c, err
	}
	c.CreatedAt = parseTS(created)
	c.Root = root.String
	c.Command = command.String
	c.Reason = reason.String
	c.SnapshotName = snapName.String
	c.SnapshotDate = snapDate.String
	c.SessionID = session.String
	if restored.Valid && restored.String != "" {
		t := parseTS(restored.String)
		c.RestoredAt = &t
	}
	return c, nil
}
