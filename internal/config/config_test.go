package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NonexistentReturnsDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.GPGRecipient != "" {
		t.Errorf("default GPGRecipient should be empty, got %q", cfg.GPGRecipient)
	}
}

func TestLoad_ExistingConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "kubegonfig")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	data := []byte("gpg_recipient: test@example.com\nshell_style: fish\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.GPGRecipient != "test@example.com" {
		t.Errorf("GPGRecipient = %q, want %q", cfg.GPGRecipient, "test@example.com")
	}
	if cfg.ShellStyle != "fish" {
		t.Errorf("ShellStyle = %q, want %q", cfg.ShellStyle, "fish")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "kubegonfig")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("not: [valid: yaml"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Error("Load(invalid YAML) should return error")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfg := &Config{
		GPGRecipient: "user@example.com",
		ShellStyle:   "posix",
		path:         filepath.Join(tmp, "kubegonfig", "config.yaml"),
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save error: %v", err)
	}
	if loaded.GPGRecipient != cfg.GPGRecipient {
		t.Errorf("round-trip GPGRecipient = %q, want %q", loaded.GPGRecipient, cfg.GPGRecipient)
	}
	if loaded.ShellStyle != cfg.ShellStyle {
		t.Errorf("round-trip ShellStyle = %q, want %q", loaded.ShellStyle, cfg.ShellStyle)
	}
}

func TestRecipients(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		want    int
		wantAll []string
	}{
		{
			name: "primary only",
			cfg:  Config{GPGRecipient: "a@b.com"},
			want: 1,
		},
		{
			name: "primary + extras",
			cfg:  Config{GPGRecipient: "a@b.com", GPGRecipients: []string{"c@d.com"}},
			want: 2,
		},
		{
			name: "dedup primary from extras",
			cfg:  Config{GPGRecipient: "a@b.com", GPGRecipients: []string{"a@b.com", "c@d.com"}},
			want: 2,
		},
		{
			name: "no recipient",
			cfg:  Config{},
			want: 0,
		},
		{
			name:    "extras only (no primary)",
			cfg:     Config{GPGRecipients: []string{"a@b.com"}},
			want:    1,
			wantAll: []string{"a@b.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Recipients()
			if len(got) != tt.want {
				t.Errorf("Recipients() len = %d, want %d (got: %v)", len(got), tt.want, got)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	// No recipients — should fail.
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("Validate() should fail with no recipients")
	}

	// With recipient — should pass.
	cfg.GPGRecipient = "user@example.com"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should pass, got error: %v", err)
	}
}

func TestResolveDataDir_Override(t *testing.T) {
	cfg := &Config{DataDir: "/custom/data"}
	got, err := cfg.ResolveDataDir()
	if err != nil {
		t.Fatalf("ResolveDataDir() error: %v", err)
	}
	if got != "/custom/data" {
		t.Errorf("ResolveDataDir() = %q, want %q", got, "/custom/data")
	}
}

func TestResolveDataDir_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/xdg/data")
	cfg := &Config{}
	got, err := cfg.ResolveDataDir()
	if err != nil {
		t.Fatalf("ResolveDataDir() error: %v", err)
	}
	want := "/xdg/data/kubegonfig"
	if got != want {
		t.Errorf("ResolveDataDir() = %q, want %q", got, want)
	}
}

func TestPath(t *testing.T) {
	cfg := &Config{path: "/some/path/config.yaml"}
	if cfg.Path() != "/some/path/config.yaml" {
		t.Errorf("Path() = %q, want %q", cfg.Path(), "/some/path/config.yaml")
	}
}
