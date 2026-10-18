package exec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/leo-ramos/als/internal/store"
)

var (
	// Regex matching {{varName}} with optional outer quotes
	// Matches: "{{var}}", '{{var}}', or {{var}}
	varPattern = regexp.MustCompile(`(["'])?\{\{([a-zA-Z0-9_-]+)\}\}(["'])?`)
)

// CompiledCommand holds the sanitized command string and extracted variable references.
type CompiledCommand struct {
	OriginalCmd string
	// ShellCmd has {{var}} transformed into "$ALS_VAR_var" without duplicate quotes
	ShellCmd  string
	Variables []string
}

// CompileTemplate transforms template variables into safe shell environment references.
func CompileTemplate(cmd string, varDefs map[string]store.VarDef) (*CompiledCommand, error) {
	varsFound := make(map[string]bool)
	var varList []string

	// Replace all occurrences of quotes + {{var}} with "$ALS_VAR_var"
	shellCmd := varPattern.ReplaceAllStringFunc(cmd, func(match string) string {
		sub := varPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		varName := sub[2]
		if !varsFound[varName] {
			varsFound[varName] = true
			varList = append(varList, varName)
		}
		// Always safely expand as double-quoted shell environment variable: "$ALS_VAR_<NAME>"
		return fmt.Sprintf(`"$%s"`, EnvVarName(varName))
	})

	return &CompiledCommand{
		OriginalCmd: cmd,
		ShellCmd:    shellCmd,
		Variables:   varList,
	}, nil
}

// EnvVarName returns the environment variable name used for an alias parameter.
func EnvVarName(varName string) string {
	clean := strings.ToUpper(varName)
	clean = strings.ReplaceAll(clean, "-", "_")
	return "ALS_VAR_" + clean
}
