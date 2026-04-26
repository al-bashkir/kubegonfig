// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
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

func TestListProfilesSkipsInvalidAndSpecialEntries(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	if err := os.WriteFile(filepath.Join(m.profilesPath(), "bad name"+gpgExt), []byte("ignore"), 0600); err != nil {
		t.Fatalf("write invalid profile name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(m.profilesPath(), gpgExt), []byte("ignore"), 0600); err != nil {
		t.Fatalf("write empty profile name: %v", err)
	}
	if err := os.WriteFile(filepath.Join(m.profilesPath(), "target"+gpgExt), []byte("ignore"), 0600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(filepath.Join(m.profilesPath(), "target"+gpgExt), filepath.Join(m.profilesPath(), "link"+gpgExt)); err != nil {
		t.Skipf("profile file symlink unavailable: %v", err)
	}
	if err := syscall.Mkfifo(filepath.Join(m.profilesPath(), "fifo"+gpgExt), 0600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	got, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	want := []string{"prod", "target"}
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

func TestSetCurrentRejectsMissingProfile(t *testing.T) {
	m := newTestManager(t)

	if err := m.SetCurrent("missing"); err == nil {
		t.Fatal("SetCurrent() error = nil, want missing profile error")
	}
	got, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if got != "" {
		t.Fatalf("GetCurrent() = %q, want empty", got)
	}
}

func TestDecryptMissingProfile(t *testing.T) {
	m := newTestManager(t)

	if _, err := m.Decrypt("missing"); err == nil {
		t.Fatal("Decrypt() error = nil, want missing profile error")
	}
}

func TestCreateRejectsSymlinkedProfilesDir(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "existing")
	replaceProfilesDirWithSymlink(t, m)

	if err := m.Create("new", validKubeconfig()); err == nil {
		t.Fatal("Create() error = nil, want symlinked profiles dir error")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.profilesPath()), "profiles-target", profileFileName("new"))); err == nil {
		t.Fatal("new profile was created through symlinked profiles dir")
	}
}

func TestUpdateRejectsSymlinkedProfilesDir(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	replaceProfilesDirWithSymlink(t, m)

	err := m.Update("prod", validKubeconfig())
	if err == nil {
		t.Fatal("Update() error = nil, want symlinked profiles dir error")
	}
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf("Update() error = %v, want storage error instead of not found", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.profilesPath()), "profiles-target", profileFileName("prod"))); err != nil {
		t.Fatalf("profile in symlink target was changed or removed: %v", err)
	}
}

func TestGetCurrentRejectsInvalidState(t *testing.T) {
	m := newTestManager(t)
	if err := os.MkdirAll(m.statePath(), 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(m.currentPath(), []byte("../prod\n"), 0600); err != nil {
		t.Fatalf("write current state: %v", err)
	}

	if _, err := m.GetCurrent(); err == nil {
		t.Fatal("GetCurrent() error = nil, want invalid state error")
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

func TestRenameRejectsInvalidCurrentState(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "old")
	writeInvalidCurrentState(t, m)

	if err := m.Rename("old", "new"); err == nil {
		t.Fatal("Rename() error = nil, want invalid current state error")
	}
	if !m.Exists("old") {
		t.Error("old profile was removed despite invalid current state")
	}
	if m.Exists("new") {
		t.Error("new profile was created despite invalid current state")
	}
}

func TestRenameRejectsSymlinkedProfilesDir(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "old")
	replaceProfilesDirWithSymlink(t, m)

	if err := m.Rename("old", "new"); err == nil {
		t.Fatal("Rename() error = nil, want symlinked profiles dir error")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.profilesPath()), "profiles-target", profileFileName("old"))); err != nil {
		t.Fatalf("old profile in symlink target was moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.profilesPath()), "profiles-target", profileFileName("new"))); err == nil {
		t.Fatal("new profile was created through symlinked profiles dir")
	}
}

func TestRenameRejectsSymlinkedStateDirBeforeMutation(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "old")

	if err := m.SetCurrent("old"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	replaceStateDirWithSymlink(t, m)

	if err := m.Rename("old", "new"); err == nil {
		t.Fatal("Rename() error = nil, want current update error")
	}
	if !m.Exists("old") {
		t.Error("old profile was not restored after current update failure")
	}
	if m.Exists("new") {
		t.Error("new profile was created despite symlinked state dir")
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

func TestDeleteRejectsInvalidCurrentState(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	writeInvalidCurrentState(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want invalid current state error")
	}
	if !m.Exists("prod") {
		t.Error("profile was deleted despite invalid current state")
	}
}

func TestDeleteRejectsSymlinkedStateDir(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	if err := m.SetCurrent("prod"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	replaceStateDirWithSymlink(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want symlinked state dir error")
	}
	if !m.Exists("prod") {
		t.Error("profile was deleted despite symlinked state dir")
	}
	if _, err := os.Stat(m.currentPath()); err != nil {
		t.Fatalf("current state was removed through symlink: %v", err)
	}
}

func TestDeleteRejectsSymlinkedProfilesDir(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	replaceProfilesDirWithSymlink(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want symlinked profiles dir error")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.profilesPath()), "profiles-target", profileFileName("prod"))); err != nil {
		t.Fatalf("profile was removed through symlinked profiles dir: %v", err)
	}
}

func TestDeleteReportsCurrentClearFailure(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	if err := m.SetCurrent("prod"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	makeStateDirReadOnly(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want current clear error")
	}
	if !m.Exists("prod") {
		t.Error("profile was deleted despite current clear failure")
	}
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if current != "prod" {
		t.Errorf("current after failed delete = %q, want prod", current)
	}
}

func TestDeleteRestoresCurrentWhenProfileRemovalFails(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	if err := m.SetCurrent("prod"); err != nil {
		t.Fatalf("SetCurrent() error: %v", err)
	}
	makeProfilesDirReadOnly(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want profile removal error")
	}
	if !m.Exists("prod") {
		t.Error("profile was deleted despite profile removal error")
	}
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if current != "prod" {
		t.Errorf("current after failed profile removal = %q, want prod", current)
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

func validKubeconfig() []byte {
	return []byte(`apiVersion: v1
kind: Config
clusters:
  - name: cluster
    cluster:
      server: https://example.com
users:
  - name: user
    user: {}
contexts:
  - name: context
    context:
      cluster: cluster
      user: user
current-context: context
`)
}

func writeInvalidCurrentState(t *testing.T, m *Manager) {
	t.Helper()
	if err := os.MkdirAll(m.statePath(), 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(m.currentPath(), []byte("../prod\n"), 0600); err != nil {
		t.Fatalf("write current state: %v", err)
	}
}

func makeStateDirReadOnly(t *testing.T, m *Manager) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission failure test requires a non-root user")
	}
	if err := os.Chmod(m.statePath(), 0500); err != nil {
		t.Fatalf("chmod state dir read-only: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(m.statePath(), 0700)
	})
}

func makeProfilesDirReadOnly(t *testing.T, m *Manager) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission failure test requires a non-root user")
	}
	if err := os.Chmod(m.profilesPath(), 0500); err != nil {
		t.Fatalf("chmod profiles dir read-only: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(m.profilesPath(), 0700)
	})
}

func replaceStateDirWithSymlink(t *testing.T, m *Manager) {
	t.Helper()

	data, err := os.ReadFile(m.currentPath())
	if err != nil {
		t.Fatalf("read current state before symlink replacement: %v", err)
	}
	target := filepath.Join(t.TempDir(), "state-target")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("create state symlink target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, currentFile), data, 0600); err != nil {
		t.Fatalf("write current state in symlink target: %v", err)
	}
	if err := os.RemoveAll(m.statePath()); err != nil {
		t.Fatalf("remove original state dir: %v", err)
	}
	if err := os.Symlink(target, m.statePath()); err != nil {
		t.Skipf("state dir symlink unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(m.statePath())
	})
}

func replaceProfilesDirWithSymlink(t *testing.T, m *Manager) {
	t.Helper()

	target := filepath.Join(filepath.Dir(m.profilesPath()), "profiles-target")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("create profiles symlink target: %v", err)
	}
	entries, err := os.ReadDir(m.profilesPath())
	if err != nil {
		t.Fatalf("read profiles before symlink replacement: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.profilesPath(), entry.Name()))
		if err != nil {
			t.Fatalf("read profile before symlink replacement: %v", err)
		}
		if err := os.WriteFile(filepath.Join(target, entry.Name()), data, 0600); err != nil {
			t.Fatalf("write profile in symlink target: %v", err)
		}
	}
	if err := os.RemoveAll(m.profilesPath()); err != nil {
		t.Fatalf("remove original profiles dir: %v", err)
	}
	if err := os.Symlink(target, m.profilesPath()); err != nil {
		t.Skipf("profiles dir symlink unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(m.profilesPath())
	})
}
