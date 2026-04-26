// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package tmpfile manages the lifecycle of temporary decrypted kubeconfig
// files. Files are created in a secure runtime directory with restricted
// permissions and can be cleaned up explicitly or on handled signals.
package tmpfile

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"kubegonfig/internal/shell"
	"kubegonfig/internal/storage"
)

var (
	// tracked holds paths of temp files created in this process.
	tracked   []string
	trackedMu sync.Mutex
)

// Create writes decrypted data to a temp file in the secure runtime dir.
// Returns the absolute path to the created file.
// The file is registered for cleanup on handled signals and CleanupAll.
func Create(name string, data []byte) (string, error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}

	dir, err := storage.RuntimeDir()
	if err != nil {
		return "", fmt.Errorf("resolve runtime dir: %w", err)
	}
	if err := storage.EnsureDir(dir, 0700); err != nil {
		return "", fmt.Errorf("create runtime dir: %w", err)
	}

	// Use a predictable name so repeated activations reuse the same path.
	path := filepath.Join(dir, name+".yaml")

	if err := storage.AtomicWrite(path, data, 0600); err != nil {
		return "", fmt.Errorf("write temp kubeconfig: %w", err)
	}

	trackedMu.Lock()
	tracked = append(tracked, path)
	trackedMu.Unlock()

	return path, nil
}

// OpenUnlinked writes decrypted data to an open temp file and immediately
// removes the directory entry. The returned file can be inherited by a child
// process without leaving a named plaintext kubeconfig in the runtime dir.
func OpenUnlinked(name string, data []byte) (*os.File, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}

	dir, err := storage.RuntimeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve runtime dir: %w", err)
	}
	tmp, err := storage.OpenUnlinkedTempFileInDir(dir, name+"-", ".yaml", data, 0600)
	if err != nil {
		return nil, fmt.Errorf("create unlinked temp kubeconfig: %w", err)
	}
	return tmp, nil
}

// Remove deletes a specific temp file.
func Remove(path string) error {
	dir, name, err := runtimeTempPathParts(path)
	if err != nil {
		return err
	}
	if err := storage.RemoveFileInDir(dir, name); err != nil {
		return err
	}
	untrack(path)
	return nil
}

// CleanupAll removes all temp files created by this process.
func CleanupAll() {
	trackedMu.Lock()
	paths := make([]string, len(tracked))
	copy(paths, tracked)
	trackedMu.Unlock()

	for _, p := range paths {
		_ = Remove(p)
	}
}

// CleanupStale removes stale kubegonfig kubeconfig temp files from the runtime directory.
// Used by the explicit "cleanup" command.
func CleanupStale() (int, error) {
	dir, err := storage.RuntimeDir()
	if err != nil {
		return 0, fmt.Errorf("resolve runtime dir: %w", err)
	}

	entries, err := storage.ReadDirInDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read runtime dir: %w", err)
	}

	count := 0
	for _, e := range entries {
		if !isKubeconfigTempName(e.Name()) {
			continue
		}
		regular, err := storage.RegularFileInDir(dir, e.Name())
		if err != nil {
			return count, fmt.Errorf("check %s: %w", filepath.Join(dir, e.Name()), err)
		}
		if !regular {
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

func runtimeTempPathParts(path string) (string, string, error) {
	dir, err := storage.RuntimeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve runtime dir: %w", err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve runtime dir path: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve temp file path: %w", err)
	}
	name := filepath.Base(absPath)
	if filepath.Dir(absPath) != absDir || !isKubeconfigTempName(name) {
		return "", "", fmt.Errorf("refusing to remove non-kubegonfig runtime temp file %q", path)
	}
	return dir, name, nil
}

func untrack(path string) {
	trackedMu.Lock()
	defer trackedMu.Unlock()

	for i, trackedPath := range tracked {
		if trackedPath == path {
			tracked = append(tracked[:i], tracked[i+1:]...)
			return
		}
	}
}

// SetupSignalHandler registers cleanup on SIGINT and SIGTERM.
// Call once at program startup.
func SetupSignalHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		CleanupAll()
		os.Exit(1)
	}()
}
