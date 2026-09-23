// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package profile manages kubeconfig profiles: encrypted storage, CRUD
// operations, and activation state.
package profile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/crypto"
	"kubegonfig/internal/kubeconfig"
	"kubegonfig/internal/shell"
	"kubegonfig/internal/storage"
)

const (
	profilesDir = "profiles"
	stateDir    = "state"
	currentFile = "current"
	lockFile    = "kubegonfig.lock"
	gpgExt      = ".yaml.gpg"
)

// Manager provides profile CRUD and state operations.
type Manager struct {
	cfg     *config.Config
	dataDir string
}

// NewManager creates a Manager from the given config.
func NewManager(cfg *config.Config) (*Manager, error) {
	dataDir, err := cfg.ResolveDataDir()
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	return &Manager{cfg: cfg, dataDir: dataDir}, nil
}

func (m *Manager) profilesPath() string {
	return filepath.Join(m.dataDir, profilesDir)
}

func (m *Manager) statePath() string {
	return filepath.Join(m.dataDir, stateDir)
}

func profileFileName(name string) string {
	return name + gpgExt
}

func (m *Manager) lockPath() string {
	return filepath.Join(m.dataDir, lockFile)
}

func (m *Manager) writeProfile(name string, data []byte) error {
	return storage.AtomicWriteInDir(m.profilesPath(), profileFileName(name), data, 0600)
}

func (m *Manager) readProfile(name string) ([]byte, error) {
	data, err := storage.ReadFileInDir(m.profilesPath(), profileFileName(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	return data, err
}

func (m *Manager) profileMtime(name string) (time.Time, error) {
	mtime, ok, err := storage.RegularFileMtimeInDir(m.profilesPath(), profileFileName(name))
	if err != nil {
		return time.Time{}, err
	}
	if !ok {
		return time.Time{}, fmt.Errorf("profile %q not found", name)
	}
	return mtime, nil
}

func (m *Manager) setCurrent(name string) error {
	return storage.AtomicWriteInDir(m.statePath(), currentFile, []byte(name+"\n"), 0600)
}

// WithLock executes fn while holding an exclusive file lock.
func (m *Manager) WithLock(fn func() error) error {
	return storage.WithLock(m.lockPath(), fn)
}

// Create validates, encrypts, and stores a new profile.
func (m *Manager) Create(name string, data []byte) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	if err := kubeconfig.Validate(data); err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}
	if err := m.cfg.Validate(); err != nil {
		return err
	}

	return m.WithLock(func() error {
		exists, err := m.profileExists(name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("profile %q already exists", name)
		}

		encrypted, err := crypto.Encrypt(data, m.cfg.Recipients())
		if err != nil {
			return fmt.Errorf("encrypt profile: %w", err)
		}

		return m.writeProfile(name, encrypted)
	})
}

// Import reads a kubeconfig from a file path, validates, encrypts, and stores it.
func (m *Manager) Import(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %s: %w", path, err)
	}
	return m.Create(name, data)
}

// List returns sorted profile names.
func (m *Manager) List() ([]string, error) {
	dir := m.profilesPath()
	entries, err := storage.ReadDirInDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), gpgExt) {
			continue
		}
		name := strings.TrimSuffix(e.Name(), gpgExt)
		if err := shell.ValidateName(name); err != nil {
			continue
		}
		// ponytail: trust readdir d_type; skips a stat per entry. Misses files
		// on DT_UNKNOWN filesystems (some FUSE) — re-add RegularFileInDir there.
		if !e.Type().IsRegular() {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Decrypt reads and decrypts a profile, returning the plaintext kubeconfig.
func (m *Manager) Decrypt(name string) ([]byte, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}
	return m.decryptProfile(name)
}

// ReadEncrypted returns the raw encrypted blob for a profile without invoking
// GPG. Used for pass-through export of the on-disk ciphertext.
func (m *Manager) ReadEncrypted(name string) ([]byte, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}
	return m.readProfile(name)
}

// WriteEncrypted atomically writes a raw encrypted blob for the named profile.
// Caller is responsible for the blob being valid GPG ciphertext for the local
// recipients; this method does NOT invoke GPG.
func (m *Manager) WriteEncrypted(name string, blob []byte) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	return m.writeProfile(name, blob)
}

func (m *Manager) decryptProfile(name string) ([]byte, error) {
	encrypted, err := m.readProfile(name)
	if err != nil {
		return nil, err
	}

	data, err := crypto.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt profile %q: %w", name, err)
	}
	return data, nil
}

// Edit decrypts a profile, passes it to edit while holding the profile lock,
// and stores the edited content if it changed.
func (m *Manager) Edit(name string, edit func([]byte) ([]byte, error)) (changed bool, err error) {
	if err := shell.ValidateName(name); err != nil {
		return false, err
	}

	err = m.WithLock(func() error {
		original, err := m.decryptProfile(name)
		if err != nil {
			return err
		}

		edited, err := edit(original)
		if err != nil {
			return err
		}
		if bytes.Equal(original, edited) {
			return nil
		}

		if err := kubeconfig.Validate(edited); err != nil {
			return fmt.Errorf("invalid kubeconfig: %w", err)
		}
		if err := m.cfg.Validate(); err != nil {
			return err
		}

		encrypted, err := crypto.Encrypt(edited, m.cfg.Recipients())
		if err != nil {
			return fmt.Errorf("encrypt profile: %w", err)
		}
		if err := m.writeProfile(name, encrypted); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

// Activate locks the manager, asks probe whether the existing activation file
// is fresh enough to reuse, decrypts the profile only on a cache miss, calls
// create when a fresh activation file must be materialized, and records the
// profile as current.
//
// probe receives the modification time of the encrypted profile and reports
// whether a usable plaintext already exists for name. On a cache miss it
// returns the path where create should write; on a hit it returns the path of
// the existing file.
//
// create is invoked only on a cache miss and receives the decrypted
// kubeconfig bytes.
//
// cleanup is invoked when setCurrent fails after a cache miss to remove the
// just-created activation file. It is NOT invoked on a cache-hit failure: the
// pre-existing file may still be in use by another shell that earlier
// evaluated `kubegonfig use`.
func (m *Manager) Activate(
	name string,
	probe func(name string, cipherMtime time.Time) (string, bool, error),
	create func(data []byte) (string, error),
	cleanup func(string) error,
) (path string, err error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}

	err = m.WithLock(func() error {
		cipherMtime, err := m.profileMtime(name)
		if err != nil {
			return err
		}

		cachedPath, hit, err := probe(name, cipherMtime)
		if err != nil {
			return err
		}

		if hit {
			path = cachedPath
		} else {
			data, err := m.decryptProfile(name)
			if err != nil {
				return err
			}
			created, err := create(data)
			if err != nil {
				return err
			}
			path = created
		}

		if err := m.setCurrent(name); err != nil {
			if !hit && cleanup != nil {
				if cleanupErr := cleanup(path); cleanupErr != nil {
					return errors.Join(err, fmt.Errorf("cleanup activation file: %w", cleanupErr))
				}
			}
			return err
		}
		return nil
	})
	return path, err
}

// Unlock decrypts a profile and writes its activation file via create,
// while holding the profile lock. Unlike Activate, it does not record
// the profile as current.
func (m *Manager) Unlock(name string, create func([]byte) (string, error)) (path string, err error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}

	err = m.WithLock(func() error {
		data, err := m.decryptProfile(name)
		if err != nil {
			return err
		}
		path, err = create(data)
		return err
	})
	return path, err
}

// Delete removes a profile. If it was active, clears the current state.
func (m *Manager) Delete(name string) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}

	return m.WithLock(func() error {
		exists, err := m.profileExists(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q not found", name)
		}
		current, err := m.GetCurrent()
		if err != nil {
			return fmt.Errorf("read current profile: %w", err)
		}

		// Clear current first: a failure there leaves the profile intact.
		// ponytail: no rollback. If the profile delete then fails, the profile
		// stays but is no longer current; re-run `use` to reactivate it.
		if current == name {
			if err := storage.RemoveFileInDir(m.statePath(), currentFile); err != nil {
				return fmt.Errorf("clear current profile: %w", err)
			}
		}
		if err := storage.RemoveFileInDir(m.profilesPath(), profileFileName(name)); err != nil {
			return fmt.Errorf("delete profile: %w", err)
		}
		return nil
	})
}

// Rename renames a profile from oldName to newName.
func (m *Manager) Rename(oldName, newName string) error {
	if err := shell.ValidateName(oldName); err != nil {
		return fmt.Errorf("old name: %w", err)
	}
	if err := shell.ValidateName(newName); err != nil {
		return fmt.Errorf("new name: %w", err)
	}

	return m.WithLock(func() error {
		exists, err := m.profileExists(oldName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q not found", oldName)
		}
		exists, err = m.profileExists(newName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("profile %q already exists", newName)
		}
		current, err := m.GetCurrent()
		if err != nil {
			return fmt.Errorf("read current profile: %w", err)
		}

		// ponytail: no rollback. If updating current fails after the rename,
		// current keeps the old name until the next `use`.
		if err := storage.RenameFileInDir(m.profilesPath(), profileFileName(oldName), profileFileName(newName)); err != nil {
			return fmt.Errorf("rename: %w", err)
		}
		if current == oldName {
			if err := m.setCurrent(newName); err != nil {
				return fmt.Errorf("update current profile: %w", err)
			}
		}
		return nil
	})
}

func (m *Manager) profileExists(name string) (bool, error) {
	return storage.RegularFileInDir(m.profilesPath(), profileFileName(name))
}

// Exists reports whether a profile with the given name is stored locally.
func (m *Manager) Exists(name string) (bool, error) {
	if err := shell.ValidateName(name); err != nil {
		return false, err
	}
	return m.profileExists(name)
}

// GetCurrent returns the name of the currently active profile.
// Returns empty string and nil error if no profile is active.
func (m *Manager) GetCurrent() (string, error) {
	data, err := storage.ReadFileInDir(m.statePath(), currentFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", nil
	}
	if err := shell.ValidateName(name); err != nil {
		return "", fmt.Errorf("invalid current profile state: %w", err)
	}
	return name, nil
}
