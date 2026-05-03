// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"

	"github.com/spf13/cobra"
)

func setupUnlockTestEnv(t *testing.T, profiles ...string) (*profile.Manager, string) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	dataDir := t.TempDir()
	manager, err := profile.NewManager(&config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "profiles"), 0700); err != nil {
		t.Fatalf("create profiles dir: %v", err)
	}
	for _, name := range profiles {
		path := filepath.Join(dataDir, "profiles", name+".yaml.gpg")
		if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: Config\nclusters: []\nusers: []\ncontexts: []\n"), 0600); err != nil {
			t.Fatalf("write profile %q: %v", name, err)
		}
	}

	gpgDir := t.TempDir()
	for _, n := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(gpgDir, n), []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
			t.Fatalf("write fake %s: %v", n, err)
		}
	}
	t.Setenv("PATH", gpgDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldMgr, oldCfg := mgr, cfg
	mgr = manager
	cfg = &config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"}
	t.Cleanup(func() { mgr = oldMgr; cfg = oldCfg })

	return manager, os.Getenv("XDG_RUNTIME_DIR")
}

func TestUnlockZeroArgsShowsHelp(t *testing.T) {
	_, runtimeDir := setupUnlockTestEnv(t)

	if err := unlockCmd.RunE(unlockCmd, nil); err != nil {
		t.Fatalf("unlock RunE error = %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(runtimeDir, "kubegonfig"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir runtime: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("runtime dir has %d entries after zero-arg invocation, want 0", len(entries))
	}
}

func TestUnlockThreeProfilesWritesAllFiles(t *testing.T) {
	_, runtimeDir := setupUnlockTestEnv(t, "alpha", "beta", "gamma")

	if err := unlockCmd.RunE(unlockCmd, []string{"alpha", "beta", "gamma"}); err != nil {
		t.Fatalf("unlock RunE error = %v", err)
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		path := filepath.Join(runtimeDir, "kubegonfig", name+".yaml")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %q after unlock: %v", path, err)
		}
	}
}

func TestUnlockDedupesRepeatedNames(t *testing.T) {
	_, runtimeDir := setupUnlockTestEnv(t, "alpha", "beta")

	if err := unlockCmd.RunE(unlockCmd, []string{"alpha", "alpha", "beta"}); err != nil {
		t.Fatalf("unlock RunE error = %v", err)
	}

	for _, name := range []string{"alpha", "beta"} {
		path := filepath.Join(runtimeDir, "kubegonfig", name+".yaml")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %q after dedupe unlock: %v", path, err)
		}
	}

	entries, _ := os.ReadDir(filepath.Join(runtimeDir, "kubegonfig"))
	if len(entries) != 2 {
		t.Errorf("runtime dir contains %d entries, want 2", len(entries))
	}
}

func TestUnlockPartialFailureKeepsSucceededFiles(t *testing.T) {
	_, runtimeDir := setupUnlockTestEnv(t, "alpha", "beta")

	err := unlockCmd.RunE(unlockCmd, []string{"alpha", "missing", "beta"})
	if err == nil {
		t.Fatal("unlock RunE error = nil, want missing-profile error")
	}
	if !strings.Contains(err.Error(), `unlock "missing"`) {
		t.Errorf("error = %v, want substring %q", err, `unlock "missing"`)
	}
	if !strings.Contains(err.Error(), `profile "missing" not found`) {
		t.Errorf("error = %v, want wrapped not-found", err)
	}

	for _, name := range []string{"alpha", "beta"} {
		path := filepath.Join(runtimeDir, "kubegonfig", name+".yaml")
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected file %q to remain after partial failure: %v", path, statErr)
		}
	}
}

func TestUnlockInvalidNameInMiddleStillUnlocksOthers(t *testing.T) {
	_, runtimeDir := setupUnlockTestEnv(t, "alpha", "beta")

	err := unlockCmd.RunE(unlockCmd, []string{"alpha", "!!!", "beta"})
	if err == nil {
		t.Fatal("unlock RunE error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), `unlock "!!!"`) {
		t.Errorf("error = %v, want substring %q", err, `unlock "!!!"`)
	}

	for _, name := range []string{"alpha", "beta"} {
		path := filepath.Join(runtimeDir, "kubegonfig", name+".yaml")
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected file %q to remain after invalid-name failure: %v", path, statErr)
		}
	}

	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) {
		t.Logf("note: errors.As(joined) returned false; the test only requires the message check above")
	}
}

func TestUnlockCommandCompletesAtEveryPosition(t *testing.T) {
	setupCompletionProfiles(t, "alpha", "beta")

	for _, args := range [][]string{nil, {"alpha"}, {"alpha", "beta"}} {
		got, directive := unlockCmd.ValidArgsFunction(unlockCmd, args, "")
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("args=%v: directive = %v, want NoFileComp", args, directive)
		}
		want := []string{"alpha", "beta"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("args=%v: completion = %v, want %v", args, got, want)
		}
	}
}
