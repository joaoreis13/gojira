package config

import (
	"testing"
)

func TestResolveActiveProfile(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "work",
		Profiles: map[string]Profile{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}

	alias, profile, err := cfg.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if alias != "work" {
		t.Errorf("alias = %q, want %q", alias, "work")
	}
	if profile.BaseURL != "https://work.atlassian.net" {
		t.Errorf("profile.BaseURL = %q, want %q", profile.BaseURL, "https://work.atlassian.net")
	}
}

func TestResolveUnknownProfile(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if _, _, err := cfg.Resolve("missing"); err == nil {
		t.Error("expected error for unknown profile, got nil")
	}
}

func TestResolveNoActiveConfigured(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{}}
	if _, _, err := cfg.Resolve(""); err == nil {
		t.Error("expected error when no profile given and no active profile configured, got nil")
	}
}

func TestParseConfigCurrentSchema(t *testing.T) {
	data := []byte(`
active_profile: work
profiles:
  work:
    base_url: https://work.atlassian.net
    client_id: abc
`)
	cfg, migrated, err := parseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Error("expected migrated = false for current-schema config")
	}
	if cfg.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "work")
	}
	if got := cfg.Profiles["work"].BaseURL; got != "https://work.atlassian.net" {
		t.Errorf("Profiles[work].BaseURL = %q", got)
	}
}

func TestParseConfigMigratesLegacySchema(t *testing.T) {
	data := []byte(`
default_site: work
sites:
  work:
    base_url: https://work.atlassian.net
    client_id: abc
    client_secret: shh
    cloud_id: cloud-123
`)
	cfg, migrated, err := parseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Fatal("expected migrated = true for legacy-schema config")
	}
	if cfg.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "work")
	}
	p, ok := cfg.Profiles["work"]
	if !ok {
		t.Fatal("expected profile \"work\" to be migrated from legacy sites")
	}
	if p.BaseURL != "https://work.atlassian.net" || p.ClientID != "abc" || p.ClientSecret != "shh" || p.CloudID != "cloud-123" {
		t.Errorf("migrated profile = %+v", p)
	}
}

func TestParseConfigEmptyFileNoMigration(t *testing.T) {
	cfg, migrated, err := parseConfig([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Error("expected migrated = false for an empty file")
	}
	if cfg.Profiles == nil || len(cfg.Profiles) != 0 {
		t.Errorf("Profiles = %+v, want empty non-nil map", cfg.Profiles)
	}
}
