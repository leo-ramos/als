package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultConfigSubdir = "als"
	DefaultBinSubdir    = "als/bin"
	GitignoreContent    = `# Secret data - NEVER commit to git
secrets.toml
.lock
*.tmp
*.bak
`
)

// Config holds the configured directory paths.
type Config struct {
	ConfigDir string
	BinDir    string
}

// Load returns the active configuration, respecting environment overrides.
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := os.Getenv("ALS_CONFIG_DIR")
	if configDir == "" {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			configDir = filepath.Join(xdgConfig, DefaultConfigSubdir)
		} else {
			configDir = filepath.Join(home, ".config", DefaultConfigSubdir)
		}
	}

	binDir := os.Getenv("ALS_BIN_DIR")
	if binDir == "" {
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData != "" {
			binDir = filepath.Join(xdgData, DefaultBinSubdir)
		} else {
			binDir = filepath.Join(home, ".local", "share", DefaultBinSubdir)
		}
	}

	return &Config{
		ConfigDir: configDir,
		BinDir:    binDir,
	}, nil
}

// EnsureDirs creates configuration and bin directories with secure permissions
// and creates the default .gitignore in the config directory.
func (c *Config) EnsureDirs() error {
	// 0700 for config directory (sensitive data like secrets.toml lives here)
	if err := os.MkdirAll(c.ConfigDir, 0700); err != nil {
		return fmt.Errorf("failed to create config dir %s: %w", c.ConfigDir, err)
	}

	// 0755 for bin directory so it can be added to PATH
	if err := os.MkdirAll(c.BinDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin dir %s: %w", c.BinDir, err)
	}

	gitignorePath := c.GitignorePath()
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte(GitignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to write .gitignore: %w", err)
		}
	}

	return nil
}

func (c *Config) CommandsPath() string {
	return filepath.Join(c.ConfigDir, "commands.toml")
}

func (c *Config) SecretsPath() string {
	return filepath.Join(c.ConfigDir, "secrets.toml")
}

func (c *Config) LockPath() string {
	return filepath.Join(c.ConfigDir, ".lock")
}

func (c *Config) GitignorePath() string {
	return filepath.Join(c.ConfigDir, ".gitignore")
}

func (c *Config) MasterBinPath() string {
	return filepath.Join(c.BinDir, ".als-bin")
}

func (c *Config) ShimPath(alias string) string {
	return filepath.Join(c.BinDir, alias)
}
