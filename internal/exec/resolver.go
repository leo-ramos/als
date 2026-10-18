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
	var preDelimiterArgs []string
	var postDelimiterArgs []string
	foundDelimiter := false

	// 1. Separate arguments by POSIX delimiter "--"
	for _, arg := range args {
		if !foundDelimiter && arg == "--" {
			foundDelimiter = true
			continue
		}
		if foundDelimiter {
			postDelimiterArgs = append(postDelimiterArgs, arg)
		} else {
			preDelimiterArgs = append(preDelimiterArgs, arg)
		}
	}

	// 2. First pass: extract explicit @var=val or @var:val overrides from CLI args
	var nonOverrideArgs []string
	for _, arg := range preDelimiterArgs {
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

	// 3. Second pass: match positional arguments (choices or positional variables)
	var remainingPositional []string
	for i, arg := range nonOverrideArgs {
		// If argument starts with "-", it is a flag intended for the underlying command, not a positional parameter
		if strings.HasPrefix(arg, "-") {
			remainingPositional = append(remainingPositional, nonOverrideArgs[i:]...)
			break
		}

		matched := false

		// 3a. Check if it matches an explicit choice for an unresolved variable
		for _, varName := range compiled.Variables {
			if _, alreadyResolved := resolved[varName]; alreadyResolved {
				continue
			}
			varDef := cmdDef.Vars[varName]
			if len(varDef.Choices) > 0 {
				for _, ch := range varDef.Choices {
					if arg == ch {
						resolved[varName] = ch
						matched = true
						break
					}
				}
			}
			if matched {
				break
			}
		}

		// 3b. If not a choice, check if it can fill the next unresolved positional variable
		if !matched {
			for _, varName := range compiled.Variables {
				if _, alreadyResolved := resolved[varName]; alreadyResolved {
					continue
				}
				varDef := cmdDef.Vars[varName]

				// A variable accepts a positional argument if:
				// - it is explicitly configured as Positional = true, OR
				// - it is configured as PromptAlways = true, OR
				// - the command has only 1 variable and it's not a pure secret credential, OR
				// - it is not a secret with an already-saved local value
				isPositionalEligible := varDef.Positional ||
					varDef.PromptAlways ||
					(len(compiled.Variables) == 1 && !varDef.Secret) ||
					(!varDef.Secret && len(varDef.Choices) == 0)

				if isPositionalEligible {
					resolved[varName] = arg
					matched = true
					break
				}
			}
		}

		if !matched {
			remainingPositional = append(remainingPositional, nonOverrideArgs[i:]...)
			break
		}
	}

	remainingArgs := append(remainingPositional, postDelimiterArgs...)

	// 4. Load secrets document
	secDoc, _ := st.LoadSecrets()

	isTerminal := term.IsTerminal(int(os.Stdin.Fd()))

	// 5. Resolve each required variable in compiled template
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

		// Check stored values (secrets.toml > default in commands.toml > env)
		var storedVal string
		if secVal, ok := st.ResolveSecret(secDoc, alias, varName, ""); ok && secVal != "" {
			storedVal = secVal
		} else if varDef.Default != "" {
			storedVal = varDef.Default
		} else if envVal := os.Getenv(varName); envVal != "" {
			storedVal = envVal
		}

		// If we have a stored value and PromptAlways is NOT set, use it directly
		if storedVal != "" && !varDef.PromptAlways {
			resolved[varName] = storedVal
			continue
		}

		// If missing and non-interactive, fail or use stored value if available
		if !isTerminal {
			if storedVal != "" {
				resolved[varName] = storedVal
				continue
			}
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
			// Pre-select if storedVal matches a choice
			if storedVal != "" {
				inputVal = storedVal
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
			// Pre-populate input with stored value if available, allowing quick [Enter] confirmation
			inputVal = storedVal

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

			// Ask if user wants to save this secret for future executions (if changed/new)
			if varDef.Secret && inputVal != "" && inputVal != storedVal {
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
