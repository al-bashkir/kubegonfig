// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Pavel Aksenov <41126916+al-bashkir@users.noreply.github.com>

package bundle

import (
	"archive/tar"
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kubegonfig/internal/config"
	"kubegonfig/internal/profile"

	"gopkg.in/yaml.v3"
)

// buildEncryptedArchive writes an archive with a manifest, the given
// encrypted profiles, and an optional config.yaml body (nil = no config).
func buildEncryptedArchive(t *testing.T, profiles map[string][]byte, configBody []byte) []byte {
	t.Helper()
	names := keysOf(profiles)
	m := Manifest{
		SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC().Truncate(time.Second),
		Encrypted: true, ProfileCount: len(names), ConfigIncluded: configBody != nil,
	}
	for _, n := range names {
		m.Profiles = append(m.Profiles, ManifestProfile{Name: n, File: ProfilesDir + n + EncryptedExt})
	}
	mb, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := writeTarEntry(tw, ManifestName, mb, m.CreatedAt); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := writeTarEntry(tw, ProfilesDir+n+EncryptedExt, profiles[n], m.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	if configBody != nil {
		if err := writeTarEntry(tw, ConfigName, configBody, m.CreatedAt); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newConfigOnDisk(t *testing.T, recipients ...string) *config.Config {
	t.Helper()
	cfg := &config.Config{DataDir: t.TempDir(), GPGRecipient: "dst@example.com", GPGRecipients: recipients}
	cfg.Path = filepath.Join(t.TempDir(), "config.yaml")
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return cfg
}

func TestRestore_SingleCollisionFails(t *testing.T) {
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("new"), "beta": []byte("b")}, nil)
	dst := newSeededManager(t, map[string][]byte{"alpha": []byte("old")})

	_, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: bytes.NewReader(archive)})
	if err == nil || !strings.Contains(err.Error(), "alpha") {
		t.Fatalf("Restore() error = %v, want collision error naming alpha", err)
	}
	if got, _ := dst.ReadEncrypted("alpha"); string(got) != "old" {
		t.Errorf("alpha = %q, want unchanged %q", got, "old")
	}
	if ok, _ := dst.Exists("beta"); ok {
		t.Error("beta written despite collision error")
	}
}

func TestRestore_ConfigActionRequiresFlagAndArchiveConfig(t *testing.T) {
	withCfg := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, []byte("gpg_recipients: [extra@example.com]\n"))
	noCfg := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")}, nil)
	tests := []struct {
		name    string
		archive []byte
		merge   bool
		action  string
		want    []string
	}{
		{"config but no flag", withCfg, false, "ignore", nil},
		{"flag but no config", noCfg, true, "ignore", nil},
		{"both", withCfg, true, "merge", []string{"extra@example.com"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfigOnDisk(t)
			mgr, _ := profile.NewManager(cfg)
			plan, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(tc.archive), MergeConfig: tc.merge})
			if err != nil {
				t.Fatalf("Restore() error: %v", err)
			}
			if plan.ConfigAction != tc.action {
				t.Errorf("ConfigAction = %q, want %q", plan.ConfigAction, tc.action)
			}
			if !reflect.DeepEqual(cfg.GPGRecipients, tc.want) {
				t.Errorf("GPGRecipients = %v, want %v", cfg.GPGRecipients, tc.want)
			}
		})
	}
}

func TestRestore_MergeSkipsBlankAndDuplicateRecipients(t *testing.T) {
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": []byte("a")},
		[]byte("gpg_recipients: ['', '  ', local@example.com, ' new@example.com ']\n"))
	cfg := newConfigOnDisk(t, "local@example.com")
	mgr, _ := profile.NewManager(cfg)

	if _, err := Restore(mgr, cfg, RestoreOptions{In: bytes.NewReader(archive), MergeConfig: true}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	want := []string{"local@example.com", "new@example.com"}
	if !reflect.DeepEqual(cfg.GPGRecipients, want) {
		t.Errorf("GPGRecipients = %v, want %v", cfg.GPGRecipients, want)
	}
}

func TestRestore_AcceptsProfileOfExactlyMaxSize(t *testing.T) {
	body := bytes.Repeat([]byte("x"), MaxProfileSize)
	archive := buildEncryptedArchive(t, map[string][]byte{"alpha": body}, nil)
	dst := newSeededManager(t, nil)
	if _, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: bytes.NewReader(archive)}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
	if got, _ := dst.ReadEncrypted("alpha"); len(got) != MaxProfileSize {
		t.Errorf("stored %d bytes, want %d", len(got), MaxProfileSize)
	}
}

func TestRestore_AcceptsManifestOfExactly64KiB(t *testing.T) {
	m := Manifest{SchemaVersion: SchemaVersion, CreatedAt: time.Now().UTC().Truncate(time.Second), Encrypted: true}
	mb, _ := yaml.Marshal(&m)
	pad := 64*1024 - len(mb)
	mb = append(mb, []byte("#"+strings.Repeat("p", pad-2)+"\n")...)
	if len(mb) != 64*1024 {
		t.Fatalf("manifest len = %d", len(mb))
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := writeTarEntry(tw, ManifestName, mb, m.CreatedAt); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	dst := newSeededManager(t, nil)
	if _, err := Restore(dst, &config.Config{GPGRecipient: "t@example.com"}, RestoreOptions{In: &buf}); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}
}
