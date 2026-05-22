// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

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

func TestActivateHoldsLockDuringCreateCallback(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	entered := make(chan struct{})
	release := make(chan struct{})
	createdPath := filepath.Join(t.TempDir(), "prod.yaml")
	activateDone := make(chan error, 1)
	go func() {
		path, err := m.Activate("prod",
			func(name string, cipherMtime time.Time) (string, bool, error) {
				return "", false, nil
			},
			func(data []byte) (string, error) {
				close(entered)
				<-release
				return createdPath, nil
			}, nil)
		if err == nil && path == "" {
			err = fmt.Errorf("Activate() path is empty")
		}
		activateDone <- err
	}()

	waitForEntry(t, entered, activateDone, "Activate callback")
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- m.WithLock(func() error { return nil })
	}()

	assertStillBlocked(t, lockDone, "WithLock while activation callback is active")
	close(release)
	if err := waitForResult(t, activateDone, "Activate"); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if err := waitForResult(t, lockDone, "WithLock after activation callback exits"); err != nil {
		t.Fatalf("WithLock() error = %v", err)
	}
}

func TestActivateCleansUpCreatedPathOnCurrentUpdateFailure(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)

	createdPath := filepath.Join(t.TempDir(), "prod.yaml")
	cleanedPath := ""
	_, err := m.Activate("prod",
		func(name string, cipherMtime time.Time) (string, bool, error) {
			return "", false, nil
		},
		func(data []byte) (string, error) {
			return createdPath, nil
		}, func(path string) error {
			cleanedPath = path
			return nil
		})
	if err == nil {
		t.Fatal("Activate() error = nil, want current update error")
	}
	if cleanedPath != createdPath {
		t.Fatalf("cleanup path = %q, want %q", cleanedPath, createdPath)
	}
}

func TestActivateReportsCleanupFailureAfterCurrentUpdateFailure(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)

	cleanupErr := errors.New("cleanup failed")
	_, err := m.Activate("prod",
		func(name string, cipherMtime time.Time) (string, bool, error) {
			return "", false, nil
		},
		func(data []byte) (string, error) {
			return filepath.Join(t.TempDir(), "prod.yaml"), nil
		}, func(path string) error {
			return cleanupErr
		})
	if err == nil {
		t.Fatal("Activate() error = nil, want current update and cleanup error")
	}
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("Activate() error does not wrap cleanup error: %v", err)
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

func TestDeleteRestoresCurrentWhenProfileRemovalFails(t *testing.T) {
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
	if current != "prod" {
		t.Errorf("current after failed profile removal = %q, want prod", current)
	}
}

func TestManager_Unlock(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	wantPath := filepath.Join(t.TempDir(), "prod.yaml")
	var gotData []byte
	gotPath, err := m.Unlock("prod", func(data []byte) (string, error) {
		gotData = append(gotData[:0], data...)
		return wantPath, nil
	})
	if err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if gotPath != wantPath {
		t.Errorf("Unlock() path = %q, want %q", gotPath, wantPath)
	}
	if string(gotData) != string(validKubeconfig()) {
		t.Errorf("create callback received %q, want decrypted profile bytes", gotData)
	}
	if _, err := os.Stat(currentPathForTest(m)); !os.IsNotExist(err) {
		t.Errorf("Unlock() must not write current state file: stat err = %v", err)
	}
}

func TestManager_Unlock_InvalidName(t *testing.T) {
	m := newTestManager(t)
	called := false
	_, err := m.Unlock("../bad", func([]byte) (string, error) {
		called = true
		return "", nil
	})
	if err == nil {
		t.Fatal("Unlock() error = nil, want invalid name error")
	}
	if called {
		t.Error("create callback was invoked despite invalid name")
	}
}

func TestManager_Unlock_NilCallback(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Unlock("prod", nil); err == nil {
		t.Fatal("Unlock() error = nil, want nil callback error")
	}
}

func TestManager_Unlock_MissingProfile(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)

	_, err := m.Unlock("missing", func([]byte) (string, error) {
		return "", nil
	})
	if err == nil {
		t.Fatal("Unlock() error = nil, want missing profile error")
	}
	if !strings.Contains(err.Error(), `profile "missing" not found`) {
		t.Fatalf("Unlock() error = %v, want %q substring", err, `profile "missing" not found`)
	}
}

func TestUnlockHoldsLockDuringCreateCallback(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	entered := make(chan struct{})
	release := make(chan struct{})
	createdPath := filepath.Join(t.TempDir(), "prod.yaml")
	unlockDone := make(chan error, 1)
	go func() {
		path, err := m.Unlock("prod", func(data []byte) (string, error) {
			close(entered)
			<-release
			return createdPath, nil
		})
		if err == nil && path == "" {
			err = fmt.Errorf("Unlock() path is empty")
		}
		unlockDone <- err
	}()

	waitForEntry(t, entered, unlockDone, "Unlock callback")
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- m.WithLock(func() error { return nil })
	}()

	assertStillBlocked(t, lockDone, "WithLock while unlock callback is active")
	close(release)
	if err := waitForResult(t, unlockDone, "Unlock"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if err := waitForResult(t, lockDone, "WithLock after unlock callback exits"); err != nil {
		t.Fatalf("WithLock() error = %v", err)
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
	exists, err := m.profileExists(name)
	if err != nil {
		t.Fatalf("profileExists(%q) error: %v", name, err)
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

func TestActivateCacheHitSkipsCreate(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")

	probeCalls := 0
	createCalls := 0
	probe := func(name string, cipherMtime time.Time) (string, bool, error) {
		probeCalls++
		if name != "prod" {
			t.Fatalf("probe got name %q, want \"prod\"", name)
		}
		return "/cached/prod.yaml", true, nil
	}
	create := func(data []byte) (string, error) {
		createCalls++
		return "/created/prod.yaml", nil
	}

	path, err := m.Activate("prod", probe, create, nil)
	if err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
	if path != "/cached/prod.yaml" {
		t.Errorf("path = %q, want \"/cached/prod.yaml\"", path)
	}
	if probeCalls != 1 {
		t.Errorf("probe called %d times, want 1", probeCalls)
	}
	if createCalls != 0 {
		t.Errorf("create called %d times on hit, want 0", createCalls)
	}
	current, err := m.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent() error: %v", err)
	}
	if current != "prod" {
		t.Errorf("GetCurrent() = %q, want \"prod\"", current)
	}
}

func TestActivateCacheMissCallsCreate(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")

	createCalls := 0
	createPath := filepath.Join(t.TempDir(), "prod.yaml")
	probe := func(name string, cipherMtime time.Time) (string, bool, error) {
		return createPath, false, nil
	}
	create := func(data []byte) (string, error) {
		createCalls++
		return createPath, nil
	}

	path, err := m.Activate("prod", probe, create, nil)
	if err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
	if path != createPath {
		t.Errorf("path = %q, want %q", path, createPath)
	}
	if createCalls != 1 {
		t.Errorf("create called %d times, want 1", createCalls)
	}
}

func TestActivateMissCleanupOnCurrentUpdateFailure(t *testing.T) {
	installFakeGPG(t)
	m := newTestManager(t)
	writePlaintextProfile(t, m, "prod")
	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)

	createdPath := filepath.Join(t.TempDir(), "prod.yaml")
	cleanedPath := ""
	probe := func(name string, cipherMtime time.Time) (string, bool, error) {
		return createdPath, false, nil
	}
	create := func(data []byte) (string, error) {
		return createdPath, nil
	}
	cleanup := func(path string) error {
		cleanedPath = path
		return nil
	}

	if _, err := m.Activate("prod", probe, create, cleanup); err == nil {
		t.Fatal("Activate() error = nil, want current update error")
	}
	if cleanedPath != createdPath {
		t.Fatalf("cleanup path = %q, want %q", cleanedPath, createdPath)
	}
}

func TestActivateHitNoCleanupOnCurrentUpdateFailure(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	setCurrentForTest(t, m, "prod")
	replaceStateDirWithSymlink(t, m)

	cachedPath := filepath.Join(t.TempDir(), "prod.yaml")
	cleanupCalls := 0
	probe := func(name string, cipherMtime time.Time) (string, bool, error) {
		return cachedPath, true, nil
	}
	create := func(data []byte) (string, error) {
		t.Fatal("create called on cache hit")
		return "", nil
	}
	cleanup := func(path string) error {
		cleanupCalls++
		return nil
	}

	if _, err := m.Activate("prod", probe, create, cleanup); err == nil {
		t.Fatal("Activate() error = nil, want current update error")
	}
	if cleanupCalls != 0 {
		t.Errorf("cleanup called %d times on hit failure, want 0", cleanupCalls)
	}
}

func TestActivateProbePassesCiphertextMtime(t *testing.T) {
	m := newTestManager(t)
	writeProfile(t, m, "prod")
	want := time.Now().Add(-7 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(profilePathForTest(m, "prod"), want, want); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	var gotMtime time.Time
	probe := func(name string, cipherMtime time.Time) (string, bool, error) {
		gotMtime = cipherMtime
		return "/cached/prod.yaml", true, nil
	}
	create := func(data []byte) (string, error) {
		t.Fatal("create called on hit")
		return "", nil
	}

	if _, err := m.Activate("prod", probe, create, nil); err != nil {
		t.Fatalf("Activate() error: %v", err)
	}
	if !gotMtime.Equal(want) {
		t.Errorf("probe got mtime %v, want %v", gotMtime, want)
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
