package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [alias]",
	Short: "Show detailed information about an alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		aliasName := args[0]

		cmdDef, err := st.GetCommand(aliasName)
		if err != nil {
			return fmt.Errorf("alias '%s' not found: %w", aliasName, err)
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
		keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
		codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

		fmt.Printf("%s: %s\n", titleStyle.Render("Alias"), aliasName)
		if cmdDef.Desc != "" {
			fmt.Printf("%s: %s\n", keyStyle.Render("Description"), cmdDef.Desc)
		}
		fmt.Printf("%s: %s\n", keyStyle.Render("Command Template"), codeStyle.Render(cmdDef.Cmd))
		if len(cmdDef.Tags) > 0 {
			fmt.Printf("%s: %s\n", keyStyle.Render("Tags"), strings.Join(cmdDef.Tags, ", "))
		}
		fmt.Printf("%s: %v\n", keyStyle.Render("Requires Confirmation"), cmdDef.Confirm)

		if len(cmdDef.Vars) > 0 {
			fmt.Println(keyStyle.Render("Variables:"))
			secDoc, _ := st.LoadSecrets()
			for vName, vDef := range cmdDef.Vars {
				status := "text"
				if vDef.Secret {
					status = "secret"
				}
				if len(vDef.Choices) > 0 {
					status = fmt.Sprintf("choices [%s]", strings.Join(vDef.Choices, ", "))
				}
				hasSecret := false
				if _, ok := st.ResolveSecret(secDoc, aliasName, vName, ""); ok {
					hasSecret = true
				}
				savedStr := "(not saved locally)"
				if hasSecret {
					savedStr = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("(saved in secrets.toml)")
				}
				fmt.Printf("  • %s (%s) %s\n", vName, status, savedStr)
			}
		}

		shimPath := cfg.ShimPath(aliasName)
		fmt.Printf("\n%s: %s\n", keyStyle.Render("Shim Location"), shimPath)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}
