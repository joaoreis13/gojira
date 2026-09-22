package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"

	"github.com/joaoreis13/gojira/internal/config"
)

const keyringService = "gojira"

// storedToken mirrors oauth2.Token for JSON persistence.
type storedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
}

func toStored(t *oauth2.Token) storedToken {
	return storedToken{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		TokenType:    t.TokenType,
		Expiry:       t.Expiry,
	}
}

func (s storedToken) toOAuth2() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		TokenType:    s.TokenType,
		Expiry:       s.Expiry,
	}
}

// fallbackPath returns the file used when the OS keyring is unavailable
// (e.g. a headless Linux box with no Secret Service / D-Bus session).
func fallbackPath(site string) (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials", site+".json"), nil
}

// SaveToken persists a token for the given site, preferring the OS keyring
// and falling back to a 0600 file under the config directory. It returns
// whether the fallback path was used, so callers can warn the user once.
func SaveToken(site string, tok *oauth2.Token) (usedFallback bool, err error) {
	data, err := json.Marshal(toStored(tok))
	if err != nil {
		return false, fmt.Errorf("encode token: %w", err)
	}

	if kerr := keyring.Set(keyringService, site, string(data)); kerr == nil {
		return false, nil
	}

	p, ferr := fallbackPath(site)
	if ferr != nil {
		return false, ferr
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return false, fmt.Errorf("create credentials dir: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return false, fmt.Errorf("write fallback credentials file: %w", err)
	}
	return true, nil
}

// LoadToken retrieves the stored token for a site, checking the OS keyring
// first and then the fallback file.
func LoadToken(site string) (*oauth2.Token, error) {
	if data, err := keyring.Get(keyringService, site); err == nil {
		var st storedToken
		if err := json.Unmarshal([]byte(data), &st); err != nil {
			return nil, fmt.Errorf("decode token from keyring: %w", err)
		}
		return st.toOAuth2(), nil
	}

	p, err := fallbackPath(site)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("not logged in to site %q; run `gojira auth login --site %s`", site, site)
	}
	if err != nil {
		return nil, fmt.Errorf("read fallback credentials file: %w", err)
	}
	var st storedToken
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("decode fallback credentials file: %w", err)
	}
	return st.toOAuth2(), nil
}

// DeleteToken removes stored credentials for a site from both backends.
func DeleteToken(site string) error {
	_ = keyring.Delete(keyringService, site)
	p, err := fallbackPath(site)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove fallback credentials file: %w", err)
	}
	return nil
}
