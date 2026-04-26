# kubegonfig

Secure kubeconfig profile manager. Stores profiles encrypted with GPG, decrypts to temporary files on demand, and exports `KUBECONFIG` via eval-friendly shell output.

## Features

- GPG encryption at rest using your existing keyring
- XDG Base Directory compliant storage
- Atomic writes with file locking and fsync
- Signal cleanup for temp files created by the running process
- Shell integration for bash, zsh, and fish
- Profile name validation against path traversal and injection
- Process isolation via `exec` with an unlinked kubeconfig file descriptor

## Install

### From release

Download the binary for your platform from the [releases page](../../releases) and place it in your `PATH`. If you want shell wrapper functions, also copy the matching file from `shell/` in this repository.

### From source

```bash
git clone <repo-url> kubegonfig
cd kubegonfig
go install .
```

Or build locally:

```bash
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

Without the wrapper, use `eval` or `source` directly:

```bash
eval "$(kubegonfig env my-cluster)"
```

```fish
kubegonfig env my-cluster --shell fish | source
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Configure GPG recipient (interactive key selection) |
| `create <name>` | Create a profile from `--from` file or `$EDITOR` |
| `import <name> --from <path>` | Import a kubeconfig file as a profile |
| `list` | List all profiles (`*` marks active) |
| `use <name>` | Decrypt and activate a profile (`--shell posix\|fish`) |
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
  state/
    current              # active profile name
  kubegonfig.lock        # advisory lock file

$XDG_CONFIG_HOME/kubegonfig/
  config.yaml             # GPG recipient, settings

$XDG_RUNTIME_DIR/kubegonfig/
  <name>.yaml             # activation temp file for env/use (0600)
```

If `XDG_RUNTIME_DIR` is not set, runtime files are stored in a private directory under the system temp directory named `kubegonfig-<uid>`.

## Configuration

Configuration is stored in `$XDG_CONFIG_HOME/kubegonfig/config.yaml`.

```yaml
gpg_recipient: user@example.com
gpg_recipients:
  - second-recipient@example.com
data_dir: /optional/custom/data/dir
shell_style: posix # posix or fish
```

- `gpg_recipient` is the primary recipient configured by `kubegonfig init`.
- `gpg_recipients` adds optional extra recipients for new or updated profiles.
- `data_dir` overrides `$XDG_DATA_HOME/kubegonfig` for encrypted profiles and active state.
- `shell_style` controls default output for `env` and `use`; `--shell` overrides it per command.

## Security

- Kubeconfig is never stored in plaintext permanently
- Runtime temp files are `0600` in `0700` directories with ownership checks
- Profile names are validated against path traversal and shell injection
- Shell output is properly escaped
- No secrets are logged or printed to stdout/stderr
- `exec` unlinks its decrypted temp file before the child command continues and passes it through an inherited file descriptor
- `env` and `use` intentionally leave a runtime temp file available for the current shell; run `kubegonfig cleanup` to remove stale activation files
- Atomic writes use temp file write, file fsync, rename, and parent directory fsync

## Development Validation

The repository currently has no `Makefile` or `golangci-lint` configuration. Useful local checks are:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -trimpath -o ./kubegonfig .
```

## License

AGPL-3.0-or-later. See [LICENSE](LICENSE) for the full text.
