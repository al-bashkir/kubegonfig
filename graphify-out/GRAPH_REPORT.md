# Graph Report - kubegonfig  (2026-09-23)

## Corpus Check
- 49 files · ~30,456 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 613 nodes · 1643 edges · 29 communities (27 shown, 2 thin omitted)
- Extraction: 74% EXTRACTED · 26% INFERRED · 0% AMBIGUOUS · INFERRED: 422 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `99d5fc4d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 21|Community 21]]

## God Nodes (most connected - your core abstractions)
1. `T` - 54 edges
2. `newTestManager()` - 46 edges
3. `T` - 37 edges
4. `T` - 32 edges
5. `Export()` - 31 edges
6. `Manager` - 28 edges
7. `writeProfile()` - 27 edges
8. `Restore()` - 26 edges
9. `ValidateName()` - 22 edges
10. `T` - 20 edges

## Surprising Connections (you probably didn't know these)
- `Create()` --calls--> `activateProfile()`  [INFERRED]
  internal/tmpfile/tmpfile.go → cmd/activate.go
- `completeProfileNames()` --calls--> `Load()`  [INFERRED]
  cmd/completion.go → internal/config/config.go
- `completeProfileNames()` --calls--> `NewManager()`  [INFERRED]
  cmd/completion.go → internal/profile/profile.go
- `runExecProfile()` --calls--> `OpenUnlinked()`  [INFERRED]
  cmd/exec.go → internal/tmpfile/tmpfile.go
- `Decrypt()` --calls--> `runExecProfile()`  [INFERRED]
  internal/crypto/gpg.go → cmd/exec.go

## Import Cycles
- None detected.

## Communities (29 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.12
Nodes (44): runUnlock(), TestGPGPathReresolvesMissingCachedBinary(), setCachedGPGForTest(), File, Time, T, FileLock, NewFileLock() (+36 more)

### Community 1 - "Community 1"
Cohesion: 0.19
Nodes (6): Config, Time, Manager, profileFileName(), ValidateName(), IsPostCommitError()

### Community 2 - "Community 2"
Cohesion: 0.08
Nodes (64): installFakeGPGForBundle(), keysOf(), newSeededManager(), readTarEntries(), TestExport_DecryptEmitsPlaintextValidatedKubeconfig(), TestExport_DecryptRejectsInvalidPlaintext(), TestExport_EncryptedPassThrough(), TestExport_EntriesSortedByName() (+56 more)

### Community 3 - "Community 3"
Cohesion: 0.14
Nodes (61): Manager, T, assertStillBlocked(), currentPathForTest(), installFakeGPG(), makeProfilesDirReadOnly(), makeStateDirReadOnly(), newTestManager() (+53 more)

### Community 4 - "Community 4"
Cohesion: 0.21
Nodes (24): T, TestListRejectsInvalidCurrentState(), T, TestValidate_ClusterMissingName(), TestValidate_ClusterMissingServer(), TestValidate_ContextMissingCluster(), TestValidate_ContextMissingUser(), TestValidate_ContextUnknownCluster() (+16 more)

### Community 5 - "Community 5"
Cohesion: 0.13
Nodes (25): initRecipientFromFlag(), selectGPGKey(), T, TestInitRecipientFromFlag(), Decrypt(), Encrypt(), gpgPath(), isSafeGPGDiagnostic() (+17 more)

### Community 6 - "Community 6"
Cohesion: 0.16
Nodes (23): completeExecProfileName(), completeProfileNames(), completeRenameOldProfile(), Command, T, setupCompletionProfiles(), TestCompleteProfileNames(), TestCompleteProfileNamesFiltersPrefix() (+15 more)

### Community 7 - "Community 7"
Cohesion: 0.23
Nodes (17): Config, Load(), normalizeDataDir(), TestLoad_ExistingConfig(), TestLoad_InvalidYAML(), TestLoad_NonexistentReturnsDefault(), TestLoadRejectsInvalidDataDir(), TestLoadRejectsInvalidShellStyle() (+9 more)

### Community 8 - "Community 8"
Cohesion: 0.10
Nodes (21): guardPlaintextDestination(), init(), runExport(), startsWithDotDot(), init(), runRestore(), Execute(), Command (+13 more)

### Community 9 - "Community 9"
Cohesion: 0.06
Nodes (31): code:bash (git clone <repo-url> kubegonfig), code:bash (kubegonfig export --all -o backup.tar), code:bash (kubegonfig restore backup.tar), code:bash (kubegonfig export production --decrypt -o /var/cache/builds/), code:yaml (gpg_recipient: user@example.com), code:bash (go test ./... -count=1), code:bash (go build -o kubegonfig .), code:bash (# Initialize with your GPG key (interactive selection)) (+23 more)

### Community 10 - "Community 10"
Cohesion: 0.23
Nodes (19): Cmd, appendEnv(), execKubeconfigPath(), File, newExecCommandWithKubeconfig(), runCommandWithKubeconfig(), runExecProfile(), runExecWithKubeconfigFile() (+11 more)

### Community 11 - "Community 11"
Cohesion: 0.26
Nodes (12): activateProfile(), T, EscapePosix(), formatExport(), formatFishSet(), FormatSet(), NormalizeStyle(), TestEscapePosix() (+4 more)

### Community 12 - "Community 12"
Cohesion: 0.31
Nodes (14): Config, Manager, T, saveAndRestoreState(), setupExportTestManager(), TestExportCmd_AllAndArgsMutuallyExclusive(), TestExportCmd_DecryptRefusedInsideDataDir(), TestExportCmd_NoArgsNoAllFails() (+6 more)

### Community 13 - "Community 13"
Cohesion: 0.07
Nodes (28): API surface, Architecture, Background, Cleanup-on-failure asymmetry, code:block1 (mgr.Activate(name, tmpfile.ProbeCached, tmpfile.Create, tmpf), code:go (// Activate decrypts a profile only when the existing activa), code:go (// profileMtime returns the ciphertext mtime for the named p), code:go (// ProbeCached reports whether a plaintext activation file f) (+20 more)

### Community 14 - "Community 14"
Cohesion: 0.09
Nodes (23): code:go (func TestRegularFileMtimeInDir(t *testing.T) {), code:go (func TestActivateCacheHitSkipsCreate(t *testing.T) {), code:go (path, err := m.Activate("prod",), code:go (_, err := m.Activate("prod",), code:go (// Activate locks the manager, decides whether the existing ), code:go (func activateProfile(name, shellFlag string) (string, error)), code:bash (git add internal/profile/profile.go internal/profile/profile), code:sh (# Pre-req: a real or fake GPG and an existing encrypted prof) (+15 more)

### Community 15 - "Community 15"
Cohesion: 0.47
Nodes (12): T, joinShellArgs(), shellQuote(), TestBashWrapperAddsDefaultPosixShellStyle(), TestBashWrapperHelpPassesThroughWithoutEval(), TestFishWrapperAddsDefaultFishShellStyle(), TestFishWrapperHelpPassesThroughWithoutEval(), testWrapperAddsDefaultPosixShellStyle() (+4 more)

### Community 17 - "Community 17"
Cohesion: 0.08
Nodes (71): DirEntry, FileMode, File, Time, Writer, T, FileExistsInDir(), AtomicWriteInDir() (+63 more)

### Community 18 - "Community 18"
Cohesion: 0.31
Nodes (9): Edit(), ResolveEditor(), splitCommandLine(), TestEditWritesTempUnderRuntimeDir(), TestResolveEditorPrefersVisual(), TestResolveEditorUsesEditorWhenVisualUnset(), TestSplitCommandLine(), TestSplitCommandLineRejectsUnterminatedQuote() (+1 more)

### Community 21 - "Community 21"
Cohesion: 0.15
Nodes (11): clusterData, clusterEntry, contextData, contextEntry, clusterData, clusterEntry, contextData, contextEntry (+3 more)

## Knowledge Gaps
- **82 isolated node(s):** `Cmd`, `File`, `T`, `T`, `Command` (+77 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ValidateName()` connect `Community 1` to `Community 8`, `Community 0`, `Community 2`, `Community 11`?**
  _High betweenness centrality (0.102) - this node is a cross-community bridge._
- **Why does `NewManager()` connect `Community 2` to `Community 1`, `Community 3`, `Community 4`, `Community 6`, `Community 7`, `Community 12`?**
  _High betweenness centrality (0.099) - this node is a cross-community bridge._
- **Why does `Restore()` connect `Community 2` to `Community 0`, `Community 17`, `Community 8`?**
  _High betweenness centrality (0.067) - this node is a cross-community bridge._
- **Are the 22 inferred relationships involving `Export()` (e.g. with `TestExport_DecryptEmitsPlaintextValidatedKubeconfig()` and `TestExport_DecryptRejectsInvalidPlaintext()`) actually correct?**
  _`Export()` has 22 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Cmd`, `File`, `T` to the rest of the system?**
  _82 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.11613475177304965 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.08370221327967807 - nodes in this community are weakly interconnected._