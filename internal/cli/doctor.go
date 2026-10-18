package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Aliases: []string{"rebuild"},
	Short:   "Check system health, permissions, and rebuild/repair all shims",
	RunE: func(cmd *cobra.Command, args []string) error {
		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

		fmt.Println(titleStyle.Render("=== als Diagnostic & Repair (Doctor) ==="))
		fmt.Println()

		// 1. Check directories
		if err := cfg.EnsureDirs(); err != nil {
			fmt.Printf("%s Config/Bin directories: %v\n", warnStyle.Render("✘"), err)
		} else {
			fmt.Printf("%s Directories exist and have proper permissions\n", successStyle.Render("✔"))
		}

		// 2. Check secrets file permissions
		if info, err := os.Stat(cfg.SecretsPath()); err == nil {
			if info.Mode().Perm() == 0600 {
				fmt.Printf("%s secrets.toml has strict 0600 permissions\n", successStyle.Render("✔"))
			} else {
				fmt.Printf("%s secrets.toml permissions are %o (fixing to 0600...)\n", warnStyle.Render("⚠"), info.Mode().Perm())
				_ = os.Chmod(cfg.SecretsPath(), 0600)
			}
		}

		// 3. Check Git security
		gitDir := filepath.Join(cfg.ConfigDir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			// Check if secrets.toml is tracked
			chkCmd := exec.Command("git", "-C", cfg.ConfigDir, "ls-files", "--error-unmatch", "secrets.toml")
			if chkCmd.Run() == nil {
				fmt.Printf("%s CRITICAL: secrets.toml is tracked by Git in %s!\n", warnStyle.Render("✘"), cfg.ConfigDir)
				fmt.Println("  Run: git -C ~/.config/als rm --cached secrets.toml")
			} else {
				fmt.Printf("%s secrets.toml is NOT tracked by Git (safe)\n", successStyle.Render("✔"))
			}
		}

		// 4. Check PATH
		pathEnv := os.Getenv("PATH")
		inPath := false
		for _, p := range filepath.SplitList(pathEnv) {
			if cleanP, err := filepath.Abs(p); err == nil {
				if cleanBin, err := filepath.Abs(cfg.BinDir); err == nil && cleanP == cleanBin {
					inPath = true
					break
				}
			}
		}
		if inPath {
			fmt.Printf("%s %s is in PATH\n", successStyle.Render("✔"), cfg.BinDir)
		} else {
			fmt.Printf("%s %s is NOT in PATH (run 'als init')\n", warnStyle.Render("⚠"), cfg.BinDir)
		}

		// 5. Reconcile and rebuild shims
		doc, err := st.LoadCommands()
		if err != nil {
			return fmt.Errorf("failed to load commands: %w", err)
		}

		if err := shimMgr.Reconcile(doc.Commands); err != nil {
			fmt.Printf("%s Failed to reconcile shims: %v\n", warnStyle.Render("✘"), err)
		} else {
			fmt.Printf("%s Reconciled and verified all %d command shims\n", successStyle.Render("✔"), len(doc.Commands))
		}

		fmt.Println("\n" + strings.Repeat("-", 40))
		fmt.Println(successStyle.Render("Diagnostics complete."))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}
