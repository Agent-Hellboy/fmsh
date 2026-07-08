package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fmsh/internal/config"
)

// ReadPID returns the pid recorded in the pid file, or 0 if absent.
func ReadPID() (int, error) {
	p, err := config.PIDPath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid pid file: %w", err)
	}
	return pid, nil
}

// WritePID writes the current pid to the pid file, creating ~/.fmsh if needed.
func WritePID(pid int) error {
	p, err := config.PIDPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(mustDir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strconv.Itoa(pid)), 0o644)
}

// RemovePID deletes the pid file.
func RemovePID() error {
	p, err := config.PIDPath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Running reports whether pid refers to a live process.
func Running(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 tests for existence without affecting the process.
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// IsRunning reports whether the daemon is currently running per the pid file.
func IsRunning() (int, bool) {
	pid, err := ReadPID()
	if err != nil || pid == 0 {
		return 0, false
	}
	return pid, Running(pid)
}

// StartBackground launches `fmsh daemon start --foreground` as a detached
// child process, redirecting its output to the daemon log. Extra watch paths
// are passed through. It returns the child pid.
func StartBackground(extraWatch []string) (int, error) {
	if pid, ok := IsRunning(); ok {
		return pid, fmt.Errorf("daemon already running (pid %d)", pid)
	}

	self, err := os.Executable()
	if err != nil {
		return 0, err
	}
	logPath, err := config.LogPath()
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(mustDir(logPath), 0o755); err != nil {
		return 0, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	args := []string{"daemon", "start", "--foreground"}
	for _, w := range extraWatch {
		args = append(args, "--watch", w)
	}
	cmd := exec.Command(self, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach from terminal
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Let the child settle; release it so it is not reaped as our child.
	_ = cmd.Process.Release()
	return pid, nil
}

// Stop signals the running daemon to shut down gracefully and waits briefly for
// it to exit.
func Stop() error {
	pid, err := ReadPID()
	if err != nil {
		return err
	}
	if pid == 0 || !Running(pid) {
		_ = RemovePID()
		return errors.New("daemon is not running")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal daemon: %w", err)
	}
	for range 50 {
		if !Running(pid) {
			_ = RemovePID()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon (pid %d) did not stop in time", pid)
}

func mustDir(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	return path[:i]
}
