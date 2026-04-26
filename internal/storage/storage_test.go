// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package storage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
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

func TestEnsureDirRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := EnsureDir(link, 0700); err == nil {
		t.Fatal("EnsureDir() error = nil, want symlink rejection")
	}
}

func TestOpenDirNoFollowRejectsFIFOImmediately(t *testing.T) {
	tmp := t.TempDir()
	fifo := filepath.Join(tmp, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	result := make(chan error, 1)
	go func() {
		f, err := openDirNoFollow(fifo)
		if f != nil {
			_ = f.Close()
		}
		result <- err
	}()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("openDirNoFollow() error = nil, want FIFO rejection")
		}
	case <-time.After(time.Second):
		t.Fatal("openDirNoFollow() blocked on FIFO")
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

func TestAtomicWriteInDirRejectsSymlinkedDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := AtomicWriteInDir(link, "current", []byte("prod\n"), 0600); err == nil {
		t.Fatal("AtomicWriteInDir() error = nil, want symlinked directory rejection")
	}
	if fileExists(filepath.Join(target, "current")) {
		t.Fatal("AtomicWriteInDir() wrote through symlinked directory")
	}
}

func TestAtomicWriteInDirIgnoresRestrictiveUmask(t *testing.T) {
	oldUmask := syscall.Umask(0077)
	t.Cleanup(func() {
		syscall.Umask(oldUmask)
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "current")
	if err := AtomicWriteInDir(tmp, "current", []byte("prod\n"), 0600); err != nil {
		t.Fatalf("AtomicWriteInDir() error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("AtomicWriteInDir() mode = %o, want 0600", got)
	}
}

func TestReadFileInDirRejectsSymlinkedDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "current"), []byte("prod\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if _, err := ReadFileInDir(link, "current"); err == nil {
		t.Fatal("ReadFileInDir() error = nil, want symlinked directory rejection")
	}
}

func TestReadFileInDirRejectsSymlinkedFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("prod\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(tmp, "current")); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if _, err := ReadFileInDir(tmp, "current"); err == nil {
		t.Fatal("ReadFileInDir() error = nil, want symlinked file rejection")
	}
}

func TestReadFileInDirRejectsFIFOImmediately(t *testing.T) {
	tmp := t.TempDir()
	name := "fifo"
	if err := syscall.Mkfifo(filepath.Join(tmp, name), 0600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := ReadFileInDir(tmp, name)
		result <- err
	}()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("ReadFileInDir() error = nil, want FIFO rejection")
		}
	case <-time.After(time.Second):
		t.Fatal("ReadFileInDir() blocked on FIFO")
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

	if fileExists(path) {
		t.Error("file still exists after RemoveFile")
	}
}

func TestRemoveFile_NotExists(t *testing.T) {
	// Should not error when file doesn't exist.
	if err := RemoveFile("/nonexistent/path/file.txt"); err != nil {
		t.Errorf("RemoveFile(nonexistent) error: %v", err)
	}
}

func TestRemoveFileInDir(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "remove.txt")
	if err := os.WriteFile(path, []byte("bye"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := RemoveFileInDir(tmp, "remove.txt"); err != nil {
		t.Fatalf("RemoveFileInDir() error: %v", err)
	}
	if fileExists(path) {
		t.Error("file still exists after RemoveFileInDir")
	}
}

func TestRemoveFileInDirRejectsSymlinkedDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "current"), []byte("prod\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := RemoveFileInDir(link, "current"); err == nil {
		t.Fatal("RemoveFileInDir() error = nil, want symlinked directory rejection")
	}
	if !fileExists(filepath.Join(target, "current")) {
		t.Fatal("RemoveFileInDir() removed file through symlinked directory")
	}
}

func TestRemoveFileInDirRejectsInvalidNames(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"", ".", "..", "a/b"} {
		t.Run(name, func(t *testing.T) {
			if err := RemoveFileInDir(tmp, name); err == nil {
				t.Fatal("RemoveFileInDir() error = nil, want invalid name error")
			}
		})
	}
}

func TestRemoveFileInDirMissingDir(t *testing.T) {
	if err := RemoveFileInDir(filepath.Join(t.TempDir(), "missing"), "current"); err != nil {
		t.Fatalf("RemoveFileInDir() missing dir error: %v", err)
	}
}

func TestRenameFileInDir(t *testing.T) {
	tmp := t.TempDir()
	oldPath := filepath.Join(tmp, "old.txt")
	newPath := filepath.Join(tmp, "new.txt")
	if err := os.WriteFile(oldPath, []byte("data"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := RenameFileInDir(tmp, "old.txt", "new.txt"); err != nil {
		t.Fatalf("RenameFileInDir() error: %v", err)
	}
	if fileExists(oldPath) {
		t.Fatal("old file still exists after RenameFileInDir")
	}
	if !fileExists(newPath) {
		t.Fatal("new file missing after RenameFileInDir")
	}
}

func TestRenameFileInDirRejectsSymlinkedDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if err := RenameFileInDir(link, "old", "new"); err == nil {
		t.Fatal("RenameFileInDir() error = nil, want symlinked directory rejection")
	}
	if !fileExists(filepath.Join(target, "old")) {
		t.Fatal("RenameFileInDir() moved old file through symlinked directory")
	}
	if fileExists(filepath.Join(target, "new")) {
		t.Fatal("RenameFileInDir() created new file through symlinked directory")
	}
}

func TestRenameFileInDirRejectsInvalidNames(t *testing.T) {
	tmp := t.TempDir()
	for _, tc := range []struct {
		name    string
		oldName string
		newName string
	}{
		{name: "invalid old", oldName: "a/b", newName: "new"},
		{name: "invalid new", oldName: "old", newName: "../new"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := RenameFileInDir(tmp, tc.oldName, tc.newName); err == nil {
				t.Fatal("RenameFileInDir() error = nil, want invalid name error")
			}
		})
	}
}

func TestFileExistsInDir(t *testing.T) {
	tmp := t.TempDir()
	if exists, err := FileExistsInDir(tmp, "exists.txt"); err != nil || exists {
		t.Fatalf("FileExistsInDir() missing file = %v, %v; want false, nil", exists, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "exists.txt"), []byte("x"), 0600); err != nil {
		t.Fatalf("setup file: %v", err)
	}
	if exists, err := FileExistsInDir(tmp, "exists.txt"); err != nil || !exists {
		t.Fatalf("FileExistsInDir() file = %v, %v; want true, nil", exists, err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "dir"), 0700); err != nil {
		t.Fatalf("setup dir: %v", err)
	}
	if exists, err := FileExistsInDir(tmp, "dir"); err != nil || exists {
		t.Fatalf("FileExistsInDir() dir = %v, %v; want false, nil", exists, err)
	}
}

func TestFileExistsInDirRejectsSymlinkedDir(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "profile"), []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if _, err := FileExistsInDir(link, "profile"); err == nil {
		t.Fatal("FileExistsInDir() error = nil, want symlinked directory rejection")
	}
}

func TestFileExistsInDirRejectsSymlinkedFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(tmp, "profile")); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	if _, err := FileExistsInDir(tmp, "profile"); err == nil {
		t.Fatal("FileExistsInDir() error = nil, want symlinked file rejection")
	}
}

func TestFileExistsInDirRejectsFIFOImmediately(t *testing.T) {
	tmp := t.TempDir()
	name := "fifo"
	if err := syscall.Mkfifo(filepath.Join(tmp, name), 0600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	result := make(chan struct {
		exists bool
		err    error
	}, 1)
	go func() {
		exists, err := FileExistsInDir(tmp, name)
		result <- struct {
			exists bool
			err    error
		}{exists: exists, err: err}
	}()

	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("FileExistsInDir() FIFO error: %v", got.err)
		}
		if got.exists {
			t.Fatal("FileExistsInDir() FIFO = true, want false")
		}
	case <-time.After(time.Second):
		t.Fatal("FileExistsInDir() blocked on FIFO")
	}
}

func TestRegularFileInDirSkipsSymlinkAndFIFO(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "regular"), []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(filepath.Join(tmp, "regular"), filepath.Join(tmp, "link")); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}
	if err := syscall.Mkfifo(filepath.Join(tmp, "fifo"), 0600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	for _, tc := range []struct {
		name string
		want bool
	}{
		{name: "regular", want: true},
		{name: "link", want: false},
		{name: "fifo", want: false},
		{name: "missing", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RegularFileInDir(tmp, tc.name)
			if err != nil {
				t.Fatalf("RegularFileInDir() error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("RegularFileInDir() = %v, want %v", got, tc.want)
			}
		})
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

	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock() second call error: %v", err)
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
