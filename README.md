# kubegonfig

Secure kubeconfig profile manager. Stores profiles encrypted with GPG, decrypts to temporary files on demand, and exports `KUBECONFIG` via eval-friendly shell output.

## Features

- GPG encryption at rest using your existing keyring
- XDG Base Directory compliant storage
- Atomic writes with file locking
- Signal-safe temp file cleanup (SIGINT/SIGTERM)
- Shell integration for bash, zsh, and fish
- Profile name validation against path traversal and injection
- Process isolation via `exec` with automatic cleanup

## Install

### From release

Download the binary for your platform from the [releases page](../../releases) and place it in your `PATH`.

### From source

```bash
go install kubegonfig@latest
```

Or build locally:

```bash
git clone https://github.com/YOUR_USER/kubegonfig.git
cd kubegonfig
go build -o kubegonfig .
```

## Quick start

```bash
# Initialize with your GPG key (interactive selection)
kubegonfig init

# Or specify directly
kubegonfig init --recipient your-key-id

# Import an existing kubeconfig
kubegonfig create production --from ~/.kube/config

# Activate a profile (must be eval'd to set KUBECONFIG)
eval "$(kubegonfig env production)"

# Run a one-off command against a profile
kubegonfig exec staging -- kubectl get pods
```

## Shell integration

Source the appropriate wrapper in your shell rc file. This lets `use` and `env` subcommands export `KUBECONFIG` into your current shell session.

### Bash

```bash
# ~/.bashrc
source /path/to/kubegonfig/shell/kubegonfig.bash
```

### Zsh

```zsh
# ~/.zshrc
source /path/to/kubegonfig/shell/kubegonfig.zsh
```

### Fish

```fish
# ~/.config/fish/config.fish
source /path/to/kubegonfig/shell/kubegonfig.fish
```

Without the wrapper, use `eval` directly:

```bash
eval "$(kubegonfig env my-cluster)"
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Configure GPG recipient (interactive key selection) |
| `create <name>` | Create a profile from `--from` file or `$EDITOR` |
| `import <name> --from <path>` | Import a kubeconfig file as a profile |
| `list` | List all profiles (`*` marks active) |
| `use <name>` | Decrypt and activate a profile |
| `env <name>` | Print shell export for a profile (`--shell posix\|fish`) |
| `exec <name> -- cmd` | Run a command with `KUBECONFIG` set |
| `edit <name>` | Decrypt, edit in `$EDITOR`, re-encrypt |
| `current` | Print the active profile name |
| `rename <old> <new>` | Rename a profile |
| `delete <name>` | Delete a profile (`--force` to skip prompt) |
| `cleanup` | Remove stale temp files from runtime dir |

## Storage layout

```
$XDG_DATA_HOME/kubegonfig/
  profiles/
    <name>.yaml.gpg      # encrypted kubeconfig

$XDG_CONFIG_HOME/kubegonfig/
  config.yaml             # GPG recipient, settings

$XDG_STATE_HOME/kubegonfig/
  current                 # active profile name

$XDG_RUNTIME_DIR/kubegonfig/
  <name>.yaml             # decrypted temp file (0600)
```

## Security

- Kubeconfig is never stored in plaintext permanently
- Temp files are `0600` in `0700` directories with ownership checks
- Profile names are validated against path traversal and shell injection
- Shell output is properly escaped
- No secrets are logged or printed to stdout/stderr
- Atomic writes via temp file + rename + fsync

## License

MIT
