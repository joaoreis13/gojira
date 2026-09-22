package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoreis13/gojira/internal/auth"
	"github.com/joaoreis13/gojira/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Log in, check status, refresh, or log out of a Jira site",
}

var authSiteFlag string

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authorize gojira for a site via your browser (OAuth 2.0 + PKCE)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, site, err := cfg.Resolve(authSiteFlag)
		if err != nil {
			return err
		}
		cloudID, err := auth.Login(cmd.Context(), alias, site)
		if err != nil {
			return err
		}
		site.CloudID = cloudID
		cfg.Sites[alias] = site
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Logged in to %q (cloud id %s).\n", alias, cloudID)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show token status for a site",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, _, err := cfg.Resolve(authSiteFlag)
		if err != nil {
			return err
		}
		st, err := auth.GetStatus(alias)
		if err != nil {
			return err
		}
		state := "valid"
		if time.Now().After(st.ExpiresAt) {
			state = "expired (will auto-refresh on next use)"
		}
		fmt.Printf("site: %s\naccess token: %s\nexpires: %s\nrefresh token present: %v\n",
			alias, state, st.ExpiresAt.Format(time.RFC3339), st.HasRefresh)
		return nil
	},
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Force a token refresh for a site",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, site, err := cfg.Resolve(authSiteFlag)
		if err != nil {
			return err
		}
		ts, err := auth.TokenSource(cmd.Context(), alias, site)
		if err != nil {
			return err
		}
		tok, err := ts.Token()
		if err != nil {
			return err
		}
		fmt.Printf("token for %q now valid until %s\n", alias, tok.Expiry.Format(time.RFC3339))
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete stored credentials for a site",
	Long:  "Deletes locally stored credentials. It does not revoke the grant on Atlassian's side — do that at https://id.atlassian.com/manage-profile/apps if needed.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, _, err := cfg.Resolve(authSiteFlag)
		if err != nil {
			return err
		}
		if err := auth.Logout(alias); err != nil {
			return err
		}
		fmt.Printf("Logged out of %q.\n", alias)
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{authLoginCmd, authStatusCmd, authRefreshCmd, authLogoutCmd} {
		c.Flags().StringVar(&authSiteFlag, "site", "", "site alias (defaults to the configured default site)")
	}
	authCmd.AddCommand(authLoginCmd, authStatusCmd, authRefreshCmd, authLogoutCmd)
}
