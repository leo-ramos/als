package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm [alias]",
	Aliases: []string{"remove", "delete"},
	Short:   "Remove a registered command alias",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		aliasName := args[0]

		if err := st.DeleteCommand(aliasName); err != nil {
			return fmt.Errorf("failed to delete alias '%s': %w", aliasName, err)
		}

		if err := shimMgr.RemoveShim(aliasName); err != nil {
			return fmt.Errorf("failed to remove shim for '%s': %w", aliasName, err)
		}

		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		fmt.Println(successStyle.Render(fmt.Sprintf("✔ Alias '%s' and its shim removed successfully.", aliasName)))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(rmCmd)
}
