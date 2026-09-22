// Package config manages gojira's on-disk configuration: named Jira sites
// (OAuth client info, base URL, cached cloud id) and which one is default.
// Tokens are never stored here — see internal/auth for credential storage.
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
// a site overrides it. It must match the callback URL registered on the
// Atlassian OAuth app.
const DefaultRedirectPort = 51837

type Site struct {
	BaseURL      string   `yaml:"base_url"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Scopes       []string `yaml:"scopes"`
	RedirectPort int      `yaml:"redirect_port"`
	CloudID      string   `yaml:"cloud_id,omitempty"`
}

type Config struct {
	DefaultSite string          `yaml:"default_site"`
	Sites       map[string]Site `yaml:"sites"`
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

// Load reads the config file, returning an empty Config if it doesn't exist yet.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{Sites: map[string]Site{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}
	if cfg.Sites == nil {
		cfg.Sites = map[string]Site{}
	}
	return &cfg, nil
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

// Resolve returns the named site, or the default site when name is empty.
func (c *Config) Resolve(name string) (string, Site, error) {
	if name == "" {
		name = c.DefaultSite
	}
	if name == "" {
		return "", Site{}, fmt.Errorf("no site specified and no default site configured; run `gojira site add` first")
	}
	site, ok := c.Sites[name]
	if !ok {
		return "", Site{}, fmt.Errorf("unknown site %q; run `gojira site list` to see configured sites", name)
	}
	return name, site, nil
}
