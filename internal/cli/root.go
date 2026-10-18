package cli

import (
	"fmt"
	"os"

	"github.com/leo-ramos/als/internal/config"
	"github.com/leo-ramos/als/internal/shim"
	"github.com/leo-ramos/als/internal/store"
	"github.com/spf13/cobra"
)

var (
	cfg     *config.Config
	st      *store.Store
	shimMgr *shim.Manager
)

// RootCmd is the main entry point for the als CLI.
var RootCmd = &cobra.Command{
	Use:   "als",
	Short: "als - Instant, parameterized command & alias manager for developers",
	Long: `als transforms complex terminal commands into direct, instant aliases in your PATH
with dynamic parameters ({{var}}), secret isolation, and Git sync.`,
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	st = store.New(cfg)
	shimMgr = shim.NewManager(cfg)
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
