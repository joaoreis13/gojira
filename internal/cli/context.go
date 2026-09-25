package cli

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/joaoreis13/gojira/internal/auth"
	"github.com/joaoreis13/gojira/internal/client"
	"github.com/joaoreis13/gojira/internal/config"
)

// newTokenSource is a seam over auth.TokenSource so tests can inject a fake
// token source without touching the OS keyring or the network.
var newTokenSource = auth.TokenSource

// refreshToken forces a token refresh for alias/profile and returns the
// refreshed token. Used both by `gojira auth refresh` and by `gojira profile
// use`, which refreshes the profile it's activating.
func refreshToken(ctx context.Context, alias string, profile config.Profile) (*oauth2.Token, error) {
	ts, err := newTokenSource(ctx, alias, profile)
	if err != nil {
		return nil, err
	}
	return ts.Token()
}

// profileClient loads config, resolves the named (or active) profile,
// ensures a valid OAuth token source, resolves+caches the Jira Cloud id, and
// returns a ready-to-use API client along with the resolved profile alias.
func profileClient(ctx context.Context, aliasFlag string) (*client.Client, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}
	alias, profile, err := cfg.Resolve(aliasFlag)
	if err != nil {
		return nil, "", err
	}

	ts, err := newTokenSource(ctx, alias, profile)
	if err != nil {
		return nil, "", err
	}

	cloudID, changed, err := auth.EnsureCloudID(ctx, ts, profile)
	if err != nil {
		return nil, "", err
	}
	if changed {
		profile.CloudID = cloudID
		cfg.Profiles[alias] = profile
		if err := config.Save(cfg); err != nil {
			return nil, "", fmt.Errorf("persist resolved cloud id: %w", err)
		}
	}

	hc := oauth2.NewClient(ctx, ts)
	return client.New(hc, cloudID), alias, nil
}
