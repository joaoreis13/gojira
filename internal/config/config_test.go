package config

import (
	"testing"
)

func TestResolveDefaultSite(t *testing.T) {
	cfg := &Config{
		DefaultSite: "work",
		Sites: map[string]Site{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}

	alias, site, err := cfg.Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if alias != "work" {
		t.Errorf("alias = %q, want %q", alias, "work")
	}
	if site.BaseURL != "https://work.atlassian.net" {
		t.Errorf("site.BaseURL = %q, want %q", site.BaseURL, "https://work.atlassian.net")
	}
}

func TestResolveUnknownSite(t *testing.T) {
	cfg := &Config{Sites: map[string]Site{}}
	if _, _, err := cfg.Resolve("missing"); err == nil {
		t.Error("expected error for unknown site, got nil")
	}
}

func TestResolveNoDefaultConfigured(t *testing.T) {
	cfg := &Config{Sites: map[string]Site{}}
	if _, _, err := cfg.Resolve(""); err == nil {
		t.Error("expected error when no site given and no default configured, got nil")
	}
}
