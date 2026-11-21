// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package editor

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestResolveEditorPrefersVisual(t *testing.T) {
	t.Setenv("VISUAL", "visual-editor")
	t.Setenv("EDITOR", "editor-command")

	got, err := ResolveEditor()
	if err != nil {
		t.Fatalf("ResolveEditor() error: %v", err)
	}
	if got != "visual-editor" {
		t.Fatalf("ResolveEditor() = %q, want visual-editor", got)
	}
}

func TestResolveEditorUsesEditorWhenVisualUnset(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "editor-command")

	got, err := ResolveEditor()
	if err != nil {
		t.Fatalf("ResolveEditor() error: %v", err)
	}
	if got != "editor-command" {
		t.Fatalf("ResolveEditor() = %q, want editor-command", got)
	}
}

func TestEditWritesTempUnderRuntimeDir(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)

	// Fake editor records the path it was handed and leaves the file unchanged.
	recordPath := filepath.Join(t.TempDir(), "edited-path")
	script := "#!/bin/sh\nprintf '%s' \"$1\" > " + recordPath + "\n"
	editorPath := filepath.Join(t.TempDir(), "fake-editor")
	if err := os.WriteFile(editorPath, []byte(script), 0700); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}
	t.Setenv("VISUAL", editorPath)

	out, err := Edit([]byte("hello"))
	if err != nil {
		t.Fatalf("Edit() error: %v", err)
	}
	if string(out) != "hello" {
		t.Fatalf("Edit() content = %q, want %q", out, "hello")
	}

	got, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read recorded path: %v", err)
	}
	wantPrefix := filepath.Join(runtime, "kubegonfig") + string(os.PathSeparator)
	if !strings.HasPrefix(string(got), wantPrefix) {
		t.Fatalf("edit temp path = %q, want under %q", got, wantPrefix)
	}
}

func TestEditRemovesTempDirWhenSignaled(t *testing.T) {
	runtime := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	// The editor announces itself, then blocks until it is signaled.
	script := "#!/bin/sh\ntouch \"$1.started\"\nexec sleep 30\n"
	editorPath := filepath.Join(t.TempDir(), "slow-editor")
	if err := os.WriteFile(editorPath, []byte(script), 0700); err != nil {
		t.Fatalf("write fake editor: %v", err)
	}
	t.Setenv("VISUAL", editorPath)

	errc := make(chan error, 1)
	go func() {
		_, err := Edit([]byte("secret"))
		errc <- err
	}()

	started := filepath.Join(runtime, "kubegonfig", "kubegonfig-edit-*", "kubeconfig.yaml.started")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if m, _ := filepath.Glob(started); len(m) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the editor to start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case err := <-errc:
		if err == nil {
			t.Error("Edit() error = nil, want an interruption error")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Edit() did not return after SIGTERM")
	}
	left, _ := filepath.Glob(filepath.Join(runtime, "kubegonfig", "kubegonfig-edit-*"))
	if len(left) != 0 {
		t.Errorf("edit dir left behind after SIGTERM: %v", left)
	}
}
