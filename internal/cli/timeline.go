package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/output"
)

func newTimelineCmd() *cobra.Command {
	var since, from, to string
	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Show a chronological timeline of recorded events",
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
			if flagJSON {
				return output.JSON(os.Stdout, evs)
			}
			output.Header(os.Stdout, "Timeline — "+capitalize(rangeLabel(since, from, to)))
			if len(evs) == 0 {
				output.EmptyNote(os.Stdout, "no events in this window")
				return nil
			}
			fmt.Println()
			renderTimeline(os.Stdout, evs)
			return nil
		},
	}
	addTimeFlags(cmd, &since, &from, &to, "1h")
	return cmd
}
