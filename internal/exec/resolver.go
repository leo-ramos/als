package exec

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/leo-ramos/als/internal/store"
	"golang.org/x/term"
)

type ResolutionResult struct {
	ResolvedVars  map[string]string
	RemainingArgs []string
}

// ResolveVariables extracts CLI arguments, secrets, and prompts for missing parameters.
func ResolveVariables(
	alias string,
	cmdDef *store.Command,
	compiled *CompiledCommand,
	args []string,
	st *store.Store,
) (*ResolutionResult, error) {
	resolved := make(map[string]string)
	var remainingArgs []string

	// 1. First pass: extract explicit @var=val or @var:val overrides from CLI args
	var nonOverrideArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") && (strings.Contains(arg, "=") || strings.Contains(arg, ":")) {
			raw := strings.TrimPrefix(arg, "@")
			sep := "="
			if !strings.Contains(raw, "=") {
				sep = ":"
			}
			parts := strings.SplitN(raw, sep, 2)
			if len(parts) == 2 {
				varName := strings.TrimSpace(parts[0])
				varVal := strings.TrimSpace(parts[1])
				resolved[varName] = varVal
				continue
			}
		}
		nonOverrideArgs = append(nonOverrideArgs, arg)
	}

	// 2. Second pass: check for positional choice match for variables with choices
	var remainingPositional []string
	for i, arg := range nonOverrideArgs {
		matchedChoice := false
		for _, varName := range compiled.Variables {
			if _, alreadyResolved := resolved[varName]; alreadyResolved {
				continue
			}
			varDef := cmdDef.Vars[varName]
			if len(varDef.Choices) > 0 {
				for _, ch := range varDef.Choices {
					if arg == ch {
						resolved[varName] = ch
						matchedChoice = true
						break
					}
				}
			}
			if matchedChoice {
				break
			}
		}
		if !matchedChoice {
			remainingPositional = append(remainingPositional, nonOverrideArgs[i:]...)
			break
		}
	}
	remainingArgs = remainingPositional

	// 3. Load secrets document
	secDoc, _ := st.LoadSecrets()

	isTerminal := term.IsTerminal(int(os.Stdin.Fd()))

	// 4. Resolve each required variable in compiled template
	for _, varName := range compiled.Variables {
		varDef := cmdDef.Vars[varName]

		// If already resolved via CLI choice, see if that choice key maps to a secret value
		currentVal, exists := resolved[varName]
		if exists && len(varDef.Choices) > 0 {
			if secretVal, ok := st.ResolveSecret(secDoc, alias, varName, currentVal); ok && secretVal != "" {
				resolved[varName] = secretVal
				continue
			}
			// If not a secret choice, the choice key itself is the value
			continue
		}

		if exists && currentVal != "" {
			continue
		}

		// Check secrets.toml (command scope > global scope)
		if secVal, ok := st.ResolveSecret(secDoc, alias, varName, ""); ok && secVal != "" {
			resolved[varName] = secVal
			continue
		}

		// Check default value in commands.toml
		if varDef.Default != "" {
			resolved[varName] = varDef.Default
			continue
		}

		// Check environment variables
		if envVal := os.Getenv(varName); envVal != "" {
			resolved[varName] = envVal
			continue
		}

		// Variable is missing
		if !isTerminal {
			return nil, fmt.Errorf("missing required variable '%s' for alias '%s' (non-interactive mode)", varName, alias)
		}

		// Prompt interactively via Huh
		var inputVal string
		promptLabel := varDef.Prompt
		if promptLabel == "" {
			promptLabel = fmt.Sprintf("Enter value for '%s'", varName)
		}

		if len(varDef.Choices) > 0 {
			var options []huh.Option[string]
			for _, ch := range varDef.Choices {
				options = append(options, huh.NewOption(ch, ch))
			}
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title(promptLabel).
						Options(options...).
						Value(&inputVal),
				),
			)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("prompt cancelled: %w", err)
			}

			// If the selected choice maps to a secret in secrets.toml, resolve it
			if secretVal, ok := st.ResolveSecret(secDoc, alias, varName, inputVal); ok && secretVal != "" {
				resolved[varName] = secretVal
			} else {
				resolved[varName] = inputVal
			}
		} else {
			inputField := huh.NewInput().
				Title(promptLabel).
				Value(&inputVal)
			if varDef.Secret {
				inputField.EchoMode(huh.EchoModePassword)
			}

			form := huh.NewForm(
				huh.NewGroup(inputField),
			)
			if err := form.Run(); err != nil {
				return nil, fmt.Errorf("prompt cancelled: %w", err)
			}
			resolved[varName] = inputVal

			// Ask if user wants to save this secret for future executions
			if varDef.Secret && inputVal != "" {
				var saveIt bool
				savePrompt := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title(fmt.Sprintf("Save '%s' locally in secrets.toml for future runs?", varName)).
							Value(&saveIt),
					),
				)
				if err := savePrompt.Run(); err == nil && saveIt {
					isGlobal := varDef.Scope == "global"
					_ = st.SetSecret(alias, varName, inputVal, isGlobal)
				}
			}
		}
	}

	return &ResolutionResult{
		ResolvedVars:  resolved,
		RemainingArgs: remainingArgs,
	}, nil
}
