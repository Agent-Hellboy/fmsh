// Package snapshot wraps macOS APFS local snapshots (tmutil / mount_apfs) to
// provide instant, copy-on-write restore points. Creating a snapshot needs no
// sudo and stores no file contents in fmsh — the OS keeps the point-in-time
// copy. Restoring an individual file mounts the snapshot read-only, which
// requires a one-time privilege elevation.
package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Snap identifies a local APFS snapshot.
type Snap struct {
	Name string // com.apple.TimeMachine.<date>.local
	Date string // <date>, e.g. 2026-07-09-032954
}

// Available reports whether APFS local snapshots are usable on this system.
func Available() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("tmutil")
	return err == nil
}

// Create takes a new local snapshot and returns it. No sudo required.
func Create(ctx context.Context) (Snap, error) {
	out, err := run(ctx, "tmutil", "localsnapshot")
	if err != nil {
		return Snap{}, fmt.Errorf("tmutil localsnapshot: %w", err)
	}
	// Output: "Created local snapshot with date: 2026-07-09-032954"
	for _, line := range strings.Split(out, "\n") {
		if i := strings.LastIndex(line, ": "); i >= 0 && strings.Contains(line, "snapshot with date") {
			date := strings.TrimSpace(line[i+2:])
			return Snap{Name: nameForDate(date), Date: date}, nil
		}
	}
	return Snap{}, fmt.Errorf("could not parse snapshot date from tmutil output: %q", strings.TrimSpace(out))
}

// List returns the TimeMachine local snapshots for the root volume group.
func List(ctx context.Context) ([]Snap, error) {
	out, err := run(ctx, "tmutil", "listlocalsnapshots", "/")
	if err != nil {
		return nil, err
	}
	var snaps []Snap
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "com.apple.TimeMachine.") {
			continue
		}
		snaps = append(snaps, Snap{Name: line, Date: dateForName(line)})
	}
	return snaps, sc.Err()
}

// Exists reports whether a snapshot with the given name is still present (the OS
// may purge local snapshots at any time).
func Exists(ctx context.Context, name string) bool {
	snaps, err := List(ctx)
	if err != nil {
		return false
	}
	for _, s := range snaps {
		if s.Name == name {
			return true
		}
	}
	return false
}

// DataDevice returns the device node of the macOS Data volume, whose snapshot
// must be mounted to read user files.
func DataDevice(ctx context.Context) (string, error) {
	out, err := run(ctx, "diskutil", "info", "/System/Volumes/Data")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Device Node:") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[len(fields)-1], nil
			}
		}
	}
	return "", fmt.Errorf("could not determine Data volume device")
}

// MountArgs returns the argv for mounting a snapshot read-only. Mounting needs
// root, so callers prepend sudo (or print these for the user to run).
func MountArgs(name, device, mountpoint string) []string {
	return []string{"mount_apfs", "-o", "nobrowse,rdonly", "-s", name, device, mountpoint}
}

// UnmountArgs returns the argv for unmounting a snapshot mountpoint.
func UnmountArgs(mountpoint string) []string {
	return []string{"umount", mountpoint}
}

func nameForDate(date string) string { return "com.apple.TimeMachine." + date + ".local" }

func dateForName(name string) string {
	s := strings.TrimPrefix(name, "com.apple.TimeMachine.")
	return strings.TrimSuffix(s, ".local")
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg != "" {
			return out.String(), fmt.Errorf("%s: %s", err, msg)
		}
		return out.String(), err
	}
	return out.String(), nil
}
