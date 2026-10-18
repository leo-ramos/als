package shim

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leo-ramos/als/internal/config"
	"github.com/leo-ramos/als/internal/store"
)

var (
	ErrInvalidAliasName = errors.New("invalid alias name")
	ErrReservedName     = errors.New("alias name is a reserved shell builtin or core command")
	ErrShadowingSystem  = errors.New("alias name shadows an existing system binary in PATH")
)

// Reserved shell built-ins and dangerous system commands.
var reservedWords = map[string]bool{
	"cd": true, "alias": true, "unalias": true, "export": true, "eval": true,
	"exit": true, "exec": true, "source": true, "return": true, "set": true,
	"unset": true, "builtin": true, "command": true, "type": true, "hash": true,
	"sudo": true, "su": true, "als": true, ".als-bin": true,
}

type Manager struct {
	cfg *config.Config
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

// ValidateAlias checks if the requested alias name is safe to use.
func (m *Manager) ValidateAlias(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "/\\ \t\n\r:") || strings.HasPrefix(name, "-") {
		return fmt.Errorf("%w: name cannot be empty, start with '-' or contain slashes/whitespace", ErrInvalidAliasName)
	}

	if reservedWords[strings.ToLower(name)] {
		return fmt.Errorf("%w: '%s'", ErrReservedName, name)
	}

	// Check if this command already exists on PATH outside of our bin dir
	sysPath, err := exec.LookPath(name)
	if err == nil && sysPath != "" {
		// Clean and compare
		cleanSys, _ := filepath.Abs(sysPath)
		cleanBinDir, _ := filepath.Abs(m.cfg.BinDir)
		if !strings.HasPrefix(cleanSys, cleanBinDir) {
			return fmt.Errorf("%w: '%s' exists at %s", ErrShadowingSystem, name, sysPath)
		}
	}

	return nil
}

// EnsureMasterBin ensures ~/.local/share/als/bin/.als-bin points to the active als binary.
func (m *Manager) EnsureMasterBin(customTarget ...string) error {
	if err := m.cfg.EnsureDirs(); err != nil {
		return err
	}

	target := ""
	if len(customTarget) > 0 && customTarget[0] != "" {
		target = customTarget[0]
	} else {
		// First try LookPath("als") outside binDir
		if p, err := exec.LookPath("als"); err == nil && p != "" {
			cleanP, _ := filepath.Abs(p)
			cleanBin, _ := filepath.Abs(m.cfg.BinDir)
			if !strings.HasPrefix(cleanP, cleanBin) {
				target = p
			}
		}
		// Fallback to os.Executable()
		if target == "" {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to determine executable path: %w", err)
			}
			target = exe
		}
	}

	masterPath := m.cfg.MasterBinPath()

	// Check if existing symlink points to target already
	if currentTarget, err := os.Readlink(masterPath); err == nil && currentTarget == target {
		return nil
	}

	_ = os.Remove(masterPath)
	if err := os.Symlink(target, masterPath); err != nil {
		return fmt.Errorf("failed to create master symlink at %s -> %s: %w", masterPath, target, err)
	}

	return nil
}

// CreateShim creates a relative symlink from ~/.local/share/als/bin/<alias> pointing to .als-bin.
func (m *Manager) CreateShim(alias string) error {
	if err := m.ValidateAlias(alias); err != nil {
		return err
	}

	if err := m.EnsureMasterBin(); err != nil {
		return err
	}

	shimPath := m.cfg.ShimPath(alias)
	_ = os.Remove(shimPath)

	// Relative symlink: target is ".als-bin" in the same directory
	if err := os.Symlink(".als-bin", shimPath); err != nil {
		return fmt.Errorf("failed to create shim symlink %s: %w", shimPath, err)
	}

	return nil
}

// RemoveShim deletes the symlink for an alias.
func (m *Manager) RemoveShim(alias string) error {
	shimPath := m.cfg.ShimPath(alias)
	if err := os.Remove(shimPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove shim %s: %w", shimPath, err)
	}
	return nil
}

// Reconcile synchronizes all symlinks with the commands defined in store.
// Removes orphan symlinks and recreates missing ones.
func (m *Manager) Reconcile(commands map[string]store.Command) error {
	if err := m.EnsureMasterBin(); err != nil {
		return err
	}

	// 1. Ensure all defined commands have shims
	for alias := range commands {
		shimPath := m.cfg.ShimPath(alias)
		target, err := os.Readlink(shimPath)
		if err != nil || target != ".als-bin" {
			_ = os.Remove(shimPath)
			if err := os.Symlink(".als-bin", shimPath); err != nil {
				return fmt.Errorf("failed to create shim for %s: %w", alias, err)
			}
		}
	}

	// 2. Remove orphan symlinks in bin dir
	entries, err := os.ReadDir(m.cfg.BinDir)
	if err != nil {
		return fmt.Errorf("failed to read bin dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == ".als-bin" || strings.HasPrefix(name, ".") {
			continue
		}
		if _, exists := commands[name]; !exists {
			_ = os.Remove(filepath.Join(m.cfg.BinDir, name))
		}
	}

	return nil
}
