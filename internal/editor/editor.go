// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package editor resolves the user's preferred text editor and provides
// a secure edit-in-tempfile workflow.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// fallbackEditors is the ordered list of editors to try when $EDITOR is unset.
var fallbackEditors = []string{"vim", "nvim", "nano"}

// ResolveEditor determines which editor to use.
// Priority: $EDITOR > $VISUAL > fallback chain.
func ResolveEditor() (string, error) {
	if e := os.Getenv("EDITOR"); e != "" {
		return e, nil
	}
	if e := os.Getenv("VISUAL"); e != "" {
		return e, nil
	}
	for _, name := range fallbackEditors {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no editor found: set $EDITOR or install vim/nvim/nano")
}

// Edit opens the user's editor with initialContent pre-filled in a secure
// temp file. Returns the file content after the editor exits.
// The temp file is created with 0600 permissions and cleaned up afterward.
func Edit(initialContent []byte) ([]byte, error) {
	editorCmd, err := ResolveEditor()
	if err != nil {
		return nil, err
	}

	// Create temp file in a private directory.
	tmpDir, err := os.MkdirTemp("", "kubegonfig-edit-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.Chmod(tmpDir, 0700); err != nil {
		return nil, fmt.Errorf("chmod temp dir: %w", err)
	}

	tmpFile := tmpDir + "/kubeconfig.yaml"
	if err := os.WriteFile(tmpFile, initialContent, 0600); err != nil {
		return nil, fmt.Errorf("write temp file: %w", err)
	}

	// Parse editor command (handles cases like "code --wait").
	parts := strings.Fields(editorCmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty editor command")
	}

	args := append(parts[1:], tmpFile)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("editor exited with error: %w", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("read edited file: %w", err)
	}

	return content, nil
}
