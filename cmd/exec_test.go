// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kubegonfig/internal/storage"
	"kubegonfig/internal/tmpfile"
)

func TestRunExecWithKubeconfigFileSetsKubeconfigAndClosesFile(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	kubeconfig := openTestExecKubeconfig(t, "exec-success", []byte("apiVersion: v1\n"))
	outPath := filepath.Join(t.TempDir(), "kubeconfig-path")
	contentPath := filepath.Join(t.TempDir(), "kubeconfig-content")

	exitCode, err := runExecWithKubeconfigFile([]string{
		"sh",
		"-c",
		`test -f "$KUBECONFIG" && printf '%s' "$KUBECONFIG" > "$1" && cat "$KUBECONFIG" > "$2"`,
		"sh",
		outPath,
		contentPath,
	}, kubeconfig)
	if err != nil {
		t.Fatalf("runExecWithKubeconfigFile() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", outPath, err)
	}
	if got := string(data); got != execKubeconfigPath {
		t.Fatalf("child KUBECONFIG = %q, want %q", got, execKubeconfigPath)
	}

	content, err := os.ReadFile(contentPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", contentPath, err)
	}
	if got := string(content); got != "apiVersion: v1\n" {
		t.Fatalf("child kubeconfig content = %q", got)
	}
	assertRuntimeDirEmpty(t)
}

func TestExecKubeconfigIsUnlinkedWhileChildRuns(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	kubeconfig := openTestExecKubeconfig(t, "exec-running", []byte("apiVersion: v1\n"))
	readyPath := filepath.Join(t.TempDir(), "ready")
	donePath := filepath.Join(t.TempDir(), "done")
	child := newExecCommandWithKubeconfig([]string{
		"sh",
		"-c",
		`cat "$KUBECONFIG" > /dev/null || exit 42; : > "$1"; while [ ! -e "$2" ]; do sleep 0.05; done`,
		"sh",
		readyPath,
		donePath,
	}, kubeconfig)

	if err := child.Start(); err != nil {
		_ = kubeconfig.Close()
		t.Fatalf("Start() error = %v", err)
	}
	waitForFile(t, readyPath)
	assertRuntimeDirEmpty(t)

	if err := os.WriteFile(donePath, []byte("done"), 0600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", donePath, err)
	}
	if err := child.Wait(); err != nil {
		_ = kubeconfig.Close()
		t.Fatalf("Wait() error = %v", err)
	}
	if err := kubeconfig.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestRunExecWithKubeconfigFilePreservesChildExit(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	kubeconfig := openTestExecKubeconfig(t, "exec-failure", []byte("apiVersion: v1\n"))

	exitCode, err := runExecWithKubeconfigFile([]string{
		"sh",
		"-c",
		`test -f "$KUBECONFIG" || exit 42; exit 7`,
	}, kubeconfig)
	if err != nil {
		t.Fatalf("runExecWithKubeconfigFile() error = %v", err)
	}
	if exitCode != 7 {
		t.Fatalf("exit code = %d, want 7", exitCode)
	}
	assertRuntimeDirEmpty(t)
}

func TestRunExecWithKubeconfigFileClosesAfterStartError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	kubeconfig := openTestExecKubeconfig(t, "exec-start-error", []byte("apiVersion: v1\n"))

	exitCode, err := runExecWithKubeconfigFile([]string{
		filepath.Join(t.TempDir(), "missing-command"),
	}, kubeconfig)
	if err == nil {
		t.Fatal("runExecWithKubeconfigFile() error = nil, want error")
	}
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	assertRuntimeDirEmpty(t)
}

func openTestExecKubeconfig(t *testing.T, name string, data []byte) *os.File {
	t.Helper()

	kubeconfig, err := tmpfile.OpenUnlinked(name, data)
	if err != nil {
		t.Fatalf("OpenUnlinked() error = %v", err)
	}
	return kubeconfig
}

func assertRuntimeDirEmpty(t *testing.T) {
	t.Helper()

	dir, err := storage.RuntimeDir()
	if err != nil {
		t.Fatalf("RuntimeDir() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("runtime dir contains %d entries, want 0", len(entries))
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
