package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/events"
	"fmsh/internal/output"
)

func newAgentReportCmd() *cobra.Command {
	var since, from, to string
	cmd := &cobra.Command{
		Use:   "agent-report",
		Short: "AI/dev-session focused audit report",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			filter, err := queryRange(since, from, to)
			if err != nil {
				return err
			}
			evs, err := st.QueryEvents(filter)
			if err != nil {
				return err
			}
			sessions, err := st.QuerySessions(events.SessionFilter{Since: filter.Since, Until: filter.Until})
			if err != nil {
				return err
			}
			s := buildSummary(evs)

			if flagJSON {
				return output.JSON(os.Stdout, map[string]any{"sessions": sessions, "summary": s})
			}

			output.Header(os.Stdout, "AI Agent Activity Report — "+capitalize(rangeLabel(since, from, to)))

			output.Section(os.Stdout, "Detected sessions:")
			if len(sessions) == 0 {
				output.EmptyNote(os.Stdout, "no sessions detected")
			} else {
				for _, sess := range sessions {
					fmt.Printf("  %s %s\n", output.Green(output.SymAdded), sessionHeadline(sess))
				}
			}

			output.Section(os.Stdout, "Commands/processes seen:")
			if len(s.Processes) == 0 {
				output.EmptyNote(os.Stdout, "none observed")
			} else {
				for _, p := range dedupeSorted(s.Processes) {
					fmt.Printf("  %s %s\n", output.SymAdded, p)
				}
			}

			output.Section(os.Stdout, "Files changed:")
			files := append(append([]string{}, s.FilesCreated...), s.FilesModified...)
			files = append(files, s.FilesDeleted...)
			if len(files) == 0 {
				output.EmptyNote(os.Stdout, "none")
			} else {
				for _, f := range dedupeSorted(files) {
					fmt.Printf("  %s %s\n", output.SymModified, f)
				}
			}

			output.Section(os.Stdout, "Risk events:")
			renderRiskLines(os.Stdout, s)

			output.Section(os.Stdout, "Suggested review:")
			for i, r := range s.suggestedReview() {
				fmt.Printf("  %d. %s\n", i+1, r)
			}
			return nil
		},
	}
	addTimeFlags(cmd, &since, &from, &to, "2h")
	return cmd
}
