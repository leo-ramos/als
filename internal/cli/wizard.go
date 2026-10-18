package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/leo-ramos/als/internal/store"
)

// configureVariablesWizard interactively asks the user how each {{var}} in rawCmd should behave.
func configureVariablesWizard(
	rawCmd string,
	aliasName string,
	existingVars map[string]store.VarDef,
	extractedValues map[string]string,
) (map[string]store.VarDef, error) {
	varDefs := make(map[string]store.VarDef)
	matches := varRegex.FindAllStringSubmatch(rawCmd, -1)
	if len(matches) == 0 {
		return varDefs, nil
	}

	infoStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	fmt.Println("\n" + infoStyle.Render("⚙ Parameter Configuration:"))

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		vName := match[1]
		if _, done := varDefs[vName]; done {
			continue
		}

		var existing store.VarDef
		if existingVars != nil {
			existing = existingVars[vName]
		}
		extracted := extractedValues[vName]

		// Determine initial mode
		currentMode := "default"
		if existing.PromptAlways {
			currentMode = "always_ask"
		} else if len(existing.Choices) > 0 {
			currentMode = "choices"
		}

		modeOptions := []huh.Option[string]{
			huh.NewOption("1. Saved default value (runs directly; accepts override: alias <val>)", "default"),
			huh.NewOption("2. Always ask on run (prompts in terminal; accepts override: alias <val>)", "always_ask"),
			huh.NewOption("3. Predefined choices (selectable options, e.g. prod, homol)", "choices"),
		}

		var selectedMode string = currentMode
		modeForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Behavior for parameter '{{%s}}'", vName)).
					Options(modeOptions...).
					Value(&selectedMode),
			),
		)
		if err := runFormWithEscConfirm(modeForm); err != nil {
			return nil, err
		}

		newDef := store.VarDef{
			Prompt:     fmt.Sprintf("Enter %s", vName),
			Positional: true,
		}

		switch selectedMode {
		case "default":
			defaultVal := existing.Default
			if defaultVal == "" && extracted != "" {
				defaultVal = extracted
			}
			// Check existing secret if any
			secDoc, _ := st.LoadSecrets()
			if secVal, ok := st.ResolveSecret(secDoc, aliasName, vName, ""); ok && secVal != "" {
				defaultVal = secVal
			}

			isSensitive := existing.Secret || strings.Contains(strings.ToLower(vName), "key") ||
				strings.Contains(strings.ToLower(vName), "ip") ||
				strings.Contains(strings.ToLower(vName), "token") ||
				strings.Contains(strings.ToLower(vName), "secret") ||
				strings.Contains(strings.ToLower(vName), "pass")

			valForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("Default value for '{{%s}}'", vName)).
						Value(&defaultVal),
					huh.NewConfirm().
						Title(fmt.Sprintf("Is '{{%s}}' a secret/credential? (Keeps out of Git)", vName)).
						Value(&isSensitive),
				),
			)
			if err := runFormWithEscConfirm(valForm); err != nil {
				return nil, err
			}

			newDef.Secret = isSensitive
			if isSensitive {
				newDef.Default = ""
				if defaultVal != "" {
					_ = st.SetSecret(aliasName, vName, defaultVal, false)
				}
			} else {
				newDef.Default = defaultVal
			}

		case "always_ask":
			newDef.PromptAlways = true
			newDef.Positional = true
			suggestedVal := existing.Default
			if suggestedVal == "" && extracted != "" {
				suggestedVal = extracted
			}
			isSensitive := existing.Secret

			valForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("Suggested initial value for '{{%s}}' (optional)", vName)).
						Value(&suggestedVal),
					huh.NewConfirm().
						Title("Mask input as password/secret?").
						Value(&isSensitive),
				),
			)
			if err := runFormWithEscConfirm(valForm); err != nil {
				return nil, err
			}
			newDef.Secret = isSensitive
			if isSensitive {
				newDef.Default = ""
				if suggestedVal != "" {
					_ = st.SetSecret(aliasName, vName, suggestedVal, false)
				}
			} else {
				newDef.Default = suggestedVal
			}

		case "choices":
			choicesStr := strings.Join(existing.Choices, ", ")
			choicesForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("Comma-separated choices for '{{%s}}'", vName)).
						Placeholder("e.g. prod, staging, dev").
						Value(&choicesStr),
				),
			)
			if err := runFormWithEscConfirm(choicesForm); err != nil {
				return nil, err
			}
			var cleanChoices []string
			for _, ch := range strings.Split(choicesStr, ",") {
				if c := strings.TrimSpace(ch); c != "" {
					cleanChoices = append(cleanChoices, c)
				}
			}
			newDef.Choices = cleanChoices
			newDef.Positional = true
		}

		varDefs[vName] = newDef
	}

	return varDefs, nil
}
