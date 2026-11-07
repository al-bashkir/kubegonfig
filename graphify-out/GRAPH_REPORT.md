# Graph Report - kubegonfig  (2026-11-07)

## Corpus Check
- 53 files · ~35,145 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 716 nodes · 1981 edges · 34 communities (30 shown, 4 thin omitted)
- Extraction: 75% EXTRACTED · 25% INFERRED · 0% AMBIGUOUS · INFERRED: 498 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b20e5952`
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
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]

## God Nodes (most connected - your core abstractions)
1. `newTestManager()` - 57 edges
2. `T` - 54 edges
3. `T` - 37 edges
4. `Export()` - 34 edges
5. `Restore()` - 34 edges
6. `T` - 31 edges
7. `writeProfile()` - 29 edges
8. `Manager` - 26 edges
9. `newSeededManager()` - 24 edges
10. `ValidateName()` - 24 edges

## Surprising Connections (you probably didn't know these)
- `completeProfileNames()` --calls--> `Load()`  [INFERRED]
  cmd/completion.go → internal/config/config.go
- `completeProfileNames()` --calls--> `NewManager()`  [INFERRED]
  cmd/completion.go → internal/profile/profile.go
- `runExecProfile()` --calls--> `OpenUnlinked()`  [INFERRED]
  cmd/exec.go → internal/tmpfile/tmpfile.go
- `Decrypt()` --calls--> `runExecProfile()`  [INFERRED]
  internal/crypto/gpg.go → cmd/exec.go
- `openTestExecKubeconfig()` --calls--> `OpenUnlinked()`  [INFERRED]
  cmd/exec_test.go → internal/tmpfile/tmpfile.go

## Import Cycles
- None detected.

## Communities (34 total, 4 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.13
Nodes (40): runUnlock(), TestGPGPathReresolvesMissingCachedBinary(), setCachedGPGForTest(), File, Time, T, FileLock, RuntimeDir() (+32 more)

### Community 1 - "Community 1"
Cohesion: 0.22
Nodes (6): Config, Time, Manager, profileFileName(), ValidateName(), IsPostCommitError()

### Community 2 - "Community 2"
Cohesion: 0.09
Nodes (58): budgetWriter, installFakeGPGForBundle(), keysOf(), newSeededManager(), readTarEntries(), TestExport_DecryptEmitsPlaintextValidatedKubeconfig(), TestExport_DecryptRejectsInvalidPlaintext(), TestExport_EncryptedPassThrough() (+50 more)

### Community 3 - "Community 3"
Cohesion: 0.10
Nodes (81): Manager, T, Manager, T, Time, readStoredForTest(), TestActivateDecryptsAndRecordsCurrent(), TestActivateErrors() (+73 more)

### Community 4 - "Community 4"
Cohesion: 0.21
Nodes (24): T, TestListRejectsInvalidCurrentState(), T, TestValidate_ClusterMissingName(), TestValidate_ClusterMissingServer(), TestValidate_ContextMissingCluster(), TestValidate_ContextMissingUser(), TestValidate_ContextUnknownCluster() (+16 more)

### Community 5 - "Community 5"
Cohesion: 0.16
Nodes (30): Decrypt(), Encrypt(), gpgPath(), isSafeGPGDiagnostic(), ListSecretKeys(), looksSensitiveGPGLine(), LookupGPG(), normalizeRecipients() (+22 more)

### Community 6 - "Community 6"
Cohesion: 0.16
Nodes (23): completeExecProfileName(), completeProfileNames(), completeRenameOldProfile(), Command, T, setupCompletionProfiles(), TestCompleteProfileNames(), TestCompleteProfileNamesFiltersPrefix() (+15 more)

### Community 7 - "Community 7"
Cohesion: 0.22
Nodes (18): Config, Load(), normalizeDataDir(), TestLoad_ExistingConfig(), TestLoad_InvalidYAML(), TestLoad_NonexistentReturnsDefault(), TestLoadRejectsInvalidDataDir(), TestLoadRejectsInvalidShellStyle() (+10 more)

### Community 8 - "Community 8"
Cohesion: 0.10
Nodes (23): exportNames(), Writer, guardPlaintextDestination(), init(), runExport(), startsWithDotDot(), init(), runRestore() (+15 more)

### Community 9 - "Community 9"
Cohesion: 0.07
Nodes (28): code:bash (git clone <repo-url> kubegonfig), code:bash (kubegonfig export --all -o backup.tar), code:bash (kubegonfig restore backup.tar), code:bash (kubegonfig export production --decrypt -o /var/cache/builds/), code:yaml (gpg_recipient: user@example.com), code:bash (go test ./... -count=1), code:bash (go build -o kubegonfig .), code:bash (# Initialize with your GPG key (interactive selection)) (+20 more)

### Community 10 - "Community 10"
Cohesion: 0.23
Nodes (19): Cmd, appendEnv(), execKubeconfigPath(), File, newExecCommandWithKubeconfig(), runCommandWithKubeconfig(), runExecProfile(), runExecWithKubeconfigFile() (+11 more)

### Community 11 - "Community 11"
Cohesion: 0.31
Nodes (11): T, EscapePosix(), formatExport(), formatFishSet(), FormatSet(), NormalizeStyle(), TestEscapePosix(), TestFormatSet() (+3 more)

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
Cohesion: 0.06
Nodes (84): DirEntry, FileMode, T, File, Time, Writer, T, TestEnsureDirRejectsRegularFile() (+76 more)

### Community 18 - "Community 18"
Cohesion: 0.31
Nodes (9): Edit(), ResolveEditor(), splitCommandLine(), TestEditWritesTempUnderRuntimeDir(), TestResolveEditorPrefersVisual(), TestResolveEditorUsesEditorWhenVisualUnset(), TestSplitCommandLine(), TestSplitCommandLineRejectsUnterminatedQuote() (+1 more)

### Community 21 - "Community 21"
Cohesion: 0.18
Nodes (18): clusterData, clusterEntry, contextData, contextEntry, clusterData, clusterEntry, contextData, contextEntry (+10 more)

### Community 31 - "Community 31"
Cohesion: 0.18
Nodes (28): applyRestore(), mergeArchivedConfig(), newRestorer(), planRestore(), prepareProfileBlob(), readManifest(), Restore(), buildEncryptedArchive() (+20 more)

### Community 32 - "Community 32"
Cohesion: 0.31
Nodes (21): cmdTestEnv(), cmdTestExists(), cmdTestResetFlags(), cmdTestRun(), cmdTestTarNames(), Command, T, TestDeleteForceSkipsPrompt() (+13 more)

### Community 33 - "Community 33"
Cohesion: 0.25
Nodes (11): Reader, Writer, keyMenu(), selectGPGKey(), T, initTestFakeGPG(), TestInitRecipientFlag(), TestSelectGPGKey() (+3 more)

## Knowledge Gaps
- **86 isolated node(s):** `Command`, `Cmd`, `File`, `Writer`, `Reader` (+81 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewManager()` connect `Community 2` to `Community 1`, `Community 3`, `Community 4`, `Community 6`, `Community 7`, `Community 12`, `Community 31`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Why does `ValidateName()` connect `Community 1` to `Community 8`, `Community 0`, `Community 2`, `Community 11`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Why does `RuntimeDir()` connect `Community 0` to `Community 3`, `Community 10`, `Community 17`, `Community 18`, `Community 31`?**
  _High betweenness centrality (0.091) - this node is a cross-community bridge._
- **Are the 16 inferred relationships involving `newTestManager()` (e.g. with `TestActivateDecryptsAndRecordsCurrent()` and `TestActivateErrors()`) actually correct?**
  _`newTestManager()` has 16 INFERRED edges - model-reasoned connections that need verification._
- **Are the 24 inferred relationships involving `Export()` (e.g. with `TestExport_DecryptEmitsPlaintextValidatedKubeconfig()` and `TestExport_DecryptRejectsInvalidPlaintext()`) actually correct?**
  _`Export()` has 24 INFERRED edges - model-reasoned connections that need verification._
- **Are the 23 inferred relationships involving `Restore()` (e.g. with `TestRestore_CollisionWithoutFlagsFails()` and `TestRestore_DryRunDoesNotWrite()`) actually correct?**
  _`Restore()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Command`, `Cmd`, `File` to the rest of the system?**
  _86 weakly-connected nodes found - possible documentation gaps or missing edges._