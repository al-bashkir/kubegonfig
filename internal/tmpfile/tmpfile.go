// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package tmpfile manages the lifecycle of temporary decrypted kubeconfig
// files. Files are created in a secure runtime directory with restricted
// permissions; the explicit cleanup command removes stale ones.
package tmpfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kubegonfig/internal/shell"
	"kubegonfig/internal/storage"
)

// Create writes decrypted data to a temp file in the secure runtime dir.
// Returns the absolute path to the created file.
func Create(name string, data []byte) (string, error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}

	// Use a predictable name so repeated activations reuse the same path.
	dir := storage.RuntimeDir()
	if err := storage.AtomicWriteInDir(dir, name+".yaml", data, 0600); err != nil {
		return "", fmt.Errorf("write temp kubeconfig: %w", err)
	}
	return filepath.Join(dir, name+".yaml"), nil
}

// ProbeCached reports whether the plaintext activation file for name is fresh
// enough to reuse. A cache hit requires a regular file in the runtime dir with
// modification time strictly greater than cipherMtime.
func ProbeCached(name string, cipherMtime time.Time) (string, bool, error) {
	if err := shell.ValidateName(name); err != nil {
		return "", false, err
	}

	dir := storage.RuntimeDir()
	fileName := name + ".yaml"
	path := filepath.Join(dir, fileName)

	mtime, ok, err := storage.RegularFileMtimeInDir(dir, fileName)
	if err != nil {
		return path, false, err
	}
	return path, ok && mtime.After(cipherMtime), nil
}

// OpenUnlinked writes decrypted data to an open temp file and immediately
// removes the directory entry. The returned file can be inherited by a child
// process without leaving a named plaintext kubeconfig in the runtime dir.
func OpenUnlinked(name string, data []byte) (*os.File, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}

	tmp, err := storage.OpenUnlinkedTempFileInDir(storage.RuntimeDir(), name+"-", ".yaml", data, 0600)
	if err != nil {
		return nil, fmt.Errorf("create unlinked temp kubeconfig: %w", err)
	}
	return tmp, nil
}

// Remove deletes the activation file for the named profile.
func Remove(name string) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	return storage.RemoveFileInDir(storage.RuntimeDir(), name+".yaml")
}

// CleanupStale removes stale kubegonfig kubeconfig temp files from the runtime directory.
// Used by the explicit "cleanup" command.
func CleanupStale() (int, error) {
	dir := storage.RuntimeDir()
	entries, err := storage.ReadDirInDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read runtime dir: %w", err)
	}

	count := 0
	for _, e := range entries {
		// ponytail: trust readdir d_type like profile.List; misses files on
		// DT_UNKNOWN filesystems (some FUSE).
		if !isKubeconfigTempName(e.Name()) || !e.Type().IsRegular() {
			continue
		}
		if err := storage.RemoveFileInDir(dir, e.Name()); err != nil {
			return count, fmt.Errorf("remove %s: %w", filepath.Join(dir, e.Name()), err)
		}
		count++
	}
	return count, nil
}

func isKubeconfigTempName(name string) bool {
	profileName, ok := strings.CutSuffix(name, ".yaml")
	return ok && shell.ValidateName(profileName) == nil
}
