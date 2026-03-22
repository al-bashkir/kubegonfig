// Package tmpfile manages the lifecycle of temporary decrypted kubeconfig
// files. Files are created in a secure runtime directory with restricted
// permissions and cleaned up on exit or signal.
package tmpfile

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"kubegonfig/internal/storage"
)

var (
	// tracked holds paths of temp files created in this process.
	tracked   []string
	trackedMu sync.Mutex
)

// Create writes decrypted data to a temp file in the secure runtime dir.
// Returns the absolute path to the created file.
// The file is registered for cleanup on process exit/signal.
func Create(name string, data []byte) (string, error) {
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

// Remove deletes a specific temp file.
func Remove(path string) error {
	return storage.RemoveFile(path)
}

// CleanupAll removes all temp files created by this process.
func CleanupAll() {
	trackedMu.Lock()
	paths := make([]string, len(tracked))
	copy(paths, tracked)
	tracked = nil
	trackedMu.Unlock()

	for _, p := range paths {
		_ = os.Remove(p)
	}
}

// CleanupStale removes all files from the runtime directory.
// Used by the explicit "cleanup" command.
func CleanupStale() (int, error) {
	dir, err := storage.RuntimeDir()
	if err != nil {
		return 0, fmt.Errorf("resolve runtime dir: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read runtime dir: %w", err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := os.Remove(path); err != nil {
			return count, fmt.Errorf("remove %s: %w", path, err)
		}
		count++
	}
	return count, nil
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
