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

func TestSanitizeGPGErrorKeywordDropsGPGPrefixedLine(t *testing.T) {
	// The gpg: prefix would make the line safe; only the keyword filter drops it.
	got := sanitizeGPGError("gpg: ok\ngpg: client-key-data: abc\ngpg: token: xyz\n")
	if got != "gpg: ok" {
		t.Errorf("sanitizeGPGError() = %q, want %q", got, "gpg: ok")
	}
}

func TestSanitizeGPGErrorBase64LineDropped(t *testing.T) {
	got := sanitizeGPGError("secret key not available\n" + strings.Repeat("A", 80) + "\n")
	if got != "secret key not available" {
		t.Errorf("sanitizeGPGError() = %q", got)
	}
}

func TestSanitizeGPGErrorLineLengthBoundary(t *testing.T) {
	atLimit := "gpg: " + strings.Repeat("x", maxGPGErrorLineLen-5)
	over := atLimit + "x"
	if got := sanitizeGPGError(atLimit); got != atLimit {
		t.Errorf("line of %d bytes should be kept, got %q", len(atLimit), got)
	}
	if got := sanitizeGPGError(over); got != "unknown gpg error" {
		t.Errorf("line of %d bytes should be dropped, got %q", len(over), got)
	}
}

func TestSanitizeGPGErrorTotalLengthBoundary(t *testing.T) {
	// Lines of 199, 199, 199 and 197 bytes joined by "; " give exactly 800 bytes.
	line := "gpg: " + strings.Repeat("x", 194)
	last := "gpg: " + strings.Repeat("x", 192)
	stderr := strings.Join([]string{line, line, line, last}, "\n")
	want := strings.Join([]string{line, line, line, last}, "; ")
	if len(want) != maxGPGErrorLen {
		t.Fatalf("test setup: len = %d", len(want))
	}
	if got := sanitizeGPGError(stderr); got != want {
		t.Errorf("message at limit must not be truncated, got len %d", len(got))
	}
	got := sanitizeGPGError(stderr + "x")
	if !strings.HasSuffix(got, "... (truncated)") || len(got) != maxGPGErrorLen+len("... (truncated)") {
		t.Errorf("message over limit must be truncated, got len %d", len(got))
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
