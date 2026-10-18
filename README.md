# `als` — Parameterized Terminal Command & Alias Manager

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)

[**English**](README.md) · [Português (BR)](README.pt-BR.md) · [Why `als`?](#why-als) · [Quick Start](#quick-start) · [Security & Git Sync](#security) · [Team Sharing](#team-sharing) · [License](#license)

> Turn long, repetitive commands into direct executables in your terminal (`$PATH`) with dynamic parameters (`{{var}}`), strict secret isolation, and Git synchronization.

---

<a id="why-als"></a>
## 💡 Why `als` Exists

The native shell `alias` command has severe day-to-day limitations:
1. **Requires manual editing:** You must edit `.zshrc`/`.bashrc` files and reload every open terminal tab (`source`).
2. **No dynamic parameters:** A standard alias cannot accept variables in the middle of a command (e.g., a changing IP address or a specific SSH key).
3. **Breaks with relative paths:** Commands referencing local files (e.g., `ssh -i "key.pem"`) fail when called from any other directory.
4. **Credential leakage risk:** Saving server IPs, private keys, or tokens inside `.zshrc` leaks sensitive data when versioning dotfiles on GitHub.

**`als`** solves this by creating **native executable shims in your `$PATH`**: any registered command becomes immediately available across all terminal tabs, with `< 5ms` startup overhead, transparent TTY passthrough (SSH, Docker, Vim), and physical separation between the command template (public) and sensitive values (local).

---

<a id="quick-start"></a>
## 🛠️ Complete Step-by-Step Guide

### 1. Installation

From the cloned project directory, build and copy the executable to your system:

```bash
# 1. Build the binary
go build -o als ./cmd/als

# 2. Move to the system executables directory
sudo cp ./als /usr/local/bin/

# 3. Verify installation
als --help
```

---

### 2. Initial Environment Setup

Run the initialization command once:

```bash
als init
```

This command creates the secure directories and the master `als` shim.
* If the shims directory (`~/.local/share/als/bin`) is not yet in your `$PATH`, `als init` will print the exact line to add to your `~/.zshrc` or `~/.bashrc`:
  ```bash
  echo 'export PATH="$HOME/.local/share/als/bin:$PATH"' >> ~/.zshrc
  source ~/.zshrc
  ```

---

### 3. Register a Parameterized Command

To register a command containing dynamic variables or sensitive data, use the `{{variable_name}}` syntax.

For example, to create the **`connectServer`** command:

```bash
als add connectServer 'ssh -i "{{key}}" ubuntu@{{server_ip}}' --desc "SSH access to main server"
```

> **Tip:** `als` includes a built-in static analyzer (Linter). It automatically detects that `key` and `server_ip` are sensitive credentials and isolates them outside the Git-synced file.

You can also use interactive mode simply by running:
```bash
als add
```

---

### 4. Configure Secrets (`key` and `server_ip`)

`als` never saves sensitive data (such as server IPs and private key paths) in the public commands file (`commands.toml`). It stores those values in `~/.config/als/secrets.toml` with strict `0600` permissions (read and write only by your user).

You can define values in **two ways**:

#### Option A: Interactive (On first run)
The first time you execute the command, the terminal detects missing variables and opens a secure prompt:
1. Enter the key path (e.g., `~/.ssh/prod-key.pem`).
2. Enter the server IP (e.g., `203.0.113.10`).
3. The terminal will ask whether you want to save these values locally in `secrets.toml` so you don't have to type them again.

#### Option B: Direct (Editing local `secrets.toml`)
Open `~/.config/als/secrets.toml` (created automatically by `als`):

```toml
# ~/.config/als/secrets.toml
# Local machine file — NEVER committed to Git (permission 0600)

[commands.connectServer]
key = "/Users/your-username/.ssh/prod-key.pem"
server_ip = "203.0.113.10"
```

*If you use the same SSH key across multiple commands, you can configure it in the global scope:*
```toml
[global]
key = "/Users/your-username/.ssh/default-key.pem"
```

---

### 5. Run the Command in Your Terminal

Once registered, **you do not need to type `als run`**. Just type the command name directly in any terminal or tab:

```bash
connectServer
```

`als` resolves `$key` and `$server_ip` in the environment within milliseconds and starts the SSH session directly.

#### Passing Extra Flags and Arguments
Any additional arguments passed to the alias are forwarded intact to the underlying command via `"$@"`:

```bash
# Adds verbose mode (-v) and custom port (-p 2222) to SSH
connectServer -v -p 2222
```

#### Quick Parameter Override
You can pass parameter values in two ways:

1. **Direct Positional (Shortest & Fastest):**
   Pass the value directly on the command line without extra flags:
   ```bash
   buildCdaForcaAdr https://tabletserveroci.casadoadubo.com.br:8601
   ```
2. **Explicit Named (`@var=value`):**
   Best for commands with multiple parameters when you want to target a specific variable:
   ```bash
   connectServer @server_ip=203.0.113.25
   ```

---

### 6. Safe Inspection & Testing (Dry-Run & Info)

To inspect what a command does before executing it:

```bash
# Displays template, registered variables, and description
connectServer --als-info

# Simulates full execution without spawning the real process
connectServer --als-dry-run
```

---

<a id="security"></a>
## 🔒 Security Architecture & Git Sync

`als` enforces a strict boundary between command templates and credentials:

```text
~/.config/als/                  (drwx------ 0700)
├── .git/                       (Dotfiles Git repository)
├── .gitignore                  (Blocks secrets.toml, .lock, *.tmp)
├── .lock                       (POSIX flock for concurrency safety)
├── commands.toml               (Public templates — SAFE for Git)
└── secrets.toml                (Local values — NEVER sent to Git)

~/.local/share/als/bin/         (In your $PATH)
├── .als-bin                    (Master symlink -> /usr/local/bin/als)
└── connectServer               (Relative symlink -> .als-bin)
```

### Why is it safe to push to Git?
1. **Protection against `ps aux`:** When executing a command, secrets are injected as environment variables (`$ALS_VAR_<NAME>`), preventing passwords or IPs from being exposed as plain text in the OS process list.
2. **Git Sync Safety Lock:** `als sync` includes an automatic check that aborts synchronization if it detects that `secrets.toml` was accidentally tracked by Git.
3. **Atomic Writes:** Concurrent modifications across multiple terminal tabs use advisory `flock` locking to prevent file corruption.

---

<a id="team-sharing"></a>
### 👥 Team Sharing: Commands vs. Credentials

One of the strongest use cases for `als` is standardizing operational workflows across IT and engineering teams. Understand the practical separation across 3 layers:

#### 1. The Open-Source Software (This Public Repository)
* Contains only the `als` engine and binary.
* **None of your commands or configurations are ever sent here.** Your saved commands live exclusively inside `~/.config/als/` on your own machine.

#### 2. Your Team's Own Repository (`als sync`)
* An IT lead or engineering team can create their own Git repository (usually private, e.g., `git@github.com:your-company/it-commands.git`).
* When configuring `als sync --remote <url>`, only the **command templates** (`commands.toml`) are synchronized.
* Every team member who runs `als sync` immediately gets the exact same commands in their terminal (e.g., `connectProd`, `backupDb`, `clusterTunnels`), ensuring instant standardization.

#### 3. Security Boundary: Why Keys NEVER Go to Git
* `als` **deliberately blocks** `secrets.toml` from being pushed to Git, even in a private company repository.
* **Reason:** Versioning private SSH keys (`.key`, `.pem`) or tokens in Git repositories is a severe compliance failure (any leaked clone or commit history exposes your infrastructure). Additionally, file paths differ across machines (e.g., `/Users/lead/.ssh/id_rsa` on macOS vs `/home/dev/.ssh/id_rsa` on Linux).
* **How it works in practice:** The team shares the **command structure** via Git (`ssh -i "{{key}}" ubuntu@{{server_ip}}`). Each developer keeps their own key path and permissions configured in their local `secrets.toml` (or answers the interactive prompt on first run).

---

## 📋 Utility Commands

```bash
# List all registered aliases
als list

# Edit an existing alias (or pick interactively from a list if name is omitted)
als edit buildCdaForcaAdr

# View details and shim location of an alias
als info connectServer

# Remove an alias and clean up its system symlink
als rm connectServer

# Run system health diagnostics and rebuild shims
als doctor

# Synchronize public commands with your dotfiles repository
als sync
```

---

<a id="license"></a>
## 📄 License

Distributed under the **Apache License 2.0**. See [LICENSE](LICENSE) for more information.
