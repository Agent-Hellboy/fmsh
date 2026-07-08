# fmsh — working notes for Claude

fmsh is a macOS-only **black box recorder for AI-driven development**: a local
`fmshd` daemon records filesystem/process/port/git/risk events into SQLite, and
the CLI queries them. See `README.md` for the full product overview and the
package layout under `cmd/fmsh` and `internal/`.

## Discovery & refactoring

For code search, discovery, or understanding relationships across the codebase,
prefer `graphify query <question>` over grep loops. The knowledge graph is built
on the first query (~1–2 min) and cached for the session. Examples:
"trace the event pipeline from collector to store", "where is RiskConfig used",
"how are sessions closed".

## Conventions

- macOS-only; no sudo for the daemon (only `fmsh restore` mounts a snapshot).
- Pure-Go SQLite (`modernc.org/sqlite`), so builds run with `CGO_ENABLED=0`.
- Run `gofmt`, `go vet ./...`, and `go test ./...` before pushing.
