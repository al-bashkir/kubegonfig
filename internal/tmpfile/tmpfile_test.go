// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package tmpfile

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndCleanup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	resetTracked(t)

	data := []byte("apiVersion: v1\nkind: Config\n")

	path, err := Create("test-profile", data)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Verify file was created with correct content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("file content = %q, want %q", got, data)
	}

	// Verify permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file perm = %o, want 0600", perm)
	}

	// Verify path structure.
	expectedName := "test-profile.yaml"
	if filepath.Base(path) != expectedName {
		t.Errorf("filename = %q, want %q", filepath.Base(path), expectedName)
	}

	// Cleanup should remove the file.
	CleanupAll()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after CleanupAll")
	}
}

func TestCreate_InvalidName(t *testing.T) {
	_, err := Create("../bad", []byte("data"))
	if err == nil {
		t.Fatal("Create with invalid profile name should fail")
	}
}

func TestOpenUnlinked(t *testing.T) {
	tmp := t.TempDir()
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	resetTracked(t)

	data := []byte("apiVersion: v1\nkind: Config\n")
	f, err := OpenUnlinked("exec-profile", data)
	if err != nil {
		t.Fatalf("OpenUnlinked() error: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()

	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll(open file) error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("file content = %q, want %q", got, data)
	}

	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error: %v", runtimeDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("runtime dir contains %d entries, want 0", len(entries))
	}

	trackedMu.Lock()
	count := len(tracked)
	trackedMu.Unlock()
	if count != 0 {
		t.Errorf("tracked count = %d, want 0", count)
	}
}

func TestOpenUnlinked_InvalidName(t *testing.T) {
	_, err := OpenUnlinked("../bad", []byte("data"))
	if err == nil {
		t.Fatal("OpenUnlinked with invalid profile name should fail")
	}
}

func TestOpenUnlinkedRejectsSymlinkedRuntimeDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("create symlink target: %v", err)
	}
	if err := os.Symlink(target, runtimeDir); err != nil {
		t.Skipf("runtime dir symlink unavailable: %v", err)
	}

	if _, err := OpenUnlinked("exec-profile", []byte("data")); err == nil {
		t.Fatal("OpenUnlinked() error = nil, want symlinked runtime dir error")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("ReadDir(%q) error: %v", target, err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target contains %d entries, want 0", len(entries))
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	resetTracked(t)

	path, err := Create("remove-me", []byte("data"))
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := Remove(path); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Remove")
	}
}

func TestRemoveRejectsPathOutsideRuntimeDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	outside := filepath.Join(t.TempDir(), "remove-me.yaml")
	if err := os.WriteFile(outside, []byte("data"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := Remove(outside); err == nil {
		t.Fatal("Remove() error = nil, want outside-runtime rejection")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}

func TestRemove_UntracksPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	resetTracked(t)

	path, err := Create("remove-me", []byte("data"))
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := Remove(path); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	trackedMu.Lock()
	count := len(tracked)
	trackedMu.Unlock()
	if count != 0 {
		t.Errorf("tracked count after Remove = %d, want 0", count)
	}
}

func TestCleanupStale(t *testing.T) {
	tmp := t.TempDir()
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Create stale kubeconfig files and unrelated files that must be preserved.
	for _, name := range []string{"old1.yaml", "old2.yaml", "old3.yaml", "notes.txt", "-bad.yaml"} {
		path := filepath.Join(runtimeDir, name)
		if err := os.WriteFile(path, []byte("stale"), 0600); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	count, err := CleanupStale()
	if err != nil {
		t.Fatalf("CleanupStale() error: %v", err)
	}
	if count != 3 {
		t.Errorf("CleanupStale() removed %d, want 3", count)
	}

	for _, name := range []string{"old1.yaml", "old2.yaml", "old3.yaml"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", name)
		}
	}
	for _, name := range []string{"notes.txt", "-bad.yaml"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); err != nil {
			t.Errorf("%s should have been preserved: %v", name, err)
		}
	}
}

func TestCleanupStale_NonexistentDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/nonexistent/runtime")

	count, err := CleanupStale()
	if err != nil {
		t.Fatalf("CleanupStale(nonexistent) error: %v", err)
	}
	if count != 0 {
		t.Errorf("CleanupStale(nonexistent) count = %d, want 0", count)
	}
}

func TestCleanupStaleRejectsSymlinkedRuntimeDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatalf("create symlink target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "old.yaml"), []byte("stale"), 0600); err != nil {
		t.Fatalf("write stale target file: %v", err)
	}
	if err := os.Symlink(target, runtimeDir); err != nil {
		t.Skipf("runtime dir symlink unavailable: %v", err)
	}

	if _, err := CleanupStale(); err == nil {
		t.Fatal("CleanupStale() error = nil, want symlinked runtime dir error")
	}
	if _, err := os.Stat(filepath.Join(target, "old.yaml")); err != nil {
		t.Fatalf("stale file in symlink target was removed or changed: %v", err)
	}
}

func TestCleanupStale_SkipsDirectories(t *testing.T) {
	tmp := t.TempDir()
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Create a file and a subdirectory.
	if err := os.WriteFile(filepath.Join(runtimeDir, "file.yaml"), []byte("data"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runtimeDir, "subdir"), 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	count, err := CleanupStale()
	if err != nil {
		t.Fatalf("CleanupStale() error: %v", err)
	}
	if count != 1 {
		t.Errorf("CleanupStale() removed %d, want 1 (should skip dirs)", count)
	}

	// Subdirectory should still exist.
	if _, err := os.Stat(filepath.Join(runtimeDir, "subdir")); os.IsNotExist(err) {
		t.Error("subdirectory should not be removed")
	}
}

func TestCreate_MultipleTracked(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	// Reset tracked state.
	trackedMu.Lock()
	tracked = nil
	trackedMu.Unlock()

	for _, name := range []string{"profile-a", "profile-b"} {
		_, err := Create(name, []byte("data"))
		if err != nil {
			t.Fatalf("Create(%q) error: %v", name, err)
		}
	}

	trackedMu.Lock()
	count := len(tracked)
	trackedMu.Unlock()

	if count != 2 {
		t.Errorf("tracked count = %d, want 2", count)
	}

	CleanupAll()

	trackedMu.Lock()
	count = len(tracked)
	trackedMu.Unlock()

	if count != 0 {
		t.Errorf("tracked count after cleanup = %d, want 0", count)
	}
}

func TestCleanupAllKeepsTrackedPathAfterRemoveFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	resetTracked(t)

	path, err := Create("retry-me", []byte("data"))
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	runtimeDir := filepath.Dir(path)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove temp file: %v", err)
	}
	if err := os.Remove(runtimeDir); err != nil {
		t.Fatalf("remove runtime dir: %v", err)
	}
	if err := os.WriteFile(runtimeDir, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("replace runtime dir with file: %v", err)
	}

	CleanupAll()

	trackedMu.Lock()
	defer trackedMu.Unlock()
	if len(tracked) != 1 || tracked[0] != path {
		t.Fatalf("tracked = %v, want retained failed cleanup path %q", tracked, path)
	}
}

func resetTracked(t *testing.T) {
	t.Helper()
	trackedMu.Lock()
	tracked = nil
	trackedMu.Unlock()
}
