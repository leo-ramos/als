package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/leo-ramos/als/internal/sync"
	"github.com/spf13/cobra"
)

var syncRemote string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize commands with a Git repository",
	Long: `Sync commits your local commands.toml, pulls remote changes via rebase,
pushes to your Git repository, and automatically reconciles your local shims.
Strict security checks ensure secrets.toml is NEVER pushed to Git.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		syncer := sync.NewSyncer(cfg, st, shimMgr)

		if syncRemote != "" {
			if err := syncer.InitRepo(syncRemote); err != nil {
				return fmt.Errorf("failed to initialize git remote: %w", err)
			}
			fmt.Printf("Git remote origin configured to: %s\n", syncRemote)
		}

		fmt.Println("Synchronizing commands with Git...")
		if err := syncer.Sync(context.Background()); err != nil {
			return err
		}

		successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
		fmt.Println(successStyle.Render("✔ Commands synchronized and local shims updated successfully!"))
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncRemote, "remote", "", "Set or update git remote repository URL (e.g. git@github.com:user/my-als-dotfiles.git)")
	RootCmd.AddCommand(syncCmd)
}
