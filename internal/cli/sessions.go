package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"fmsh/internal/events"
	"fmsh/internal/output"
)

func eventsActiveFilter() events.SessionFilter {
	return events.SessionFilter{Status: events.SessionActive}
}

// sessionHeadline renders a one-line session description.
func sessionHeadline(s events.Session) string {
	repo := s.RepoPath
	if repo != "" {
		repo = shortPath(repo)
	} else {
		repo = "(unknown)"
	}
	end := "active"
	if s.EndedAt != nil {
		end = output.Clock(*s.EndedAt)
	}
	agent := s.DetectedAgent
	if agent == "" {
		agent = "unknown_dev_activity"
	}
	return fmt.Sprintf("%s session in %s   %s → %s", agent, repo, output.Clock(s.StartedAt), end)
}

func newSessionsCmd() *cobra.Command {
	var since, from, to string
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List recorded AI/dev sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			start, end, err := timeRange(since, from, to)
			if err != nil {
				return err
			}
			sessions, err := st.QuerySessions(events.SessionFilter{Since: start, Until: end})
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, sessions)
			}
			output.Header(os.Stdout, "Sessions")
			if len(sessions) == 0 {
				output.EmptyNote(os.Stdout, "no sessions recorded yet")
				return nil
			}
			fmt.Printf("\n%s %s %s %s %s\n",
				output.Dim(output.Pad("ID", 14)), output.Dim(output.Pad("START", 10)),
				output.Dim(output.Pad("END", 10)), output.Dim(output.Pad("AGENT", 22)),
				output.Dim("RISKS  REPO"))
			for _, s := range sessions {
				end := "active"
				if s.EndedAt != nil {
					end = output.Clock(*s.EndedAt)
				}
				repo := "(unknown)"
				if s.RepoPath != "" {
					repo = shortPath(s.RepoPath)
				}
				fmt.Printf("%s %s %s %s %s  %s\n",
					output.Pad(s.ID, 14), output.Pad(output.Clock(s.StartedAt), 10),
					output.Pad(end, 10), output.Pad(truncate(s.DetectedAgent, 22), 22),
					output.Pad(fmt.Sprintf("%d", s.RiskCount), 5), repo)
			}
			return nil
		},
	}
	addTimeFlags(cmd, &since, &from, &to, "")
	return cmd
}

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Inspect a single session (show / diff / risks)",
	}
	cmd.AddCommand(newSessionShowCmd(), newSessionDiffCmd(), newSessionRisksCmd())
	return cmd
}

func newSessionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <session_id>",
		Short: "Show full details for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			sess, err := st.GetSession(args[0])
			if err != nil {
				return err
			}
			evs, err := st.QueryEvents(events.EventFilter{SessionID: sess.ID})
			if err != nil {
				return err
			}
			s := buildSummary(evs)
			if flagJSON {
				return output.JSON(os.Stdout, map[string]any{"session": sess, "events": evs})
			}

			output.Header(os.Stdout, "Session "+sess.ID)
			agent := sess.DetectedAgent
			if agent == "" {
				agent = "unknown_dev_activity"
			}
			fmt.Printf("\n%s %s (confidence %.1f)\n", output.Dim("Agent:"), agent, sess.Confidence)
			repo := "(unknown)"
			if sess.RepoPath != "" {
				repo = shortPath(sess.RepoPath)
			}
			fmt.Printf("%s %s\n", output.Dim("Repo: "), repo)
			end := "active"
			if sess.EndedAt != nil {
				end = output.ClockDate(*sess.EndedAt)
			}
			fmt.Printf("%s %s → %s\n", output.Dim("Time: "), output.ClockDate(sess.StartedAt), end)

			renderWhatChanged(os.Stdout, s)
			return nil
		},
	}
}

func newSessionRisksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "risks <session_id>",
		Short: "Show only the risk events in a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			sess, err := st.GetSession(args[0])
			if err != nil {
				return err
			}
			evs, err := st.QueryEvents(events.EventFilter{SessionID: sess.ID, Category: events.CategoryRisk})
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, evs)
			}
			output.Header(os.Stdout, "Risks in session "+sess.ID)
			if len(evs) == 0 {
				output.EmptyNote(os.Stdout, "no risks in this session")
				return nil
			}
			fmt.Println()
			renderTimeline(os.Stdout, evs)
			return nil
		},
	}
}

func newSessionDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <session_id>",
		Short: "Show a Git/file diff summary for a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			sess, err := st.GetSession(args[0])
			if err != nil {
				return err
			}
			output.Header(os.Stdout, "Diff for session "+sess.ID)

			// Prefer a live Git diff when the repo still exists.
			if sess.RepoPath != "" {
				if _, err := os.Stat(filepath.Join(sess.RepoPath, ".git")); err == nil {
					out, err := exec.CommandContext(context.Background(), "git", "-C", sess.RepoPath, "diff", "--stat").Output()
					if err == nil && strings.TrimSpace(string(out)) != "" {
						fmt.Printf("\n%s %s\n\n", output.Dim("repo:"), shortPath(sess.RepoPath))
						fmt.Print(string(out))
						return nil
					}
				}
			}

			// Fall back to recorded file/git events.
			evs, err := st.QueryEvents(events.EventFilter{SessionID: sess.ID})
			if err != nil {
				return err
			}
			var fileEvents []events.Event
			for _, ev := range evs {
				if ev.Category == events.CategoryFile || ev.Category == events.CategoryGit {
					fileEvents = append(fileEvents, ev)
				}
			}
			if len(fileEvents) == 0 {
				output.EmptyNote(os.Stdout, "no file changes recorded for this session")
				return nil
			}
			fmt.Println()
			renderTimeline(os.Stdout, fileEvents)
			return nil
		},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
