# fmsh Examples & Use Cases

Practical scenarios and commands for using fmsh to audit AI agents, development workflows, and recoveries.

## Use Case 1: Audit Claude Code Changes

You let Claude Code run for 30 minutes. What did it change?

```bash
# Timeline of all events in the last 30 minutes
$ fmsh timeline --since 30m

2:04 PM    PROCESS_START  claude-code
2:05 PM    FILE_CREATE    ~/projects/app/src/new-feature.ts
2:05 PM    FILE_WRITE     ~/projects/app/package.json
2:06 PM    FILE_CREATE    ~/projects/app/.env.test
2:07 PM    PROCESS_EXIT   claude-code
2:08 PM    SESSION_END    claude-code session

# Human-readable summary
$ fmsh what-changed --since 30m

What changed — Last 30m

Files:
  ~ 1 file(s) modified (package.json)
  + 2 file(s) created (new-feature.ts, .env.test)
  - 0 file(s) deleted

Processes:
  + claude-code

Network:
  no ports opened

Risks:
  ! .env.test (secret/config file) was touched

Suggested review:
  1. Review .env.test for sensitive values before committing
  2. Review package.json changes (new dependencies?)
  3. Test the new feature
```

**What to check next:**
```bash
# See exactly what was modified in package.json
$ fmsh blame --path ~/projects/app/package.json

# Get a full AI-session report
$ fmsh agent-report --since 30m

# List all files modified by Claude Code
$ fmsh events --since 30m --type file.write --category file
```

---

## Use Case 2: Catch a Malicious Download

You accidentally ran a sketchy script. Did it install anything dangerous?

```bash
# Check what happened in the last 5 minutes
$ fmsh timeline --since 5m

2:15 PM    PROCESS_START  curl
2:15 PM    FILE_DOWNLOAD  ~/Downloads/install.sh
2:16 PM    PROCESS_START  bash
2:16 PM    FILE_CREATE    ~/.bashrc.bak
2:16 PM    FILE_WRITE     ~/.bashrc
2:17 PM    FILE_CREATE    /Users/proshan/Library/LaunchAgents/com.sneaky.plist
2:18 PM    RISK_DETECTED  launch_agent_changed (HIGH SEVERITY)

# Get the risk summary
$ fmsh risks --since 5m

Risks — Last 5m
  ! 2:16 PM   ~/.bashrc was modified by bash
  ! 2:17 PM   /Users/proshan/Library/LaunchAgents/com.sneaky.plist (launch agent) was created
             SEVERITY: HIGH

# Undo the damage from the checkpoint taken before the script ran
$ fmsh checkpoints

Checkpoints

e7dd63fa10e6    2:15 PM   saved      ~/Library/LaunchAgents
  trigger: destructive_command
  snapshot: 2026-07-09-140538
  
# Restore the launch agent directory
$ fmsh restore e7dd63fa10e6 --path ~/Library/LaunchAgents

# Verify it's gone
$ fmsh timeline --since 5m | grep LaunchAgents
  (no results)
```

---

## Use Case 3: Recover Deleted Files

You deleted an entire directory by mistake. Can you get it back?

```bash
# Check what was deleted
$ fmsh events --since 1h --type file.delete

2:20 PM    FILE_DELETE    ~/projects/old-feature/main.go
2:20 PM    FILE_DELETE    ~/projects/old-feature/main_test.go
2:20 PM    FILE_DELETE    ~/projects/old-feature/README.md
2:20 PM    FILE_DELETE    ~/projects/old-feature

# Find the session or checkpoint before the deletion
$ fmsh sessions --since 1h

ID             START      END        AGENT                  RISKS  REPO
1a2b3c4d5e6f   2:10 PM    2:20 PM    manual_dev_activity    0      ~/projects

# Get the checkpoint for that session
$ fmsh session show 1a2b3c4d5e6f

Session 1a2b3c4d5e6f — 2:10 PM → 2:20 PM (10 min)
  agent: manual_dev_activity
  repo: ~/projects
  events: 45
  risks: 0
  checkpoint: abc123def456

# Restore the entire old-feature directory
$ fmsh restore abc123def456 --path ~/projects/old-feature

# Verify files are back
$ ls -la ~/projects/old-feature/
-rw-r--r-- main.go
-rw-r--r-- main_test.go
-rw-r--r-- README.md
```

---

## Use Case 4: Detect Dependency Confusion Attacks

A dependency change happened. What was it? Did it come from an agent?

```bash
# Check for dependency changes in the last 24 hours
$ fmsh risks --since 24h --type dependency_changed

Risks — Last 24h

  ! 3:45 PM   ~/projects/app/package.json was modified
             Rule: dependency_changed
             Diff: +1 -0 (1 new dependency)

  ! 4:12 PM   ~/projects/app/go.mod was modified
             Rule: dependency_changed
             Diff: +2 -3 (2 new, 3 removed)

# See exactly which dependencies were added
$ fmsh blame --path ~/projects/app/package.json

Activity for /Users/proshan/projects/app/package.json

3:45 PM    FILE_WRITE     ~/projects/app/package.json
  process: npm
  cmdline: npm install some-new-pkg@latest

# Check if an AI agent triggered it
$ fmsh timeline --since 24h --category process | grep -E "3:45|npm|claude|cursor"

3:45 PM    PROCESS_START  npm (npm)
3:45 PM    PROCESS_START  node (node)
  session_agent: claude-code

# Decision: Claude Code installed a new npm package. Verify it's safe before committing.
```

---

## Use Case 5: Pre-commit Audit (Before Pushing)

Before you git push, what changed in this session?

```bash
# Show what changed since session start
$ fmsh session diff d50ae3bb6d57

Session d50ae3bb6d57 diff

Files changed:
  ~ src/main.rs (102 lines changed)
  + src/utils.rs (45 new lines)
  + tests/utils_test.rs (67 new lines)
  - src/old.rs (deleted)

Processes seen:
  + rustc (compiler)
  + cargo (build tool)
  + git (version control)

Ports opened:
  none

Risks detected:
  none

Git status:
  On branch: main
  Staged: src/main.rs, src/utils.rs, tests/utils_test.rs
  Deleted: src/old.rs

# Safe to push!
```

---

## Use Case 6: Enable Guard for Dangerous Commands

Protect yourself from destructive commands with pre-command snapshots.

```bash
# Add the guard hook to your shell (once)
$ echo 'eval "$(fmsh shell-init)"' >> ~/.zshrc

# Now reload your shell
$ source ~/.zshrc

# Try a destructive command
$ rm -rf ~/important_files

! fmsh: snapshot taken before destructive command — undo with `fmsh restore e7dd63fa10e6 --path ~/important_files`

# Check what happened
$ fmsh checkpoints

Checkpoints

e7dd63fa10e6    2:35 PM   saved      ~/important_files
  trigger: destructive_command
  cmd: rm -rf ~/important_files
  snapshot: 2026-07-09-143500

# Restore the files
$ fmsh restore e7dd63fa10e6 --path ~/important_files
# (will prompt for sudo once)

$ ls ~/important_files
file1.txt
file2.txt
```

---

## Use Case 7: Investigate Port Activity

Did any new servers or suspicious ports open during an agent session?

```bash
# Check for port activity in the last hour
$ fmsh events --since 1h --type port.open

2:30 PM    PORT_OPEN      localhost:3000
  process: node
  cmdline: npm start

2:32 PM    PORT_OPEN      0.0.0.0:8080
  process: python
  cmdline: python -m http.server 8080

2:35 PM    PORT_OPEN      localhost:5432
  process: postgres
  cmdline: postgres -D /usr/local/var/postgres

# Check if any suspicious ports opened
$ fmsh risks --since 1h | grep port

# None found — just your normal dev servers.
```

---

## Use Case 8: Machine-Readable Output for Scripts

Integrate fmsh into your CI/CD or analysis scripts.

```bash
# Get all high-severity risks in JSON
$ fmsh risks --since 24h --json | jq '.[] | select(.severity == "high")'

{
  "id": 42,
  "ts": "2026-07-09T14:05:20.205911+05:30",
  "type": "risk.secret_touched",
  "severity": "high",
  "subject": ".env was modified",
  "session_id": "a35b4b0ef972",
  "metadata": {
    "path": "/Users/proshan/projects/app/.env",
    "process": "claude-code"
  }
}

# Alert if any high-severity risks found
if fmsh risks --since 24h --json | jq '.[] | select(.severity == "high")' | grep -q .; then
  echo "⚠️  High-severity risks detected in the last 24 hours"
  exit 1
fi

# Get JSON timeline for parsing
$ fmsh timeline --since 1h --json | jq '.[] | {time: .ts, type: .type, path: .path}' > /tmp/timeline.json
```

---

## Use Case 9: Audit Multiple Sessions

Compare what different agents changed.

```bash
# List all sessions in the last 24 hours
$ fmsh sessions --since 24h

ID             START      END        AGENT                  RISKS  REPO
16138ec26608   2:04 PM    2:45 PM    claude-code            0      ~/fmsh
d50ae3bb6d57   3:00 PM    3:30 PM    cursor                 1      ~/fmsh
a35b4b0ef972   4:00 PM    active     manual_dev_activity    2      ~/fmsh

# Compare Claude Code vs Cursor changes
$ fmsh session show 16138ec26608 | head -20

Session 16138ec26608 — Claude Code
  start: 2:04 PM
  end: 2:45 PM
  files_modified: 5
  files_created: 2
  files_deleted: 0
  risks: 0

$ fmsh session show d50ae3bb6d57 | head -20

Session d50ae3bb6d57 — Cursor
  start: 3:00 PM
  end: 3:30 PM
  files_modified: 8
  files_created: 3
  files_deleted: 1
  risks: 1  (large_file_created: 150 MB)

# Decision: Claude Code was more conservative; Cursor created a large file to investigate.
```

---

## Use Case 10: Diagnose Issues with fmsh Doctor

Something looks off. Run the diagnostic.

```bash
$ fmsh doctor

fmsh doctor
  [ok  ] config           — /Users/proshan/.fmsh/config.yaml
  [ok  ] database         — /Users/proshan/.fmsh/activity.db
  [ok  ] wal mode         — enabled
  [ok  ] daemon           — running (pid 79867)
  [ok  ] heartbeat        — 6s ago
  [warn] watch paths      — 2 of 4 missing (skipped)
  [ok  ] tool: ps         — available
  [ok  ] tool: lsof       — available
  [ok  ] git             — available
  [warn] permissions      — if file events are missing, grant your terminal Full Disk Access

# Interpret warnings:
# - "2 of 4 missing": two watch paths don't exist (e.g., ~/code, ~/Developer)
#   → Create them or remove from config
# - "permissions": file events might be incomplete without Full Disk Access
#   → Go to System Settings > Privacy & Security > Full Disk Access > add your terminal
```

---

## Filtering & Querying Examples

### By time
```bash
fmsh timeline --since 30m           # last 30 minutes
fmsh timeline --since 2h            # last 2 hours
fmsh timeline --since 1d            # last 1 day
fmsh events --from "5pm" --to "6pm" # between specific times
```

### By type
```bash
fmsh events --type file.create      # only file creates
fmsh events --type process.start    # only process starts
fmsh events --type port.open        # only port opens
fmsh events --category file         # all file events
fmsh events --category process      # all process events
fmsh events --category risk         # all risk events
```

### By path
```bash
fmsh events --path ~/projects/app   # all events in a directory
fmsh blame --path ~/.env            # who touched a file
fmsh risks --since 24h --severity high  # high-severity risks only
```

### Combinations
```bash
fmsh events --since 1h --type file.write --path ~/code
  # → all file writes in ~/code in the last hour

fmsh risks --since 24h --severity high --json
  # → high-severity risks in last 24h as JSON (for scripts)

fmsh timeline --from "yesterday 6pm" --to "today 6pm"
  # → events from yesterday 6 PM to today 6 PM
```

---

## Configuration Tweaks

### Catch more file activity
```yaml
daemon:
  process_poll_interval: 1s    # faster file polling (more CPU)
```

### Reduce database size
```yaml
session:
  inactivity_timeout: 5m       # shorter sessions = smaller DB
```

### Be more aggressive with snapshots
```yaml
guard:
  snapshot_on_high_risk: true  # snapshot before risky operations
  min_interval: 1m             # more frequent snapshots (uses more disk)
```

### Stricter privacy (never store command lines)
```yaml
privacy:
  store_cmdline: false         # don't store what was run
  redact_secrets: true         # still redact just in case
```

---

## Tips & Best Practices

1. **Enable the guard hook** — it's the easiest way to avoid accidental deletions
   ```bash
   echo 'eval "$(fmsh shell-init)"' >> ~/.zshrc
   ```

2. **Check risks before committing** — high-severity risks should be reviewed
   ```bash
   fmsh risks --since 1d | grep HIGH
   ```

3. **Audit agent sessions regularly** — understand what your tools are doing
   ```bash
   fmsh agent-report --since 8h  # what did AI agents do today?
   ```

4. **Use JSON for automation** — integrate into CI/CD or monitoring
   ```bash
   fmsh risks --json | jq '.[] | select(.severity == "high")' | wc -l
   ```

5. **Grant Full Disk Access** — without it, file events will be incomplete
   - System Settings → Privacy & Security → Full Disk Access → add Terminal/IDE

6. **Watch key directories** — add the paths where you do most work
   ```bash
   fmsh watch add ~/code
   fmsh watch add ~/projects
   ```

7. **Review before pushing** — use `fmsh session diff` before `git push`
   ```bash
   fmsh session diff <session_id>
   ```
