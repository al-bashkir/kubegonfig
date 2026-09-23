// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
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
