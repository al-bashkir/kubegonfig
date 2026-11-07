// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kubegonfig/internal/bundle"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const cmdTestBody = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster: {server: "https://example.invalid"}
users:
- name: u
  user: {}
contexts:
- name: x
  context: {cluster: c, user: u}
current-context: x
`

// cmdTestEnv isolates XDG dirs, installs a pass-through fake gpg, writes a
// config and seeds profiles. It returns the data dir.
func cmdTestEnv(t *testing.T, profiles ...string) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(base, "data"))
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	cfgDir := filepath.Join(base, "config", "kubegonfig")
	dataDir := filepath.Join(base, "data", "kubegonfig")
	for _, d := range []string{cfgDir, filepath.Join(dataDir, "profiles")} {
		if err := os.MkdirAll(d, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("gpg_recipient: t@example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range profiles {
		if err := os.WriteFile(filepath.Join(dataDir, "profiles", name+".yaml.gpg"), []byte(cmdTestBody), 0600); err != nil {
			t.Fatal(err)
		}
	}
	gpgDir := t.TempDir()
	for _, n := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(gpgDir, n), []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", gpgDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dataDir
}

// cmdTestRun executes the real command tree and resets flag state afterwards
// (cobra keeps flag values between Execute calls).
func cmdTestRun(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	rootCmd.SetArgs(args)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errb)
	oldCfg, oldMgr := cfg, mgr
	defer func() {
		cfg, mgr = oldCfg, oldMgr
		rootCmd.SetArgs(nil)
		rootCmd.SetIn(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		cmdTestResetFlags(rootCmd)
	}()
	err = rootCmd.Execute()
	return out.String(), errb.String(), err
}

func cmdTestResetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		cmdTestResetFlags(sub)
	}
}

func cmdTestTarNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var names []string
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
}

func cmdTestExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDeleteWithoutForcePrompts(t *testing.T) {
	dataDir := cmdTestEnv(t, "alpha")
	path := filepath.Join(dataDir, "profiles", "alpha.yaml.gpg")

	for _, answer := range []string{"n\n", ""} {
		_, stderr, err := cmdTestRun(t, answer, "delete", "alpha")
		if err != nil {
			t.Fatalf("delete error = %v", err)
		}
		if !strings.Contains(stderr, "[y/N]") || !strings.Contains(stderr, "Aborted") {
			t.Errorf("stderr = %q, want prompt and Aborted", stderr)
		}
		if !cmdTestExists(path) {
			t.Fatalf("profile deleted after answer %q", answer)
		}
	}

	if _, _, err := cmdTestRun(t, "yes\n", "delete", "alpha"); err != nil {
		t.Fatal(err)
	}
	if cmdTestExists(path) {
		t.Error("profile kept after yes")
	}
}

func TestDeleteForceSkipsPrompt(t *testing.T) {
	dataDir := cmdTestEnv(t, "alpha")
	_, stderr, err := cmdTestRun(t, "", "delete", "-f", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr, "[y/N]") {
		t.Errorf("stderr = %q, want no prompt", stderr)
	}
	if cmdTestExists(filepath.Join(dataDir, "profiles", "alpha.yaml.gpg")) {
		t.Error("profile kept after --force")
	}
}

func TestQuietSuppressesInfo(t *testing.T) {
	cmdTestEnv(t, "alpha")
	_, stderr, err := cmdTestRun(t, "", "delete", "-f", "alpha")
	if err != nil || !strings.Contains(stderr, `Profile "alpha" deleted`) {
		t.Fatalf("without -q: err = %v, stderr = %q, want info line", err, stderr)
	}

	cmdTestEnv(t, "alpha")
	_, stderr, err = cmdTestRun(t, "", "-q", "delete", "-f", "alpha")
	if err != nil || stderr != "" {
		t.Fatalf("with -q: err = %v, stderr = %q, want empty", err, stderr)
	}
}

func TestExportThroughCommand(t *testing.T) {
	cmdTestEnv(t, "alpha", "beta")
	dir := t.TempDir()

	// named profile, default includes config.yaml
	out := filepath.Join(dir, "named.tar")
	_, stderr, err := cmdTestRun(t, "", "export", "-o", out, "alpha")
	if err != nil {
		t.Fatalf("export named: %v", err)
	}
	if !strings.Contains(stderr, "Wrote "+out) {
		t.Errorf("stderr = %q, want Wrote line", stderr)
	}
	names := cmdTestTarNames(t, out)
	if !slices.Contains(names, bundle.ConfigName) || !slices.Contains(names, "profiles/alpha.yaml.gpg") || slices.Contains(names, "profiles/beta.yaml.gpg") {
		t.Errorf("named archive entries = %v", names)
	}

	// --profiles-only omits config.yaml
	out = filepath.Join(dir, "po.tar")
	if _, _, err := cmdTestRun(t, "", "export", "-o", out, "--profiles-only", "alpha"); err != nil {
		t.Fatal(err)
	}
	if names := cmdTestTarNames(t, out); slices.Contains(names, bundle.ConfigName) {
		t.Errorf("--profiles-only entries = %v, want no config.yaml", names)
	}

	// --all gets every profile
	out = filepath.Join(dir, "all.tar")
	if _, _, err := cmdTestRun(t, "", "export", "-o", out, "--all"); err != nil {
		t.Fatal(err)
	}
	if names := cmdTestTarNames(t, out); !slices.Contains(names, "profiles/alpha.yaml.gpg") || !slices.Contains(names, "profiles/beta.yaml.gpg") {
		t.Errorf("--all entries = %v", names)
	}
}

func TestExportAllWithOneNameRefused(t *testing.T) {
	cmdTestEnv(t, "alpha")
	out := filepath.Join(t.TempDir(), "x.tar")
	_, _, err := cmdTestRun(t, "", "export", "-o", out, "--all", "alpha")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutually exclusive", err)
	}
}

func TestExportNoSelectionRefused(t *testing.T) {
	cmdTestEnv(t, "alpha")
	out := filepath.Join(t.TempDir(), "x.tar")
	_, _, err := cmdTestRun(t, "", "export", "-o", out)
	if err == nil || !strings.Contains(err.Error(), "specify profile names or --all") {
		t.Fatalf("error = %v, want specify profile names", err)
	}
}

func TestExportAllNoProfilesFails(t *testing.T) {
	cmdTestEnv(t)
	out := filepath.Join(t.TempDir(), "x.tar")
	_, _, err := cmdTestRun(t, "", "export", "-o", out, "--all")
	if err == nil || !strings.Contains(err.Error(), "no profiles to export") {
		t.Fatalf("error = %v, want no profiles", err)
	}
}

func TestExportUnknownProfileLeavesNoFile(t *testing.T) {
	cmdTestEnv(t, "alpha")
	dir := t.TempDir()
	out := filepath.Join(dir, "x.tar")
	_, stderr, err := cmdTestRun(t, "", "export", "-o", out, "missing")
	if err == nil {
		t.Fatal("error = nil, want unknown-profile error")
	}
	if strings.Contains(stderr, "Wrote") || cmdTestExists(out) {
		t.Errorf("output left behind or success printed: stderr = %q", stderr)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("dir has %d leftover entries", len(entries))
	}
}

func TestExportDecryptDestinationGuard(t *testing.T) {
	dataDir := cmdTestEnv(t, "alpha")

	out := filepath.Join(t.TempDir(), "plain.tar")
	if _, _, err := cmdTestRun(t, "", "export", "-o", out, "--decrypt", "alpha"); err != nil {
		t.Fatalf("decrypt outside data dir: %v", err)
	}
	if !cmdTestExists(out) {
		t.Error("archive not written outside data dir")
	}

	inside := filepath.Join(dataDir, "leak.tar")
	_, _, err := cmdTestRun(t, "", "export", "-o", inside, "--decrypt", "alpha")
	if err == nil || !strings.Contains(err.Error(), "data directory") {
		t.Fatalf("error = %v, want data directory refusal", err)
	}
	if cmdTestExists(inside) {
		t.Error("plaintext archive written inside data dir")
	}

	// a sibling that merely shares the prefix is not inside
	sibling := dataDir + "-other"
	if err := os.MkdirAll(sibling, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cmdTestRun(t, "", "export", "-o", filepath.Join(sibling, "p.tar"), "--decrypt", "alpha"); err != nil {
		t.Fatalf("decrypt into sibling dir: %v", err)
	}
}

func TestExportToStdout(t *testing.T) {
	cmdTestEnv(t, "alpha")
	stdout, stderr, err := cmdTestRun(t, "", "export", "-o", "-", "--decrypt", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	tr := tar.NewReader(strings.NewReader(stdout))
	var found bool
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		body, _ := io.ReadAll(tr)
		if strings.HasSuffix(h.Name, "alpha.yaml") && string(body) == cmdTestBody {
			found = true
		}
	}
	if !found {
		t.Error("stdout archive lacks plaintext alpha.yaml")
	}
}

func TestRestoreCollisionNeedsForce(t *testing.T) {
	cmdTestEnv(t, "alpha")
	archive := filepath.Join(t.TempDir(), "a.tar")
	if _, _, err := cmdTestRun(t, "", "export", "-o", archive, "alpha"); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := cmdTestRun(t, "", "restore", archive)
	if err == nil {
		t.Fatal("restore over existing profile: error = nil, want collision")
	}
	if strings.Contains(stderr, "Restored") {
		t.Errorf("stderr = %q, want no Restored line", stderr)
	}

	_, stderr, err = cmdTestRun(t, "", "restore", "--force", archive)
	if err != nil || !strings.Contains(stderr, "overwrote 1") {
		t.Fatalf("--force: err = %v, stderr = %q", err, stderr)
	}

	_, stderr, err = cmdTestRun(t, "", "restore", "--skip-existing", archive)
	if err != nil || !strings.Contains(stderr, "skipped 1") {
		t.Fatalf("--skip-existing: err = %v, stderr = %q", err, stderr)
	}
}

func TestRestoreMergeConfigFlag(t *testing.T) {
	cmdTestEnv(t, "alpha")
	archive := filepath.Join(t.TempDir(), "a.tar")
	if _, _, err := cmdTestRun(t, "", "export", "-o", archive, "alpha"); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := cmdTestRun(t, "", "restore", "--force", archive)
	if err != nil {
		t.Fatal(err)
	}
	withoutMerge := stderr
	_, stderr, err = cmdTestRun(t, "", "restore", "--force", "--merge-config", archive)
	if err != nil {
		t.Fatal(err)
	}
	if stderr == withoutMerge {
		t.Errorf("--merge-config did not change config action: %q", stderr)
	}
}

func TestExecRunsChildWithKubeconfig(t *testing.T) {
	cmdTestEnv(t, "alpha")
	out := filepath.Join(t.TempDir(), "seen")
	_, _, err := cmdTestRun(t, "", "exec", "alpha", "--", "sh", "-c", `cat "$KUBECONFIG" > "$1"`, "sh", out)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != cmdTestBody {
		t.Fatalf("child saw %q, err %v, want %q", got, err, cmdTestBody)
	}
	assertRuntimeDirEmpty(t)

	if _, _, err := cmdTestRun(t, "", "exec", "missing", "--", "true"); err == nil {
		t.Error("exec of missing profile: error = nil")
	}
}

func TestExecFailsWhenRuntimeDirUnusable(t *testing.T) {
	cmdTestEnv(t, "alpha")
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", blocker)
	if _, _, err := cmdTestRun(t, "", "exec", "alpha", "--", "true"); err == nil {
		t.Error("exec with unusable runtime dir: error = nil")
	}
}
