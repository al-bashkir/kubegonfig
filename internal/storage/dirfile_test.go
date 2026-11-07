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

	"golang.org/x/sys/unix"
)

func TestEnsureDirRejectsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	err := EnsureDir(path, 0700)
	if err == nil || !strings.Contains(err.Error(), "create directory") {
		t.Fatalf("EnsureDir() error = %v, want create directory failure", err)
	}
}

func TestRemoveFileInDirMissingFileIsIdempotent(t *testing.T) {
	if err := RemoveFileInDir(t.TempDir(), "absent"); err != nil {
		t.Fatalf("RemoveFileInDir() missing file error: %v", err)
	}
}

func TestRemoveFileInDirFailsOnNonEmptyDirectory(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFileInDir(tmp, "sub"); err == nil {
		t.Fatal("RemoveFileInDir() error = nil, want failure on directory")
	}
	if _, err := os.Stat(sub); err != nil {
		t.Fatal("directory was removed")
	}
}

func TestReadFileInDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "f"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFileInDir(tmp, "f")
	if err != nil {
		t.Fatalf("ReadFileInDir() error: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadFileInDir() = %q, want %q", got, "hello")
	}
}

func TestReadFileInDirRejectsDirectoryEntry(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmp, "d"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileInDir(tmp, "d"); err == nil {
		t.Fatal("ReadFileInDir() error = nil, want not-regular error")
	}
}

func TestOpenUnlinkedTempFileInDir(t *testing.T) {
	tmp := t.TempDir()
	f, err := OpenUnlinkedTempFileInDir(tmp, "kc-", ".yaml", []byte("secret"), 0600)
	if err != nil {
		t.Fatalf("OpenUnlinkedTempFileInDir() error: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := io.ReadAll(f)
	if err != nil || string(got) != "secret" {
		t.Fatalf("read back = %q, %v; want %q", got, err, "secret")
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dir has %d entries, want 0 (unlinked)", len(entries))
	}
}

func TestOpenUnlinkedTempFileInDirRejects(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "link")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if f, err := OpenUnlinkedTempFileInDir(link, "p-", "", nil, 0600); err == nil {
		_ = f.Close()
		t.Fatal("want symlinked dir rejection")
	}
	if f, err := OpenUnlinkedTempFileInDir(target, "a/b", "", nil, 0600); err == nil {
		_ = f.Close()
		t.Fatal("want invalid prefix rejection")
	}
}

func TestReadDirInDir(t *testing.T) {
	tmp := t.TempDir()
	for _, n := range []string{"b", "a"} {
		if err := os.WriteFile(filepath.Join(tmp, n), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadDirInDir(tmp)
	if err != nil {
		t.Fatalf("ReadDirInDir() error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestReadDirInDirMissingAndSymlink(t *testing.T) {
	tmp := t.TempDir()
	entries, err := ReadDirInDir(filepath.Join(tmp, "missing"))
	if err != nil || entries != nil {
		t.Fatalf("missing dir = %v, %v; want nil, nil", entries, err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(tmp, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDirInDir(link); err == nil {
		t.Fatal("want symlinked dir rejection")
	}
}

func TestWithLockCreatesMissingDirAndHoldsLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "sub", "x.lock")
	err := WithLock(lockPath, func() error {
		f, err := os.OpenFile(lockPath, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		if unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil {
			return errors.New("lock not held inside fn")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRegularFileMtimeInDirErrorsReportNotOK(t *testing.T) {
	tmp := t.TempDir()
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(tmp, link); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := RegularFileMtimeInDir(link, "f"); err == nil || ok {
		t.Errorf("symlinked dir: ok=%v err=%v, want ok=false and error", ok, err)
	}
	if _, ok, err := RegularFileMtimeInDir(tmp, "../x"); err == nil || ok {
		t.Errorf("invalid name: ok=%v err=%v, want ok=false and error", ok, err)
	}
}

func TestWithLockRejectsInvalidAndNonRegular(t *testing.T) {
	tmp := t.TempDir()
	if err := WithLock(tmp+"/..", func() error { return nil }); err == nil {
		t.Error("want invalid name rejection")
	}
	fifo := filepath.Join(tmp, "fifo")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if err := WithLock(fifo, func() error { return nil }); err == nil {
		t.Error("want non-regular rejection")
	}
}
