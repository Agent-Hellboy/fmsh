// Package doctor runs environment and health checks for fmsh.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/daemon"
	"fmsh/internal/db"
	"fmsh/internal/store"
)

// Status of a single check.
type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
)

// Check is one diagnostic result.
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

// Run performs all checks and returns them in order.
func Run() []Check {
	var checks []Check
	add := func(name string, st Status, detail string) {
		checks = append(checks, Check{Name: name, Status: st, Detail: detail})
	}

	// Config file.
	cfgPath, _ := config.Path()
	if _, err := os.Stat(cfgPath); err == nil {
		add("config", OK, cfgPath)
	} else {
		add("config", Warn, "not found — run `fmsh init`")
	}

	cfg, err := config.Load()
	if err != nil {
		add("config parse", Fail, err.Error())
		return checks
	}

	// Database reachable + WAL.
	dbPath := cfg.ResolvedDBPath()
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		add("database", Fail, err.Error())
		return checks
	}
	defer sqlDB.Close()
	add("database", OK, dbPath)

	if wal, err := db.WALEnabled(sqlDB); err == nil && wal {
		add("wal mode", OK, "enabled")
	} else {
		add("wal mode", Warn, "not enabled")
	}

	st := store.New(sqlDB)

	// Daemon pid + heartbeat.
	if pid, running := daemon.IsRunning(); running {
		add("daemon", OK, fmt.Sprintf("running (pid %d)", pid))
		if ds, _ := st.GetDaemonStatus(); ds != nil && !ds.LastHeartbeatAt.IsZero() {
			age := time.Since(ds.LastHeartbeatAt)
			if age < 60*time.Second {
				add("heartbeat", OK, fmt.Sprintf("%s ago", age.Round(time.Second)))
			} else {
				add("heartbeat", Warn, fmt.Sprintf("stale (%s ago)", age.Round(time.Second)))
			}
		}
	} else {
		add("daemon", Warn, "not running — run `fmsh daemon start`")
	}

	// Watched paths exist.
	missing := 0
	for _, p := range cfg.ResolvedWatchPaths() {
		if _, err := os.Stat(p); err != nil {
			missing++
		}
	}
	if missing == 0 {
		add("watch paths", OK, fmt.Sprintf("%d configured, all present", len(cfg.WatchPaths)))
	} else {
		add("watch paths", Warn, fmt.Sprintf("%d of %d missing (skipped)", missing, len(cfg.WatchPaths)))
	}

	// External tools.
	for _, tool := range []string{"ps", "lsof", "git"} {
		if _, err := exec.LookPath(tool); err == nil {
			add("tool: "+tool, OK, "available")
		} else {
			add("tool: "+tool, Warn, "not found on PATH")
		}
	}

	// macOS permission hint.
	add("permissions", Warn, "if file events are missing, grant your terminal Full Disk Access in System Settings > Privacy & Security")

	return checks
}

// HasFailures reports whether any check failed.
func HasFailures(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}
