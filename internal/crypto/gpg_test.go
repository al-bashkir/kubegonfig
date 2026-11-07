// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package crypto

import (
	"os"
	"path/filepath"
	"reflect"
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

func writeFakeGPG(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"gpg2", "gpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
			t.Fatalf("write fake gpg: %v", err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestEncryptPassesRecipientsAndStdin(t *testing.T) {
	writeFakeGPG(t, `echo "$@"; cat`)
	out, err := Encrypt([]byte("plain"), []string{"a@x", "B2"})
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	want := "--batch --yes --encrypt --armor --trust-model always --recipient a@x --recipient B2\nplain"
	if string(out) != want {
		t.Errorf("Encrypt() = %q, want %q", out, want)
	}
}

func TestDecryptPassesArgsAndStdin(t *testing.T) {
	writeFakeGPG(t, `echo "$@"; cat`)
	out, err := Decrypt([]byte("cipher"))
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if want := "--batch --yes --decrypt\ncipher"; string(out) != want {
		t.Errorf("Decrypt() = %q, want %q", out, want)
	}
}

func TestRunGPGFailureIsWrappedAndSanitized(t *testing.T) {
	writeFakeGPG(t, `echo "gpg: decryption failed: No secret key" >&2
echo "client-key-data: SECRET" >&2
exit 2`)
	out, err := Decrypt([]byte("cipher"))
	if err == nil {
		t.Fatal("Decrypt() error = nil, want failure")
	}
	if out != nil {
		t.Errorf("Decrypt() output = %q, want nil", out)
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "gpg decrypt: gpg: decryption failed: No secret key: ") || !strings.Contains(msg, "exit status 2") {
		t.Errorf("error = %q", msg)
	}
	if strings.Contains(msg, "SECRET") {
		t.Errorf("error leaks stderr data: %q", msg)
	}
}

func TestRunGPGMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Decrypt(nil); err == nil || !strings.Contains(err.Error(), "gpg not found") {
		t.Errorf("Decrypt() error = %v, want gpg not found", err)
	}
}

func TestListSecretKeys(t *testing.T) {
	writeFakeGPG(t, `if [ "$*" != "--list-secret-keys --with-colons" ]; then echo "bad args: $*" >&2; exit 1; fi
cat <<'EOF'
tru::1:1700000000:0:3:1:5
short:line
fpr:::::::::ORPHANFPR:
uid:u::::1700000000::HASH::Orphan <o@x>:
sec:u:255:22:AAAA1111AAAA1111:1700000000:::u:::scESC:::+:::23::0:
fpr:::::::::PRIMARYFPR1:
uid:u::::1700000000::HASH1::Alice <a@x>:
uid:u::::1700000000::HASH2::Alice Work <a@w>:
uid:u::::1700000000::HASH3:::
uid:::::::::Bob <b@x>
ssb:u:255:18:SUBKEYID:1700000000::::::e:::+:::23:
fpr:::::::::SUBFPR:
sec:u:255:22:BBBB2222BBBB2222:1700000000:::u:::scESC:::+:::23::0:
fpr:::
EOF`)
	keys, err := ListSecretKeys()
	if err != nil {
		t.Fatalf("ListSecretKeys() error = %v", err)
	}
	want := []GPGKey{
		{KeyID: "AAAA1111AAAA1111", Fingerprint: "PRIMARYFPR1", UIDs: []string{"Alice <a@x>", "Alice Work <a@w>", "Bob <b@x>"}},
		{KeyID: "BBBB2222BBBB2222"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("ListSecretKeys() = %+v, want %+v", keys, want)
	}
}

func TestListSecretKeysError(t *testing.T) {
	writeFakeGPG(t, `echo "gpg: keyring broken" >&2; exit 2`)
	keys, err := ListSecretKeys()
	if err == nil || !strings.Contains(err.Error(), "gpg list-secret-keys: gpg: keyring broken") {
		t.Errorf("ListSecretKeys() = %v, %v", keys, err)
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
