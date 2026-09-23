// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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
	dir := RuntimeDir()
	want := "/run/user/1000/kubegonfig"
	if dir != want {
		t.Errorf("RuntimeDir() = %q, want %q", dir, want)
	}
}

func TestRuntimeDir_Fallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	dir := RuntimeDir()
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
	if err := unix.Mkfifo(fifo, 0600); err != nil {
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

	if err := AtomicWriteInDir(filepath.Dir(path), filepath.Base(path), content, 0600); err != nil {
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

	if err := AtomicWriteInDir(filepath.Dir(path), filepath.Base(path), []byte("first"), 0600); err != nil {
		t.Fatalf("AtomicWrite(first) error: %v", err)
	}

	if err := AtomicWriteInDir(filepath.Dir(path), filepath.Base(path), []byte("second"), 0600); err != nil {
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

	if err := AtomicWriteInDir(filepath.Dir(path), filepath.Base(path), []byte("nested"), 0600); err != nil {
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
	oldUmask := unix.Umask(0077)
	t.Cleanup(func() {
		unix.Umask(oldUmask)
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
	if err := unix.Mkfifo(filepath.Join(tmp, name), 0600); err != nil {
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

func TestRegularFileInDirSkipsSymlinkAndFIFO(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "regular"), []byte("data"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(filepath.Join(tmp, "regular"), filepath.Join(tmp, "link")); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}
	if err := unix.Mkfifo(filepath.Join(tmp, "fifo"), 0600); err != nil {
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

func TestWithLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	ran := false
	if err := WithLock(lockPath, func() error { ran = true; return nil }); err != nil {
		t.Fatalf("WithLock() error: %v", err)
	}
	if !ran {
		t.Fatal("WithLock() did not run fn")
	}

	want := errors.New("boom")
	if err := WithLock(lockPath, func() error { return want }); !errors.Is(err, want) {
		t.Fatalf("WithLock() error = %v, want %v", err, want)
	}
	assertLockFree(t, lockPath)
}

func TestWithLockReleasesOnPanic(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	func() {
		defer func() { _ = recover() }()
		_ = WithLock(lockPath, func() error { panic("boom") })
	}()
	assertLockFree(t, lockPath)
}

func assertLockFree(t *testing.T, lockPath string) {
	t.Helper()
	f, err := os.OpenFile(lockPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("lock still held: %v", err)
	}
}

func TestWithLockRejectsSymlinkLockFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	lockPath := filepath.Join(tmp, "test.lock")
	if err := os.WriteFile(target, []byte("target"), 0600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Symlink(target, lockPath); err != nil {
		t.Skipf("Symlink() unavailable: %v", err)
	}

	if err := WithLock(lockPath, func() error { return nil }); err == nil {
		t.Fatal("WithLock() error = nil, want symlink rejection")
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func TestAtomicWriteStream_Success(t *testing.T) {
	dir := t.TempDir()

	err := AtomicWriteStream(dir, "out.dat", 0600, func(w io.Writer) error {
		if _, err := w.Write([]byte("hello, ")); err != nil {
			return err
		}
		_, err := w.Write([]byte("world"))
		return err
	})
	if err != nil {
		t.Fatalf("AtomicWriteStream() error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "out.dat"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != "hello, world" {
		t.Fatalf("file contents = %q, want %q", got, "hello, world")
	}

	info, err := os.Stat(filepath.Join(dir, "out.dat"))
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestAtomicWriteStream_CallbackErrorRemovesTemp(t *testing.T) {
	dir := t.TempDir()
	want := errors.New("boom")

	err := AtomicWriteStream(dir, "out.dat", 0600, func(w io.Writer) error {
		if _, werr := w.Write([]byte("partial")); werr != nil {
			return werr
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("AtomicWriteStream() error = %v, want wrap of %v", err, want)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "out.dat")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination file should not exist after callback error: stat err = %v", statErr)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("temp file %q left behind after callback error", e.Name())
		}
	}
}

func TestAtomicWriteStream_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	err := AtomicWriteStream(dir, "../escape", 0600, func(w io.Writer) error { return nil })
	if err == nil {
		t.Fatal("AtomicWriteStream() error = nil, want invalid name error")
	}
}

func TestRegularFileMtimeInDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	want := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, want, want); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	mtime, ok, err := RegularFileMtimeInDir(dir, "file")
	if err != nil {
		t.Fatalf("RegularFileMtimeInDir() error: %v", err)
	}
	if !ok {
		t.Fatal("RegularFileMtimeInDir() ok = false, want true")
	}
	if !mtime.Equal(want) {
		t.Errorf("mtime = %v, want %v", mtime, want)
	}
}

func TestRegularFileMtimeInDirMissing(t *testing.T) {
	dir := t.TempDir()
	mtime, ok, err := RegularFileMtimeInDir(dir, "missing")
	if err != nil {
		t.Fatalf("RegularFileMtimeInDir() error: %v", err)
	}
	if ok {
		t.Errorf("ok = true, want false")
	}
	if !mtime.IsZero() {
		t.Errorf("mtime = %v, want zero", mtime)
	}
}

func TestRegularFileMtimeInDirRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, ok, err := RegularFileMtimeInDir(dir, "link")
	if err != nil {
		t.Fatalf("RegularFileMtimeInDir() error: %v", err)
	}
	if ok {
		t.Errorf("ok = true for symlink, want false")
	}
}

func TestRegularFileMtimeInDirRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, ok, err := RegularFileMtimeInDir(dir, "sub")
	if err != nil {
		t.Fatalf("RegularFileMtimeInDir() error: %v", err)
	}
	if ok {
		t.Errorf("ok = true for directory, want false")
	}
}

func TestRegularFileMtimeInDirMissingDir(t *testing.T) {
	_, ok, err := RegularFileMtimeInDir("/nonexistent/dir", "file")
	if err != nil {
		t.Fatalf("RegularFileMtimeInDir() error: %v", err)
	}
	if ok {
		t.Errorf("ok = true for missing dir, want false")
	}
}

func TestRegularFileMtimeInDirRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := RegularFileMtimeInDir(dir, "../etc"); err == nil {
		t.Fatal("RegularFileMtimeInDir() error = nil, want validation error")
	}
}
