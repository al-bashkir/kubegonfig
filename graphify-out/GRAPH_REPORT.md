# Graph Report - kubegonfig.feat-multi-unlock  (2026-05-03)

## Corpus Check
- 40 files · ~28,115 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 346 nodes · 887 edges · 14 communities detected
- Extraction: 64% EXTRACTED · 36% INFERRED · 0% AMBIGUOUS · INFERRED: 319 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

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

## God Nodes (most connected - your core abstractions)
1. `newTestManager()` - 27 edges
2. `Manager` - 23 edges
3. `writeProfile()` - 17 edges
4. `Create()` - 16 edges
5. `Load()` - 15 edges
6. `ValidateName()` - 15 edges
7. `openVerifiedDir()` - 12 edges
8. `Remove()` - 12 edges
9. `profileFileName()` - 11 edges
10. `setCurrentForTest()` - 11 edges

## Surprising Connections (you probably didn't know these)
- `RuntimeDir()` --calls--> `assertRuntimeDirEmpty()`  [INFERRED]
  internal/storage/storage.go → cmd/exec_test.go
- `Load()` --calls--> `completeProfileNames()`  [INFERRED]
  internal/config/config.go → cmd/completion.go
- `Decrypt()` --calls--> `runExecProfile()`  [INFERRED]
  internal/crypto/gpg.go → cmd/exec.go
- `Create()` --calls--> `activateProfile()`  [INFERRED]
  internal/tmpfile/tmpfile.go → cmd/activate.go
- `OpenUnlinked()` --calls--> `openTestExecKubeconfig()`  [INFERRED]
  internal/tmpfile/tmpfile.go → cmd/exec_test.go

## Communities

### Community 0 - "Community 0"
Cohesion: 0.13
Nodes (44): profileFileName(), assertStillBlocked(), currentPathForTest(), installFakeGPG(), makeProfilesDirReadOnly(), makeStateDirReadOnly(), newTestManager(), profileExistsForTest() (+36 more)

### Community 1 - "Community 1"
Cohesion: 0.09
Nodes (48): GPGKey, CheckGPG(), Decrypt(), gpgPath(), isSafeGPGDiagnostic(), ListSecretKeys(), looksBase64Line(), looksSensitiveGPGLine() (+40 more)

### Community 2 - "Community 2"
Cohesion: 0.09
Nodes (47): AtomicWrite(), AtomicWriteInDir(), ConfigDir(), createTempFileInDir(), DataDir(), EnsureDir(), FileExistsInDir(), NewFileLock() (+39 more)

### Community 3 - "Community 3"
Cohesion: 0.19
Nodes (22): TestValidate(), PostCommitError, TestValidate_ClusterMissingName(), TestValidate_ClusterMissingServer(), TestValidate_ContextMissingCluster(), TestValidate_ContextMissingUser(), TestValidate_ContextUnknownCluster(), TestValidate_ContextUnknownUser() (+14 more)

### Community 4 - "Community 4"
Cohesion: 0.13
Nodes (20): completeExecProfileName(), completeProfileNames(), completeRenameOldProfile(), setupCompletionProfiles(), TestCompleteProfileNames(), TestCompleteProfileNamesFiltersPrefix(), TestExecCommandCompletesProfileName(), TestExecCompletionUsesDefaultAfterProfileArg() (+12 more)

### Community 5 - "Community 5"
Cohesion: 0.16
Nodes (16): Config, Load(), normalizeDataDir(), TestLoad_ExistingConfig(), TestLoad_InvalidYAML(), TestLoad_NonexistentReturnsDefault(), TestLoadRejectsInvalidDataDir(), TestLoadRejectsInvalidShellStyle() (+8 more)

### Community 6 - "Community 6"
Cohesion: 0.21
Nodes (14): activateProfile(), activeShellStyle(), EscapePosix(), formatExport(), formatFishSet(), FormatSet(), NormalizeStyle(), TestEscapePosix() (+6 more)

### Community 7 - "Community 7"
Cohesion: 0.26
Nodes (15): appendEnv(), execKubeconfigPath(), newExecCommandWithKubeconfig(), runCommandWithKubeconfig(), runExecProfile(), runExecWithKubeconfigFile(), assertRuntimeDirEmpty(), openTestExecKubeconfig() (+7 more)

### Community 8 - "Community 8"
Cohesion: 0.32
Nodes (4): Encrypt(), TestEncrypt_NoRecipients(), Manager, ValidateName()

### Community 9 - "Community 9"
Cohesion: 0.19
Nodes (11): main(), Execute(), info(), isHelpRequest(), SetVersion(), skipSetup(), TestHelpRequestIgnoresArgsAfterSeparator(), TestSkipSetupDoesNotTreatParsedArgsAsHelpRequest() (+3 more)

### Community 10 - "Community 10"
Cohesion: 0.42
Nodes (11): joinShellArgs(), shellQuote(), TestBashWrapperAddsDefaultPosixShellStyle(), TestBashWrapperHelpPassesThroughWithoutEval(), TestFishWrapperAddsDefaultFishShellStyle(), TestFishWrapperHelpPassesThroughWithoutEval(), testWrapperAddsDefaultPosixShellStyle(), testWrapperHelpPassesThroughWithoutEval() (+3 more)

### Community 11 - "Community 11"
Cohesion: 0.36
Nodes (7): Edit(), ResolveEditor(), splitCommandLine(), TestResolveEditorPrefersVisual(), TestResolveEditorUsesEditorWhenVisualUnset(), TestSplitCommandLine(), TestSplitCommandLineRejectsUnterminatedQuote()

### Community 12 - "Community 12"
Cohesion: 0.25
Nodes (6): clusterData, clusterEntry, contextData, contextEntry, kubeConfig, userEntry

### Community 13 - "Community 13"
Cohesion: 0.4
Nodes (2): initRecipientFromFlag(), TestInitRecipientFromFlag()

## Knowledge Gaps
- **7 isolated node(s):** `kubeConfig`, `clusterEntry`, `clusterData`, `contextEntry`, `contextData` (+2 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 13`** (5 nodes): `init.go`, `init_test.go`, `init()`, `initRecipientFromFlag()`, `TestInitRecipientFromFlag()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ValidateName()` connect `Community 8` to `Community 0`, `Community 1`, `Community 2`, `Community 6`?**
  _High betweenness centrality (0.101) - this node is a cross-community bridge._
- **Why does `completeProfileNames()` connect `Community 4` to `Community 2`, `Community 5`?**
  _High betweenness centrality (0.065) - this node is a cross-community bridge._
- **Why does `Create()` connect `Community 1` to `Community 0`, `Community 8`, `Community 2`, `Community 6`?**
  _High betweenness centrality (0.064) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `Create()` (e.g. with `TestCreateRejectsSymlinkedProfilesDir()` and `ValidateName()`) actually correct?**
  _`Create()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 13 inferred relationships involving `Load()` (e.g. with `ConfigDir()` and `ReadFileInDir()`) actually correct?**
  _`Load()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **What connects `kubeConfig`, `clusterEntry`, `clusterData` to the rest of the system?**
  _7 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.13 - nodes in this community are weakly interconnected._