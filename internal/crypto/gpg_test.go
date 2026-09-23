// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeGPGError(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{
			name:   "normal error",
			stderr: "gpg: encryption failed: No public key\n",
			want:   "gpg: encryption failed: No public key",
		},
		{
			name:   "keeps secret key diagnostics",
			stderr: "gpg: error\nsecret key not available\ngpg: failed\n",
			want:   "gpg: error; secret key not available; gpg: failed",
		},
		{
			name:   "keeps no secret key diagnostics",
			stderr: "gpg: public key decryption failed: No secret key\ngpg: decryption failed: No secret key\n",
			want:   "gpg: public key decryption failed: No secret key; gpg: decryption failed: No secret key",
		},
		{
			name:   "filters key data lines",
			stderr: "gpg: error\nkey data block\ngpg: done\n",
			want:   "gpg: error; gpg: done",
		},
		{
			name:   "empty stderr",
			stderr: "",
			want:   "unknown gpg error",
		},
		{
			name:   "only filtered lines",
			stderr: "key data block\nliteral data packet\n-----BEGIN PGP MESSAGE-----\n",
			want:   "unknown gpg error",
		},
		{
			name:   "filters kubeconfig looking lines",
			stderr: "gpg: error\napiVersion: v1\nclusters:\n- cluster:\n    server: https://secret.example\ngpg: failed\n",
			want:   "gpg: error; gpg: failed",
		},
		{
			name:   "filters base64 looking lines",
			stderr: "gpg: error\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\ngpg: failed\n",
			want:   "gpg: error; gpg: failed",
		},
		{
			name:   "whitespace only",
			stderr: "  \n  \n",
			want:   "unknown gpg error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeGPGError(tt.stderr)
			if got != tt.want {
				t.Errorf("sanitizeGPGError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeGPGErrorCapsOutput(t *testing.T) {
	var stderr strings.Builder
	for i := 0; i < 80; i++ {
		stderr.WriteString("gpg: repeated diagnostic line\n")
	}

	got := sanitizeGPGError(stderr.String())
	if len(got) > maxGPGErrorLen+len("... (truncated)") {
		t.Fatalf("sanitizeGPGError() length = %d, want capped output", len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("sanitizeGPGError() = %q, want truncation marker", got)
	}
}

func TestEncrypt_NoRecipients(t *testing.T) {
	_, err := Encrypt([]byte("test"), nil)
	if err == nil {
		t.Error("Encrypt(nil recipients) should return error")
	}
	if !strings.Contains(err.Error(), "at least one GPG recipient") {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = Encrypt([]byte("test"), []string{})
	if err == nil {
		t.Error("Encrypt(empty recipients) should return error")
	}
}

func TestNonBlank(t *testing.T) {
	got := nonBlank([]string{"user@example.com", "", "second@example.com", "\t"})
	want := []string{"user@example.com", "second@example.com"}
	if len(got) != len(want) {
		t.Fatalf("nonBlank() len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("nonBlank()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLookupGPGResolvesFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpg2")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write gpg: %v", err)
	}
	t.Setenv("PATH", dir)

	got, err := LookupGPG()
	if err != nil {
		t.Fatalf("LookupGPG() error = %v", err)
	}
	if got != path {
		t.Fatalf("LookupGPG() = %q, want %q", got, path)
	}
}
