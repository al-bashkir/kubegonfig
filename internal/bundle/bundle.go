// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

// Package bundle implements the kubegonfig export/restore archive format.
//
// An archive is an uncompressed ustar tar containing:
//   - kubegonfig-manifest.yaml (always first)
//   - profiles/<name>.yaml or profiles/<name>.yaml.gpg
//   - config.yaml (optional)
package bundle

const (
	// SchemaVersion is the archive manifest version this package emits and
	// the maximum it accepts on Restore.
	SchemaVersion = 1

	// ManifestName is the path of the manifest entry inside the tar.
	ManifestName = "kubegonfig-manifest.yaml"

	// ProfilesDir is the tar prefix for profile entries.
	ProfilesDir = "profiles/"

	// ConfigName is the optional tar entry holding a copy of config.yaml.
	ConfigName = "config.yaml"

	// EncryptedExt is the extension used for ciphertext profile entries.
	EncryptedExt = ".yaml.gpg"

	// PlaintextExt is the extension used for plaintext profile entries.
	PlaintextExt = ".yaml"

	// MaxProfileSize bounds the payload size of any single profile entry.
	MaxProfileSize = 1 << 20 // 1 MiB
)
