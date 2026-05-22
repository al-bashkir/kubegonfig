// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package storage provides XDG-compliant directory management,
// atomic file writes, and file locking for kubegonfig.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const appName = "kubegonfig"

// PostCommitError reports a durability failure after a filesystem mutation
// already succeeded. Callers may need to reconcile higher-level state.
type PostCommitError struct {
	Op   string
	Path string
	Err  error
}

func (e *PostCommitError) Error() string {
	return fmt.Sprintf("%s succeeded but sync directory %s: %v", e.Op, e.Path, e.Err)
}

func (e *PostCommitError) Unwrap() error {
	return e.Err
}

// IsPostCommitError reports whether err contains a post-commit durability error.
func IsPostCommitError(err error) bool {
	var postCommitErr *PostCommitError
	return errors.As(err, &postCommitErr)
}

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
// It also verifies the directory has correct ownership on non-Windows systems.
func EnsureDir(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}
	dir, err := openDirNoFollow(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	if err := validateOpenDir(path, dir); err != nil {
		return err
	}
	if err := dir.Chmod(perm); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return validateOpenDir(path, dir)
}

func openDirNoFollow(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open directory %s: %w", path, err)
	}
	return os.NewFile(uintptr(fd), path), nil
}

func validateOpenDir(path string, dir *os.File) error {
	info, err := dir.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	stat, ok := info.Sys().(*unix.Stat_t)
	if ok && stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("directory %s is not owned by current user", path)
	}
	return nil
}

// AtomicWrite writes data to a file atomically using a temp-file + rename
// strategy. This prevents partial writes from corrupting existing files.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	return AtomicWriteInDir(filepath.Dir(path), filepath.Base(path), data, perm)
}

// RemoveFileInDir removes a file relative to a verified non-symlink directory.
func RemoveFileInDir(dirPath, name string) error {
	if err := validateRelativeFileName(name); err != nil {
		return err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	path := filepath.Join(dirPath, name)
	if err := unix.Unlinkat(int(dir.Fd()), name, 0); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	if err := dir.Sync(); err != nil {
		return &PostCommitError{Op: "remove", Path: dirPath, Err: err}
	}
	return nil
}

// RenameFileInDir renames a file relative to a verified non-symlink directory.
func RenameFileInDir(dirPath, oldName, newName string) error {
	if err := validateRelativeFileName(oldName); err != nil {
		return err
	}
	if err := validateRelativeFileName(newName); err != nil {
		return err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	oldPath := filepath.Join(dirPath, oldName)
	newPath := filepath.Join(dirPath, newName)
	if err := unix.Renameat(int(dir.Fd()), oldName, int(dir.Fd()), newName); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", oldPath, newPath, err)
	}
	if err := dir.Sync(); err != nil {
		return &PostCommitError{Op: "rename", Path: dirPath, Err: err}
	}
	return nil
}

// AtomicWriteInDir atomically writes a file relative to a verified non-symlink directory.
func AtomicWriteInDir(dirPath, name string, data []byte, perm os.FileMode) error {
	if err := validateRelativeFileName(name); err != nil {
		return err
	}
	if err := EnsureDir(dirPath, 0700); err != nil {
		return err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	tmpName, tmp, err := createTempFileInDir(dir, dirPath, ".tmp-", "", perm)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = unix.Unlinkat(int(dir.Fd()), tmpName, 0)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := unix.Renameat(int(dir.Fd()), tmpName, int(dir.Fd()), name); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", filepath.Join(dirPath, tmpName), filepath.Join(dirPath, name), err)
	}
	if err := dir.Sync(); err != nil {
		return &PostCommitError{Op: "rename", Path: dirPath, Err: err}
	}
	success = true
	return nil
}

// AtomicWriteStream atomically writes a file produced by fn into a verified
// non-symlink directory. fn receives an io.Writer that streams into a temp
// file; on success the temp is fsync'd, closed, and renamed into place. On
// fn or write error the temp is removed and the destination is untouched.
func AtomicWriteStream(dirPath, name string, perm os.FileMode, fn func(io.Writer) error) error {
	if err := validateRelativeFileName(name); err != nil {
		return err
	}
	if fn == nil {
		return fmt.Errorf("AtomicWriteStream: fn must not be nil")
	}
	if err := EnsureDir(dirPath, 0700); err != nil {
		return err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	tmpName, tmp, err := createTempFileInDir(dir, dirPath, ".tmp-", "", perm)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = unix.Unlinkat(int(dir.Fd()), tmpName, 0)
		}
	}()

	if err := fn(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := unix.Renameat(int(dir.Fd()), tmpName, int(dir.Fd()), name); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", filepath.Join(dirPath, tmpName), filepath.Join(dirPath, name), err)
	}
	if err := dir.Sync(); err != nil {
		return &PostCommitError{Op: "rename", Path: dirPath, Err: err}
	}
	success = true
	return nil
}

// OpenUnlinkedTempFileInDir writes data to a temporary file in a verified
// non-symlink directory, unlinks it, and returns the still-open file handle.
func OpenUnlinkedTempFileInDir(dirPath, prefix, suffix string, data []byte, perm os.FileMode) (*os.File, error) {
	if err := validateTempNamePattern(prefix, suffix); err != nil {
		return nil, err
	}
	if err := EnsureDir(dirPath, 0700); err != nil {
		return nil, err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = dir.Close()
	}()

	tmpName, tmp, err := createTempFileInDir(dir, dirPath, prefix, suffix, perm)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = unix.Unlinkat(int(dir.Fd()), tmpName, 0)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("rewind temp file: %w", err)
	}
	if err := unix.Unlinkat(int(dir.Fd()), tmpName, 0); err != nil {
		return nil, fmt.Errorf("unlink %s: %w", filepath.Join(dirPath, tmpName), err)
	}
	if err := dir.Sync(); err != nil {
		return nil, &PostCommitError{Op: "unlink", Path: dirPath, Err: err}
	}

	success = true
	return tmp, nil
}

// ReadFileInDir reads a file relative to a verified non-symlink directory.
func ReadFileInDir(dirPath, name string) ([]byte, error) {
	if err := validateRelativeFileName(name); err != nil {
		return nil, err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = dir.Close()
	}()

	path := filepath.Join(dirPath, name)
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	f := os.NewFile(uintptr(fd), path)
	defer func() {
		_ = f.Close()
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read %s: not a regular file", path)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// ReadDirInDir lists entries in a verified non-symlink directory.
func ReadDirInDir(dirPath string) ([]os.DirEntry, error) {
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		_ = dir.Close()
	}()
	return dir.ReadDir(-1)
}

// FileExistsInDir checks for a regular file inside a verified non-symlink directory.
func FileExistsInDir(dirPath, name string) (bool, error) {
	if err := validateRelativeFileName(name); err != nil {
		return false, err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer func() {
		_ = dir.Close()
	}()

	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("open %s: %w", filepath.Join(dirPath, name), err)
	}
	f := os.NewFile(uintptr(fd), filepath.Join(dirPath, name))
	defer func() {
		_ = f.Close()
	}()
	info, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", filepath.Join(dirPath, name), err)
	}
	return info.Mode().IsRegular(), nil
}

// RegularFileInDir checks a file entry without following symlinks.
func RegularFileInDir(dirPath, name string) (bool, error) {
	if err := validateRelativeFileName(name); err != nil {
		return false, err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer func() {
		_ = dir.Close()
	}()

	var stat unix.Stat_t
	if err := unix.Fstatat(int(dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", filepath.Join(dirPath, name), err)
	}
	return stat.Mode&unix.S_IFMT == unix.S_IFREG, nil
}

// RegularFileMtimeInDir returns the modification time of a regular file inside
// a verified non-symlink directory. ok is false when the entry is missing or
// is not a regular file (symlink, directory, device, socket, FIFO).
func RegularFileMtimeInDir(dirPath, name string) (time.Time, bool, error) {
	if err := validateRelativeFileName(name); err != nil {
		return time.Time{}, false, err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	defer func() {
		_ = dir.Close()
	}()

	var stat unix.Stat_t
	if err := unix.Fstatat(int(dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("stat %s: %w", filepath.Join(dirPath, name), err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return time.Time{}, false, nil
	}
	return time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec)), true, nil
}

func openVerifiedDir(path string) (*os.File, error) {
	dir, err := openDirNoFollow(path)
	if err != nil {
		return nil, err
	}
	if err := validateOpenDir(path, dir); err != nil {
		_ = dir.Close()
		return nil, err
	}
	return dir, nil
}

func createTempFileInDir(dir *os.File, dirPath, prefix, suffix string, perm os.FileMode) (string, *os.File, error) {
	for range 100 {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return "", nil, fmt.Errorf("generate temp file name: %w", err)
		}
		name := prefix + hex.EncodeToString(buf) + suffix
		fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, uint32(perm))
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", nil, fmt.Errorf("create temp file %s: %w", filepath.Join(dirPath, name), err)
		}
		f := os.NewFile(uintptr(fd), filepath.Join(dirPath, name))
		if err := f.Chmod(perm); err != nil {
			_ = f.Close()
			_ = unix.Unlinkat(int(dir.Fd()), name, 0)
			return "", nil, fmt.Errorf("chmod temp file %s: %w", filepath.Join(dirPath, name), err)
		}
		return name, f, nil
	}
	return "", nil, fmt.Errorf("create temp file in %s: exhausted name attempts", dirPath)
}

func validateRelativeFileName(name string) error {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid file name %q", name)
	}
	return nil
}

func validateTempNamePattern(prefix, suffix string) error {
	if prefix == "" && suffix == "" {
		return nil
	}
	return validateRelativeFileName(prefix + "x" + suffix)
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
	dirPath := filepath.Dir(l.path)
	name := filepath.Base(l.path)
	if err := validateRelativeFileName(name); err != nil {
		return err
	}
	dir, err := openVerifiedDir(dirPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return fmt.Errorf("open lock file %s: %w", l.path, err)
	}
	f := os.NewFile(uintptr(fd), l.path)
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("stat lock file %s: %w", l.path, err)
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return fmt.Errorf("lock file %s is not a regular file", l.path)
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return fmt.Errorf("chmod lock file %s: %w", l.path, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return fmt.Errorf("flock %s: %w", l.path, err)
	}
	l.f = f
	return nil
}

// Unlock releases the advisory lock.
func (l *FileLock) Unlock() error {
	if l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		_ = f.Close()
		return fmt.Errorf("funlock %s: %w", l.path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close lock file %s: %w", l.path, err)
	}
	return nil
}
