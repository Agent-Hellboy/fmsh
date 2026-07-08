package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/output"
)

func newEventsCmd() *cobra.Command {
	var since, from, to string
	var typ, category, path, severity, sessionID string
	var limit int
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Query recorded events with filters",
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
			filter.Type = typ
			filter.Category = category
			filter.Path = path
			filter.Severity = severity
			filter.SessionID = sessionID
			filter.Limit = limit

			evs, err := st.QueryEvents(filter)
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, evs)
			}
			output.Header(os.Stdout, "Events — "+capitalize(rangeLabel(since, from, to)))
			if len(evs) == 0 {
				output.EmptyNote(os.Stdout, "no matching events")
				return nil
			}
			fmt.Println()
			renderTimeline(os.Stdout, evs)
			fmt.Printf("\n%s\n", output.Dim(fmt.Sprintf("%d event(s)", len(evs))))
			return nil
		},
	}
	addTimeFlags(cmd, &since, &from, &to, "1h")
	cmd.Flags().StringVar(&typ, "type", "", "filter by event type, e.g. file.write")
	cmd.Flags().StringVar(&category, "category", "", "filter by category: file, process, network, git, risk, agent")
	cmd.Flags().StringVar(&path, "path", "", "filter by path (exact or basename suffix)")
	cmd.Flags().StringVar(&severity, "severity", "", "filter by severity: info, low, medium, high")
	cmd.Flags().StringVar(&sessionID, "session", "", "filter by session id")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of events (0 = no limit)")
	return cmd
}
