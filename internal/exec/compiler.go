package exec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/leo-ramos/als/internal/store"
)

var (
	varRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9_-]+)\}\}`)
)

// CompiledCommand holds the sanitized command string and extracted variable references.
type CompiledCommand struct {
	OriginalCmd string
	// ShellCmd has {{var}} transformed into safe shell environment variable expansions
	ShellCmd  string
	Variables []string
}

// CompileTemplate transforms template variables into safe shell environment references,
// properly handling unquoted, double-quoted, single-quoted, and prefix contexts (e.g. URL={{url}}).
func CompileTemplate(cmd string, varDefs map[string]store.VarDef) (*CompiledCommand, error) {
	varsFound := make(map[string]bool)
	var varList []string

	var sb strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	n := len(cmd)

	for i := 0; i < n; {
		c := cmd[i]

		// Handle escape sequences inside double quotes
		if c == '\\' && inDoubleQuote && i+1 < n {
			sb.WriteByte(c)
			sb.WriteByte(cmd[i+1])
			i += 2
			continue
		}

		// Handle quote state transitions
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			sb.WriteByte(c)
			i++
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			sb.WriteByte(c)
			i++
			continue
		}

		// Check for {{varName}}
		if strings.HasPrefix(cmd[i:], "{{") {
			loc := varRegex.FindStringSubmatchIndex(cmd[i:])
			if len(loc) >= 4 && loc[0] == 0 {
				varStart := i + loc[2]
				varEnd := i + loc[3]
				varName := cmd[varStart:varEnd]
				fullMatchEnd := i + loc[1]

				if !varsFound[varName] {
					varsFound[varName] = true
					varList = append(varList, varName)
				}

				envRef := EnvVarName(varName)

				// Check if the placeholder was immediately wrapped by quotes:
				// e.g. "{{var}}" (where the opening quote was just added to sb, and next char is ")
				sbStr := sb.String()
				if !inSingleQuote && len(sbStr) > 0 && sbStr[len(sbStr)-1] == '"' && fullMatchEnd < n && cmd[fullMatchEnd] == '"' {
					// We are inside double quotes that started immediately before {{var}}
					// Strip the preceding '"' from sb and skip the trailing '"'
					sbTrunc := sbStr[:len(sbStr)-1]
					sb.Reset()
					sb.WriteString(sbTrunc)
					sb.WriteString(fmt.Sprintf(`"$%s"`, envRef))
					inDoubleQuote = false
					i = fullMatchEnd + 1
					continue
				}

				if !inDoubleQuote && len(sbStr) > 0 && sbStr[len(sbStr)-1] == '\'' && fullMatchEnd < n && cmd[fullMatchEnd] == '\'' {
					// Immediately wrapped in '{{var}}'
					sbTrunc := sbStr[:len(sbStr)-1]
					sb.Reset()
					sb.WriteString(sbTrunc)
					sb.WriteString(fmt.Sprintf(`"$%s"`, envRef))
					inSingleQuote = false
					i = fullMatchEnd + 1
					continue
				}

				// Contextual replacement based on current quote state:
				if inSingleQuote {
					// In POSIX shell, single quotes don't expand variables. Break out, expand, and re-enter:
					// '...'"$ALS_VAR_VAR"'...'
					sb.WriteString(fmt.Sprintf(`'"$%s"'`, envRef))
				} else if inDoubleQuote {
					// Inside existing double quotes, $ALS_VAR_VAR expands safely without word splitting
					sb.WriteString(fmt.Sprintf(`$%s`, envRef))
				} else {
					// Outside quotes: wrap in double quotes to prevent word splitting
					// e.g. URL={{url}} -> URL="$ALS_VAR_URL"
					sb.WriteString(fmt.Sprintf(`"$%s"`, envRef))
				}

				i = fullMatchEnd
				continue
			}
		}

		sb.WriteByte(c)
		i++
	}

	return &CompiledCommand{
		OriginalCmd: cmd,
		ShellCmd:    sb.String(),
		Variables:   varList,
	}, nil
}

// EnvVarName returns the environment variable name used for an alias parameter.
func EnvVarName(varName string) string {
	clean := strings.ToUpper(varName)
	clean = strings.ReplaceAll(clean, "-", "_")
	return "ALS_VAR_" + clean
}
