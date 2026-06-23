// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package crypto provides GPG encryption and decryption via the external
// gpg binary. It uses the user's existing GPG keyring and agent.
package crypto

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const (
	maxGPGErrorLineLen = 240
	maxGPGErrorLen     = 800
)

// CheckGPG verifies that a usable gpg binary exists in PATH.
func CheckGPG() error {
	_, err := lookupGPG()
	return err
}

// lookupGPG resolves the gpg binary in PATH. exec.LookPath is cheap and runs a
// handful of times per short-lived CLI invocation, so the result is not cached.
func lookupGPG() (string, error) {
	for _, name := range []string{"gpg2", "gpg"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("gpg not found in PATH; install GnuPG to use kubegonfig")
}

// Encrypt encrypts data for the given recipients using GPG. At least one
// non-blank recipient must be specified; callers (config.Recipients) already
// trim and deduplicate. Blank entries are dropped here as a fail-closed guard.
func Encrypt(data []byte, recipients []string) ([]byte, error) {
	recipients = nonBlank(recipients)
	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one GPG recipient is required")
	}

	bin, err := lookupGPG()
	if err != nil {
		return nil, err
	}

	args := []string{
		"--batch", "--yes",
		"--encrypt",
		"--armor",
		"--trust-model", "always",
	}
	for _, r := range recipients {
		args = append(args, "--recipient", r)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Strip any potential secret data from stderr; only show gpg errors.
		errMsg := sanitizeGPGError(stderr.String())
		return nil, fmt.Errorf("gpg encrypt: %s: %w", errMsg, err)
	}

	return stdout.Bytes(), nil
}

func nonBlank(recipients []string) []string {
	out := make([]string, 0, len(recipients))
	for _, r := range recipients {
		if strings.TrimSpace(r) != "" {
			out = append(out, r)
		}
	}
	return out
}

// Decrypt decrypts GPG-encrypted data. Relies on gpg-agent for passphrase.
func Decrypt(data []byte) ([]byte, error) {
	bin, err := lookupGPG()
	if err != nil {
		return nil, err
	}

	args := []string{
		"--batch", "--yes",
		"--decrypt",
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := sanitizeGPGError(stderr.String())
		return nil, fmt.Errorf("gpg decrypt: %s: %w", errMsg, err)
	}

	return stdout.Bytes(), nil
}

// GPGKey represents a secret key from the user's GPG keyring.
type GPGKey struct {
	KeyID       string   // Long key ID (16 hex chars)
	Fingerprint string   // Full fingerprint
	UIDs        []string // User ID strings (e.g. "Name <email>")
}

// ListSecretKeys returns all secret keys available in the GPG keyring.
func ListSecretKeys() ([]GPGKey, error) {
	bin, err := lookupGPG()
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin,
		"--list-secret-keys",
		"--with-colons",
		"--keyid-format", "long",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := sanitizeGPGError(stderr.String())
		return nil, fmt.Errorf("gpg list-secret-keys: %s: %w", errMsg, err)
	}

	var keys []GPGKey
	var current *GPGKey

	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 10 {
			continue
		}

		switch fields[0] {
		case "sec":
			keys = append(keys, GPGKey{KeyID: fields[4]})
			current = &keys[len(keys)-1]
		case "fpr":
			if current != nil && current.Fingerprint == "" {
				current.Fingerprint = fields[9]
			}
		case "uid":
			if current != nil {
				uid := fields[9]
				if uid != "" {
					current.UIDs = append(current.UIDs, uid)
				}
			}
		}
	}

	return keys, nil
}

// sanitizeGPGError keeps short actionable GPG diagnostics while dropping lines
// that look like raw input, armored data, or unbounded wrapper output.
func sanitizeGPGError(stderr string) string {
	var safe []string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isSafeGPGDiagnostic(line) {
			safe = append(safe, line)
		}
	}
	if len(safe) == 0 {
		return "unknown gpg error"
	}
	msg := strings.Join(safe, "; ")
	if len(msg) > maxGPGErrorLen {
		return msg[:maxGPGErrorLen] + "... (truncated)"
	}
	return msg
}

func isSafeGPGDiagnostic(line string) bool {
	if len(line) > maxGPGErrorLineLen {
		return false
	}
	lower := strings.ToLower(line)
	if looksSensitiveGPGLine(lower) || looksBase64Line(line) {
		return false
	}
	if strings.HasPrefix(lower, "gpg:") {
		return true
	}
	return lower == "secret key not available"
}

func looksSensitiveGPGLine(lower string) bool {
	sensitive := []string{
		"-----begin pgp",
		"-----end pgp",
		"key data",
		"literal data",
		"apiversion:",
		"kind:",
		"clusters:",
		"contexts:",
		"users:",
		"current-context:",
		"server:",
		"certificate-authority-data:",
		"client-certificate-data:",
		"client-key-data:",
		"token:",
		"password:",
	}
	for _, marker := range sensitive {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func looksBase64Line(line string) bool {
	if len(line) < 80 {
		return false
	}
	for _, r := range line {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '=' {
			continue
		}
		return false
	}
	return true
}
