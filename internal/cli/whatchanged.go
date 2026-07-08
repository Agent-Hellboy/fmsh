package cli

import (
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/output"
)

func newWhatChangedCmd() *cobra.Command {
	var since, from, to string
	cmd := &cobra.Command{
		Use:   "what-changed",
		Short: "Human-readable summary of files, processes, ports, and risks",
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
			s := buildSummary(evs)
			if flagJSON {
				return output.JSON(os.Stdout, s)
			}
			output.Header(os.Stdout, "What changed — "+capitalize(rangeLabel(since, from, to)))
			renderWhatChanged(os.Stdout, s)
			return nil
		},
	}
	addTimeFlags(cmd, &since, &from, &to, "30m")
	return cmd
}
