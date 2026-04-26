// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"kubegonfig/internal/config"
)

func TestListProfiles(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "beta")
	writeProfile(t, m, "alpha")

	if err := os.WriteFile(filepath.Join(m.profilesPath(), "notes.txt"), []byte("ignore"), 0600); err != nil {
		t.Fatalf("setup notes file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(m.profilesPath(), "dir.yaml.gpg"), 0700); err != nil {
		t.Fatalf("setup profile-like dir: %v", err)
	}

	got, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}
}

func TestSetAndGetCurrent(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	got, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() without current error: %v", err)
	}
	if got != "" {
		t.Fatalf("GetCurrent() without current = %q, want empty", got)
	}

	if err := m.SetCurrent("prod"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	got, err = m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if got != "prod" {
		t.Errorf("GetCurrent() = %q, want prod", got)
	}
}

func TestRenameUpdatesCurrent(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "old")

	if err := m.SetCurrent("old"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	if err := m.Rename("old", "new"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}
	if m.Exists("old") {
		t.Error("old profile still exists after rename")
	}
	if !m.Exists("new") {
		t.Error("new profile does not exist after rename")
	}
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if current != "new" {
		t.Errorf("current after rename = %q, want new", current)
	}
}

func TestDeleteClearsCurrent(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	if err := m.SetCurrent("prod"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	if err := m.Delete("prod"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if m.Exists("prod") {
		t.Error("profile still exists after delete")
	}
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if current != "" {
		t.Errorf("current after deleting active profile = %q, want empty", current)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(&config.Config{DataDir: t.TempDir(), GPGRecipient: "test@example.com"})
	if err != nil {
		t.Fatalf("NewManager() error: %v", err)
	}
	return m
}

func writeProfile(t *testing.T, m *Manager, name string) {
	t.Helper()
	if err := os.MkdirAll(m.profilesPath(), 0700); err != nil {
		t.Fatalf("create profiles dir: %v", err)
	}
	if err := os.WriteFile(m.profilePath(name), []byte("encrypted"), 0600); err != nil {
		t.Fatalf("write profile %q: %v", name, err)
	}
}
