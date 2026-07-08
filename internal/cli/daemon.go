package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"fmsh/internal/config"
	"fmsh/internal/daemon"
	"fmsh/internal/db"
	"fmsh/internal/output"
	"fmsh/internal/store"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the fmshd recording daemon",
	}
	cmd.AddCommand(newDaemonStartCmd(), newDaemonStopCmd(), newDaemonRestartCmd(), newDaemonStatusCmd())
	return cmd
}

func newDaemonStartCmd() *cobra.Command {
	var foreground bool
	var watch []string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the recording daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if foreground {
				return runForeground(watch)
			}
			pid, err := daemon.StartBackground(watch)
			if err != nil {
				return err
			}
			fmt.Printf("%s fmshd started (pid %d)\n", output.Green(output.SymAdded), pid)
			fmt.Printf("  logs: ")
			if lp, err := config.LogPath(); err == nil {
				fmt.Println(lp)
			} else {
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&foreground, "foreground", false, "run in the foreground (used internally)")
	cmd.Flags().StringArrayVar(&watch, "watch", nil, "additional path to watch for this run")
	return cmd
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the recording daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := daemon.Stop(); err != nil {
				return err
			}
			fmt.Printf("%s fmshd stopped\n", output.Yellow(output.SymRemoved))
			return nil
		},
	}
}

func newDaemonRestartCmd() *cobra.Command {
	var watch []string
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the recording daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, running := daemon.IsRunning(); running {
				if err := daemon.Stop(); err != nil {
					return err
				}
			}
			pid, err := daemon.StartBackground(watch)
			if err != nil {
				return err
			}
			fmt.Printf("%s fmshd restarted (pid %d)\n", output.Green(output.SymAdded), pid)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&watch, "watch", nil, "additional path to watch for this run")
	return cmd
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			pid, running := daemon.IsRunning()

			type statusOut struct {
				Running       bool     `json:"running"`
				PID           int      `json:"pid"`
				StartedAt     string   `json:"started_at,omitempty"`
				LastHeartbeat string   `json:"last_heartbeat,omitempty"`
				DBPath        string   `json:"db_path"`
				WatchPaths    []string `json:"watch_paths"`
				EventCount24h int      `json:"event_count_24h"`
				ActiveSession int      `json:"active_sessions"`
			}
			out := statusOut{Running: running, PID: pid, DBPath: cfg.ResolvedDBPath(), WatchPaths: cfg.ResolvedWatchPaths()}

			// Enrich from DB if reachable.
			if _, err := os.Stat(cfg.ResolvedDBPath()); err == nil {
				if sqlDB, err := db.Open(cfg.ResolvedDBPath()); err == nil {
					defer sqlDB.Close()
					st := store.New(sqlDB)
					if ds, _ := st.GetDaemonStatus(); ds != nil {
						if !ds.StartedAt.IsZero() {
							out.StartedAt = output.ClockDate(ds.StartedAt)
						}
						if !ds.LastHeartbeatAt.IsZero() {
							out.LastHeartbeat = output.ClockDate(ds.LastHeartbeatAt)
						}
					}
					if n, err := st.CountEventsSince(time.Now().Add(-24 * time.Hour)); err == nil {
						out.EventCount24h = n
					}
					if sessions, err := st.QuerySessions(eventsActiveFilter()); err == nil {
						out.ActiveSession = len(sessions)
					}
				}
			}

			if flagJSON {
				return output.JSON(os.Stdout, out)
			}

			state := output.Red("stopped")
			if running {
				state = output.Green("running")
			}
			output.Header(os.Stdout, "fmshd status")
			fmt.Printf("  state:          %s\n", state)
			if running {
				fmt.Printf("  pid:            %d\n", pid)
			}
			if out.StartedAt != "" {
				fmt.Printf("  started:        %s\n", out.StartedAt)
			}
			if out.LastHeartbeat != "" {
				fmt.Printf("  last heartbeat: %s\n", out.LastHeartbeat)
			}
			fmt.Printf("  db:             %s\n", out.DBPath)
			fmt.Printf("  events (24h):   %d\n", out.EventCount24h)
			fmt.Printf("  active sessions: %d\n", out.ActiveSession)
			fmt.Printf("  watch paths:\n")
			for _, p := range out.WatchPaths {
				fmt.Printf("    - %s\n", p)
			}
			return nil
		},
	}
}

// runForeground runs the daemon in the current process until signalled.
func runForeground(extraWatch []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// Register any extra watch paths for this run.
	dbPath := cfg.ResolvedDBPath()
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	for _, w := range extraWatch {
		abs := config.Expand(w)
		_ = st.AddWatchedPath(abs)
		cfg.WatchPaths = append(cfg.WatchPaths, abs)
	}

	pid := os.Getpid()
	if err := daemon.WritePID(pid); err != nil {
		return err
	}
	defer daemon.RemovePID()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logf := func(format string, a ...any) { logger.Printf(format, a...) }

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	d := daemon.New(cfg, st, pid, logf)
	return d.Run(ctx)
}
