// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"reflect"
	"testing"
	"time"
)

func TestManifest_RoundTrip(t *testing.T) {
	want := Manifest{
		SchemaVersion:     1,
		CreatedAt:         time.Date(2026, 5, 5, 14, 23, 1, 0, time.UTC),
		KubegonfigVersion: "0.2.0",
		Encrypted:         true,
		ProfileCount:      2,
		Profiles: []ManifestProfile{
			{Name: "production", File: "profiles/production.yaml.gpg"},
			{Name: "staging", File: "profiles/staging.yaml.gpg"},
		},
		ConfigIncluded: true,
	}

	encoded, err := MarshalManifest(&want)
	if err != nil {
		t.Fatalf("MarshalManifest() error: %v", err)
	}

	got, err := ParseManifest(encoded)
	if err != nil {
		t.Fatalf("ParseManifest() error: %v", err)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", *got, want)
	}
}

func TestParseManifest_RejectsFutureSchemaVersion(t *testing.T) {
	src := []byte("schema_version: 999\ncreated_at: 2026-05-05T14:23:01Z\nkubegonfig_version: 0.2.0\nencrypted: true\nprofile_count: 0\nprofiles: []\nconfig_included: false\n")
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want unsupported schema error")
	}
}

func TestParseManifest_RejectsZeroSchemaVersion(t *testing.T) {
	src := []byte("schema_version: 0\ncreated_at: 2026-05-05T14:23:01Z\nkubegonfig_version: 0.2.0\nencrypted: true\nprofile_count: 0\nprofiles: []\nconfig_included: false\n")
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want invalid schema version error")
	}
}

func TestParseManifest_RejectsCountMismatch(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 5
profiles:
  - name: a
    file: profiles/a.yaml.gpg
config_included: false
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want count mismatch error")
	}
}

func TestParseManifest_RejectsInvalidProfileName(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 1
profiles:
  - name: "../escape"
    file: profiles/escape.yaml.gpg
config_included: false
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want invalid profile name error")
	}
}

func TestParseManifest_RejectsExtensionMismatch(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 1
profiles:
  - name: a
    file: profiles/a.yaml
config_included: false
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want extension mismatch error")
	}
}

func TestParseManifest_RejectsUnknownField(t *testing.T) {
	src := []byte(`schema_version: 1
created_at: 2026-05-05T14:23:01Z
kubegonfig_version: 0.2.0
encrypted: true
profile_count: 0
profiles: []
config_included: false
extra_field: oops
`)
	_, err := ParseManifest(src)
	if err == nil {
		t.Fatal("ParseManifest() error = nil, want unknown field error")
	}
}
