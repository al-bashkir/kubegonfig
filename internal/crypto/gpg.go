// Package crypto provides GPG encryption and decryption via the external
// gpg binary. It uses the user's existing GPG keyring and agent.
package crypto

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// gpgBinary caches the resolved path to the gpg executable.
var gpgBinary string

// CheckGPG verifies that a usable gpg binary exists in PATH.
func CheckGPG() error {
	for _, name := range []string{"gpg2", "gpg"} {
		path, err := exec.LookPath(name)
		if err == nil {
			gpgBinary = path
			return nil
		}
	}
	return fmt.Errorf("gpg not found in PATH; install GnuPG to use kubegonfig")
}

// gpgPath returns the resolved gpg binary path, calling CheckGPG if needed.
func gpgPath() (string, error) {
	if gpgBinary != "" {
		return gpgBinary, nil
	}
	if err := CheckGPG(); err != nil {
		return "", err
	}
	return gpgBinary, nil
}

// Encrypt encrypts data for the given recipients using GPG.
// At least one recipient must be specified.
func Encrypt(data []byte, recipients []string) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one GPG recipient is required")
	}

	bin, err := gpgPath()
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

// Decrypt decrypts GPG-encrypted data. Relies on gpg-agent for passphrase.
func Decrypt(data []byte) ([]byte, error) {
	bin, err := gpgPath()
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

// sanitizeGPGError removes lines that could leak sensitive info from gpg stderr.
func sanitizeGPGError(stderr string) string {
	var safe []string
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Keep gpg status/error lines, drop anything that looks like data.
		lower := strings.ToLower(line)
		if strings.Contains(lower, "secret") ||
			strings.Contains(lower, "key") && strings.Contains(lower, "data") {
			continue
		}
		safe = append(safe, line)
	}
	if len(safe) == 0 {
		return "unknown gpg error"
	}
	return strings.Join(safe, "; ")
}
