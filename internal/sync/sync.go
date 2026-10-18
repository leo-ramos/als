package sync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/leo-ramos/als/internal/config"
	"github.com/leo-ramos/als/internal/shim"
	"github.com/leo-ramos/als/internal/store"
)

var (
	ErrSecretsTracked = errors.New("security check failed: secrets.toml is tracked in Git")
	ErrGitNotInstalled = errors.New("git binary not found in PATH")
)

type Syncer struct {
	cfg     *config.Config
	st      *store.Store
	shimMgr *shim.Manager
	Timeout time.Duration
}

func NewSyncer(cfg *config.Config, st *store.Store, shimMgr *shim.Manager) *Syncer {
	return &Syncer{
		cfg:     cfg,
		st:      st,
		shimMgr: shimMgr,
		Timeout: 20 * time.Second,
	}
}

// InitRepo initializes git in the config directory and configures the remote repository.
func (s *Syncer) InitRepo(remoteURL string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrGitNotInstalled
	}

	if err := s.cfg.EnsureDirs(); err != nil {
		return err
	}

	gitDir := filepath.Join(s.cfg.ConfigDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if out, err := s.runGit(context.Background(), "init", "-b", "main"); err != nil {
			return fmt.Errorf("git init failed: %s (%w)", out, err)
		}
	}

	if remoteURL != "" {
		// Remove existing origin if any
		_, _ = s.runGit(context.Background(), "remote", "remove", "origin")
		if out, err := s.runGit(context.Background(), "remote", "add", "origin", remoteURL); err != nil {
			return fmt.Errorf("git remote add failed: %s (%w)", out, err)
		}
	}

	return nil
}

// Sync pulls changes via rebase, commits local commands.toml, pushes, and reconciles shims.
func (s *Syncer) Sync(parentCtx context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return ErrGitNotInstalled
	}

	ctx, cancel := context.WithTimeout(parentCtx, s.Timeout)
	defer cancel()

	// 1. Mandatory Security Check: Verify secrets.toml is NOT tracked
	if s.isSecretsTracked(ctx) {
		return fmt.Errorf("%w: remove it with 'git -C %s rm --cached secrets.toml'", ErrSecretsTracked, s.cfg.ConfigDir)
	}

	// 2. Stage commands.toml and .gitignore
	if _, err := s.runGit(ctx, "add", "commands.toml", ".gitignore"); err != nil {
		return fmt.Errorf("failed to stage files: %w", err)
	}

	// 3. Commit local changes if working tree has staged changes
	statusOut, _ := s.runGit(ctx, "status", "--porcelain")
	if strings.Contains(statusOut, "commands.toml") || strings.Contains(statusOut, ".gitignore") {
		msg := fmt.Sprintf("als sync: update commands [%s]", time.Now().Format("2006-01-02 15:04:05"))
		if _, err := s.runGit(ctx, "commit", "-m", msg); err != nil {
			return fmt.Errorf("git commit failed: %w", err)
		}
	}

	// 4. Check if remote origin is configured
	remotes, _ := s.runGit(ctx, "remote")
	if strings.Contains(remotes, "origin") {
		// Pull with rebase
		if out, err := s.runGit(ctx, "pull", "--rebase", "origin", "main"); err != nil {
			return fmt.Errorf("git pull --rebase failed: %s (%w)", out, err)
		}

		// Push to remote
		if out, err := s.runGit(ctx, "push", "origin", "main"); err != nil {
			return fmt.Errorf("git push failed: %s (%w)", out, err)
		}
	}

	// 5. Reconcile shims so any new commands from remote exist locally
	doc, err := s.st.LoadCommands()
	if err != nil {
		return fmt.Errorf("failed to reload commands after sync: %w", err)
	}

	if err := s.shimMgr.Reconcile(doc.Commands); err != nil {
		return fmt.Errorf("failed to reconcile shims after sync: %w", err)
	}

	return nil
}

func (s *Syncer) isSecretsTracked(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", s.cfg.ConfigDir, "ls-files", "--error-unmatch", "secrets.toml")
	return cmd.Run() == nil
}

func (s *Syncer) runGit(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{"-C", s.cfg.ConfigDir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
