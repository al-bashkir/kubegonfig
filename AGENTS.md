# AGENTS.md — kubegonfig

## Project overview

kubegonfig is a production-grade CLI tool for secure management of kubeconfig files.
It stores profiles encrypted with GPG, provides safe shell integration, and follows
Unix process model conventions for environment variable export.

## Agent workflow rules

1. **Always read this file first** before starting any work.
2. Maintain `.agent/tasks/` — one file per task (`TASK-NNNN.md`).
3. Maintain `.agent/WORKLOG.md` — current phase, progress, blockers.
4. Update task statuses as work progresses (`todo` / `in_progress` / `blocked` / `done`).
5. Never merge multiple large tasks into one file.
6. Record architectural decisions in task files or WORKLOG.

## Architecture

- **Language:** Go
- **CLI framework:** Cobra
- **Encryption:** GPG (external binary, user's keyring)
- **Storage:** XDG Base Directory compliant
- **Shell integration:** `eval "$(kubegonfig env <name>)"` pattern

### Package layout

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra command definitions |
| `internal/config` | App configuration (GPG recipient, paths) |
| `internal/crypto` | GPG encrypt / decrypt |
| `internal/editor` | EDITOR resolution and temp-file editing |
| `internal/kubeconfig` | Kubeconfig YAML validation |
| `internal/profile` | Profile CRUD, activation state |
| `internal/shell` | Shell escaping, env output formatting |
| `internal/storage` | Atomic writes, XDG paths, file locking |
| `internal/tmpfile` | Temp file lifecycle, signal-safe cleanup |

## Security invariants

- Kubeconfig is **never** stored in plaintext permanently.
- Temp decrypted files live in a `0700` directory, are `0600`, and are cleaned up.
- Profile names are validated against path traversal and shell injection.
- Shell output is properly escaped.
- No secrets are logged or printed to stdout/stderr.
- Atomic writes via temp-file + rename.

## Git rules

- `.agent/` is in `.gitignore` — never commit it.
- Do not commit directly to `main`; use feature branches.
- One commit per user request; never auto-push.
