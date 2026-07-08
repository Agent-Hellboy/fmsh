package snapshot

import (
	"strings"
	"testing"
)

func TestNameDateRoundTrip(t *testing.T) {
	date := "2026-07-09-033534"
	name := nameForDate(date)
	if name != "com.apple.TimeMachine.2026-07-09-033534.local" {
		t.Fatalf("nameForDate = %q", name)
	}
	if got := dateForName(name); got != date {
		t.Errorf("dateForName(%q) = %q, want %q", name, got, date)
	}
}

func TestMountArgs(t *testing.T) {
	args := MountArgs("com.apple.TimeMachine.X.local", "/dev/disk3s5", "/tmp/mp")
	joined := strings.Join(args, " ")
	for _, want := range []string{"mount_apfs", "rdonly", "-s", "com.apple.TimeMachine.X.local", "/dev/disk3s5", "/tmp/mp"} {
		if !strings.Contains(joined, want) {
			t.Errorf("MountArgs missing %q in %v", want, args)
		}
	}
}
