// Package cli implements the fmsh command surface using Cobra. The CLI only
// reads and renders; the daemon (fmshd) is the sole writer of activity events.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"fmsh/internal/config"
	"fmsh/internal/db"
	"fmsh/internal/events"
	"fmsh/internal/output"
	"fmsh/internal/store"
	"fmsh/internal/timeparse"
)

var (
	flagJSON    bool
	flagNoColor bool
)

const longDescription = `fmsh — Forensic Machine Shell

A black box recorder for AI-driven development on macOS.

It runs locally, records activity events, groups them into sessions, detects
risky changes, and gives developers an audit trail for agentic coding workflows.

The fmshd daemon is always watching. Every important local activity becomes an
event. These commands let you ask what happened.`

// NewRootCmd builds the root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "fmsh",
		Short:         "A black box recorder for AI-driven development on macOS",
		Long:          longDescription,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if flagNoColor {
				output.SetColor(false)
			}
		},
	}
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "output machine-readable JSON")
	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable colored output")

	root.AddCommand(
		newInitCmd(),
		newDaemonCmd(),
		newWatchCmd(),
		newTimelineCmd(),
		newEventsCmd(),
		newWhatChangedCmd(),
		newAgentReportCmd(),
		newSessionsCmd(),
		newSessionCmd(),
		newRisksCmd(),
		newBlameCmd(),
		newDoctorCmd(),
		newGuardCmd(),
		newShellInitCmd(),
		newCheckpointsCmd(),
		newRestoreCmd(),
	)
	return root
}

// Execute runs the CLI.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, output.Red("error: ")+err.Error())
		os.Exit(1)
	}
}

// openStore loads config and opens the event store.
func openStore() (*config.Config, *store.Store, *sql.DB, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, err
	}
	dbPath := cfg.ResolvedDBPath()
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil, nil, fmt.Errorf("database not found at %s — run `fmsh init` first", dbPath)
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, nil, err
	}
	return cfg, store.New(sqlDB), sqlDB, nil
}

// timeRange resolves --since / --from / --to flags into a [since, until] range.
// An empty range (both zero) means "all time".
func timeRange(since, from, to string) (start, end time.Time, err error) {
	if since != "" {
		start, err = timeparse.ParseSince(since)
		return start, time.Time{}, err
	}
	if from != "" {
		start, err = timeparse.ParseMoment(from)
		if err != nil {
			return
		}
	}
	if to != "" {
		end, err = timeparse.ParseMoment(to)
		if err != nil {
			return
		}
	}
	return start, end, nil
}

// addTimeFlags registers the standard --since/--from/--to flags.
func addTimeFlags(cmd *cobra.Command, since, from, to *string, defaultSince string) {
	cmd.Flags().StringVar(since, "since", defaultSince, "relative window, e.g. 30m, 1h, 24h")
	cmd.Flags().StringVar(from, "from", "", "start time, e.g. \"5pm\" or \"yesterday 6pm\"")
	cmd.Flags().StringVar(to, "to", "", "end time, e.g. \"6pm\"")
}

// queryRange builds an EventFilter time window from the flags, preferring
// --from/--to when supplied.
func queryRange(since, from, to string) (events.EventFilter, error) {
	var f events.EventFilter
	// If from/to given, since default should be ignored.
	if from != "" || to != "" {
		since = ""
	}
	start, end, err := timeRange(since, from, to)
	if err != nil {
		return f, err
	}
	f.Since = start
	f.Until = end
	return f, nil
}

// capitalize upper-cases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// rangeLabel describes the active window for report headers.
func rangeLabel(since, from, to string) string {
	if from != "" || to != "" {
		if from != "" && to != "" {
			return fmt.Sprintf("%s → %s", from, to)
		}
		if from != "" {
			return "since " + from
		}
		return "until " + to
	}
	if since != "" {
		return "last " + since
	}
	return "all time"
}
