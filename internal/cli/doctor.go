package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/doctor"
	"fmsh/internal/output"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose fmsh configuration, database, and daemon health",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := doctor.Run()
			if flagJSON {
				return output.JSON(os.Stdout, checks)
			}
			output.Header(os.Stdout, "fmsh doctor")
			for _, c := range checks {
				var mark string
				switch c.Status {
				case doctor.OK:
					mark = output.Green("ok  ")
				case doctor.Warn:
					mark = output.Yellow("warn")
				default:
					mark = output.Red("fail")
				}
				fmt.Printf("  [%s] %s — %s\n", mark, output.Pad(c.Name, 16), c.Detail)
			}
			if doctor.HasFailures(checks) {
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}
}
