// Package risk turns ordinary activity events into risk events. Rules are
// intentionally conservative to limit false positives; each rule documents its
// severity. The windowed "many files changed" rule lives in the daemon because
// it needs cross-event state.
package risk

import (
	"os"
	"path/filepath"
	"strings"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

// secretPatterns match sensitive files (substring match against the full path,
// case-sensitive where it matters).
var secretPatterns = []string{
	".env",
	".env.local",
	".env.production",
	"id_rsa",
	"id_ed25519",
	"/.ssh/",
	"/.aws/credentials",
	"/.aws/config",
	"/.kube/config",
	"credentials.json",
	"secrets.yaml",
	"secrets.yml",
}

// dependencyFiles are lockfiles/manifests whose changes warrant review.
var dependencyFiles = map[string]bool{
	"package.json":      true,
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"go.mod":            true,
	"go.sum":            true,
	"Cargo.toml":        true,
	"Cargo.lock":        true,
	"requirements.txt":  true,
	"poetry.lock":       true,
	"pyproject.toml":    true,
}

// destructiveSubstrings flag risky command lines.
var destructiveSubstrings = []string{
	"rm -rf",
	"chmod -R",
	"chown -R",
	"curl | sh",
	"curl|sh",
	"wget | sh",
	"wget|sh",
	"sudo ",
}

// downloadExecExts flag new executable-like files under Downloads.
var downloadExecExts = map[string]bool{
	".dmg":     true,
	".pkg":     true,
	".app":     true,
	".command": true,
	".sh":      true,
}

// EvaluateEvent inspects a single source event and returns any risk events it
// implies. It never mutates the input.
func EvaluateEvent(ev events.Event, cfg config.RiskConfig) []events.Event {
	var out []events.Event

	switch ev.Category {
	case events.CategoryFile, events.CategoryGit:
		out = append(out, evalPath(ev, cfg)...)
	case events.CategoryProcess:
		if r, ok := evalCommand(ev); ok {
			out = append(out, r)
		}
	}
	return out
}

func evalPath(ev events.Event, cfg config.RiskConfig) []events.Event {
	var out []events.Event
	path := ev.Path
	if path == "" {
		return out
	}
	base := filepath.Base(path)

	// Secret file touched.
	if matchesSecret(path) {
		out = append(out, derive(ev, events.TypeRiskSecretTouched, events.SeverityHigh,
			base+" (secret/config file) was touched", map[string]any{"rule": "secret_touched", "trigger": ev.Type}))
	}

	// Dependency / lockfile changed.
	if dependencyFiles[base] {
		out = append(out, derive(ev, events.TypeRiskDependencyChanged, events.SeverityMedium,
			base+" dependency manifest changed", map[string]any{"rule": "dependency_changed", "trigger": ev.Type}))
	}

	// LaunchAgent / LaunchDaemon changed.
	if isLaunchAgent(path) {
		out = append(out, derive(ev, events.TypeRiskLaunchAgentChanged, events.SeverityHigh,
			base+" launch agent/daemon changed", map[string]any{"rule": "launch_agent_changed", "trigger": ev.Type}))
	}

	// Large file created/modified.
	if cfg.LargeFileMB > 0 && (ev.Type == events.TypeFileCreate || ev.Type == events.TypeFileWrite) {
		if size, ok := metaInt(ev.Metadata, "size_bytes"); ok {
			if size >= int64(cfg.LargeFileMB)*1024*1024 {
				out = append(out, derive(ev, events.TypeRiskLargeFileCreated, events.SeverityMedium,
					base+" is a large file", map[string]any{"rule": "large_file_created", "size_bytes": size}))
			}
		}
	}

	// New executable-like download.
	if ev.Type == events.TypeFileCreate && isUnderDownloads(path) {
		ext := strings.ToLower(filepath.Ext(path))
		if downloadExecExts[ext] {
			out = append(out, derive(ev, events.TypeRiskNewExecutableDownload, events.SeverityMedium,
				base+" (executable) appeared in Downloads", map[string]any{"rule": "new_executable_downloads", "ext": ext}))
		}
	}

	return out
}

func evalCommand(ev events.Event) (events.Event, bool) {
	if ev.Type != events.TypeProcessStart {
		return events.Event{}, false
	}
	cmd := ev.Cmdline
	if cmd == "" {
		cmd = ev.ProcessName
	}
	if matched, ok := IsDestructiveCommand(cmd); ok {
		return derive(ev, events.TypeRiskDestructiveCommand, events.SeverityHigh,
			"destructive command observed: "+truncate(cmd, 80),
			map[string]any{"rule": "destructive_command", "matched": matched}), true
	}
	return events.Event{}, false
}

// IsDestructiveCommand reports whether a command line matches a known
// destructive pattern, returning the matched pattern. It is shared by the risk
// detector and the shell guard so both use one rule set.
func IsDestructiveCommand(cmd string) (matched string, ok bool) {
	lower := strings.ToLower(cmd)
	for _, sub := range destructiveSubstrings {
		if strings.Contains(lower, sub) {
			return strings.TrimSpace(sub), true
		}
	}
	return "", false
}

func matchesSecret(path string) bool {
	base := filepath.Base(path)
	for _, p := range secretPatterns {
		if strings.HasPrefix(p, "/") || strings.Contains(p, "/") {
			if strings.Contains(path, p) {
				return true
			}
		} else if base == p {
			return true
		}
	}
	// gcloud credentials directory
	if strings.Contains(path, "/gcloud/") && strings.Contains(base, "credential") {
		return true
	}
	return false
}

func isLaunchAgent(path string) bool {
	home, _ := homeDir()
	candidates := []string{
		"/Library/LaunchAgents/",
		"/Library/LaunchDaemons/",
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, "Library/LaunchAgents")+"/")
	}
	for _, c := range candidates {
		if strings.Contains(path, c) {
			return true
		}
	}
	return false
}

func isUnderDownloads(path string) bool {
	return strings.Contains(path, "/Downloads/")
}

func derive(src events.Event, typ, sev, subject string, meta map[string]any) events.Event {
	return events.Event{
		Timestamp:   src.Timestamp,
		Type:        typ,
		Category:    events.CategoryRisk,
		Source:      events.SourceRiskDetector,
		Severity:    sev,
		Subject:     subject,
		Path:        src.Path,
		PID:         src.PID,
		ProcessName: src.ProcessName,
		Cmdline:     src.Cmdline,
		RepoPath:    src.RepoPath,
		SessionID:   src.SessionID,
		Metadata:    meta,
	}
}

func metaInt(m map[string]any, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	switch v := m[key].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	}
	return 0, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// homeDir is a small indirection so risk rules can be tested.
var homeDir = os.UserHomeDir
