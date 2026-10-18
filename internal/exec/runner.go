package exec

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/leo-ramos/als/internal/store"
	"golang.org/x/term"
)

// ExecFunc allows swapping the actual syscall.Exec during unit tests.
var ExecFunc = syscall.Exec

type Runner struct {
	st *store.Store
}

func NewRunner(st *store.Store) *Runner {
	return &Runner{st: st}
}

// RunShim executes an alias by name with the given command line arguments.
func (r *Runner) RunShim(alias string, args []string) error {
	cmdDef, err := r.st.GetCommand(alias)
	if err != nil {
		return fmt.Errorf("alias '%s' not found in store: %w", alias, err)
	}

	// 1. Check for inspection flags
	if len(args) > 0 {
		switch args[0] {
		case "--als-info":
			r.printInfo(alias, cmdDef)
			return nil
		case "--als-dry-run":
			return r.dryRun(alias, cmdDef, args[1:])
		}
	}

	// 2. Compile template
	compiled, err := CompileTemplate(cmdDef.Cmd, cmdDef.Vars)
	if err != nil {
		return fmt.Errorf("failed to compile template for '%s': %w", alias, err)
	}

	// 3. Resolve variables
	res, err := ResolveVariables(alias, cmdDef, compiled, args, r.st)
	if err != nil {
		return err
	}

	// 4. Handle confirmation prompt if required
	if cmdDef.Confirm && term.IsTerminal(int(os.Stdin.Fd())) {
		var confirmed bool
		confirmTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")).
			Render(fmt.Sprintf("Confirm execution of '%s'?", alias))

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(confirmTitle).
					Description(compiled.ShellCmd).
					Value(&confirmed),
			),
		)
		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Execution cancelled.")
			return nil
		}
	}

	// 5. Build environment slice
	env := os.Environ()
	for k, v := range res.ResolvedVars {
		env = append(env, fmt.Sprintf("%s=%s", EnvVarName(k), v))
	}

	// 6. Construct /bin/sh invocation
	shellCmd := compiled.ShellCmd
	if !strings.Contains(shellCmd, "$@") {
		shellCmd += ` "$@"`
	}

	shArgs := append([]string{"sh", "-c", shellCmd, alias}, res.RemainingArgs...)

	// 7. Execute process replacement via syscall.Exec
	return ExecFunc("/bin/sh", shArgs, env)
}

func (r *Runner) printInfo(alias string, cmd *store.Command) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))

	fmt.Printf("%s: %s\n", titleStyle.Render("Alias"), alias)
	if cmd.Desc != "" {
		fmt.Printf("%s: %s\n", keyStyle.Render("Description"), cmd.Desc)
	}
	fmt.Printf("%s: %s\n", keyStyle.Render("Command Template"), cmd.Cmd)
	if len(cmd.Tags) > 0 {
		fmt.Printf("%s: %s\n", keyStyle.Render("Tags"), strings.Join(cmd.Tags, ", "))
	}
	if len(cmd.Vars) > 0 {
		fmt.Println(keyStyle.Render("Variables:"))
		for vName, vDef := range cmd.Vars {
			status := "text"
			if vDef.Secret {
				status = "secret"
			}
			if len(vDef.Choices) > 0 {
				status = fmt.Sprintf("choices [%s]", strings.Join(vDef.Choices, ", "))
			}
			fmt.Printf("  • %s (%s)\n", vName, status)
		}
	}
}

func (r *Runner) dryRun(alias string, cmdDef *store.Command, args []string) error {
	compiled, err := CompileTemplate(cmdDef.Cmd, cmdDef.Vars)
	if err != nil {
		return err
	}
	res, err := ResolveVariables(alias, cmdDef, compiled, args, r.st)
	if err != nil {
		return err
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	codeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

	fmt.Println(titleStyle.Render("=== ALS DRY RUN ==="))
	fmt.Printf("Command to execute: %s\n", codeStyle.Render(compiled.ShellCmd))
	fmt.Println("Remaining args passed to $@:", res.RemainingArgs)
	fmt.Println("Environment Variables:")
	for k, v := range res.ResolvedVars {
		masked := v
		if cmdDef.Vars[k].Secret && len(v) > 4 {
			masked = v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
		}
		fmt.Printf("  %s=%s\n", EnvVarName(k), masked)
	}
	return nil
}
