# Graph Report - kubegonfig.chore-reuse-opened-config  (2026-05-22)

## Corpus Check
- 52 files · ~38,851 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 815 nodes · 1913 edges · 37 communities
- Extraction: 70% EXTRACTED · 30% INFERRED · 0% AMBIGUOUS · INFERRED: 574 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `08fbcacb`
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
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]

## God Nodes (most connected - your core abstractions)
1. `newTestManager()` - 43 edges
2. `Export()` - 27 edges
3. `Manager` - 27 edges
4. `newTestManager()` - 27 edges
5. `writeProfile()` - 23 edges
6. `ValidateName()` - 22 edges
7. `Restore()` - 20 edges
8. `newSeededManager()` - 17 edges
9. `NewManager()` - 17 edges
10. `writeProfile()` - 17 edges

## Surprising Connections (you probably didn't know these)
- `TestListRejectsInvalidCurrentState()` --calls--> `NewManager()`  [INFERRED]
  cmd/list_test.go → internal/profile/profile.go
- `completeProfileNames()` --calls--> `Load()`  [INFERRED]
  cmd/completion.go → internal/config/config.go
- `completeProfileNames()` --calls--> `NewManager()`  [INFERRED]
  cmd/completion.go → internal/profile/profile.go
- `runExecProfile()` --calls--> `OpenUnlinked()`  [INFERRED]
  cmd/exec.go → internal/tmpfile/tmpfile.go
- `openTestExecKubeconfig()` --calls--> `OpenUnlinked()`  [INFERRED]
  cmd/exec_test.go → internal/tmpfile/tmpfile.go

## Communities (37 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.06
Nodes (72): setCachedGPGForTest(), TestGPGPathReresolvesMissingCachedBinary(), CheckGPG(), gpgPath(), lookupGPG(), setCachedGPGForTest(), TestGPGPathConcurrentCacheAccess(), TestGPGPathReresolvesMissingCachedBinary() (+64 more)

### Community 1 - "Community 1"
Cohesion: 0.09
Nodes (48): Encrypt(), Manager, NewManager(), profileFileName(), profileFileName(), assertStillBlocked(), currentPathForTest(), installFakeGPG() (+40 more)

### Community 2 - "Community 2"
Cohesion: 0.07
Nodes (61): installFakeGPGForBundle(), keysOf(), newSeededManager(), readTarEntries(), TestExport_DecryptEmitsPlaintextValidatedKubeconfig(), TestExport_DecryptRejectsInvalidPlaintext(), TestExport_EncryptedPassThrough(), TestExport_EntriesSortedByName() (+53 more)

### Community 3 - "Community 3"
Cohesion: 0.11
Nodes (58): assertStillBlocked(), currentPathForTest(), installFakeGPG(), makeProfilesDirReadOnly(), makeStateDirReadOnly(), newTestManager(), profileExistsForTest(), profilePathForTest() (+50 more)

### Community 4 - "Community 4"
Cohesion: 0.07
Nodes (26): TestListRejectsInvalidCurrentState(), TestValidate(), TestEncrypt_NoRecipients(), TestListRejectsInvalidCurrentState(), TestManager_Unlock_InvalidName(), PostCommitError, TestValidate_ClusterMissingName(), TestValidate_ClusterMissingServer() (+18 more)

### Community 5 - "Community 5"
Cohesion: 0.07
Nodes (36): prepareProfileBlob(), initRecipientFromFlag(), selectGPGKey(), TestInitRecipientFromFlag(), CheckGPG(), Decrypt(), Encrypt(), gpgPath() (+28 more)

### Community 6 - "Community 6"
Cohesion: 0.09
Nodes (37): completeExecProfileName(), completeProfileNames(), completeRenameOldProfile(), setupCompletionProfiles(), TestCompleteProfileNames(), TestCompleteProfileNamesFiltersPrefix(), TestExecCommandCompletesProfileName(), TestExecCompletionUsesDefaultAfterProfileArg() (+29 more)

### Community 7 - "Community 7"
Cohesion: 0.09
Nodes (27): Config, Load(), normalizeDataDir(), TestLoad_ExistingConfig(), TestLoad_InvalidYAML(), TestLoad_NonexistentReturnsDefault(), TestLoadRejectsInvalidDataDir(), TestLoadRejectsInvalidShellStyle() (+19 more)

### Community 8 - "Community 8"
Cohesion: 0.08
Nodes (28): guardPlaintextDestination(), runExport(), startsWithDotDot(), runRestore(), Execute(), info(), isHelpRequest(), SetVersion() (+20 more)

### Community 9 - "Community 9"
Cohesion: 0.06
Nodes (31): Backup and restore, Bash, code:bash (git clone <repo-url> kubegonfig), code:bash (kubegonfig export --all -o backup.tar), code:bash (kubegonfig restore backup.tar), code:bash (kubegonfig export production --decrypt -o /var/cache/builds/), code:yaml (gpg_recipient: user@example.com), code:bash (go test ./... -count=1) (+23 more)

### Community 10 - "Community 10"
Cohesion: 0.15
Nodes (29): appendEnv(), execKubeconfigPath(), newExecCommandWithKubeconfig(), runCommandWithKubeconfig(), runExecProfile(), runExecWithKubeconfigFile(), assertRuntimeDirEmpty(), openTestExecKubeconfig() (+21 more)

### Community 11 - "Community 11"
Cohesion: 0.12
Nodes (28): activateProfile(), activeShellStyle(), activateProfile(), activeShellStyle(), EscapePosix(), formatExport(), formatFishSet(), FormatSet() (+20 more)

### Community 12 - "Community 12"
Cohesion: 0.09
Nodes (30): AtomicWrite(), FileExistsInDir(), FileExistsInDir(), NewFileLock(), RegularFileMtimeInDir(), TestFileExistsInDir(), TestFileExistsInDirRejectsFIFOImmediately(), TestFileExistsInDirRejectsSymlinkedDir() (+22 more)

### Community 13 - "Community 13"
Cohesion: 0.07
Nodes (28): API surface, Architecture, Background, Cleanup-on-failure asymmetry, code:block1 (mgr.Activate(name, tmpfile.ProbeCached, tmpfile.Create, tmpf), code:go (// Activate decrypts a profile only when the existing activa), code:go (// profileMtime returns the ciphertext mtime for the named p), code:go (// ProbeCached reports whether a plaintext activation file f) (+20 more)

### Community 14 - "Community 14"
Cohesion: 0.08
Nodes (25): code:go (func TestRegularFileMtimeInDir(t *testing.T) {), code:go (func TestActivateCacheHitSkipsCreate(t *testing.T) {), code:go (path, err := m.Activate("prod",), code:go (_, err := m.Activate("prod",), code:go (_, err := m.Activate("prod",), code:go (// Activate locks the manager, decides whether the existing ), code:go (func activateProfile(name, shellFlag string) (string, error)), code:bash (git add internal/profile/profile.go internal/profile/profile) (+17 more)

### Community 15 - "Community 15"
Cohesion: 0.22
Nodes (22): joinShellArgs(), shellQuote(), TestBashWrapperAddsDefaultPosixShellStyle(), TestBashWrapperHelpPassesThroughWithoutEval(), TestFishWrapperAddsDefaultFishShellStyle(), TestFishWrapperHelpPassesThroughWithoutEval(), testWrapperAddsDefaultPosixShellStyle(), testWrapperHelpPassesThroughWithoutEval() (+14 more)

### Community 16 - "Community 16"
Cohesion: 0.19
Nodes (20): AtomicWriteInDir(), ConfigDir(), createTempFileInDir(), EnsureDir(), NewFileLock(), openDirNoFollow(), OpenUnlinkedTempFileInDir(), openVerifiedDir() (+12 more)

### Community 17 - "Community 17"
Cohesion: 0.14
Nodes (21): AtomicWriteInDir(), AtomicWriteStream(), createTempFileInDir(), EnsureDir(), openDirNoFollow(), OpenUnlinkedTempFileInDir(), openVerifiedDir(), ReadDirInDir() (+13 more)

### Community 18 - "Community 18"
Cohesion: 0.22
Nodes (14): Edit(), Edit(), ResolveEditor(), splitCommandLine(), TestResolveEditorPrefersVisual(), TestResolveEditorUsesEditorWhenVisualUnset(), TestSplitCommandLine(), TestSplitCommandLineRejectsUnterminatedQuote() (+6 more)

### Community 19 - "Community 19"
Cohesion: 0.22
Nodes (11): RemoveFileInDir(), RenameFileInDir(), fileExists(), TestAtomicWriteInDirRejectsSymlinkedDir(), TestRemoveFileInDir(), TestRemoveFileInDirMissingDir(), TestRemoveFileInDirRejectsInvalidNames(), TestRemoveFileInDirRejectsSymlinkedDir() (+3 more)

### Community 20 - "Community 20"
Cohesion: 0.24
Nodes (10): RemoveFileInDir(), RenameFileInDir(), fileExists(), TestRemoveFileInDir(), TestRemoveFileInDirMissingDir(), TestRemoveFileInDirRejectsInvalidNames(), TestRemoveFileInDirRejectsSymlinkedDir(), TestRenameFileInDir() (+2 more)

### Community 21 - "Community 21"
Cohesion: 0.22
Nodes (6): clusterData, clusterEntry, contextData, contextEntry, kubeConfig, userEntry

### Community 22 - "Community 22"
Cohesion: 0.5
Nodes (4): AtomicWrite(), TestAtomicWrite(), TestAtomicWrite_NestedDir(), TestAtomicWrite_Overwrite()

### Community 23 - "Community 23"
Cohesion: 0.5
Nodes (4): ReadFileInDir(), TestReadFileInDirRejectsFIFOImmediately(), TestReadFileInDirRejectsSymlinkedDir(), TestReadFileInDirRejectsSymlinkedFile()

### Community 29 - "Community 29"
Cohesion: 0.67
Nodes (3): DataDir(), TestDataDir_Default(), TestDataDir_WithEnv()

### Community 30 - "Community 30"
Cohesion: 0.67
Nodes (3): DataDir(), TestDataDir_Default(), TestDataDir_WithEnv()

## Knowledge Gaps
- **67 isolated node(s):** `ExportOptions`, `ManifestProfile`, `RestoreOptions`, `RestorePlan`, `GPGKey` (+62 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ValidateName()` connect `Community 1` to `Community 0`, `Community 2`, `Community 6`, `Community 8`, `Community 11`?**
  _High betweenness centrality (0.104) - this node is a cross-community bridge._
- **Why does `NewManager()` connect `Community 2` to `Community 1`, `Community 3`, `Community 4`, `Community 6`?**
  _High betweenness centrality (0.054) - this node is a cross-community bridge._
- **Why does `Export()` connect `Community 2` to `Community 8`, `Community 1`?**
  _High betweenness centrality (0.054) - this node is a cross-community bridge._
- **Are the 22 inferred relationships involving `Export()` (e.g. with `runExport()` and `TestRestoreCmd_RoundTripFromFile()`) actually correct?**
  _`Export()` has 22 INFERRED edges - model-reasoned connections that need verification._
- **What connects `ExportOptions`, `ManifestProfile`, `RestoreOptions` to the rest of the system?**
  _67 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.06 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._