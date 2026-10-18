package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize als directories and configure PATH in your shell",
	RunE: func(cmd *cobra.Command, args []string) error {
		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		warnStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

		// 1. Ensure directories exist
		if err := cfg.EnsureDirs(); err != nil {
			return fmt.Errorf("failed to ensure directories: %w", err)
		}

		// 2. Ensure .als-bin points to the als binary
		if err := shimMgr.EnsureMasterBin(); err != nil {
			return fmt.Errorf("failed to link master binary: %w", err)
		}

		fmt.Println(successStyle.Render("✔ Directories and master shim created successfully!"))
		fmt.Printf("  Config dir: %s\n", cfg.ConfigDir)
		fmt.Printf("  Bin dir:    %s\n\n", cfg.BinDir)

		// 3. Check if BinDir is in PATH
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
			fmt.Println(successStyle.Render("✔ Your PATH already includes:") + " " + cfg.BinDir)
			return nil
		}

		// 4. Offer shell configuration snippet
		home, _ := os.UserHomeDir()
		shell := os.Getenv("SHELL")
		rcFile := filepath.Join(home, ".zshrc")
		if strings.Contains(shell, "bash") {
			rcFile = filepath.Join(home, ".bashrc")
		}

		exportLine := fmt.Sprintf("\n# als command manager\nexport PATH=\"%s:$PATH\"\n", cfg.BinDir)

		fmt.Println(warnStyle.Render("⚠ Action required:") + " Add the als bin directory to your PATH.")
		fmt.Println("To do this automatically, you can run:")
		fmt.Println(infoStyle.Render(fmt.Sprintf("  echo '%s' >> %s", strings.TrimSpace(exportLine), rcFile)))
		fmt.Printf("  source %s\n", rcFile)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}
