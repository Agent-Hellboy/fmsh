<h1 align="center">fmsh — Forensic Machine Shell</h1>

<p align="center"><b>A black box recorder for AI-driven development on macOS.</b></p>

---

**fmsh** is a local audit layer for AI-driven development. It continuously records filesystem, process, port, and Git activity so you can audit what agents, scripts, and tools changed on your machine — and recover from mistakes instantly.

```
fmshd is always watching. Every important local activity becomes an event.
The CLI lets you ask what happened.
```

## Core Features

- **🔍 Continuous Recording**: File changes, processes, ports, Git activity, and risks
- **📸 Instant Snapshots**: APFS local snapshots before dangerous operations (via guard hook)
- **⏮️ One-Command Recovery**: Restore deleted files from checkpoints
- **⚠️ Risk Detection**: Catches secret files touched, dependency changes, destructive commands, launch agents modified
- **📋 Session Audit**: Groups activity into sessions, detects which tool (Claude Code, Cursor, etc.) made changes
- **🔒 Privacy First**: No cloud, no telemetry, all data in `~/.fmsh`, redacts secrets in command lines

## Quick Start

```bash
# Install
go install github.com/Agent-Hellboy/fmsh/cmd/fmsh@latest

# Initialize
fmsh init
fmsh watch add ~/code ~/projects

# Start daemon
fmsh daemon start

# ... let an agent run ...

# Audit changes
fmsh what-changed --since 30m
fmsh agent-report --since 2h
fmsh risks --since 24h

# Recover a file
fmsh restore <checkpoint_id> --path ~/deleted/file.txt
```

**For detailed examples, see [EXAMPLES.md](docs/EXAMPLES.md)**

**For architecture & internals, see [ARCHITECTURE.md](docs/ARCHITECTURE.md)**

## Requirements

- **macOS 10.15+** on an APFS volume (for APFS snapshots)
- **Full Disk Access** granted to your terminal in System Settings → Privacy & Security

## Install & Build

```bash
# Latest release
go install github.com/Agent-Hellboy/fmsh/cmd/fmsh@latest

# Or build from source
git clone https://github.com/Agent-Hellboy/fmsh
cd fmsh
go build -o fmsh ./cmd/fmsh
```

## Commands Overview

```bash
# Setup
fmsh init                        # Initialize ~/.fmsh
fmsh watch add/remove/list       # Manage watched paths
fmsh daemon start/stop/status    # Manage daemon

# Query activity
fmsh timeline --since 30m        # Chronological events
fmsh what-changed --since 30m    # Human-readable summary
fmsh agent-report --since 2h     # AI-session audit
fmsh events --type file.write    # Filter by type
fmsh risks --since 24h           # Risk summary
fmsh sessions                    # List sessions
fmsh blame --path <file>         # Who touched this file?

# Recovery
fmsh checkpoints                 # List restore points
fmsh restore <id> --path <file>  # Recover from snapshot

# Utilities
fmsh doctor                      # Diagnose health
fmsh shell-init                  # Setup pre-command guard
```

**Full command reference & examples: [EXAMPLES.md](docs/EXAMPLES.md)**

## Enable Guard (Pre-command Snapshots)

Automatically snapshot before dangerous commands:

```bash
echo 'eval "$(fmsh shell-init)"' >> ~/.zshrc
source ~/.zshrc

# Now before: rm -rf, chmod -R, curl | sh, sudo, etc.
# fmsh automatically creates a snapshot and tells you how to undo it
```

## Privacy & Security

✅ **What fmsh stores:**
- File paths, modification times, process names, Git metadata
- Command lines (with token/password redaction)

❌ **What fmsh never stores:**
- File contents, browser history, clipboard, keystrokes

All data stays in `~/.fmsh`. No cloud, no telemetry, no AI in the core.

## Architecture

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed technical documentation including:
- System architecture diagram
- Daemon collector design
- Risk detection logic
- Event model
- Recovery mechanism

## License

MIT — see [LICENSE](LICENSE).
