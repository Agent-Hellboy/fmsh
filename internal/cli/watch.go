package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/config"
	"fmsh/internal/output"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage watched paths",
	}
	cmd.AddCommand(newWatchAddCmd(), newWatchRemoveCmd(), newWatchListCmd())
	return cmd
}

func newWatchAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path>",
		Short: "Add a path to watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			abs := config.Expand(args[0])
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("path does not exist: %s", abs)
			}
			if err := st.AddWatchedPath(abs); err != nil {
				return err
			}
			fmt.Printf("%s watching %s\n", output.Green(output.SymAdded), abs)
			fmt.Println(output.Dim("  restart the daemon to pick up new paths: fmsh daemon restart"))
			return nil
		},
	}
}

func newWatchRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <path>",
		Short: "Stop watching a path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			abs := config.Expand(args[0])
			if err := st.RemoveWatchedPath(abs); err != nil {
				return err
			}
			fmt.Printf("%s stopped watching %s\n", output.Yellow(output.SymRemoved), abs)
			return nil
		},
	}
}

func newWatchListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List watched paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, st, sqlDB, err := openStore()
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			paths, err := st.ListWatchedPaths()
			if err != nil {
				return err
			}
			if flagJSON {
				return output.JSON(os.Stdout, paths)
			}
			if len(paths) == 0 {
				output.EmptyNote(os.Stdout, "no watched paths — add one with `fmsh watch add <path>`")
				return nil
			}
			output.Header(os.Stdout, "Watched paths")
			for _, p := range paths {
				fmt.Printf("  %s %s\n", output.SymAdded, p)
			}
			return nil
		},
	}
}
