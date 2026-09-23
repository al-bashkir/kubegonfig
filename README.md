# kubegonfig

Secure kubeconfig profile manager. Stores profiles encrypted with GPG, decrypts to temporary files on demand, and exports `KUBECONFIG` via eval-friendly shell output.

## Features

- GPG encryption at rest using your existing keyring
- XDG Base Directory compliant storage
- Atomic writes with file locking and fsync
- Shell integration for bash, zsh, and fish
- Profile name validation against path traversal and injection
- Process isolation via `exec` with an unlinked kubeconfig file descriptor

## Install

### From release

Download the binary for your platform from the [releases page](../../releases) and place it in your `PATH`. If you want shell wrapper functions, copy the matching file from `shell/` in this repository or from the release source archive.

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
kubegonfig import production --from ~/.kube/config

# Activate a profile (must be eval'd to set KUBECONFIG in this shell)
eval "$(kubegonfig env production)"

# Run a one-off command against a profile
kubegonfig exec staging -- kubectl get pods
```

## Shell integration

Source the appropriate wrapper in your shell rc file. This lets `use` and `env` subcommands export `KUBECONFIG` into your current shell session without wrapping them in `eval` manually. The bash/zsh wrappers request POSIX shell output by default (passing `--shell posix`); the fish wrapper requests fish output (passing `--shell fish`). The wrappers also recognize global `--quiet` / `-q` before `use` or `env`.

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

To invoke manually without the wrapper function, use `eval` or `source` directly:

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
| `create <name>` | Create a profile from `--from` file or `$VISUAL`/`$EDITOR` |
| `import <name> --from <path>` | Import a kubeconfig file as a profile |
| `list` (`ls`) | List all profiles (`*` marks active) |
| `use <name>` | Decrypt and activate a profile (`--shell posix\|fish`) |
| `env <name>` | Print shell export and mark the profile active (`--shell posix\|fish`) |
| `exec <name> -- cmd` | Run a command with `KUBECONFIG` set |
| `edit <name>` | Decrypt, edit in `$VISUAL`/`$EDITOR`, re-encrypt |
| `current` | Print the active profile name |
| `rename <old> <new>` | Rename a profile |
| `delete <name>` (`rm`) | Delete a profile (`--force` to skip prompt) |
| `unlock <name>...` | Decrypt one or more profiles to the runtime dir without activating any |
| `export [name...] -o <file>` | Write a tar archive of selected (or `--all`) profiles. `--decrypt` for plaintext (refused into a TTY or the data directory); `--profiles-only` to skip `config.yaml` |
| `restore <archive>` | Restore profiles from an archive. `--force` overwrites; `--skip-existing` keeps local; `--merge-config` unions recipients; `--dry-run` previews |
| `cleanup` | Remove stale temp files from runtime dir |

Global options include `--quiet` (`-q`) to suppress informational stderr messages and the standard Cobra `--help`/`--version` output.

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

## Backup and restore

Back up the encrypted profile store, optionally including `config.yaml`:

```bash
kubegonfig export --all -o backup.tar
```

Restore on the same or another machine with kubegonfig and a usable GPG keyring:

```bash
kubegonfig restore backup.tar
```

Round-trips keep profiles encrypted end-to-end. To export plaintext for interop with non-kubegonfig tooling, pass `--decrypt`:

```bash
kubegonfig export production --decrypt -o /var/cache/builds/prod.tar
```

Plaintext export is refused when stdout is a terminal or when the destination resolves inside the kubegonfig data directory.

Restore is additive: per-profile failures do not roll back profiles that were already written. Use `--dry-run` to validate an archive and preview the plan without writing.

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
  Blank recipients are rejected; recipients are trimmed and deduplicated before use.
- `data_dir` overrides `$XDG_DATA_HOME/kubegonfig` for encrypted profiles and active state. When set, it must be a non-blank absolute path.
- `shell_style` controls default output for `env` and `use`; `--shell` overrides it per command. Only `posix` and `fish` are accepted.
- Configuration is read from verified non-symlink directories. Unknown configuration keys, invalid `shell_style` values, and invalid `data_dir` values are rejected when the config is loaded so typos or unsafe paths do not silently change behavior.

## Security

- Kubeconfig is never stored in plaintext permanently
- Runtime temp files are `0600` in non-symlink `0700` directories with ownership checks
- Profile names are validated against path traversal and shell injection
- Shell output is properly escaped and environment variable names are validated before shell assignment output is emitted
- No secrets are logged or printed to stdout/stderr; GPG stderr is reduced to short actionable diagnostics before it is included in errors
- `exec` unlinks its decrypted temp file before the child command continues and passes it through an inherited file descriptor (`/proc/self/fd/3` on Linux, `/dev/fd/3` elsewhere)
- `env` and `use` intentionally leave a runtime temp file available for the current shell; run `kubegonfig cleanup` to remove stale activation files
- Runtime cleanup removes valid profile-name-shaped `*.yaml` activation files only from the kubegonfig runtime directory
- The active profile state name syntax is validated before it is printed or used to update profile state
- Kubeconfig validation requires `apiVersion: v1`, `kind: Config`, at least one cluster/context/user, unique cluster/context/user names, cluster servers, context cluster/user references that point at defined entries, and a valid `current-context` when set
- Atomic writes use temp file write, file fsync, rename, and parent directory fsync
- Lock files and runtime activation cleanup use verified non-symlink directories and descriptor-relative operations where filesystem mutations occur

## Development Validation

The repository currently has no `Makefile` or `golangci-lint` configuration. Useful local checks are:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -trimpath -o ./kubegonfig .
golangci-lint run
bash -n shell/kubegonfig.bash
zsh -n shell/kubegonfig.zsh
fish -n shell/kubegonfig.fish
```

## License

AGPL-3.0-or-later. See [LICENSE](LICENSE) for the full text.
