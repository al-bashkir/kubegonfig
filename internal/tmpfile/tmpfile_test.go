package tmpfile

import (
	"os"
	"path/filepath"
	"testing"
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

	// Cleanup should remove the file.
	CleanupAll()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after CleanupAll")
	}
}

func TestRemove(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "remove-me.yaml")

	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := Remove(path); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after Remove")
	}
}

func TestCleanupStale(t *testing.T) {
	tmp := t.TempDir()
	runtimeDir := filepath.Join(tmp, "kubegonfig")
	t.Setenv("XDG_RUNTIME_DIR", tmp)

	if err := os.MkdirAll(runtimeDir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Create some stale files.
	for _, name := range []string{"old1.yaml", "old2.yaml", "old3.yaml"} {
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

	// Verify all files are gone.
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("directory should be empty, has %d entries", len(entries))
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
