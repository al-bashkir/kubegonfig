// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package tmpfile

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateAndCleanup(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

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

	if err := Remove("test-profile"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Remove")
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

	path, err := Create("remove-me", []byte("data"))
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := Remove("remove-me"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Remove")
	}
}

func TestRemove_InvalidName(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if err := Remove("../bad"); err == nil {
		t.Fatal("Remove() error = nil, want invalid name error")
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

func TestProbeCachedHit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	plaintext := filepath.Join(runtimeDir, "prod.yaml")
	if err := os.WriteFile(plaintext, []byte("hi"), 0600); err != nil {
		t.Fatalf("write plaintext: %v", err)
	}
	cipherMtime := time.Now().Add(-2 * time.Hour)
	plaintextMtime := cipherMtime.Add(time.Hour)
	if err := os.Chtimes(plaintext, plaintextMtime, plaintextMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	path, hit, err := ProbeCached("prod", cipherMtime)
	if err != nil {
		t.Fatalf("ProbeCached() error: %v", err)
	}
	if !hit {
		t.Fatal("ProbeCached() hit = false, want true")
	}
	if path != plaintext {
		t.Errorf("path = %q, want %q", path, plaintext)
	}
}

func TestProbeCachedMissMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	path, hit, err := ProbeCached("prod", time.Now())
	if err != nil {
		t.Fatalf("ProbeCached() error: %v", err)
	}
	if hit {
		t.Error("hit = true, want false (file missing)")
	}
	want := filepath.Join(tmp, "kubegonfig", "prod.yaml")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

func TestProbeCachedMissEqualMtime(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	plaintext := filepath.Join(runtimeDir, "prod.yaml")
	if err := os.WriteFile(plaintext, []byte("hi"), 0600); err != nil {
		t.Fatalf("write plaintext: %v", err)
	}
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(plaintext, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	_, hit, err := ProbeCached("prod", mtime)
	if err != nil {
		t.Fatalf("ProbeCached() error: %v", err)
	}
	if hit {
		t.Error("hit = true for equal mtime, want false (strict >)")
	}
}

func TestProbeCachedMissStale(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	plaintext := filepath.Join(runtimeDir, "prod.yaml")
	if err := os.WriteFile(plaintext, []byte("hi"), 0600); err != nil {
		t.Fatalf("write plaintext: %v", err)
	}
	plaintextMtime := time.Now().Add(-2 * time.Hour)
	cipherMtime := plaintextMtime.Add(time.Hour)
	if err := os.Chtimes(plaintext, plaintextMtime, plaintextMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	_, hit, err := ProbeCached("prod", cipherMtime)
	if err != nil {
		t.Fatalf("ProbeCached() error: %v", err)
	}
	if hit {
		t.Error("hit = true for stale plaintext, want false")
	}
}

func TestProbeCachedMissSymlink(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("mkdir runtime: %v", err)
	}
	target := filepath.Join(runtimeDir, "target.yaml")
	if err := os.WriteFile(target, []byte("hi"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(runtimeDir, "prod.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, hit, err := ProbeCached("prod", time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("ProbeCached() error: %v", err)
	}
	if hit {
		t.Error("hit = true for symlink, want false")
	}
}

func TestProbeCachedInvalidName(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if _, _, err := ProbeCached("../bad", time.Now()); err == nil {
		t.Fatal("ProbeCached() error = nil, want validation error")
	}
}
