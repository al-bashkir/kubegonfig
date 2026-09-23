// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kubegonfig/internal/bundle"
	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"
)

func setupExportTestManager(t *testing.T, dataDir string) (*config.Config, *profile.Manager) {
	t.Helper()
	c := &config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"}
	c.Path = filepath.Join(t.TempDir(), "config.yaml")
	if err := c.Save(); err != nil {
		t.Fatalf("save cfg: %v", err)
	}
	m, err := profile.NewManager(c)
	if err != nil {
		t.Fatalf("NewManager(): %v", err)
	}
	return c, m
}

// saveAndRestoreState swaps the package-level cfg / mgr for a test instance
// and restores the originals on cleanup.
func saveAndRestoreState(t *testing.T, c *config.Config, m *profile.Manager) {
	t.Helper()
	oldCfg, oldMgr := cfg, mgr
	cfg, mgr = c, m
	t.Cleanup(func() { cfg, mgr = oldCfg, oldMgr })
}

func TestExportCmd_RequiresOutput(t *testing.T) {
	dataDir := t.TempDir()
	c, m := setupExportTestManager(t, dataDir)
	saveAndRestoreState(t, c, m)

	exportAll = true
	t.Cleanup(func() { exportAll = false })

	err := exportCmd.RunE(exportCmd, nil)
	if err == nil {
		t.Fatal("export RunE error = nil, want missing --output error")
	}
}

func TestExportCmd_AllAndArgsMutuallyExclusive(t *testing.T) {
	dataDir := t.TempDir()
	c, m := setupExportTestManager(t, dataDir)
	saveAndRestoreState(t, c, m)

	exportOutput = "-"
	exportAll = true
	t.Cleanup(func() { exportOutput = ""; exportAll = false })

	err := exportCmd.RunE(exportCmd, []string{"alpha"})
	if err == nil {
		t.Fatal("export RunE error = nil, want mutual-exclusion error")
	}
}

func TestExportCmd_NoArgsNoAllFails(t *testing.T) {
	dataDir := t.TempDir()
	c, m := setupExportTestManager(t, dataDir)
	saveAndRestoreState(t, c, m)

	exportOutput = "-"
	t.Cleanup(func() { exportOutput = "" })

	err := exportCmd.RunE(exportCmd, nil)
	if err == nil {
		t.Fatal("export RunE error = nil, want missing-selection error")
	}
}

func TestExportCmd_ToFileWritesArchive(t *testing.T) {
	dataDir := t.TempDir()
	c, m := setupExportTestManager(t, dataDir)
	if err := m.WriteEncrypted("alpha", []byte("a")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	saveAndRestoreState(t, c, m)

	outDir := t.TempDir()
	out := filepath.Join(outDir, "backup.tar")
	exportOutput = out
	exportAll = true
	t.Cleanup(func() { exportOutput = ""; exportAll = false })

	if err := exportCmd.RunE(exportCmd, nil); err != nil {
		t.Fatalf("export RunE error: %v", err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	tr := tar.NewReader(bytes.NewReader(body))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	if hdr.Name != bundle.ManifestName {
		t.Errorf("first entry = %q, want %q", hdr.Name, bundle.ManifestName)
	}
}

func TestExportCmd_DecryptRefusedInsideDataDir(t *testing.T) {
	dataDir := t.TempDir()
	c, m := setupExportTestManager(t, dataDir)
	if err := m.WriteEncrypted("alpha", []byte("a")); err != nil {
		t.Fatalf("seed: %v", err)
	}
	saveAndRestoreState(t, c, m)

	outFile := filepath.Join(dataDir, "leaked.tar")
	exportOutput = outFile
	exportAll = true
	exportDecrypt = true
	t.Cleanup(func() {
		exportOutput = ""
		exportAll = false
		exportDecrypt = false
	})

	err := exportCmd.RunE(exportCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "data directory") {
		t.Fatalf("export RunE error = %v, want plaintext-into-data-dir error", err)
	}
	if _, statErr := os.Stat(outFile); statErr == nil {
		t.Error("plaintext archive file created despite guard")
	}
}
