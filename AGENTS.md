# AGENTS.md — kubegonfig

## Project overview

kubegonfig is a production-grade CLI tool for secure kubeconfig profile
management. It stores profile contents encrypted with GPG, creates temporary
plaintext kubeconfig files only when needed for activation or command execution,
and uses shell output so callers can opt in to environment changes.

## Agent workflow rules

1. **Always read this file first** before starting any work.
2. Maintain `.agent/tasks/` with one file per task (`TASK-NNNN.md`).
3. Maintain `.agent/WORKLOG.md` with current phase, progress, blockers, and
   important validation notes.
4. Update task statuses as work progresses (`todo` / `in_progress` / `blocked` /
   `done`).
5. Never merge multiple large tasks into one task file.
6. Record architectural decisions in the task file or WORKLOG.
7. For broad audit or repository-improvement work, create `.plan/` before major
   edits and keep the required planning files updated with purpose, findings,
   intended changes, risks, validation steps, and status.
8. Treat `.plan/` as local working state only; it must remain ignored by Git.

## Repository structure

| Path | Responsibility |
|------|----------------|
| `main.go` | CLI entry point and version injection |
| `cmd/` | Cobra command definitions and command-level orchestration |
| `internal/config` | App configuration, config file paths, recipient handling |
| `internal/crypto` | GPG binary discovery, encryption/decryption, key listing |
| `internal/editor` | `$VISUAL` / `$EDITOR` resolution and edit-in-temp-file flow |
| `internal/kubeconfig` | Kubeconfig YAML validation, including context reference checks |
| `internal/profile` | Profile CRUD, encrypted storage coordination, active profile state |
| `internal/shell` | Profile-name validation, shell escaping, env output formatting |
| `internal/storage` | XDG paths, directory safety checks, atomic writes, file locking |
| `internal/tmpfile` | Runtime kubeconfig temp-file creation and cleanup |
| `shell/` | Optional shell wrappers for activation commands |
| `.github/workflows/` | Release workflow and release-time validation |

## Coding expectations

- Preserve existing intended CLI behavior unless the user explicitly requests a
  behavior change or a bug fix requires it.
- Prefer small, focused changes with clear ownership in the relevant package.
- Keep command packages thin when reusable behavior belongs in `internal/*`.
- Return actionable errors with context, but never include kubeconfig secrets or
  unsanitized GPG stderr in user-facing output.
- Preserve actionable GPG diagnostics such as missing-key errors while filtering
  stderr lines that look like raw, armored, YAML, base64-like, or unbounded data.
- Use existing validation helpers instead of duplicating path, profile-name,
  shell-style, or kubeconfig checks.
- Do not add compatibility layers or aliases unless there is a concrete shipped
  behavior, persisted data, external consumer, or explicit user requirement.
- Run `gofmt` on edited Go files and keep `go.mod` / `go.sum` tidy when imports
  change.

## Refactoring expectations

- Preserve intended functionality and public CLI behavior while refactoring.
- Prefer minimal, reviewable changes over broad rewrites.
- Refactor only when it improves clarity, correctness, security, or removes
  verified duplication.
- Keep exported APIs only when they are used by production code, command wiring,
  tests for meaningful behavior, documentation examples, or intentional internal
  package boundaries.
- Avoid adding compatibility layers or aliases without shipped behavior,
  persisted data, external consumers, or an explicit requirement.

## Dead-code policy

- Remove code only after verifying it is not used by commands, packages, tests,
  wrappers, documentation examples, or external-facing behavior.
- Prefer deleting unused helpers, placeholders, and tests for deleted helpers over
  keeping speculative APIs.
- If a helper appears unused but may represent intended public CLI behavior,
  document the uncertainty and ask before removing it.

## Deprecated-code review

- During audit work, check Go code, dependencies, CLI flags, config keys,
  documentation, tests, scripts, manifests, and workflows for deprecated or
  obsolete APIs and patterns.
- Replace deprecated usage when behavior-preserving and low risk.
- If replacement is not safe in the current pass, document the deprecated item,
  impact, and recommended migration path in `.plan/`, the task file, and the
  final report.

## Security invariants

- Kubeconfig profile contents must never be stored permanently in plaintext.
- Decrypted runtime files must live in an app-owned non-symlink `0700` directory
  and be written with `0600` permissions.
- `exec`-scoped kubeconfigs should be unlinked before the child command continues
  and passed through an inherited file descriptor where supported (`/proc/self/fd/3`
  on Linux and `/dev/fd/3` on other Unix-like release targets).
- `exec` temp-file creation and unlinking should use descriptor-relative
  operations against the same verified runtime directory handle.
- `env` / `use` activation files may remain in the runtime directory so the
  caller's shell can keep using them; `cleanup` is responsible for stale files.
- Runtime cleanup must only remove valid profile-name-shaped `*.yaml` activation
  files from the app runtime directory.
- Profile names must be validated before filesystem use, shell output, or active
  state reporting.
- GPG recipients must be non-blank after trimming and deduplicated before GPG is
  invoked.
- Shell output must be escaped for the target shell style, and environment
  variable names must be validated before shell assignment output is emitted.
- Kubeconfig validation must reject contexts that reference undefined clusters or
  users, duplicate cluster/context/user names, and non-empty `current-context`
  values that do not reference a defined context.
- Config loading must read through verified non-symlink directories and reject
  unknown YAML keys, invalid `shell_style` values, and non-empty `data_dir`
  values that are blank after trimming or not absolute paths.
- Atomic writes must use temp-file write, file fsync, rename, and parent directory
  fsync where supported by the platform.
- File locks must be released on normal errors and during panic unwinding.
- No secrets should be logged or printed to stdout/stderr.
- Avoid printing input kubeconfig paths in success messages because paths may
  contain sensitive cluster or customer names.

## Testing expectations

- Add focused unit tests for changed behavior and previously uncovered core logic.
- Prefer tests that avoid real GPG unless the goal is explicitly integration
  verification; unit tests may write dummy encrypted profile files directly when
  testing storage/state behavior.
- Cover security-sensitive regressions: profile-name validation, shell-style
  validation, config path validation, temp-file cleanup scope, file permissions,
  active-state updates, lock lifecycle, GPG stderr sanitization, env-key
  validation, and shell escaping.
- Preserve existing tests unless the tested code is verified dead and removed.
- Use local host-based validation when helpful, including builds, linters,
  shell syntax checks, Graphify updates, fake-GPG unit tests, and isolated
  GPG-backed CLI smoke checks when available.

## Documentation expectations

- Keep README examples aligned with actual command behavior and storage paths.
- Distinguish persistent activation temp files from descriptor-backed `exec`
  kubeconfigs that are unlinked while the child runs.
- Document configuration keys when command behavior depends on them.
- Do not claim tooling, Make targets, CI jobs, shell support, or install paths that
  are not present and locally verifiable.

## Required validation before completion

Run and record results for the checks that are applicable to the change:

1. `go test ./... -count=1`
2. `go test -race ./... -count=1`
3. `go vet ./...`
4. `go build -trimpath -o ./kubegonfig .`
5. `golangci-lint run` when the installed linter supports the module's Go version
6. Available Make targets only if a Makefile exists
7. Shell wrapper syntax checks when the relevant shells are installed
8. Isolated runtime verification for user-facing CLI flows when GPG is available

If any validation cannot be run locally, document the exact reason in the task
file, WORKLOG, and final report.

## Git rules

- `.agent/` is in `.gitignore` — never commit it.
- Do not commit directly to `main`; use feature branches.
- One commit per user request; never auto-push.

## graphify

This project has a graphify knowledge graph at graphify-out/.

Rules:
- Before answering architecture or codebase questions, read graphify-out/GRAPH_REPORT.md for god nodes and community structure
- If graphify-out/wiki/index.md exists, navigate it instead of reading raw files
- For cross-module "how does X relate to Y" questions, prefer `graphify query "<question>"`, `graphify path "<A>" "<B>"`, or `graphify explain "<concept>"` over grep — these traverse the graph's EXTRACTED + INFERRED edges instead of scanning files
- After modifying code files in this session, run `graphify update .` to keep the graph current (AST-only, no API cost)
