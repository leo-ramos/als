package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/leo-ramos/als/internal/linter"
	"github.com/leo-ramos/als/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var editCmd = &cobra.Command{
	Use:   "edit [alias]",
	Short: "Edit an existing command alias",
	Long: `Edit an existing registered alias. You can change the command,
description, tags, confirmation settings, and parameter behaviors.
If [alias] is omitted in an interactive terminal, a picker is shown.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		infoStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

		doc, err := st.LoadCommands()
		if err != nil {
			return fmt.Errorf("failed to load commands: %w", err)
		}
		if len(doc.Commands) == 0 {
			fmt.Println(warnStyle.Render("No aliases registered yet. Add one with: als add"))
			return nil
		}

		isInteractive := term.IsTerminal(int(os.Stdin.Fd()))
		var aliasName string

		if len(args) >= 1 {
			aliasName = args[0]
		} else if isInteractive {
			// Show interactive picker of available aliases
			keys := make([]string, 0, len(doc.Commands))
			for k := range doc.Commands {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			var options []huh.Option[string]
			for _, k := range keys {
				c := doc.Commands[k]
				desc := c.Desc
				if desc == "" {
					desc = c.Cmd
				}
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}
				options = append(options, huh.NewOption(fmt.Sprintf("%-20s (%s)", k, desc), k))
			}

			pickerForm := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Select an alias to edit").
						Options(options...).
						Value(&aliasName),
				),
			)
			if err := runFormWithEscConfirm(pickerForm); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Println(warnStyle.Render("✗ Operation cancelled."))
					return nil
				}
				return err
			}
		} else {
			return fmt.Errorf("alias name is required in non-interactive mode")
		}

		cmdDef, err := st.GetCommand(aliasName)
		if err != nil {
			return fmt.Errorf("alias '%s' not found: %w", aliasName, err)
		}

		newAliasName := aliasName
		rawCmd := cmdDef.Cmd
		desc := cmdDef.Desc
		tagsStr := strings.Join(cmdDef.Tags, ", ")
		confirm := cmdDef.Confirm

		if isInteractive {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Alias Name").
						Value(&newAliasName).
						Validate(func(s string) error {
							s = strings.TrimSpace(s)
							if s != aliasName {
								if _, exists := doc.Commands[s]; exists {
									return fmt.Errorf("alias '%s' already exists", s)
								}
								return shimMgr.ValidateAlias(s)
							}
							return nil
						}),
					huh.NewInput().
						Title("Command to execute").
						Description("Tip: Use {{var}} for dynamic parameters (e.g. URL={{url}})").
						Value(&rawCmd).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("command cannot be empty")
							}
							return nil
						}),
					huh.NewInput().
						Title("Description").
						Value(&desc),
					huh.NewInput().
						Title("Tags (comma-separated)").
						Value(&tagsStr),
					huh.NewConfirm().
						Title("Require confirmation [y/N] before running?").
						Value(&confirm),
				),
			)
			if err := runFormWithEscConfirm(form); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Println(warnStyle.Render("✗ Operation cancelled."))
					return nil
				}
				return err
			}
		}

		extractedValues := make(map[string]string)

		// Run Linter suggestions on the edited command
		warnings := linter.Analyze(rawCmd)
		if len(warnings) > 0 {
			fmt.Println("\n" + warnStyle.Render("⚡ Linter Suggestions:"))
			for _, w := range warnings {
				fmt.Printf("  • %s\n", w.Suggestion)
			}

			if isInteractive {
				for _, w := range warnings {
					var fixIt bool
					promptTitle := fmt.Sprintf("Auto-fix '%s' by parameterizing with {{%s}}?", w.TargetSpan, w.VariableKey)
					confirmForm := huh.NewForm(
						huh.NewGroup(
							huh.NewConfirm().
								Title(promptTitle).
								Value(&fixIt),
						),
					)
					if err := runFormWithEscConfirm(confirmForm); err != nil {
						if errors.Is(err, huh.ErrUserAborted) {
							fmt.Println(warnStyle.Render("✗ Operation cancelled."))
							return nil
						}
						return err
					}
					if fixIt {
						rawCmd = strings.Replace(rawCmd, w.TargetSpan, w.ReplacementSpan, 1)
						extractedValues[w.VariableKey] = w.RawMatch
						fmt.Printf("    Replaced with %s\n", infoStyle.Render(w.ReplacementSpan))
					}
				}
			}
		}

		// Configure variables wizard
		var varDefs map[string]store.VarDef
		if isInteractive {
			configured, err := configureVariablesWizard(rawCmd, newAliasName, cmdDef.Vars, extractedValues)
			if err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Println(warnStyle.Render("✗ Operation cancelled."))
					return nil
				}
				return err
			}
			varDefs = configured
		} else {
			varDefs = make(map[string]store.VarDef)
			for _, match := range varRegex.FindAllStringSubmatch(rawCmd, -1) {
				if len(match) > 1 {
					vName := match[1]
					if existing, ok := cmdDef.Vars[vName]; ok {
						varDefs[vName] = existing
					} else {
						varDefs[vName] = store.VarDef{
							Positional: true,
							Prompt:     fmt.Sprintf("Enter %s", vName),
						}
					}
				}
			}
		}

		var cleanTags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				if c := strings.TrimSpace(t); c != "" {
					cleanTags = append(cleanTags, c)
				}
			}
		}

		updatedDef := store.Command{
			Cmd:     rawCmd,
			Desc:    desc,
			Tags:    cleanTags,
			Confirm: confirm,
			Vars:    varDefs,
		}

		// Save updated command atomically (and migrate secrets if renamed)
		if err := st.UpdateCommand(aliasName, newAliasName, updatedDef); err != nil {
			return fmt.Errorf("failed to update command: %w", err)
		}

		// Update shims if renamed
		if aliasName != newAliasName {
			_ = shimMgr.RemoveShim(aliasName)
			if err := shimMgr.CreateShim(newAliasName); err != nil {
				return fmt.Errorf("failed to create new shim: %w", err)
			}
		}

		fmt.Println("\n" + successStyle.Render(fmt.Sprintf("✔ Alias '%s' updated successfully!", newAliasName)))
		fmt.Printf("  Command: %s\n", infoStyle.Render(rawCmd))
		if len(varDefs) > 0 {
			var pList []string
			for p := range varDefs {
				pList = append(pList, fmt.Sprintf("{{%s}}", p))
			}
			sort.Strings(pList)
			fmt.Printf("  Parameters: %s (direct positional invocation supported)\n", strings.Join(pList, ", "))
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(editCmd)
}
