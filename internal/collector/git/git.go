// Package git periodically samples the working-tree state of watched Git
// repositories using cheap porcelain commands and emits git.* events when the
// state changes. AI coding agents mostly modify repositories, so this is the
// most important collector for audit purposes.
package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

// Collector polls Git repositories under watched paths.
type Collector struct {
	cfg      *config.Config
	interval time.Duration
	logf     func(string, ...any)

	// prev holds the last-seen porcelain status per repo, so we only emit on
	// change.
	prevStatus  map[string]map[string]string // repo -> path -> status
	prevSummary map[string]string            // repo -> diff --stat summary
}

// New builds a git collector.
func New(cfg *config.Config, logf func(string, ...any)) *Collector {
	iv := cfg.Daemon.GitPollInterval.Duration
	if iv <= 0 {
		iv = 15 * time.Second
	}
	return &Collector{
		cfg:         cfg,
		interval:    iv,
		logf:        logf,
		prevStatus:  map[string]map[string]string{},
		prevSummary: map[string]string{},
	}
}

// Name implements events.Collector.
func (c *Collector) Name() string { return "git" }

// Run polls until ctx is cancelled.
func (c *Collector) Run(ctx context.Context, sink events.EventSink) error {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	// Prime state without emitting, so we don't flood on first poll.
	c.poll(ctx, sink, true)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.poll(ctx, sink, false)
		}
	}
}

func (c *Collector) poll(ctx context.Context, sink events.EventSink, prime bool) {
	for _, repo := range c.discoverRepos() {
		c.pollRepo(ctx, sink, repo, prime)
	}
}

// discoverRepos returns Git repos: each watched path that is a repo, plus its
// immediate (depth-1) child repos. Bounded to avoid deep scans.
func (c *Collector) discoverRepos() []string {
	seen := map[string]bool{}
	var repos []string
	add := func(p string) {
		if isRepo(p) && !seen[p] {
			seen[p] = true
			repos = append(repos, p)
		}
	}
	for _, root := range c.cfg.ResolvedWatchPaths() {
		add(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	return repos
}

func (c *Collector) pollRepo(ctx context.Context, sink events.EventSink, repo string, prime bool) {
	status, err := c.gitStatus(ctx, repo)
	if err != nil {
		return // not fatal; repo may be mid-operation
	}
	prev := c.prevStatus[repo]
	if !prime {
		for path, code := range status {
			if prev[path] == code {
				continue // unchanged
			}
			typ, sev := classify(code)
			ev := events.Event{
				Timestamp: time.Now(),
				Type:      typ,
				Category:  events.CategoryGit,
				Source:    events.SourceGit,
				Severity:  sev,
				Subject:   filepath.Base(path),
				Path:      filepath.Join(repo, path),
				RepoPath:  repo,
				Metadata:  map[string]any{"status": code},
			}
			if err := sink.Emit(ev); err != nil {
				c.logf("git: emit failed: %v", err)
			}
		}
	}
	c.prevStatus[repo] = status

	// Diff summary.
	summary, err := c.gitDiffStat(ctx, repo)
	if err == nil && summary != "" && summary != c.prevSummary[repo] {
		c.prevSummary[repo] = summary
		if !prime {
			ev := events.Event{
				Timestamp: time.Now(),
				Type:      events.TypeGitDiffSummary,
				Category:  events.CategoryGit,
				Source:    events.SourceGit,
				Severity:  events.SeverityInfo,
				Subject:   filepath.Base(repo),
				RepoPath:  repo,
				Metadata:  map[string]any{"summary": summary},
			}
			if err := sink.Emit(ev); err != nil {
				c.logf("git: emit failed: %v", err)
			}
		}
	} else if err == nil {
		c.prevSummary[repo] = summary
	}
}

// gitStatus returns porcelain status keyed by path.
func (c *Collector) gitStatus(ctx context.Context, repo string) (map[string]string, error) {
	out, err := runGit(ctx, repo, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		code := strings.TrimSpace(line[:2])
		rest := line[3:]
		// Handle renames "old -> new".
		if i := strings.Index(rest, " -> "); i >= 0 {
			rest = rest[i+4:]
		}
		result[strings.TrimSpace(rest)] = code
	}
	return result, nil
}

// gitDiffStat returns a short diff --stat summary line.
func (c *Collector) gitDiffStat(ctx context.Context, repo string) (string, error) {
	out, err := runGit(ctx, repo, "diff", "--stat")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 0 {
		return "", nil
	}
	// The final line of diff --stat is the summary ("N files changed, ...").
	last := strings.TrimSpace(lines[len(lines)-1])
	if strings.Contains(last, "changed") {
		return last, nil
	}
	return "", nil
}

// classify maps a porcelain status code to an event type and severity.
func classify(code string) (string, string) {
	switch {
	case strings.Contains(code, "D"):
		return events.TypeGitFileDeleted, events.SeverityInfo
	case strings.Contains(code, "A") || code == "??":
		return events.TypeGitFileAdded, events.SeverityInfo
	default:
		return events.TypeGitFileModified, events.SeverityInfo
	}
}

func isRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular() // .git dir or gitfile (worktrees)
}

func runGit(ctx context.Context, repo string, args ...string) (string, error) {
	full := append([]string{"-C", repo}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}
