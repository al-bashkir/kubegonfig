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
	"kubegonfig/internal/tmpfile"
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
		exists, err := m.Exists(name)
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

		return m.WriteEncrypted(name, encrypted)
	})
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
		// on DT_UNKNOWN filesystems (some FUSE) — re-add a RegularFileMtimeInDir check there.
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
	encrypted, err := m.ReadEncrypted(name)
	if err != nil {
		return nil, err
	}
	data, err := crypto.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt profile %q: %w", name, err)
	}
	return data, nil
}

// ReadEncrypted returns the raw encrypted blob for a profile without invoking
// GPG. Used for pass-through export of the on-disk ciphertext.
func (m *Manager) ReadEncrypted(name string) ([]byte, error) {
	if err := shell.ValidateName(name); err != nil {
		return nil, err
	}
	data, err := storage.ReadFileInDir(m.profilesPath(), profileFileName(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("profile %q not found", name)
	}
	return data, err
}

// WriteEncrypted atomically writes a raw encrypted blob for the named profile.
// Caller is responsible for the blob being valid GPG ciphertext for the local
// recipients; this method does NOT invoke GPG.
func (m *Manager) WriteEncrypted(name string, blob []byte) error {
	if err := shell.ValidateName(name); err != nil {
		return err
	}
	return storage.AtomicWriteInDir(m.profilesPath(), profileFileName(name), blob, 0600)
}

// Edit decrypts a profile, passes it to edit while holding the profile lock,
// and stores the edited content if it changed.
func (m *Manager) Edit(name string, edit func([]byte) ([]byte, error)) (changed bool, err error) {
	if err := shell.ValidateName(name); err != nil {
		return false, err
	}

	err = m.WithLock(func() error {
		original, err := m.Decrypt(name)
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
		if err := m.WriteEncrypted(name, encrypted); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

// Activate writes the profile's runtime activation file, reusing it when it
// is newer than the ciphertext, and records the profile as current.
func (m *Manager) Activate(name string) (path string, err error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}

	err = m.WithLock(func() error {
		cipherMtime, err := m.profileMtime(name)
		if err != nil {
			return err
		}

		var hit bool
		path, hit, err = tmpfile.ProbeCached(name, cipherMtime)
		if err != nil {
			return err
		}
		if !hit {
			data, err := m.Decrypt(name)
			if err != nil {
				return err
			}
			if path, err = tmpfile.Create(name, data); err != nil {
				return err
			}
		}

		if err := m.setCurrent(name); err != nil {
			// A reused file may be in use by another shell; only remove one
			// created here.
			if !hit {
				if cleanupErr := tmpfile.Remove(name); cleanupErr != nil {
					return errors.Join(err, fmt.Errorf("cleanup activation file: %w", cleanupErr))
				}
			}
			return err
		}
		return nil
	})
	return path, err
}

// Unlock decrypts a profile to its runtime activation file while holding the
// profile lock. Unlike Activate, it does not record the profile as current.
func (m *Manager) Unlock(name string) (path string, err error) {
	if err := shell.ValidateName(name); err != nil {
		return "", err
	}

	err = m.WithLock(func() error {
		data, err := m.Decrypt(name)
		if err != nil {
			return err
		}
		path, err = tmpfile.Create(name, data)
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
		exists, err := m.Exists(name)
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
		exists, err := m.Exists(oldName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q not found", oldName)
		}
		exists, err = m.Exists(newName)
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

// Exists reports whether a profile with the given name is stored locally.
func (m *Manager) Exists(name string) (bool, error) {
	if err := shell.ValidateName(name); err != nil {
		return false, err
	}
	_, ok, err := storage.RegularFileMtimeInDir(m.profilesPath(), profileFileName(name))
	return ok, err
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
