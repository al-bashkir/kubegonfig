// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package editor

import (
	"reflect"
	"testing"
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

func TestSplitCommandLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{name: "simple", in: "code --wait", want: []string{"code", "--wait"}},
		{name: "quoted arg", in: `code --user-data-dir "/tmp/my dir" --wait`, want: []string{"code", "--user-data-dir", "/tmp/my dir", "--wait"}},
		{name: "single quoted", in: `vim '+set ft=yaml'`, want: []string{"vim", "+set ft=yaml"}},
		{name: "escaped space", in: `nano /tmp/my\ file`, want: []string{"nano", "/tmp/my file"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := splitCommandLine(tc.in)
			if err != nil {
				t.Fatalf("splitCommandLine() error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("splitCommandLine() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestSplitCommandLineRejectsUnterminatedQuote(t *testing.T) {
	if _, err := splitCommandLine(`code "unterminated`); err == nil {
		t.Fatal("splitCommandLine() error = nil, want unterminated quote error")
	}
}
