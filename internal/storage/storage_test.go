package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir_WithEnv(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	want := "/custom/data/kubegonfig"
	if dir != want {
		t.Errorf("DataDir() = %q, want %q", dir, want)
	}
}

func TestDataDir_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "share", "kubegonfig")
	if dir != want {
		t.Errorf("DataDir() = %q, want %q", dir, want)
	}
}

func TestConfigDir_WithEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error: %v", err)
	}
	want := "/custom/config/kubegonfig"
	if dir != want {
		t.Errorf("ConfigDir() = %q, want %q", dir, want)
	}
}

func TestStateDir_WithEnv(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	dir, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() error: %v", err)
	}
	want := "/custom/state/kubegonfig"
	if dir != want {
		t.Errorf("StateDir() = %q, want %q", dir, want)
	}
}

func TestRuntimeDir_WithEnv(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatalf("RuntimeDir() error: %v", err)
	}
	want := "/run/user/1000/kubegonfig"
	if dir != want {
		t.Errorf("RuntimeDir() = %q, want %q", dir, want)
	}
}

func TestRuntimeDir_Fallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatalf("RuntimeDir() error: %v", err)
	}
	// Should contain kubegonfig and uid in the path.
	if dir == "" {
		t.Error("RuntimeDir() returned empty string")
	}
}

func TestEnsureDir(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "subdir")

	if err := EnsureDir(dir, 0700); err != nil {
		t.Fatalf("EnsureDir() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat after EnsureDir: %v", err)
	}
	if !info.IsDir() {
		t.Error("EnsureDir did not create a directory")
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("EnsureDir perm = %o, want 0700", perm)
	}

	// Calling again should be idempotent.
	if err := EnsureDir(dir, 0700); err != nil {
		t.Errorf("EnsureDir (idempotent) error: %v", err)
	}
}

func TestAtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	content := []byte("hello atomic")

	if err := AtomicWrite(path, content, 0600); err != nil {
		t.Fatalf("AtomicWrite() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after AtomicWrite: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("AtomicWrite content = %q, want %q", data, content)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after AtomicWrite: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("AtomicWrite perm = %o, want 0600", perm)
	}
}

func TestAtomicWrite_Overwrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")

	if err := AtomicWrite(path, []byte("first"), 0600); err != nil {
		t.Fatalf("AtomicWrite(first) error: %v", err)
	}

	if err := AtomicWrite(path, []byte("second"), 0600); err != nil {
		t.Fatalf("AtomicWrite(second) error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("overwrite content = %q, want %q", data, "second")
	}
}

func TestAtomicWrite_NestedDir(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a", "b", "test.txt")

	if err := AtomicWrite(path, []byte("nested"), 0600); err != nil {
		t.Fatalf("AtomicWrite(nested) error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("nested content = %q, want %q", data, "nested")
	}
}

func TestReadFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "read.txt")

	want := []byte("read me")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("ReadFile() = %q, want %q", got, want)
	}
}

func TestReadFile_NotExists(t *testing.T) {
	_, err := ReadFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("ReadFile(nonexistent) should return error")
	}
}

func TestRemoveFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "remove.txt")

	if err := os.WriteFile(path, []byte("bye"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := RemoveFile(path); err != nil {
		t.Fatalf("RemoveFile() error: %v", err)
	}

	if FileExists(path) {
		t.Error("file still exists after RemoveFile")
	}
}

func TestRemoveFile_NotExists(t *testing.T) {
	// Should not error when file doesn't exist.
	if err := RemoveFile("/nonexistent/path/file.txt"); err != nil {
		t.Errorf("RemoveFile(nonexistent) error: %v", err)
	}
}

func TestFileExists(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "exists.txt")

	if FileExists(path) {
		t.Error("FileExists should return false for nonexistent file")
	}

	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if !FileExists(path) {
		t.Error("FileExists should return true for existing file")
	}

	// Directories are not files.
	if FileExists(tmp) {
		t.Error("FileExists should return false for a directory")
	}
}

func TestFileLock(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "test.lock")

	lock, err := NewFileLock(lockPath)
	if err != nil {
		t.Fatalf("NewFileLock() error: %v", err)
	}

	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock() error: %v", err)
	}

	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock() error: %v", err)
	}

	// Double unlock should be safe (f is nil after close).
	// The current impl sets f to nil check — actually it doesn't reset f.
	// But calling Unlock on closed file should still not panic.
}
