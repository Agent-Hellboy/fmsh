package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"fmsh/internal/config"
	"fmsh/internal/db"
	"fmsh/internal/output"
	"fmsh/internal/store"
)

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create ~/.fmsh, write default config, and initialize the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := config.Home()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(home, 0o755); err != nil {
				return err
			}

			cfgPath, _ := config.Path()
			cfg := config.Default()
			if _, err := os.Stat(cfgPath); err == nil && !force {
				fmt.Printf("%s config already exists at %s (use --force to overwrite)\n", output.Dim("~"), cfgPath)
				cfg, err = config.Load()
				if err != nil {
					return err
				}
			} else {
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Printf("%s wrote config %s\n", output.Green(output.SymAdded), cfgPath)
			}

			dbPath := cfg.ResolvedDBPath()
			sqlDB, err := db.Open(dbPath)
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			st := store.New(sqlDB)

			// Register default watch paths that exist.
			registered := 0
			for _, p := range cfg.ResolvedWatchPaths() {
				if _, err := os.Stat(p); err != nil {
					continue
				}
				if err := st.AddWatchedPath(p); err == nil {
					registered++
				}
			}
			fmt.Printf("%s initialized database %s\n", output.Green(output.SymAdded), dbPath)
			fmt.Printf("%s registered %d watch path(s)\n", output.Green(output.SymAdded), registered)

			fmt.Printf("\nNext steps:\n")
			fmt.Printf("  fmsh daemon start\n")
			fmt.Printf("  fmsh timeline --since 10m\n")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config")
	return cmd
}
