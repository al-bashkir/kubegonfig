// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestNormalizeRecipients(t *testing.T) {
	got := normalizeRecipients([]string{" user@example.com ", "", "second@example.com", "user@example.com", "\t"})
	want := []string{"user@example.com", "second@example.com"}
	if len(got) != len(want) {
		t.Fatalf("normalizeRecipients() len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeRecipients()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGPGPathReresolvesMissingCachedBinary(t *testing.T) {
	setCachedGPGForTest(t, "")

	oldDir := t.TempDir()
	oldPath := filepath.Join(oldDir, "gpg2")
	if err := os.WriteFile(oldPath, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write old gpg: %v", err)
	}
	gpgMu.Lock()
	gpgBinary = oldPath
	gpgMu.Unlock()
	if err := os.Remove(oldPath); err != nil {
		t.Fatalf("remove old gpg: %v", err)
	}

	newDir := t.TempDir()
	newPath := filepath.Join(newDir, "gpg2")
	if err := os.WriteFile(newPath, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write new gpg: %v", err)
	}
	t.Setenv("PATH", newDir)

	got, err := gpgPath()
	if err != nil {
		t.Fatalf("gpgPath() error = %v", err)
	}
	if got != newPath {
		t.Fatalf("gpgPath() = %q, want %q", got, newPath)
	}
}

func TestGPGPathConcurrentCacheAccess(t *testing.T) {
	setCachedGPGForTest(t, "")

	dir := t.TempDir()
	path := filepath.Join(dir, "gpg2")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatalf("write gpg: %v", err)
	}
	t.Setenv("PATH", dir)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := CheckGPG(); err != nil {
				t.Errorf("CheckGPG() error = %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			got, err := gpgPath()
			if err != nil {
				t.Errorf("gpgPath() error = %v", err)
				return
			}
			if got != path {
				t.Errorf("gpgPath() = %q, want %q", got, path)
			}
		}()
	}
	wg.Wait()
}

func setCachedGPGForTest(t *testing.T, path string) string {
	t.Helper()

	var oldBinary string
	gpgMu.Lock()
	oldBinary = gpgBinary
	gpgBinary = path
	gpgMu.Unlock()
	t.Cleanup(func() {
		gpgMu.Lock()
		gpgBinary = oldBinary
		gpgMu.Unlock()
	})
	return oldBinary
}
