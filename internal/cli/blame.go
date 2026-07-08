package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/config"
	"fmsh/internal/events"
	"fmsh/internal/output"
)

func newBlameCmd() *cobra.Command {
	var path string
	var since string
	cmd := &cobra.Command{
		Use:   "blame",
		Short: "Show recent activity involving a path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" && len(args) > 0 {
				path = args[0]
			}
			if path == "" {
				return fmt.Errorf("--path is required")
			}
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			filter := events.EventFilter{Path: config.Expand(path)}
			if since != "" {
				start, err := queryRange(since, "", "")
				if err != nil {
					return err
				}
				filter.Since = start.Since
			}
			evs, err := st.QueryEvents(filter)
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, evs)
			}
			output.Header(os.Stdout, "Activity for "+path)
			if len(evs) == 0 {
				output.EmptyNote(os.Stdout, "no recorded activity for this path")
				return nil
			}
			fmt.Println()
			renderTimeline(os.Stdout, evs)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "path to investigate (e.g. package.json, .env)")
	cmd.Flags().StringVar(&since, "since", "", "optional relative window, e.g. 24h")
	return cmd
}
