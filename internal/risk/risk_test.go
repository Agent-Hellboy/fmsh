package risk

import (
	"testing"

	"fmsh/internal/config"
	"fmsh/internal/events"
)

func riskCfg() config.RiskConfig {
	return config.RiskConfig{LargeFileMB: 100, ManyFilesThreshold: 50}
}

func hasType(evs []events.Event, typ string) bool {
	for _, e := range evs {
		if e.Type == typ {
			return true
		}
	}
	return false
}

func TestSecretTouched(t *testing.T) {
	ev := events.Event{Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/Users/x/app/.env"}
	out := EvaluateEvent(ev, riskCfg())
	if !hasType(out, events.TypeRiskSecretTouched) {
		t.Fatalf(".env should trigger secret_touched, got %+v", out)
	}
	if out[0].Severity != events.SeverityHigh {
		t.Errorf("secret risk should be high severity")
	}
}

func TestSSHKeyTouched(t *testing.T) {
	ev := events.Event{Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/Users/x/.ssh/id_rsa"}
	if !hasType(EvaluateEvent(ev, riskCfg()), events.TypeRiskSecretTouched) {
		t.Error("id_rsa under .ssh should trigger secret_touched")
	}
}

func TestDependencyChanged(t *testing.T) {
	for _, name := range []string{"package.json", "go.mod", "Cargo.lock", "requirements.txt"} {
		ev := events.Event{Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/repo/" + name}
		if !hasType(EvaluateEvent(ev, riskCfg()), events.TypeRiskDependencyChanged) {
			t.Errorf("%s should trigger dependency_changed", name)
		}
	}
}

func TestNotADependency(t *testing.T) {
	ev := events.Event{Type: events.TypeFileWrite, Category: events.CategoryFile, Path: "/repo/main.go"}
	if hasType(EvaluateEvent(ev, riskCfg()), events.TypeRiskDependencyChanged) {
		t.Error("main.go should not be a dependency risk")
	}
}

func TestDestructiveCommand(t *testing.T) {
	ev := events.Event{Type: events.TypeProcessStart, Category: events.CategoryProcess, Cmdline: "rm -rf /tmp/foo"}
	out := EvaluateEvent(ev, riskCfg())
	if !hasType(out, events.TypeRiskDestructiveCommand) {
		t.Fatal("rm -rf should trigger destructive_command")
	}
}

func TestSafeCommand(t *testing.T) {
	ev := events.Event{Type: events.TypeProcessStart, Category: events.CategoryProcess, Cmdline: "ls -la"}
	if hasType(EvaluateEvent(ev, riskCfg()), events.TypeRiskDestructiveCommand) {
		t.Error("ls should not be destructive")
	}
}

func TestLargeFile(t *testing.T) {
	ev := events.Event{
		Type: events.TypeFileCreate, Category: events.CategoryFile, Path: "/repo/big.bin",
		Metadata: map[string]any{"size_bytes": int64(200 * 1024 * 1024)},
	}
	if !hasType(EvaluateEvent(ev, riskCfg()), events.TypeRiskLargeFileCreated) {
		t.Error("200MB file should trigger large_file_created")
	}
}

func TestNewExecutableDownload(t *testing.T) {
	ev := events.Event{Type: events.TypeFileCreate, Category: events.CategoryFile, Path: "/Users/x/Downloads/installer.dmg"}
	if !hasType(EvaluateEvent(ev, riskCfg()), events.TypeRiskNewExecutableDownload) {
		t.Error(".dmg in Downloads should trigger new_executable_downloads")
	}
}
