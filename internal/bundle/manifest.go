// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"bytes"
	"fmt"
	"time"

	"kubegonfig/internal/shell"

	"gopkg.in/yaml.v3"
)

// Manifest is the YAML document at ManifestName inside the archive.
type Manifest struct {
	SchemaVersion     int               `yaml:"schema_version"`
	CreatedAt         time.Time         `yaml:"created_at"`
	KubegonfigVersion string            `yaml:"kubegonfig_version"`
	Encrypted         bool              `yaml:"encrypted"`
	ProfileCount      int               `yaml:"profile_count"`
	Profiles          []ManifestProfile `yaml:"profiles"`
	ConfigIncluded    bool              `yaml:"config_included"`
}

// ManifestProfile describes one profile entry inside the archive.
type ManifestProfile struct {
	Name string `yaml:"name"`
	File string `yaml:"file"`
}

// MarshalManifest serializes a manifest to YAML.
func MarshalManifest(m *Manifest) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("MarshalManifest: nil manifest")
	}
	return yaml.Marshal(m)
}

// ParseManifest decodes a manifest YAML body and validates its invariants.
// Unknown fields are rejected so older binaries refuse archives written with
// newer additive fields they cannot interpret correctly.
func ParseManifest(data []byte) (*Manifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks the manifest's internal invariants.
func (m *Manifest) Validate() error {
	if m.SchemaVersion < 1 {
		return fmt.Errorf("manifest schema_version %d is invalid", m.SchemaVersion)
	}
	if m.SchemaVersion > SchemaVersion {
		return fmt.Errorf("manifest schema_version %d is newer than supported (%d); upgrade kubegonfig", m.SchemaVersion, SchemaVersion)
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("manifest created_at must be set")
	}
	if m.ProfileCount != len(m.Profiles) {
		return fmt.Errorf("manifest profile_count %d does not match profiles list length %d", m.ProfileCount, len(m.Profiles))
	}
	wantExt := PlaintextExt
	if m.Encrypted {
		wantExt = EncryptedExt
	}
	seen := make(map[string]struct{}, len(m.Profiles))
	for i, p := range m.Profiles {
		if err := shell.ValidateName(p.Name); err != nil {
			return fmt.Errorf("manifest profiles[%d]: %w", i, err)
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("manifest profiles[%d]: duplicate name %q", i, p.Name)
		}
		seen[p.Name] = struct{}{}
		want := ProfilesDir + p.Name + wantExt
		if p.File != want {
			return fmt.Errorf("manifest profiles[%d]: file %q does not match expected %q", i, p.File, want)
		}
	}
	return nil
}
