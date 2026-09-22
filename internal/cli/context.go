package cli

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/joaoreis13/gojira/internal/auth"
	"github.com/joaoreis13/gojira/internal/client"
	"github.com/joaoreis13/gojira/internal/config"
)

// siteClient loads config, resolves the named (or default) site, ensures a
// valid OAuth token source, resolves+caches the Jira Cloud id, and returns a
// ready-to-use API client along with the resolved site alias.
func siteClient(ctx context.Context, aliasFlag string) (*client.Client, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, "", err
	}
	alias, site, err := cfg.Resolve(aliasFlag)
	if err != nil {
		return nil, "", err
	}

	ts, err := auth.TokenSource(ctx, alias, site)
	if err != nil {
		return nil, "", err
	}

	cloudID, changed, err := auth.EnsureCloudID(ctx, ts, site)
	if err != nil {
		return nil, "", err
	}
	if changed {
		site.CloudID = cloudID
		cfg.Sites[alias] = site
		if err := config.Save(cfg); err != nil {
			return nil, "", fmt.Errorf("persist resolved cloud id: %w", err)
		}
	}

	hc := oauth2.NewClient(ctx, ts)
	return client.New(hc, cloudID), alias, nil
}
