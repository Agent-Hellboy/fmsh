package daemon

import (
	"os"
	"testing"
)

func TestPIDFileRoundTrip(t *testing.T) {
	// Isolate ~/.fmsh to a temp HOME.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if pid, err := ReadPID(); err != nil || pid != 0 {
		t.Fatalf("expected no pid initially, got %d (err %v)", pid, err)
	}

	if err := WritePID(os.Getpid()); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	pid, err := ReadPID()
	if err != nil {
		t.Fatal(err)
	}
	if pid != os.Getpid() {
		t.Errorf("read pid = %d, want %d", pid, os.Getpid())
	}

	// The current process is definitely running.
	if !Running(pid) {
		t.Error("current process should be reported running")
	}
	if _, ok := IsRunning(); !ok {
		t.Error("IsRunning should be true for our own pid")
	}

	if err := RemovePID(); err != nil {
		t.Fatal(err)
	}
	if pid, _ := ReadPID(); pid != 0 {
		t.Error("pid file should be gone after RemovePID")
	}
}

func TestRunningFalseForDeadPID(t *testing.T) {
	// PID 0 / negative are never valid live processes.
	if Running(0) {
		t.Error("pid 0 should not be running")
	}
	// A very high pid is extremely unlikely to exist.
	if Running(999999999) {
		t.Error("nonexistent pid should not be running")
	}
}
