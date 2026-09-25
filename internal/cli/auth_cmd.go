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
	Short: "Log in, check status, refresh, or log out of a Jira profile",
}

var authProfileFlag string
var authNoBrowserFlag bool

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authorize gojira for a profile via your browser (OAuth 2.0 3LO, authorization code)",
	Long: `Authorize gojira for a profile via your browser.

By default this tries to open the authorization URL in your OS's default
browser. Pass --no-browser to skip that and just print the URL instead,
so you can paste it into whichever browser you actually want to authorize
with (useful if your default browser isn't signed into the right
Atlassian account, or you run multiple browser profiles).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, profile, err := cfg.Resolve(authProfileFlag)
		if err != nil {
			return err
		}
		cloudID, err := auth.Login(cmd.Context(), alias, profile, !authNoBrowserFlag)
		if err != nil {
			return err
		}
		profile.CloudID = cloudID
		cfg.Profiles[alias] = profile
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Logged in to %q (cloud id %s).\n", alias, cloudID)
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show token status for a profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, _, err := cfg.Resolve(authProfileFlag)
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
		fmt.Printf("profile: %s\naccess token: %s\nexpires: %s\nrefresh token present: %v\n",
			alias, state, st.ExpiresAt.Format(time.RFC3339), st.HasRefresh)
		return nil
	},
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Force a token refresh for a profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, profile, err := cfg.Resolve(authProfileFlag)
		if err != nil {
			return err
		}
		tok, err := refreshToken(cmd.Context(), alias, profile)
		if err != nil {
			return err
		}
		fmt.Printf("token for %q now valid until %s\n", alias, tok.Expiry.Format(time.RFC3339))
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete stored credentials for a profile",
	Long:  "Deletes locally stored credentials. It does not revoke the grant on Atlassian's side — do that at https://id.atlassian.com/manage-profile/apps if needed.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		alias, _, err := cfg.Resolve(authProfileFlag)
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
		c.Flags().StringVar(&authProfileFlag, "profile", "", "profile alias (defaults to the active profile)")
	}
	authLoginCmd.Flags().BoolVar(&authNoBrowserFlag, "no-browser", false, "print the authorization URL instead of opening it automatically")
	authCmd.AddCommand(authLoginCmd, authStatusCmd, authRefreshCmd, authLogoutCmd)
}
