// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"
)

func TestListRejectsInvalidCurrentState(t *testing.T) {
	dataDir := t.TempDir()
	manager, err := profile.NewManager(&config.Config{DataDir: dataDir, GPGRecipient: "test@example.com"})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}

	oldMgr := mgr
	mgr = manager
	t.Cleanup(func() { mgr = oldMgr })

	if err := os.MkdirAll(filepath.Join(dataDir, "profiles"), 0700); err != nil {
		t.Fatalf("create profiles dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "profiles", "prod.yaml.gpg"), []byte("encrypted"), 0600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "state"), 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state", "current"), []byte("../prod\n"), 0600); err != nil {
		t.Fatalf("write current state: %v", err)
	}

	err = listCmd.RunE(listCmd, nil)
	if err == nil {
		t.Fatal("list RunE error = nil, want invalid current state error")
	}
	if !strings.Contains(err.Error(), "invalid current profile state") {
		t.Fatalf("list RunE error = %v, want invalid current state", err)
	}
}
