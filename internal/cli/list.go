package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all registered command aliases",
	RunE: func(cmd *cobra.Command, args []string) error {
		doc, err := st.LoadCommands()
		if err != nil {
			return err
		}

		if len(doc.Commands) == 0 {
			fmt.Println("No aliases registered yet. Add one with: als add")
			return nil
		}

		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
		aliasStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
		tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

		fmt.Println(titleStyle.Render(fmt.Sprintf("Registered Aliases (%d):", len(doc.Commands))))
		fmt.Println()

		// Sort keys for deterministic display
		keys := make([]string, 0, len(doc.Commands))
		for k := range doc.Commands {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			cmdDef := doc.Commands[name]
			tagStr := ""
			if len(cmdDef.Tags) > 0 {
				tagStr = " " + tagStyle.Render("["+strings.Join(cmdDef.Tags, ", ")+"]")
			}
			fmt.Printf("• %s%s\n", aliasStyle.Render(name), tagStr)
			if cmdDef.Desc != "" {
				fmt.Printf("  %s\n", cmdDef.Desc)
			}
			fmt.Printf("  %s\n\n", cmdStyle.Render(cmdDef.Cmd))
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
