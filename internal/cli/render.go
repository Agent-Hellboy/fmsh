package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"fmsh/internal/events"
	"fmsh/internal/output"
)

// homeDir is a small indirection for path shortening.
var homeDir = os.UserHomeDir

// typeLabel returns a fixed-width, upper-snake label for an event type.
func typeLabel(t string) string {
	switch t {
	case events.TypeProcessStart:
		return "PROCESS_START"
	case events.TypeProcessExit:
		return "PROCESS_EXIT"
	case events.TypeFileCreate:
		return "FILE_CREATE"
	case events.TypeFileWrite:
		return "FILE_WRITE"
	case events.TypeFileDelete:
		return "FILE_DELETE"
	case events.TypeFileRename:
		return "FILE_RENAME"
	case events.TypeFileChmod:
		return "FILE_CHMOD"
	case events.TypePortOpen:
		return "PORT_OPEN"
	case events.TypePortClose:
		return "PORT_CLOSE"
	case events.TypeGitFileModified:
		return "GIT_MODIFIED"
	case events.TypeGitFileAdded:
		return "GIT_ADDED"
	case events.TypeGitFileDeleted:
		return "GIT_DELETED"
	case events.TypeGitDiffSummary:
		return "GIT_DIFF"
	case events.TypeAgentSessionStarted:
		return "SESSION_START"
	case events.TypeAgentSessionEnded:
		return "SESSION_END"
	case events.TypeCheckpointCreated:
		return "CHECKPOINT"
	case events.TypeCheckpointRestored:
		return "RESTORE"
	default:
		if strings.HasPrefix(t, "risk.") {
			return "RISK"
		}
		return strings.ToUpper(t)
	}
}

// describe returns a compact human description of an event's subject.
func describe(ev events.Event) string {
	switch ev.Category {
	case events.CategoryFile, events.CategoryGit:
		if ev.Path != "" {
			return shortPath(ev.Path)
		}
		if ev.RepoPath != "" {
			if s, ok := ev.Metadata["summary"].(string); ok {
				return filepath.Base(ev.RepoPath) + ": " + s
			}
			return filepath.Base(ev.RepoPath)
		}
	case events.CategoryProcess:
		name := ev.ProcessName
		if tool, ok := ev.Metadata["detected_tool"].(string); ok && tool != "" {
			return name + output.Dim(" ("+tool+")")
		}
		return name
	case events.CategoryNetwork:
		if ev.ProcessName != "" {
			return ev.Subject + " by " + ev.ProcessName
		}
		return ev.Subject
	case events.CategoryRisk:
		return ev.Subject
	case events.CategoryAgent:
		return ev.Subject
	}
	if ev.Subject != "" {
		return ev.Subject
	}
	return ev.Path
}

// shortPath abbreviates the home dir and long prefixes for readability.
func shortPath(p string) string {
	if home, err := homeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// renderTimeline prints events chronologically with clock, label, and detail.
func renderTimeline(w io.Writer, evs []events.Event) {
	for _, ev := range evs {
		clock := output.Dim(output.Pad(output.Clock(ev.Timestamp), 8))
		label := typeLabel(ev.Type)
		if ev.Category == events.CategoryRisk {
			fmt.Fprintf(w, "%s %s %s %s\n", clock, output.SeveritySymbol(ev.Severity),
				colorLabel(ev, output.Pad(label, 14)), describe(ev))
		} else {
			fmt.Fprintf(w, "%s   %s %s\n", clock, colorLabel(ev, output.Pad(label, 14)), describe(ev))
		}
	}
}

func colorLabel(ev events.Event, label string) string {
	switch ev.Category {
	case events.CategoryRisk:
		if ev.Severity == events.SeverityHigh {
			return output.Red(label)
		}
		return output.Yellow(label)
	case events.CategoryProcess:
		return output.Cyan(label)
	case events.CategoryNetwork:
		return output.Cyan(label)
	case events.CategoryGuard:
		return output.Green(label)
	default:
		return label
	}
}
