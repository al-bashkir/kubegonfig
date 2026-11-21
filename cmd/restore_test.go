// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kubegonfig/internal/bundle"
	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"
)

func TestRestoreCmd_RequiresArchiveArg(t *testing.T) {
	c := &config.Config{DataDir: t.TempDir(), GPGRecipient: "x@example.com"}
	c.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = c.Save()
	mgrLocal, err := profile.NewManager(c)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	saveAndRestoreState(t, c, mgrLocal)

	if err := restoreCmd.Args(restoreCmd, nil); err == nil {
		t.Fatal("restore Args = nil, want missing-archive error")
	}
}

func TestRestoreCmd_ForceAndSkipMutuallyExclusive(t *testing.T) {
	c := &config.Config{DataDir: t.TempDir(), GPGRecipient: "x@example.com"}
	c.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = c.Save()
	mgrLocal, _ := profile.NewManager(c)
	saveAndRestoreState(t, c, mgrLocal)

	restoreForce, restoreSkipExisting = true, true
	t.Cleanup(func() { restoreForce, restoreSkipExisting = false, false })

	err := restoreCmd.RunE(restoreCmd, []string{"/tmp/whatever.tar"})
	if err == nil {
		t.Fatal("restore RunE error = nil, want mutual-exclusion error")
	}
}

func TestRestoreCmd_RoundTripFromFile(t *testing.T) {
	srcDataDir := t.TempDir()
	srcCfg := &config.Config{DataDir: srcDataDir, GPGRecipient: "x@example.com"}
	srcCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = srcCfg.Save()
	srcMgr, _ := profile.NewManager(srcCfg)
	if err := srcMgr.WriteEncrypted("alpha", []byte("alpha-cipher")); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	var buf bytes.Buffer
	if err := bundle.Export(srcMgr, srcCfg, bundle.ExportOptions{
		Names: []string{"alpha"}, KubegonfigVersion: "test", Out: &buf,
	}); err != nil {
		t.Fatalf("Export(): %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "in.tar")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	dstDataDir := t.TempDir()
	dstCfg := &config.Config{DataDir: dstDataDir, GPGRecipient: "x@example.com"}
	dstCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = dstCfg.Save()
	dstMgr, _ := profile.NewManager(dstCfg)
	saveAndRestoreState(t, dstCfg, dstMgr)

	if err := restoreCmd.RunE(restoreCmd, []string{archivePath}); err != nil {
		t.Fatalf("restore RunE error: %v", err)
	}
	got, err := dstMgr.ReadEncrypted("alpha")
	if err != nil {
		t.Fatalf("ReadEncrypted(): %v", err)
	}
	if string(got) != "alpha-cipher" {
		t.Errorf("restored body = %q, want %q", got, "alpha-cipher")
	}
}

func TestRestoreCmd_MergeConfigNamesAddedRecipients(t *testing.T) {
	cmdTestEnv(t)
	srcCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "src@example.com", GPGRecipients: []string{"shared@example.com"}}
	srcCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = srcCfg.Save()
	srcMgr, _ := profile.NewManager(srcCfg)
	if err := srcMgr.WriteEncrypted("alpha", []byte("alpha-cipher")); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	var buf bytes.Buffer
	if err := bundle.Export(srcMgr, srcCfg, bundle.ExportOptions{
		Names: []string{"alpha"}, IncludeConfig: true, KubegonfigVersion: "test", Out: &buf,
	}); err != nil {
		t.Fatalf("Export(): %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "in.tar")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	_, stderr, err := cmdTestRun(t, "", "restore", "--merge-config", archivePath)
	if err != nil {
		t.Fatalf("restore error: %v", err)
	}
	if !strings.Contains(stderr, "added recipient shared@example.com") {
		t.Errorf("stderr = %q, want the added recipient named", stderr)
	}
}

func TestRestoreCmd_MergeConfigNamesAddedRecipientsEvenWhenAnotherFails(t *testing.T) {
	cmdTestEnv(t)
	// Resolves good@ but not unknown@; placed before the pass-through gpg.
	gpgDir := t.TempDir()
	script := "#!/bin/sh\ncase \"$*\" in *--list-keys*unknown@*) echo 'gpg: No public key' >&2; exit 2;; esac\ncat\n"
	for _, n := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(gpgDir, n), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", gpgDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	srcCfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "src@example.com", GPGRecipients: []string{"unknown@example.com", "good@example.com"}}
	srcCfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	_ = srcCfg.Save()
	srcMgr, _ := profile.NewManager(srcCfg)
	if err := srcMgr.WriteEncrypted("alpha", []byte("alpha-cipher")); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	var buf bytes.Buffer
	if err := bundle.Export(srcMgr, srcCfg, bundle.ExportOptions{
		Names: []string{"alpha"}, IncludeConfig: true, KubegonfigVersion: "test", Out: &buf,
	}); err != nil {
		t.Fatalf("Export(): %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "in.tar")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	_, stderr, err := cmdTestRun(t, "", "restore", "--merge-config", archivePath)
	if err == nil {
		t.Fatal("restore error = nil, want the unmerged recipient reported")
	}
	if !strings.Contains(stderr, "added recipient good@example.com") {
		t.Errorf("stderr = %q, want the recipient that was added to the config named", stderr)
	}
}
