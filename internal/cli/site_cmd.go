package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoreis13/gojira/internal/auth"
	"github.com/joaoreis13/gojira/internal/config"
)

var siteCmd = &cobra.Command{
	Use:   "site",
	Short: "Manage configured Jira sites (OAuth app info per Jira instance)",
}

var (
	siteBaseURL      string
	siteClientID     string
	siteClientSecret string
	siteScopesFlag   string
	siteRedirectPort int
	siteSetDefault   bool
)

var siteAddCmd = &cobra.Command{
	Use:   "add <alias>",
	Short: "Register a Jira site and the OAuth app used to authorize it",
	Long: `Register a Jira site under a local alias. You must first create an
"OAuth 2.0 (3LO)" app at https://developer.atlassian.com/console/myapps/,
add the Jira Cloud REST API permissions/scopes you need, and add
http://localhost:<redirect-port>/callback (51837 by default) as its
callback URL. See README.md for the full walkthrough.

Atlassian's OAuth 2.0 (3LO) apps do not support PKCE/public clients, so
--client-secret is required, not optional.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		if siteBaseURL == "" || siteClientID == "" || siteClientSecret == "" {
			return fmt.Errorf("--base-url, --client-id, and --client-secret are all required")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		site := config.Site{
			BaseURL:      siteBaseURL,
			ClientID:     siteClientID,
			ClientSecret: siteClientSecret,
			RedirectPort: siteRedirectPort,
		}
		if siteScopesFlag != "" {
			site.Scopes = strings.Split(siteScopesFlag, ",")
		}
		cfg.Sites[alias] = site
		if siteSetDefault || cfg.DefaultSite == "" {
			cfg.DefaultSite = alias
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Site %q saved. Run `gojira auth login --site %s` to authorize it.\n", alias, alias)
		return nil
	},
}

var siteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured sites",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Sites) == 0 {
			fmt.Println("No sites configured. Run `gojira site add <alias> --base-url ... --client-id ...`.")
			return nil
		}
		for alias, site := range cfg.Sites {
			marker := ""
			if alias == cfg.DefaultSite {
				marker = " (default)"
			}
			cloud := "not resolved yet"
			if site.CloudID != "" {
				cloud = site.CloudID
			}
			fmt.Printf("%s%s\t%s\tcloud_id=%s\n", alias, marker, site.BaseURL, cloud)
		}
		return nil
	},
}

var siteUseCmd = &cobra.Command{
	Use:   "use <alias>",
	Short: "Set the default site",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := cfg.Sites[alias]; !ok {
			return fmt.Errorf("unknown site %q; run `gojira site list`", alias)
		}
		cfg.DefaultSite = alias
		return config.Save(cfg)
	},
}

var siteRemoveCmd = &cobra.Command{
	Use:   "remove <alias>",
	Short: "Remove a configured site and its stored credentials",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if _, ok := cfg.Sites[alias]; !ok {
			return fmt.Errorf("unknown site %q", alias)
		}
		delete(cfg.Sites, alias)
		if cfg.DefaultSite == alias {
			cfg.DefaultSite = ""
		}
		if err := auth.DeleteToken(alias); err != nil {
			return err
		}
		return config.Save(cfg)
	},
}

func init() {
	siteAddCmd.Flags().StringVar(&siteBaseURL, "base-url", "", "Jira Cloud site URL, e.g. https://yourteam.atlassian.net (required)")
	siteAddCmd.Flags().StringVar(&siteClientID, "client-id", "", "OAuth app client id (required)")
	siteAddCmd.Flags().StringVar(&siteClientSecret, "client-secret", "", "OAuth app client secret (required)")
	siteAddCmd.Flags().StringVar(&siteScopesFlag, "scopes", "", "comma-separated OAuth scopes (defaults to a broad read/write/admin set)")
	siteAddCmd.Flags().IntVar(&siteRedirectPort, "redirect-port", config.DefaultRedirectPort, "local port for the OAuth callback listener")
	siteAddCmd.Flags().BoolVar(&siteSetDefault, "default", false, "make this the default site")

	siteCmd.AddCommand(siteAddCmd, siteListCmd, siteUseCmd, siteRemoveCmd)
}
