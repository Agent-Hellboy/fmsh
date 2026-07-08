// Package events defines the normalized event model that flows from
// collectors, through the risk detector and sessionizer, into the store.
//
// fmsh is a black box recorder for AI-driven development on macOS: every
// important local activity becomes an Event. Collectors emit events, the
// store persists them, and the CLI queries them to reconstruct what happened.
package events

import (
	"context"
	"time"
)

// Categories group event types for coarse filtering.
const (
	CategoryProcess = "process"
	CategoryFile    = "file"
	CategoryNetwork = "network"
	CategoryGit     = "git"
	CategoryRisk    = "risk"
	CategoryAgent   = "agent"
	CategoryGuard   = "guard"
)

// Severity levels.
const (
	SeverityInfo   = "info"
	SeverityLow    = "low"
	SeverityMedium = "medium"
	SeverityHigh   = "high"
)

// Event types. These are the normalized, stable identifiers stored in the DB.
const (
	TypeProcessStart = "process.start"
	TypeProcessExit  = "process.exit"

	TypeFileCreate = "file.create"
	TypeFileWrite  = "file.write"
	TypeFileDelete = "file.delete"
	TypeFileRename = "file.rename"
	TypeFileChmod  = "file.chmod"

	TypePortOpen  = "port.open"
	TypePortClose = "port.close"

	TypeGitFileModified = "git.file_modified"
	TypeGitFileAdded    = "git.file_added"
	TypeGitFileDeleted  = "git.file_deleted"
	TypeGitDiffSummary  = "git.diff_summary"

	TypeRiskSecretTouched         = "risk.secret_touched"
	TypeRiskDependencyChanged     = "risk.dependency_changed"
	TypeRiskLaunchAgentChanged    = "risk.launch_agent_changed"
	TypeRiskLargeFileCreated      = "risk.large_file_created"
	TypeRiskManyFilesChanged      = "risk.many_files_changed"
	TypeRiskDestructiveCommand    = "risk.destructive_command"
	TypeRiskNewExecutableDownload = "risk.new_executable_downloads"

	TypeAgentSessionStarted = "agent.session_started"
	TypeAgentSessionEnded   = "agent.session_ended"

	TypeCheckpointCreated  = "checkpoint.created"
	TypeCheckpointRestored = "checkpoint.restored"
)

// Sources identify which collector produced an event.
const (
	SourceFSEvents     = "fsevents"
	SourceProcess      = "process_collector"
	SourcePort         = "port_collector"
	SourceGit          = "git_collector"
	SourceRiskDetector = "risk_detector"
	SourceSessionizer  = "sessionizer"
	SourceGuard        = "shell_guard"
)

// Event is a single normalized activity record.
type Event struct {
	ID          int64          `json:"id"`
	Timestamp   time.Time      `json:"ts"`
	Type        string         `json:"type"`
	Category    string         `json:"category"`
	Source      string         `json:"source"`
	Severity    string         `json:"severity"`
	Subject     string         `json:"subject,omitempty"`
	Path        string         `json:"path,omitempty"`
	PID         int            `json:"pid,omitempty"`
	ProcessName string         `json:"process_name,omitempty"`
	Cmdline     string         `json:"cmdline,omitempty"`
	RepoPath    string         `json:"repo_path,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Session groups a burst of related AI/dev activity.
type Session struct {
	ID                string     `json:"id"`
	StartedAt         time.Time  `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	DetectedAgent     string     `json:"detected_agent"`
	Confidence        float64    `json:"confidence"`
	RepoPath          string     `json:"repo_path"`
	Status            string     `json:"status"`
	Summary           string     `json:"summary"`
	RiskCount         int        `json:"risk_count"`
	FilesChangedCount int        `json:"files_changed_count"`
	CommandsSeenCount int        `json:"commands_seen_count"`
	PortsOpenedCount  int        `json:"ports_opened_count"`
}

// Session statuses.
const (
	SessionActive = "active"
	SessionEnded  = "ended"
)

// EventFilter constrains QueryEvents. Zero values mean "no constraint".
type EventFilter struct {
	Since     time.Time
	Until     time.Time
	Type      string
	Category  string
	Path      string
	Severity  string
	SessionID string
	RepoPath  string
	Limit     int
}

// SessionFilter constrains QuerySessions.
type SessionFilter struct {
	Since    time.Time
	Until    time.Time
	Status   string
	RepoPath string
	ID       string
	Limit    int
}

// EventSink receives events emitted by collectors. Implementations are
// responsible for persistence, session assignment, and risk evaluation.
type EventSink interface {
	Emit(event Event) error
}

// Collector is an independent loop that observes some aspect of the machine
// and emits events until its context is cancelled.
type Collector interface {
	Name() string
	Run(ctx context.Context, sink EventSink) error
}
