package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/events"
	"fmsh/internal/output"
)

func newRisksCmd() *cobra.Command {
	var since, from, to string
	cmd := &cobra.Command{
		Use:   "risks",
		Short: "Show a risk summary grouped by severity",
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
			filter.Category = events.CategoryRisk
			evs, err := st.QueryEvents(filter)
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, evs)
			}

			output.Header(os.Stdout, "Risks — "+capitalize(rangeLabel(since, from, to)))
			if len(evs) == 0 {
				output.EmptyNote(os.Stdout, "no risks detected")
				return nil
			}

			var high, medium, low []events.Event
			for _, ev := range evs {
				switch ev.Severity {
				case events.SeverityHigh:
					high = append(high, ev)
				case events.SeverityMedium:
					medium = append(medium, ev)
				default:
					low = append(low, ev)
				}
			}
			printRiskGroup(os.Stdout, "High:", high)
			printRiskGroup(os.Stdout, "Medium:", medium)
			printRiskGroup(os.Stdout, "Low:", low)
			return nil
		},
	}
	addTimeFlags(cmd, &since, &from, &to, "24h")
	return cmd
}

func printRiskGroup(w *os.File, label string, evs []events.Event) {
	if len(evs) == 0 {
		return
	}
	output.Section(w, label)
	for _, ev := range evs {
		detail := ev.Subject
		if ev.RepoPath != "" {
			detail += output.Dim(" in " + shortPath(ev.RepoPath))
		}
		fmt.Fprintf(w, "  %s %s %s\n", output.SeveritySymbol(ev.Severity), output.Dim(output.Clock(ev.Timestamp)), detail)
	}
}
