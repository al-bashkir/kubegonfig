// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readStoredForTest(t *testing.T, m *Manager, name string) string {
	t.Helper()
	data, err := os.ReadFile(profilePathForTest(m, name))
	if err != nil {
		t.Fatalf("read stored profile %q: %v", name, err)
	}
	return string(data)
}

func TestCreateStoresEncryptedProfile(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)

	if err := m.Create("prod", validKubeconfig()); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if !profileExistsForTest(t, m, "prod") {
		t.Fatal("profile does not exist after Create()")
	}
	got, err := m.Decrypt("prod")
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if string(got) != string(validKubeconfig()) {
		t.Errorf("Decrypt() = %q, want created content", got)
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	installFakeGPG(t)
	tests := []struct {
		name, profile, wantErr string
		data                   []byte
		noRecipient            bool
	}{
		{"invalid kubeconfig", "prod", "invalid kubeconfig", []byte("not: a kubeconfig"), false},
		{"invalid name", "bad name", "", validKubeconfig(), false},
		{"no recipient", "prod", "", validKubeconfig(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestManager(t)
			if tt.noRecipient {
				m.cfg.GPGRecipient = ""
			}
			err := m.Create(tt.profile, tt.data)
			if err == nil {
				t.Fatal("Create() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Create() error = %v, want containing %q", err, tt.wantErr)
			}
			if names, _ := m.List(); len(names) != 0 {
				t.Errorf("profiles after failed Create() = %v, want none", names)
			}
		})
	}
}

func TestCreateRejectsExistingProfile(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	err := m.Create("prod", validKubeconfig())
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Create() error = %v, want already exists", err)
	}
	if got := readStoredForTest(t, m, "prod"); got != "encrypted" {
		t.Errorf("existing profile = %q, want untouched", got)
	}
}

func TestEditStoresEditedContent(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	edited := strings.Replace(string(validKubeconfig()), "example.com", "edited.example.com", 1)

	changed, err := m.Edit("prod", func([]byte) ([]byte, error) { return []byte(edited), nil })
	if err != nil || !changed {
		t.Fatalf("Edit() = %v, %v, want true, nil", changed, err)
	}
	if got := readStoredForTest(t, m, "prod"); got != edited {
		t.Errorf("stored profile = %q, want edited content", got)
	}
}

func TestEditLeavesProfileUnchanged(t *testing.T) {
	installFakeGPG(t)
	original := string(validKubeconfig())
	tests := []struct {
		name, wantErr string
		edit          func([]byte) ([]byte, error)
	}{
		{"edit error", "boom", func([]byte) ([]byte, error) { return nil, fmt.Errorf("boom") }},
		{"same content", "", func(b []byte) ([]byte, error) { return b, nil }},
		{"invalid kubeconfig", "invalid kubeconfig", func([]byte) ([]byte, error) { return []byte("not: a kubeconfig"), nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestManager(t)
			writePlaintextProfile(t, m, "prod")

			changed, err := m.Edit("prod", tt.edit)
			if changed {
				t.Error("Edit() changed = true, want false")
			}
			if (err == nil) != (tt.wantErr == "") || (err != nil && !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("Edit() error = %v, want containing %q", err, tt.wantErr)
			}
			if got := readStoredForTest(t, m, "prod"); got != original {
				t.Errorf("stored profile = %q, want unchanged", got)
			}
		})
	}
}

func TestEditMissingProfile(t *testing.T) {
	m := newTestManager(t)
	called := false
	changed, err := m.Edit("missing", func(b []byte) ([]byte, error) { called = true; return b, nil })
	if err == nil || changed || called {
		t.Fatalf("Edit() = %v, %v (called %v), want error without calling edit", changed, err, called)
	}
}

func TestEditRejectsInvalidName(t *testing.T) {
	m := newTestManager(t)
	changed, err := m.Edit("../bad", func(b []byte) ([]byte, error) { return b, nil })
	if err == nil || changed {
		t.Fatalf("Edit() = %v, %v, want false and error", changed, err)
	}
}

func TestEditRejectsMissingRecipient(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	m.cfg.GPGRecipient = ""

	changed, err := m.Edit("prod", func([]byte) ([]byte, error) {
		return []byte(strings.Replace(string(validKubeconfig()), "example.com", "other.example.com", 1)), nil
	})
	if err == nil || changed {
		t.Fatalf("Edit() = %v, %v, want false and error", changed, err)
	}
	if got := readStoredForTest(t, m, "prod"); got != string(validKubeconfig()) {
		t.Errorf("stored profile = %q, want unchanged", got)
	}
}

func TestExistsInvalidNameReturnsFalse(t *testing.T) {
	m := newTestManager(t)
	if ok, err := m.Exists("../bad"); ok || err == nil {
		t.Fatalf("Exists() = %v, %v, want false and error", ok, err)
	}
}

func TestRenameRejectsBadNamesAndConflicts(t *testing.T) {
	tests := []struct {
		name, from, to, wantErr string
	}{
		{"invalid old", "../bad", "new", "old name"},
		{"invalid new", "old", "../bad", "new name"},
		{"missing old", "ghost", "new", `"ghost" not found`},
		{"new exists", "old", "taken", `"taken" already exists`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestManager(t)
			writeProfile(t, m, "old")
			writeProfile(t, m, "taken")

			err := m.Rename(tt.from, tt.to)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Rename() error = %v, want containing %q", err, tt.wantErr)
			}
			if names, _ := m.List(); strings.Join(names, ",") != "old,taken" {
				t.Errorf("profiles after failed Rename() = %v, want old,taken", names)
			}
		})
	}
}

func TestRenameKeepsOtherCurrent(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "old")
	writeProfile(t, m, "other")
	setCurrentForTest(t, m, "other")

	if err := m.Rename("old", "new"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}
	if current, _ := m.GetCurrent(); current != "other" {
		t.Errorf("current after renaming another profile = %q, want other", current)
	}
}

func TestActivateDecryptsAndRecordsCurrent(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	path, err := m.Activate("prod")
	if err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
	assertFileContent(t, path, string(validKubeconfig()))
	if current, _ := m.GetCurrent(); current != "prod" {
		t.Errorf("current after Activate() = %q, want prod", current)
	}
}

func TestActivateReportsDecryptFailure(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	t.Setenv("PATH", t.TempDir()) // no gpg available

	if _, err := m.Activate("prod"); err == nil {
		t.Fatal("Activate() error = nil, want decrypt error")
	}
	if current, _ := m.GetCurrent(); current != "" {
		t.Errorf("current after failed Activate() = %q, want empty", current)
	}
}

func TestActivateReportsRuntimeDirFailure(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	bad := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(bad, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", bad)

	if _, err := m.Activate("prod"); err == nil {
		t.Fatal("Activate() error = nil, want runtime dir error")
	}
	if current, _ := m.GetCurrent(); current != "" {
		t.Errorf("current after failed Activate() = %q, want empty", current)
	}
}

func TestActivateErrors(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Activate("../bad"); err == nil {
		t.Error("Activate(invalid name) error = nil")
	}
	if path, err := m.Activate("missing"); err == nil || path != "" {
		t.Errorf("Activate(missing) = %q, %v, want empty path and error", path, err)
	}
}
