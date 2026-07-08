// Package config loads and persists fmsh configuration and resolves the
// standard local paths under ~/.fmsh. Everything is local-first: no cloud,
// no telemetry.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML-friendly wrapper around time.Duration that marshals to
// strings like "2s" or "5m".
type Duration struct{ time.Duration }

func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(strings.TrimSpace(value.Value))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = parsed
	return nil
}

// DaemonConfig holds collector poll cadences.
type DaemonConfig struct {
	ProcessPollInterval Duration `yaml:"process_poll_interval"`
	PortPollInterval    Duration `yaml:"port_poll_interval"`
	GitPollInterval     Duration `yaml:"git_poll_interval"`
	HeartbeatInterval   Duration `yaml:"heartbeat_interval"`
}

// RiskConfig holds thresholds for the risk detector.
type RiskConfig struct {
	LargeFileMB        int      `yaml:"large_file_mb"`
	ManyFilesThreshold int      `yaml:"many_files_threshold"`
	ManyFilesWindow    Duration `yaml:"many_files_window"`
}

// SessionConfig holds sessionizer tuning.
type SessionConfig struct {
	InactivityTimeout Duration `yaml:"inactivity_timeout"`
}

// PrivacyConfig controls what fmsh records.
type PrivacyConfig struct {
	StoreCmdline    bool `yaml:"store_cmdline"`
	RedactSecrets   bool `yaml:"redact_secrets"`
	StoreFileHashes bool `yaml:"store_file_hashes"`
}

// GuardConfig controls automatic APFS snapshot restore points.
type GuardConfig struct {
	Enabled            bool     `yaml:"enabled"`
	SnapshotOnSession  bool     `yaml:"snapshot_on_session"`
	SnapshotOnHighRisk bool     `yaml:"snapshot_on_high_risk"`
	MinInterval        Duration `yaml:"min_interval"`
}

// Config is the full fmsh configuration.
type Config struct {
	DBPath     string        `yaml:"db_path"`
	Daemon     DaemonConfig  `yaml:"daemon"`
	WatchPaths []string      `yaml:"watch_paths"`
	IgnoreDirs []string      `yaml:"ignore_dirs"`
	Risk       RiskConfig    `yaml:"risk"`
	Session    SessionConfig `yaml:"session"`
	Privacy    PrivacyConfig `yaml:"privacy"`
	Guard      GuardConfig   `yaml:"guard"`
}

// Default returns a Config populated with the documented defaults.
func Default() *Config {
	return &Config{
		DBPath: "~/.fmsh/activity.db",
		Daemon: DaemonConfig{
			ProcessPollInterval: Duration{2 * time.Second},
			PortPollInterval:    Duration{10 * time.Second},
			GitPollInterval:     Duration{15 * time.Second},
			HeartbeatInterval:   Duration{10 * time.Second},
		},
		WatchPaths: []string{"~/code", "~/Developer", "~/Downloads", "~/Desktop"},
		IgnoreDirs: []string{
			".git", "node_modules", "target", "dist", "build", ".venv",
			"__pycache__", "Library/Caches", "Library/Logs",
			"Library/Developer/Xcode/DerivedData",
		},
		Risk: RiskConfig{
			LargeFileMB:        100,
			ManyFilesThreshold: 50,
			ManyFilesWindow:    Duration{5 * time.Minute},
		},
		Session: SessionConfig{
			InactivityTimeout: Duration{10 * time.Minute},
		},
		Privacy: PrivacyConfig{
			StoreCmdline:    true,
			RedactSecrets:   true,
			StoreFileHashes: false,
		},
		Guard: GuardConfig{
			Enabled:            true,
			SnapshotOnSession:  true,
			SnapshotOnHighRisk: true,
			MinInterval:        Duration{2 * time.Minute},
		},
	}
}

// Home returns the ~/.fmsh directory.
func Home() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".fmsh"), nil
}

// Path returns the default config.yaml path.
func Path() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "config.yaml"), nil
}

// PIDPath returns ~/.fmsh/fmshd.pid.
func PIDPath() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "fmshd.pid"), nil
}

// LogPath returns ~/.fmsh/fmshd.log.
func LogPath() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "fmshd.log"), nil
}

// Expand resolves a leading ~ to the user's home directory.
func Expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// ResolvedDBPath returns the DB path with ~ expanded.
func (c *Config) ResolvedDBPath() string { return Expand(c.DBPath) }

// ResolvedWatchPaths returns watch paths with ~ expanded.
func (c *Config) ResolvedWatchPaths() []string {
	out := make([]string, 0, len(c.WatchPaths))
	for _, p := range c.WatchPaths {
		out = append(out, Expand(p))
	}
	return out
}

// Load reads config from the default path, returning defaults if it is absent.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
}

// LoadFrom reads config from an explicit path. A missing file yields defaults.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config to the default path, creating ~/.fmsh if needed.
func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	return c.SaveTo(p)
}

// SaveTo writes the config to an explicit path.
func (c *Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
