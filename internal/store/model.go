package store

// CommandsDoc represents the schema of commands.toml (Git-safe, public definitions).
type CommandsDoc struct {
	Version  int                `toml:"version"`
	Commands map[string]Command `toml:"commands"`
}

// Command defines a single saved executable alias.
type Command struct {
	Cmd     string            `toml:"cmd"`
	Desc    string            `toml:"desc,omitempty"`
	Tags    []string          `toml:"tags,omitempty"`
	Confirm bool              `toml:"confirm,omitempty"`
	Vars    map[string]VarDef `toml:"vars,omitempty"`
}

// VarDef defines a parameter template inside a command.
type VarDef struct {
	Secret       bool     `toml:"secret,omitempty"`
	Prompt       string   `toml:"prompt,omitempty"`
	Scope        string   `toml:"scope,omitempty"` // "command" (default) or "global"
	Choices      []string `toml:"choices,omitempty"`
	Default      string   `toml:"default,omitempty"`
	Positional   bool     `toml:"positional,omitempty"`    // Accepts direct CLI positional argument
	PromptAlways bool     `toml:"prompt_always,omitempty"` // Always opens interactive prompt if not provided via CLI
}

// SecretsDoc represents the schema of secrets.toml (Strictly local, 0600 perm, never committed).
type SecretsDoc struct {
	Global   map[string]string                       `toml:"global,omitempty"`
	Commands map[string]map[string]string           `toml:"commands,omitempty"`
	Choices  map[string]map[string]map[string]string `toml:"choices,omitempty"` // [choices.<cmd>.<var>.<choice_key>] -> secret_val
}

// NewCommandsDoc returns an initialized empty CommandsDoc with version 1.
func NewCommandsDoc() *CommandsDoc {
	return &CommandsDoc{
		Version:  1,
		Commands: make(map[string]Command),
	}
}

// NewSecretsDoc returns an initialized empty SecretsDoc.
func NewSecretsDoc() *SecretsDoc {
	return &SecretsDoc{
		Global:   make(map[string]string),
		Commands: make(map[string]map[string]string),
		Choices:  make(map[string]map[string]map[string]string),
	}
}
