package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"fmsh/internal/config"
	"fmsh/internal/db"
	"fmsh/internal/guard"
	"fmsh/internal/output"
	"fmsh/internal/store"
)

// newGuardCmd is the target of the shell preexec hook. It runs before each
// command; for destructive commands it takes an APFS snapshot first. It ALWAYS
// exits 0 so it can never break the user's command.
func newGuardCmd() *cobra.Command {
	var cwd string
	cmd := &cobra.Command{
		Use:    "guard [command...]",
		Short:  "Pre-command hook: snapshot before a destructive command",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			command := strings.TrimSpace(strings.Join(args, " "))
			if command == "" {
				return
			}
			// Fast path: only recognized destructive commands do any work.
			if _, ok := guard.IsDestructive(command); !ok {
				return
			}
			if cwd == "" {
				cwd, _ = os.Getwd()
			}

			cfg, err := config.Load()
			if err != nil {
				return
			}
			dbPath := cfg.ResolvedDBPath()
			if _, err := os.Stat(dbPath); err != nil {
				return // not initialized; stay silent
			}
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return
			}
			defer sqlDB.Close()
			st := store.New(sqlDB)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, _ := guard.Guard(ctx, st, cwd, command)
			if res == nil || !res.Triggered || res.Checkpoint == nil {
				return
			}
			cp := res.Checkpoint
			switch cp.Status {
			case store.CheckpointSaved:
				fmt.Fprintf(os.Stderr, "%s fmsh: snapshot taken before destructive command — undo with `fmsh restore %s --path <file>`\n",
					output.Yellow(output.SymRisk), cp.ID)
			case store.CheckpointSkipped:
				fmt.Fprintf(os.Stderr, "%s fmsh: destructive command detected but no snapshot taken (%s)\n",
					output.Yellow(output.SymRisk), cp.Reason)
			}
		},
	}
	cmd.Flags().StringVar(&cwd, "cwd", "", "working directory the command runs in (defaults to current)")
	return cmd
}

func newShellInitCmd() *cobra.Command {
	var shell string
	cmd := &cobra.Command{
		Use:   "shell-init",
		Short: "Print the shell hook that snapshots before destructive commands",
		Long: `Print a shell snippet that installs the fmsh pre-command guard.

Add this to your shell startup file, e.g.:

  # ~/.zshrc
  eval "$(fmsh shell-init)"

Then, before any command matching a destructive pattern (rm -rf, chmod -R,
curl | sh, sudo, ...), fmsh takes an APFS local snapshot so you can revert with
'fmsh restore'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if shell == "" {
				shell = detectShell()
			}
			switch shell {
			case "bash":
				fmt.Print(bashHook)
			default:
				fmt.Print(zshHook)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "", "shell to target: zsh (default) or bash")
	return cmd
}

func detectShell() string {
	if strings.Contains(os.Getenv("SHELL"), "bash") {
		return "bash"
	}
	return "zsh"
}

const zshHook = `# fmsh pre-command guard (zsh)
_fmsh_guard_preexec() {
  command fmsh guard --cwd "$PWD" -- "$1"
}
if autoload -Uz add-zsh-hook 2>/dev/null; then
  add-zsh-hook preexec _fmsh_guard_preexec
else
  preexec_functions+=(_fmsh_guard_preexec)
fi
`

const bashHook = `# fmsh pre-command guard (bash)
_fmsh_guard_debug() {
  [ -n "$COMP_LINE" ] && return
  [ "$BASH_COMMAND" = "$PROMPT_COMMAND" ] && return
  command fmsh guard --cwd "$PWD" -- "$BASH_COMMAND"
}
trap '_fmsh_guard_debug' DEBUG
`

func newCheckpointsCmd() *cobra.Command {
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:   "checkpoints",
		Short: "List snapshot restore points taken before risky moments",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			var start time.Time
			if since != "" {
				f, err := queryRange(since, "", "")
				if err != nil {
					return err
				}
				start = f.Since
			}
			cps, err := st.ListCheckpoints(start, limit)
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, cps)
			}
			output.Header(os.Stdout, "Checkpoints")
			if len(cps) == 0 {
				output.EmptyNote(os.Stdout, "no checkpoints yet — install the guard with `fmsh shell-init`")
				return nil
			}
			fmt.Println()
			for _, c := range cps {
				status := colorStatus(c.Status)
				root := shortPath(c.Root)
				if root == "" {
					root = output.Dim("(machine-wide)")
				}
				fmt.Printf("%s  %s  %s  %s\n",
					output.Pad(c.ID, 14), output.Dim(output.ClockDate(c.CreatedAt)),
					output.Pad(status, 9), root)
				fmt.Printf("  %s %s\n", output.Dim("trigger:"), c.Trigger)
				if c.Command != "" {
					fmt.Printf("  %s %s\n", output.Dim("cmd:    "), truncate(c.Command, 70))
				}
				if c.Status == store.CheckpointSaved {
					fmt.Printf("  %s %s — `fmsh restore %s --path <file>`\n", output.Dim("snapshot:"), c.SnapshotDate, c.ID)
				} else if c.Reason != "" {
					fmt.Printf("  %s %s\n", output.Dim("note:    "), c.Reason)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "only checkpoints since, e.g. 24h")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number to show")
	return cmd
}

func colorStatus(s string) string {
	switch s {
	case store.CheckpointSaved:
		return output.Green(s)
	case store.CheckpointRestored:
		return output.Cyan(s)
	default:
		return output.Yellow(s)
	}
}

func newRestoreCmd() *cobra.Command {
	var path string
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "restore <checkpoint_id> --path <file>",
		Short: "Restore a file or directory from a checkpoint's snapshot",
		Long: `Restore a file or directory from the APFS snapshot recorded by a checkpoint.

Mounting the snapshot requires a one-time privilege elevation, so restore uses
sudo and will prompt for your password. Use --print to see the exact commands
instead of running them.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			cp, err := st.GetCheckpoint(args[0])
			if err != nil {
				return err
			}
			ctx := context.Background()
			plan, restored, err := guard.Restore(ctx, st, cp, path, printOnly)
			if err != nil {
				return err
			}

			if printOnly {
				output.Header(os.Stdout, "Restore plan for checkpoint "+cp.ID)
				fmt.Printf("\n  # mount the snapshot read-only\n  %s\n", strings.Join(plan.MountCmd, " "))
				fmt.Printf("\n  # copy the file(s) back\n  cp -a %q %q\n", plan.CopyFrom, plan.CopyTo)
				fmt.Printf("\n  # unmount\n  %s\n", strings.Join(plan.UnmountCmd, " "))
				return nil
			}

			output.Header(os.Stdout, fmt.Sprintf("Restored %d file(s) from checkpoint %s", len(restored), cp.ID))
			for _, f := range restored {
				fmt.Printf("  %s %s\n", output.Green(output.SymModified), shortPath(f))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "absolute path of the file or directory to recover (required)")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print the mount/copy/unmount commands instead of running them")
	return cmd
}
