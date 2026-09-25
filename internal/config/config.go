// Package config manages gojira's on-disk configuration: named Jira
// profiles (OAuth client info, base URL, cached cloud id) and which one is
// active. Tokens are never stored here — see internal/auth for credential
// storage.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultScopes covers the common Jira Platform + Software REST API surface.
// offline_access is required to receive a refresh token from Atlassian.
var DefaultScopes = []string{
	"read:jira-work",
	"write:jira-work",
	"read:jira-user",
	"manage:jira-project",
	"manage:jira-configuration",
	"manage:jira-webhook",
	"offline_access",
}

// DefaultRedirectPort is used for the local OAuth callback listener unless
// a profile overrides it. It must match the callback URL registered on the
// Atlassian OAuth app.
const DefaultRedirectPort = 51837

type Profile struct {
	BaseURL      string   `yaml:"base_url"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Scopes       []string `yaml:"scopes"`
	RedirectPort int      `yaml:"redirect_port"`
	CloudID      string   `yaml:"cloud_id,omitempty"`
}

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

// legacyConfig mirrors the pre-profiles schema (sites/default_site), read
// only to migrate old config files. Never written.
type legacyConfig struct {
	DefaultSite string             `yaml:"default_site"`
	Sites       map[string]Profile `yaml:"sites"`
}

// Dir returns the OS-appropriate config directory for gojira
// (~/.config/gojira on Linux, ~/Library/Application Support/gojira on
// macOS, %AppData%\gojira on Windows).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(base, "gojira"), nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// parseConfig unmarshals config YAML. If the new profiles/active_profile
// keys are absent, it falls back to reading the legacy sites/default_site
// keys and migrates them in memory; migrated reports whether that happened,
// so the caller can persist the migration once.
func parseConfig(data []byte) (cfg *Config, migrated bool, err error) {
	cfg = &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, false, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if len(cfg.Profiles) == 0 && cfg.ActiveProfile == "" {
		var legacy legacyConfig
		if err := yaml.Unmarshal(data, &legacy); err == nil && len(legacy.Sites) > 0 {
			cfg.Profiles = legacy.Sites
			cfg.ActiveProfile = legacy.DefaultSite
			return cfg, true, nil
		}
	}
	return cfg, false, nil
}

// Load reads the config file, returning an empty Config if it doesn't exist
// yet. A file still using the old sites/default_site schema is migrated to
// profiles/active_profile in place, once, with a note on stderr.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	cfg, migrated, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}
	if migrated {
		if err := Save(cfg); err != nil {
			return nil, fmt.Errorf("migrate legacy config (sites -> profiles): %w", err)
		}
		fmt.Fprintln(os.Stderr, "gojira: migrated config.yaml from the old 'sites'/'default_site' schema to 'profiles'/'active_profile'")
	}
	return cfg, nil
}

// Save writes the config file, creating its directory (mode 0700) as needed.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	p, err := path()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", p, err)
	}
	return nil
}

// Resolve returns the named profile, or the active profile when name is
// empty.
func (c *Config) Resolve(name string) (string, Profile, error) {
	if name == "" {
		name = c.ActiveProfile
	}
	if name == "" {
		return "", Profile{}, fmt.Errorf("no profile specified and no active profile set; run `gojira profile add` then `gojira profile use`")
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return "", Profile{}, fmt.Errorf("unknown profile %q; run `gojira profile list` to see configured profiles", name)
	}
	return name, profile, nil
}
