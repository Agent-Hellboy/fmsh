// Package process polls the macOS process table and emits process.start and
// process.exit events by diffing successive snapshots. It shells out to `ps`
// so it needs no sudo and no special entitlements.
package process

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

// Collector polls processes.
type Collector struct {
	cfg      *config.Config
	interval time.Duration
	logf     func(string, ...any)
}

// New builds a process collector.
func New(cfg *config.Config, logf func(string, ...any)) *Collector {
	iv := cfg.Daemon.ProcessPollInterval.Duration
	if iv <= 0 {
		iv = 2 * time.Second
	}
	return &Collector{cfg: cfg, interval: iv, logf: logf}
}

// Name implements events.Collector.
func (c *Collector) Name() string { return "process" }

type procInfo struct {
	pid     int
	ppid    int
	user    string
	name    string
	cmdline string
}

// Run polls until ctx is cancelled.
func (c *Collector) Run(ctx context.Context, sink events.EventSink) error {
	prev, err := c.snapshot(ctx)
	if err != nil {
		c.logf("process: initial snapshot failed: %v", err)
		prev = map[int]procInfo{}
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			cur, err := c.snapshot(ctx)
			if err != nil {
				c.logf("process: snapshot failed: %v", err)
				continue
			}
			c.diff(prev, cur, sink)
			prev = cur
		}
	}
}

func (c *Collector) diff(prev, cur map[int]procInfo, sink events.EventSink) {
	for pid, p := range cur {
		if _, ok := prev[pid]; !ok {
			c.emit(sink, events.TypeProcessStart, p)
		}
	}
	for pid, p := range prev {
		if _, ok := cur[pid]; !ok {
			c.emit(sink, events.TypeProcessExit, p)
		}
	}
}

func (c *Collector) emit(sink events.EventSink, typ string, p procInfo) {
	// Focus on audit signal: only record processes that match a known AI/dev
	// tool. This keeps the timeline free of macOS system churn (mdworker,
	// GUI apps) and our own `ps`/`lsof` probes. Everything else is ignored in
	// v1; a broader capture mode can be added later.
	tool, ttype, conf, reason := DetectTool(p.name, p.cmdline)
	if tool == "" || c.isOwnProbe(p) {
		return
	}

	cmdline := p.cmdline
	if !c.cfg.Privacy.StoreCmdline {
		cmdline = ""
	}
	ev := events.Event{
		Timestamp:   time.Now(),
		Type:        typ,
		Category:    events.CategoryProcess,
		Source:      events.SourceProcess,
		Severity:    events.SeverityInfo,
		Subject:     p.name,
		PID:         p.pid,
		ProcessName: p.name,
		Cmdline:     redact(cmdline, c.cfg.Privacy.RedactSecrets),
		Metadata: map[string]any{
			"ppid":          p.ppid,
			"user":          p.user,
			"detected_tool": tool,
			"tool_type":     ttype,
			"confidence":    conf,
			"reason":        reason,
		},
	}
	if err := sink.Emit(ev); err != nil {
		c.logf("process: emit failed: %v", err)
	}
}

// isOwnProbe reports whether p is one of fmsh's own polling commands, which we
// never want to record.
func (c *Collector) isOwnProbe(p procInfo) bool {
	switch p.name {
	case "ps", "lsof":
		return true
	}
	return false
}

// snapshot returns the current process set keyed by pid.
func (c *Collector) snapshot(ctx context.Context) (map[int]procInfo, error) {
	cmd := exec.CommandContext(ctx, "ps", "-axww", "-o", "pid=,ppid=,user=,command=")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	result := make(map[int]procInfo)
	sc := bufio.NewScanner(&out)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		p, ok := parsePSLine(line)
		if !ok {
			continue
		}
		result[p.pid] = p
	}
	return result, sc.Err()
}

// parsePSLine parses "  pid ppid user command with args...".
func parsePSLine(line string) (procInfo, bool) {
	line = strings.TrimLeft(line, " ")
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return procInfo{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return procInfo{}, false
	}
	ppid, _ := strconv.Atoi(fields[1])
	user := fields[2]

	// Reconstruct the command by skipping the first three fields in the
	// original (leading-trimmed) line.
	idx := 0
	seen := 0
	for i := 0; i < len(line) && seen < 3; i++ {
		if line[i] == ' ' {
			// collapse runs of spaces
			for i < len(line) && line[i] == ' ' {
				i++
			}
			seen++
			idx = i
			i--
		}
	}
	cmdline := strings.TrimSpace(line[idx:])
	name := cmdline
	if first := strings.Fields(cmdline); len(first) > 0 {
		name = filepath.Base(first[0])
	}
	return procInfo{pid: pid, ppid: ppid, user: user, name: name, cmdline: cmdline}, true
}

func redact(s string, on bool) string {
	if !on {
		return s
	}
	return Redact(s)
}
