// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/storage"
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

func TestGetCurrentAfterStateWrite(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	got, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() without current error: %v", err)
	}
	if got != "" {
		t.Fatalf("GetCurrent() without current = %q, want empty", got)
	}

	setCurrentForTest(t, m, "prod")
	got, err = m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if got != "prod" {
		t.Errorf("GetCurrent() = %q, want prod", got)
	}
}

func TestDecryptMissingProfile(t *testing.T) {
	m := newTestManager(t)

	if _, err := m.Decrypt("missing"); err == nil {
		t.Fatal("Decrypt() error = nil, want missing profile error")
	}
}

func TestEditHoldsLockDuringCallback(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	entered := make(chan struct{})
	release := make(chan struct{})
	editDone := make(chan error, 1)
	go func() {
		changed, err := m.Edit("prod", func(original []byte) ([]byte, error) {
			close(entered)
			<-release
			return original, nil
		})
		if err == nil && changed {
			err = fmt.Errorf("Edit() changed profile, want no change")
		}
		editDone <- err
	}()

	waitForEntry(t, entered, editDone, "Edit callback")
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- m.WithLock(func() error { return nil })
	}()

	assertStillBlocked(t, lockDone, "WithLock while edit callback is active")
	close(release)
	if err := waitForResult(t, editDone, "Edit"); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if err := waitForResult(t, lockDone, "WithLock after edit callback exits"); err != nil {
		t.Fatalf("WithLock() error = %v", err)
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

func TestGetCurrentRejectsInvalidState(t *testing.T) {
	m := newTestManager(t)
	if err := os.MkdirAll(m.statePath(), 0700); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(currentPathForTest(m), []byte("../prod\n"), 0600); err != nil {
		t.Fatalf("write current state: %v", err)
	}

	if _, err := m.GetCurrent(); err == nil {
		t.Fatal("GetCurrent() error = nil, want invalid state error")
	}
}

func TestRenameUpdatesCurrent(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "old")

	setCurrentForTest(t, m, "old")
	if err := m.Rename("old", "new"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}
	if profileExistsForTest(t, m, "old") {
		t.Error("old profile still exists after rename")
	}
	if !profileExistsForTest(t, m, "new") {
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
	if !profileExistsForTest(t, m, "old") {
		t.Error("old profile was removed despite invalid current state")
	}
	if profileExistsForTest(t, m, "new") {
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

	setCurrentForTest(t, m, "old")
	replaceStateDirWithSymlink(t, m)

	if err := m.Rename("old", "new"); err == nil {
		t.Fatal("Rename() error = nil, want current update error")
	}
	if !profileExistsForTest(t, m, "old") {
		t.Error("old profile was not restored after current update failure")
	}
	if profileExistsForTest(t, m, "new") {
		t.Error("new profile was created despite symlinked state dir")
	}
}

func TestDeleteClearsCurrent(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	setCurrentForTest(t, m, "prod")
	if err := m.Delete("prod"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if profileExistsForTest(t, m, "prod") {
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
	if !profileExistsForTest(t, m, "prod") {
		t.Error("profile was deleted despite invalid current state")
	}
}

func TestDeleteRejectsSymlinkedStateDir(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want symlinked state dir error")
	}
	if !profileExistsForTest(t, m, "prod") {
		t.Error("profile was deleted despite symlinked state dir")
	}
	if _, err := os.Stat(currentPathForTest(m)); err != nil {
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

	setCurrentForTest(t, m, "prod")
	makeStateDirReadOnly(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want current clear error")
	}
	if !profileExistsForTest(t, m, "prod") {
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

func TestDeleteKeepsProfileWhenProfileRemovalFails(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	setCurrentForTest(t, m, "prod")
	makeProfilesDirReadOnly(t, m)

	if err := m.Delete("prod"); err == nil {
		t.Fatal("Delete() error = nil, want profile removal error")
	}
	if !profileExistsForTest(t, m, "prod") {
		t.Error("profile was deleted despite profile removal error")
	}
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	// No rollback: current was cleared before the profile removal failed.
	if current != "" {
		t.Errorf("current after failed profile removal = %q, want empty", current)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
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
	if err := os.WriteFile(profilePathForTest(m, name), []byte("encrypted"), 0600); err != nil {
		t.Fatalf("write profile %q: %v", name, err)
	}
}

func writePlaintextProfile(t *testing.T, m *Manager, name string) {
	t.Helper()
	if err := os.MkdirAll(m.profilesPath(), 0700); err != nil {
		t.Fatalf("create profiles dir: %v", err)
	}
	if err := os.WriteFile(profilePathForTest(m, name), validKubeconfig(), 0600); err != nil {
		t.Fatalf("write plaintext profile %q: %v", name, err)
	}
}

func setCurrentForTest(t *testing.T, m *Manager, name string) {
	t.Helper()
	if err := m.setCurrent(name); err != nil {
		t.Fatalf("setCurrent(%q) error: %v", name, err)
	}
}

func profileExistsForTest(t *testing.T, m *Manager, name string) bool {
	t.Helper()
	exists, err := m.Exists(name)
	if err != nil {
		t.Fatalf("Exists(%q) error: %v", name, err)
	}
	return exists
}

func installFakeGPG(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"gpg2", "gpg"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func waitForEntry(t *testing.T, entered <-chan struct{}, done <-chan error, op string) {
	t.Helper()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("%s exited before entry: %v", op, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", op)
	}
}

func assertStillBlocked(t *testing.T, ch <-chan error, op string) {
	t.Helper()
	select {
	case err := <-ch:
		t.Fatalf("%s completed while lock should be held: %v", op, err)
	case <-time.After(100 * time.Millisecond):
	}
}

func waitForResult(t *testing.T, ch <-chan error, op string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", op)
		return nil
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
	if err := os.WriteFile(currentPathForTest(m), []byte("../prod\n"), 0600); err != nil {
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

	data, err := os.ReadFile(currentPathForTest(m))
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

func profilePathForTest(m *Manager, name string) string {
	return filepath.Join(m.profilesPath(), profileFileName(name))
}

func currentPathForTest(m *Manager) string {
	return filepath.Join(m.statePath(), currentFile)
}

func TestManager_Exists_True(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "alpha")

	got, err := m.Exists("alpha")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if !got {
		t.Error("Exists(\"alpha\") = false, want true")
	}
}

func TestManager_Exists_False(t *testing.T) {
	m := newTestManager(t)
	got, err := m.Exists("missing")
	if err != nil {
		t.Fatalf("Exists() error: %v", err)
	}
	if got {
		t.Error("Exists(\"missing\") = true, want false")
	}
}

func TestManager_Exists_InvalidName(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Exists("../bad"); err == nil {
		t.Fatal("Exists() error = nil, want invalid name error")
	}
}

func TestManager_ReadEncrypted_ReturnsRawBlob(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "alpha")

	got, err := m.ReadEncrypted("alpha")
	if err != nil {
		t.Fatalf("ReadEncrypted() error: %v", err)
	}
	if string(got) != "encrypted" {
		t.Errorf("ReadEncrypted() = %q, want %q", got, "encrypted")
	}
}

func TestManager_ReadEncrypted_MissingProfile(t *testing.T) {
	m := newTestManager(t)
	_, err := m.ReadEncrypted("missing")
	if err == nil {
		t.Fatal("ReadEncrypted() error = nil, want missing profile error")
	}
	if !strings.Contains(err.Error(), `profile "missing" not found`) {
		t.Fatalf("ReadEncrypted() error = %v, want %q substring", err, `profile "missing" not found`)
	}
}

func TestManager_ReadEncrypted_InvalidName(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.ReadEncrypted("../bad"); err == nil {
		t.Fatal("ReadEncrypted() error = nil, want invalid name error")
	}
}

func TestManager_WriteEncrypted_StoresBlob(t *testing.T) {
	m := newTestManager(t)
	if err := m.WriteEncrypted("alpha", []byte("ciphertext")); err != nil {
		t.Fatalf("WriteEncrypted() error: %v", err)
	}
	got, err := m.ReadEncrypted("alpha")
	if err != nil {
		t.Fatalf("ReadEncrypted() error: %v", err)
	}
	if string(got) != "ciphertext" {
		t.Errorf("stored = %q, want %q", got, "ciphertext")
	}
}

func TestManager_WriteEncrypted_OverwritesExisting(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "alpha")
	if err := m.WriteEncrypted("alpha", []byte("rewritten")); err != nil {
		t.Fatalf("WriteEncrypted() error: %v", err)
	}
	got, err := m.ReadEncrypted("alpha")
	if err != nil {
		t.Fatalf("ReadEncrypted() error: %v", err)
	}
	if string(got) != "rewritten" {
		t.Errorf("after overwrite = %q, want %q", got, "rewritten")
	}
}

func TestProfileMtimeReturnsCiphertextMtime(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	want := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(profilePathForTest(m, "prod"), want, want); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	got, err := m.profileMtime("prod")
	if err != nil {
		t.Fatalf("profileMtime() error: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("profileMtime() = %v, want %v", got, want)
	}
}

func TestProfileMtimeMissingProfile(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.profileMtime("ghost"); err == nil {
		t.Fatal("profileMtime() error = nil, want not-found error")
	} else if !strings.Contains(err.Error(), `profile "ghost" not found`) {
		t.Fatalf("profileMtime() error = %v, want contains \"profile \\\"ghost\\\" not found\"", err)
	}
}

func TestManager_WriteEncrypted_InvalidName(t *testing.T) {
	m := newTestManager(t)
	if err := m.WriteEncrypted("../bad", []byte("x")); err == nil {
		t.Fatal("WriteEncrypted() error = nil, want invalid name error")
	}
}

func TestActivateAndUnlockHoldLockWhileDecrypting(t *testing.T) {
	for _, tc := range []struct {
		op  string
		run func(*Manager) error
	}{
		{"Activate", func(m *Manager) error { _, err := m.Activate("prod"); return err }},
		{"Unlock", func(m *Manager) error { _, err := m.Unlock("prod"); return err }},
	} {
		t.Run(tc.op, func(t *testing.T) {
			entered, release := installBlockingGPG(t)
			m := newTestManager(t)
			writePlaintextProfile(t, m, "prod")

			done := make(chan error, 1)
			go func() { done <- tc.run(m) }()
			waitForFile(t, entered, done, tc.op)

			lockDone := make(chan error, 1)
			go func() { lockDone <- m.WithLock(func() error { return nil }) }()
			assertStillBlocked(t, lockDone, "WithLock while "+tc.op+" decrypts")

			if err := os.WriteFile(release, nil, 0600); err != nil {
				t.Fatalf("release fake gpg: %v", err)
			}
			if err := waitForResult(t, done, tc.op); err != nil {
				t.Fatalf("%s() error = %v", tc.op, err)
			}
			if err := waitForResult(t, lockDone, "WithLock after "+tc.op); err != nil {
				t.Fatalf("WithLock() error = %v", err)
			}
		})
	}
}

func TestActivateReusesFreshActivationFile(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	cipherMtime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(profilePathForTest(m, "prod"), cipherMtime, cipherMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	want := writeRuntimeFileForTest(t, "prod", "cached", time.Now())

	path, err := m.Activate("prod")
	if err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	assertFileContent(t, path, "cached")
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if current != "prod" {
		t.Errorf("GetCurrent() = %q, want \"prod\"", current)
	}
}

func TestActivateRewritesStaleActivationFile(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	want := writeRuntimeFileForTest(t, "prod", "stale", time.Now().Add(-time.Hour))

	path, err := m.Activate("prod")
	if err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	assertFileContent(t, path, string(validKubeconfig()))
}

func TestActivateRemovesCreatedFileOnCurrentUpdateFailure(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)

	if _, err := m.Activate("prod"); err == nil {
		t.Fatal("Activate() error = nil, want current update error")
	}
	if _, err := os.Stat(runtimeFileForTest("prod")); !os.IsNotExist(err) {
		t.Fatalf("activation file left behind: stat err = %v", err)
	}
}

func TestActivateKeepsReusedFileOnCurrentUpdateFailure(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)
	path := writeRuntimeFileForTest(t, "prod", "cached", time.Now().Add(time.Hour))

	if _, err := m.Activate("prod"); err == nil {
		t.Fatal("Activate() error = nil, want current update error")
	}
	assertFileContent(t, path, "cached")
}

func TestManager_Unlock(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	path, err := m.Unlock("prod")
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if want := runtimeFileForTest("prod"); path != want {
		t.Errorf("Unlock() path = %q, want %q", path, want)
	}
	assertFileContent(t, path, string(validKubeconfig()))
	if _, err := os.Stat(currentPathForTest(m)); !os.IsNotExist(err) {
		t.Errorf("Unlock() must not write current state file: stat err = %v", err)
	}
}

func TestManager_Unlock_InvalidName(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Unlock("../bad"); err == nil {
		t.Fatal("Unlock() error = nil, want invalid name error")
	}
}

func TestManager_Unlock_MissingProfile(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)

	_, err := m.Unlock("missing")
	if err == nil {
		t.Fatal("Unlock() error = nil, want missing profile error")
	}
	if !strings.Contains(err.Error(), `profile "missing" not found`) {
		t.Fatalf("Unlock() error = %v, want %q substring", err, `profile "missing" not found`)
	}
}

func runtimeFileForTest(name string) string {
	return filepath.Join(storage.RuntimeDir(), name+".yaml")
}

func writeRuntimeFileForTest(t *testing.T, name, content string, mtime time.Time) string {
	t.Helper()
	path := runtimeFileForTest(name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("create runtime dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write activation file: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes activation file: %v", err)
	}
	return path
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s content = %q, want %q", path, got, want)
	}
}

// installBlockingGPG installs a fake gpg that creates entered, waits for
// release to exist, then echoes stdin.
func installBlockingGPG(t *testing.T) (entered, release string) {
	t.Helper()
	dir := t.TempDir()
	entered = filepath.Join(dir, "entered")
	release = filepath.Join(dir, "release")
	script := fmt.Sprintf("#!/bin/sh\ntouch %q\nwhile [ ! -e %q ]; do sleep 0.01; done\ncat\n", entered, release)
	for _, name := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return entered, release
}

func waitForFile(t *testing.T, path string, done <-chan error, op string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("%s exited before entry: %v", op, err)
		case <-time.After(10 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", op)
		}
	}
}
