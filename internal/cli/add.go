package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/leo-ramos/als/internal/linter"
	"github.com/leo-ramos/als/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	flagLast    bool
	flagDesc    string
	flagTags    string
	flagConfirm bool
)

var varRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9_-]+)\}\}`)

type escConfirmModel struct {
	form             *huh.Form
	confirmingCancel bool
	aborted          bool
}

func (m *escConfirmModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *escConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.Type == tea.KeyEsc {
			if m.confirmingCancel {
				m.aborted = true
				m.form.State = huh.StateAborted
				return m, tea.Interrupt
			}
			m.confirmingCancel = true
			return m, nil
		}

		if m.confirmingCancel {
			if keyMsg.Type == tea.KeyCtrlC {
				m.aborted = true
				m.form.State = huh.StateAborted
				return m, tea.Interrupt
			}
			// Swallow the key used to dismiss the cancel confirmation so it does
			// not mutate the focused input or accidentally submit on Enter.
			m.confirmingCancel = false
			return m, nil
		}
	}

	model, cmd := m.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

func (m *escConfirmModel) View() string {
	if m.aborted || m.form.State != huh.StateNormal {
		return ""
	}
	view := m.form.View()
	if m.confirmingCancel {
		warn := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")).
			Render("\n⚠ Want to cancel? Press ESC again to confirm (or any other key to continue).")
		return view + warn
	}
	return view
}

func runFormWithEscConfirm(form *huh.Form) error {
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt

	m := &escConfirmModel{form: form}
	res, err := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithReportFocus()).Run()
	if finalModel, ok := res.(*escConfirmModel); ok && (finalModel.aborted || finalModel.form.State == huh.StateAborted) {
		return huh.ErrUserAborted
	}
	if errors.Is(err, tea.ErrInterrupted) {
		return huh.ErrUserAborted
	}
	return err
}

var addCmd = &cobra.Command{
	Use:   "add [alias] [command]",
	Short: "Add and register a new command alias",
	Long: `Add a new alias. You can supply arguments directly or run interactively.
Use {{var}} in the command to define dynamic parameters.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var aliasName string
		var rawCmd string

		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		infoStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

		// Check --last flag
		if flagLast {
			lastHistory, err := getLastHistoryCommand()
			if err != nil {
				return fmt.Errorf("failed to read last command from history: %w", err)
			}
			rawCmd = lastHistory
			fmt.Printf("Loaded last command from history: %s\n", infoStyle.Render(rawCmd))
		}

		if len(args) >= 1 && aliasName == "" {
			aliasName = args[0]
		}
		if len(args) >= 2 && rawCmd == "" {
			rawCmd = strings.Join(args[1:], " ")
		}

		isInteractive := term.IsTerminal(int(os.Stdin.Fd()))

		// Interactive form if fields missing
		if isInteractive && (aliasName == "" || rawCmd == "") {
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Alias Name (what you will type in terminal)").
						Placeholder("e.g. connectServerProd").
						Value(&aliasName).
						Validate(func(s string) error {
							return shimMgr.ValidateAlias(s)
						}),
					huh.NewInput().
						Title("Command to execute").
						Description("Tip: Use {{var}} for dynamic parameters (e.g. URL={{url}})").
						Placeholder(`e.g. ssh -i "key.pem" ubuntu@1.2.3.4`).
						Value(&rawCmd).
						Validate(func(s string) error {
							if strings.TrimSpace(s) == "" {
								return fmt.Errorf("command cannot be empty")
							}
							return nil
						}),
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

		if aliasName == "" || rawCmd == "" {
			return fmt.Errorf("both alias name and command are required")
		}

		if err := shimMgr.ValidateAlias(aliasName); err != nil {
			return err
		}

		extractedValues := make(map[string]string)

		// Run Linter
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

		// Configure variables
		var varDefs map[string]store.VarDef
		if isInteractive {
			configured, err := configureVariablesWizard(rawCmd, aliasName, nil, extractedValues)
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
					isSecret := strings.Contains(strings.ToLower(vName), "key") ||
						strings.Contains(strings.ToLower(vName), "ip") ||
						strings.Contains(strings.ToLower(vName), "secret") ||
						strings.Contains(strings.ToLower(vName), "token") ||
						strings.Contains(strings.ToLower(vName), "pass")

					defVal := ""
					if val, ok := extractedValues[vName]; ok {
						if isSecret {
							_ = st.SetSecret(aliasName, vName, val, false)
						} else {
							defVal = val
						}
					}

					varDefs[vName] = store.VarDef{
						Secret:     isSecret,
						Default:    defVal,
						Positional: true,
						Prompt:     fmt.Sprintf("Enter %s", vName),
					}
				}
			}
		}

		var tags []string
		if flagTags != "" {
			for _, t := range strings.Split(flagTags, ",") {
				if clean := strings.TrimSpace(t); clean != "" {
					tags = append(tags, clean)
				}
			}
		}

		cmdDef := store.Command{
			Cmd:     rawCmd,
			Desc:    flagDesc,
			Tags:    tags,
			Confirm: flagConfirm,
			Vars:    varDefs,
		}

		// Save command
		if err := st.SaveCommand(aliasName, cmdDef); err != nil {
			return fmt.Errorf("failed to save command: %w", err)
		}

		// Create shim symlink
		if err := shimMgr.CreateShim(aliasName); err != nil {
			return fmt.Errorf("failed to create shim: %w", err)
		}

		fmt.Println("\n" + successStyle.Render(fmt.Sprintf("✔ Alias '%s' created and registered successfully!", aliasName)))
		fmt.Printf("  Command: %s\n", infoStyle.Render(rawCmd))
		fmt.Printf("  You can now run it anywhere in your terminal simply by typing: %s\n", lipgloss.NewStyle().Bold(true).Render(aliasName))

		return nil
	},
}

func getLastHistoryCommand() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	histFiles := []string{
		filepath.Join(home, ".zsh_history"),
		filepath.Join(home, ".bash_history"),
	}

	for _, file := range histFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			// zsh history format: ": <timestamp>:0;<command>"
			if strings.HasPrefix(line, ":") && strings.Contains(line, ";") {
				parts := strings.SplitN(line, ";", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
					return strings.TrimSpace(parts[1]), nil
				}
			}
			return line, nil
		}
	}

	return "", fmt.Errorf("no history file found")
}

func init() {
	addCmd.Flags().BoolVar(&flagLast, "last", false, "Use the last executed command from shell history")
	addCmd.Flags().StringVarP(&flagDesc, "desc", "d", "", "Description of the command")
	addCmd.Flags().StringVarP(&flagTags, "tags", "t", "", "Comma-separated tags")
	addCmd.Flags().BoolVarP(&flagConfirm, "confirm", "c", false, "Ask for confirmation before running")
	RootCmd.AddCommand(addCmd)
}
