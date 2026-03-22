// Package storage provides XDG-compliant directory management,
// atomic file writes, and file locking for kubegonfig.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

const appName = "kubegonfig"

// DataDir returns the XDG_DATA_HOME/kubegonfig path.
func DataDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, appName), nil
}

// ConfigDir returns the XDG_CONFIG_HOME/kubegonfig path.
func ConfigDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, appName), nil
}

// StateDir returns the XDG_STATE_HOME/kubegonfig path.
func StateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, appName), nil
}

// RuntimeDir returns a secure temporary directory for decrypted files.
// Uses XDG_RUNTIME_DIR if available, otherwise falls back to a private
// directory under the OS temp dir.
func RuntimeDir() (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		// Fallback: /tmp/kubegonfig-<uid>
		base = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", appName, os.Getuid()))
	} else {
		base = filepath.Join(base, appName)
	}
	return base, nil
}

// EnsureDir creates a directory with the given permissions if it does not exist.
// It also verifies the directory has correct ownership on Linux/macOS.
func EnsureDir(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	// Enforce permissions even if directory already existed.
	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	// Verify ownership on Unix systems.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if ok && stat.Uid != uint32(os.Getuid()) {
			return fmt.Errorf("directory %s is not owned by current user", path)
		}
	}
	return nil
}

// AtomicWrite writes data to a file atomically using a temp-file + rename
// strategy. This prevents partial writes from corrupting existing files.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir, 0700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Ensure cleanup on any failure path.
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}

	success = true
	return nil
}

// ReadFile reads an entire file and returns its contents.
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// RemoveFile removes a file if it exists. Does not error if the file
// is already absent.
func RemoveFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// FileExists returns true if path exists and is a regular file.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FileLock provides advisory file locking using flock(2).
type FileLock struct {
	path string
	f    *os.File
}

// NewFileLock creates a lock file at the given path.
func NewFileLock(path string) (*FileLock, error) {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir, 0700); err != nil {
		return nil, err
	}
	return &FileLock{path: path}, nil
}

// Lock acquires an exclusive advisory lock. Blocks until acquired.
func (l *FileLock) Lock() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open lock file %s: %w", l.path, err)
	}
	l.f = f
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return fmt.Errorf("flock %s: %w", l.path, err)
	}
	return nil
}

// Unlock releases the advisory lock.
func (l *FileLock) Unlock() error {
	if l.f == nil {
		return nil
	}
	if err := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN); err != nil {
		_ = l.f.Close()
		return fmt.Errorf("funlock %s: %w", l.path, err)
	}
	return l.f.Close()
}
