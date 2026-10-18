package cli

import (
	"github.com/leo-ramos/als/internal/exec"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                "run [alias] [flags/args...]",
	Short:              "Directly execute a registered alias (useful before PATH reload)",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		aliasName := args[0]
		remainingArgs := args[1:]

		runner := exec.NewRunner(st)
		return runner.RunShim(aliasName, remainingArgs)
	},
}

func init() {
	RootCmd.AddCommand(runCmd)
}
