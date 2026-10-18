package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/leo-ramos/als/internal/config"
	"github.com/pelletier/go-toml/v2"
)

var (
	ErrCommandNotFound = errors.New("command not found")
	ErrCommandExists   = errors.New("command already exists")
)

type Store struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Store {
	return &Store{cfg: cfg}
}

// WithLock acquires an exclusive POSIX advisory lock on .lock file during the execution of fn.
func (s *Store) WithLock(fn func() error) error {
	if err := s.cfg.EnsureDirs(); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(s.cfg.LockPath(), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file %s: %w", s.cfg.LockPath(), err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire flock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return fn()
}

// LoadCommands reads commands.toml. If the file does not exist, an empty doc is returned.
func (s *Store) LoadCommands() (*CommandsDoc, error) {
	path := s.cfg.CommandsPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewCommandsDoc(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var doc CommandsDoc
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse TOML in %s: %w", path, err)
	}
	if doc.Commands == nil {
		doc.Commands = make(map[string]Command)
	}
	return &doc, nil
}

// SaveCommands writes commands.toml atomically.
func (s *Store) SaveCommands(doc *CommandsDoc) error {
	return s.WithLock(func() error {
		return s.atomicWriteTOML(s.cfg.CommandsPath(), doc, 0644)
	})
}

// LoadSecrets reads secrets.toml. If the file does not exist, an empty doc is returned.
func (s *Store) LoadSecrets() (*SecretsDoc, error) {
	path := s.cfg.SecretsPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewSecretsDoc(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var doc SecretsDoc
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse TOML in %s: %w", path, err)
	}
	if doc.Global == nil {
		doc.Global = make(map[string]string)
	}
	if doc.Commands == nil {
		doc.Commands = make(map[string]map[string]string)
	}
	if doc.Choices == nil {
		doc.Choices = make(map[string]map[string]map[string]string)
	}
	return &doc, nil
}

// SaveSecrets writes secrets.toml atomically with strict 0600 permissions.
func (s *Store) SaveSecrets(doc *SecretsDoc) error {
	return s.WithLock(func() error {
		return s.atomicWriteTOML(s.cfg.SecretsPath(), doc, 0600)
	})
}

// GetCommand retrieves a command by its alias name.
func (s *Store) GetCommand(name string) (*Command, error) {
	doc, err := s.LoadCommands()
	if err != nil {
		return nil, err
	}
	cmd, ok := doc.Commands[name]
	if !ok {
		return nil, ErrCommandNotFound
	}
	return &cmd, nil
}

// SaveCommand adds or updates a command definition.
func (s *Store) SaveCommand(name string, cmd Command) error {
	return s.WithLock(func() error {
		doc, err := s.LoadCommands()
		if err != nil {
			return err
		}
		doc.Commands[name] = cmd
		return s.atomicWriteTOML(s.cfg.CommandsPath(), doc, 0644)
	})
}

// DeleteCommand removes a command and cleanups related secrets.
func (s *Store) DeleteCommand(name string) error {
	return s.WithLock(func() error {
		doc, err := s.LoadCommands()
		if err != nil {
			return err
		}
		if _, ok := doc.Commands[name]; !ok {
			return ErrCommandNotFound
		}
		delete(doc.Commands, name)
		if err := s.atomicWriteTOML(s.cfg.CommandsPath(), doc, 0644); err != nil {
			return err
		}

		// Also cleanup associated secrets if any
		secDoc, err := s.LoadSecrets()
		if err == nil {
			delete(secDoc.Commands, name)
			delete(secDoc.Choices, name)
			_ = s.atomicWriteTOML(s.cfg.SecretsPath(), secDoc, 0600)
		}
		return nil
	})
}

// SetSecret saves a secret value for a command or globally.
func (s *Store) SetSecret(alias, varName, value string, isGlobal bool) error {
	return s.WithLock(func() error {
		secDoc, err := s.LoadSecrets()
		if err != nil {
			return err
		}

		if isGlobal {
			secDoc.Global[varName] = value
		} else {
			if secDoc.Commands[alias] == nil {
				secDoc.Commands[alias] = make(map[string]string)
			}
			secDoc.Commands[alias][varName] = value
		}

		return s.atomicWriteTOML(s.cfg.SecretsPath(), secDoc, 0600)
	})
}

// SetChoiceSecret saves a secret mapped to a choice key for a variable in an alias.
func (s *Store) SetChoiceSecret(alias, varName, choiceKey, value string) error {
	return s.WithLock(func() error {
		secDoc, err := s.LoadSecrets()
		if err != nil {
			return err
		}

		if secDoc.Choices[alias] == nil {
			secDoc.Choices[alias] = make(map[string]map[string]string)
		}
		if secDoc.Choices[alias][varName] == nil {
			secDoc.Choices[alias][varName] = make(map[string]string)
		}
		secDoc.Choices[alias][varName][choiceKey] = value

		return s.atomicWriteTOML(s.cfg.SecretsPath(), secDoc, 0600)
	})
}

// ResolveSecret searches secrets for a variable value.
// Precedence: Command-level choices -> Command-level vars -> Global vars.
func (s *Store) ResolveSecret(secDoc *SecretsDoc, alias, varName, choiceKey string) (string, bool) {
	if secDoc == nil {
		return "", false
	}

	// 1. Check choice secret if choiceKey provided
	if choiceKey != "" && secDoc.Choices != nil {
		if cmdChoices, ok := secDoc.Choices[alias]; ok {
			if varChoices, ok := cmdChoices[varName]; ok {
				if val, ok := varChoices[choiceKey]; ok && val != "" {
					return val, true
				}
			}
		}
	}

	// 2. Check command-level secret
	if secDoc.Commands != nil {
		if cmdVars, ok := secDoc.Commands[alias]; ok {
			if val, ok := cmdVars[varName]; ok && val != "" {
				return val, true
			}
		}
	}

	// 3. Check global secret
	if secDoc.Global != nil {
		if val, ok := secDoc.Global[varName]; ok && val != "" {
			return val, true
		}
	}

	return "", false
}

func (s *Store) atomicWriteTOML(targetPath string, v any, perm os.FileMode) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to ensure dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "tmp-*.toml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to chmod temp file: %w", err)
	}

	data, err := toml.Marshal(v)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("failed to marshal TOML: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to fsync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", targetPath, err)
	}

	return nil
}
