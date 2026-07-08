// Package ports polls listening TCP sockets via `lsof` and emits port.open and
// port.close events by diffing successive snapshots.
package ports

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

// Collector polls listening ports.
type Collector struct {
	cfg      *config.Config
	interval time.Duration
	logf     func(string, ...any)
}

// New builds a port collector.
func New(cfg *config.Config, logf func(string, ...any)) *Collector {
	iv := cfg.Daemon.PortPollInterval.Duration
	if iv <= 0 {
		iv = 10 * time.Second
	}
	return &Collector{cfg: cfg, interval: iv, logf: logf}
}

// Name implements events.Collector.
func (c *Collector) Name() string { return "ports" }

type listener struct {
	proto   string
	addr    string
	port    int
	pid     int
	command string
}

func (l listener) key() string {
	return l.proto + "|" + l.addr + ":" + strconv.Itoa(l.port) + "|" + strconv.Itoa(l.pid)
}

// Run polls until ctx is cancelled.
func (c *Collector) Run(ctx context.Context, sink events.EventSink) error {
	prev, err := c.snapshot(ctx)
	if err != nil {
		c.logf("ports: initial snapshot failed: %v", err)
		prev = map[string]listener{}
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
				c.logf("ports: snapshot failed: %v", err)
				continue
			}
			for k, l := range cur {
				if _, ok := prev[k]; !ok {
					c.emit(sink, events.TypePortOpen, l)
				}
			}
			for k, l := range prev {
				if _, ok := cur[k]; !ok {
					c.emit(sink, events.TypePortClose, l)
				}
			}
			prev = cur
		}
	}
}

func (c *Collector) emit(sink events.EventSink, typ string, l listener) {
	subject := l.addr + ":" + strconv.Itoa(l.port)
	ev := events.Event{
		Timestamp:   time.Now(),
		Type:        typ,
		Category:    events.CategoryNetwork,
		Source:      events.SourcePort,
		Severity:    events.SeverityInfo,
		Subject:     subject,
		PID:         l.pid,
		ProcessName: l.command,
		Metadata: map[string]any{
			"protocol":   l.proto,
			"local_addr": l.addr,
			"port":       l.port,
		},
	}
	if err := sink.Emit(ev); err != nil {
		c.logf("ports: emit failed: %v", err)
	}
}

// snapshot returns currently-listening TCP sockets keyed by identity.
func (c *Collector) snapshot(ctx context.Context) (map[string]listener, error) {
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
	var out bytes.Buffer
	cmd.Stdout = &out
	// lsof exits non-zero when there are no matches; tolerate that if we got
	// parseable output.
	_ = cmd.Run()

	result := make(map[string]listener)
	sc := bufio.NewScanner(&out)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			if strings.HasPrefix(line, "COMMAND") {
				continue
			}
		}
		l, ok := parseLSOFLine(line)
		if !ok {
			continue
		}
		result[l.key()] = l
	}
	return result, sc.Err()
}

// parseLSOFLine parses a listening line:
// COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
func parseLSOFLine(line string) (listener, bool) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return listener{}, false
	}
	command := fields[0]
	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return listener{}, false
	}
	node := fields[7] // TCP
	name := fields[8] // e.g. 127.0.0.1:8000 or *:8080 or [::1]:8080
	addr, port, ok := splitAddr(name)
	if !ok {
		return listener{}, false
	}
	proto := strings.ToLower(node)
	if proto == "" {
		proto = "tcp"
	}
	return listener{proto: proto, addr: addr, port: port, pid: pid, command: command}, true
}

// splitAddr splits "host:port" handling IPv6 brackets and "*".
func splitAddr(s string) (addr string, port int, ok bool) {
	// Strip trailing state like "(LISTEN)" just in case.
	if i := strings.Index(s, " "); i >= 0 {
		s = s[:i]
	}
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return "", 0, false
	}
	addr = s[:idx]
	p, err := strconv.Atoi(s[idx+1:])
	if err != nil {
		return "", 0, false
	}
	addr = strings.Trim(addr, "[]")
	if addr == "*" {
		addr = "0.0.0.0"
	}
	return addr, p, true
}
