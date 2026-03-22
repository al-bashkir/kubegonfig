// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package crypto

import (
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
			name:   "filters secret lines",
			stderr: "gpg: error\nsecret key not available\ngpg: failed\n",
			want:   "gpg: error; gpg: failed",
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
			stderr: "secret key info\nkey data block\n",
			want:   "unknown gpg error",
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
