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

		paramStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("51"))
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

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

			// Highlight {{var}} in command template
			renderedCmd := varRegex.ReplaceAllStringFunc(cmdDef.Cmd, func(m string) string {
				return paramStyle.Render(m)
			})
			fmt.Printf("  %s\n", cmdStyle.Render(renderedCmd))

			// Show parameter usage hint if command has variables
			if len(cmdDef.Vars) > 0 {
				var varSummaries []string
				for vName, vDef := range cmdDef.Vars {
					state := "positional"
					if vDef.Secret {
						state = "secret"
					} else if vDef.Default != "" {
						state = fmt.Sprintf("default: %s", vDef.Default)
					} else if len(vDef.Choices) > 0 {
						state = fmt.Sprintf("choices: %s", strings.Join(vDef.Choices, "|"))
					}
					varSummaries = append(varSummaries, fmt.Sprintf("{{%s}} (%s)", vName, state))
				}
				sort.Strings(varSummaries)
				fmt.Printf("  %s %s\n", hintStyle.Render("↳ params:"), hintStyle.Render(strings.Join(varSummaries, ", ")))
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
