// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package shell

import (
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names.
		{"simple", "prod", false},
		{"with-dash", "my-cluster", false},
		{"with-dot", "cluster.v2", false},
		{"with-underscore", "my_cluster", false},
		{"numeric-start", "1cluster", false},
		{"single-char", "a", false},
		{"max-length", "a234567890123456789012345678901234567890123456789012345678901234", false}, // 64 chars

		// Invalid names.
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"slash", "a/b", true},
		{"backslash", "a\\b", true},
		{"starts-with-dash", "-cluster", true},
		{"starts-with-dot", ".hidden", true},
		{"starts-with-underscore", "_cluster", true},
		{"space", "my cluster", true},
		{"special-chars", "cluster@prod", true},
		{"too-long", "a2345678901234567890123456789012345678901234567890123456789012345", true}, // 65 chars
		{"shell-metachar-semicolon", "a;b", true},
		{"shell-metachar-pipe", "a|b", true},
		{"shell-metachar-ampersand", "a&b", true},
		{"shell-metachar-dollar", "a$b", true},
		{"shell-metachar-backtick", "a`b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestEscapePosix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", "'hello'"},
		{"empty", "", "''"},
		{"with-space", "hello world", "'hello world'"},
		{"with-single-quote", "it's", "'it'\\''s'"},
		{"with-double-quote", `say "hi"`, `'say "hi"'`},
		{"with-dollar", "cost=$100", "'cost=$100'"},
		{"with-backtick", "run `cmd`", "'run `cmd`'"},
		{"with-path", "/tmp/kube/config", "'/tmp/kube/config'"},
		{"multiple-single-quotes", "a'b'c", "'a'\\''b'\\''c'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapePosix(tt.input)
			if got != tt.want {
				t.Errorf("EscapePosix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeStyle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"default", "", StylePosix, false},
		{"posix", StylePosix, StylePosix, false},
		{"fish", StyleFish, StyleFish, false},
		{"invalid", "powershell", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeStyle(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeStyle(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NormalizeStyle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatSet(t *testing.T) {
	if got, want := FormatSet(StylePosix, "KUBECONFIG", "/tmp/my config"), "export KUBECONFIG='/tmp/my config'"; got != want {
		t.Errorf("FormatSet(posix) = %q, want %q", got, want)
	}
	if got, want := FormatSet(StyleFish, "KUBECONFIG", "/tmp/config"), "set -gx KUBECONFIG '/tmp/config'"; got != want {
		t.Errorf("FormatSet(fish) = %q, want %q", got, want)
	}
}
