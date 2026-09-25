package cli

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoreis13/gojira/internal/auth"
	"github.com/joaoreis13/gojira/internal/config"
)

// validAliasPattern restricts profile aliases to a safe charset. The alias
// is used verbatim as a keyring key and as a filename component
// (internal/auth/store.go's fallbackPath: dir/credentials/<alias>.json), so
// without this, an alias like "../../etc/cron.d/x" could write the
// credentials fallback file outside the config directory (CWE-22).
var validAliasPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateAlias(alias string) error {
	if !validAliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid profile alias %q: use only letters, digits, '-' and '_'", alias)
	}
	return nil
}

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configured Jira profiles (a base URL + OAuth app + authenticated user) and which one is active",
}

var (
	profileBaseURL      string
	profileClientID     string
	profileClientSecret string
	profileScopesFlag   string
	profileRedirectPort int
	profileActivate     bool
)

var profileAddCmd = &cobra.Command{
	Use:   "add <alias>",
	Short: "Register a Jira profile and the OAuth app used to authorize it",
	Long: `Register a Jira profile under a local alias. You must first create an
"OAuth 2.0 (3LO)" app at https://developer.atlassian.com/console/myapps/,
add the Jira Cloud REST API permissions/scopes you need, and add
http://localhost:<redirect-port>/callback (51837 by default) as its
callback URL. See README.md for the full walkthrough.

Atlassian's OAuth 2.0 (3LO) apps do not support PKCE/public clients, so
--client-secret is required, not optional.

Adding a profile never activates it automatically — pass --activate, or
run "gojira profile use <alias>" afterward. This holds even for your very
first profile: switching which profile is active is always a manual step.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		if err := validateAlias(alias); err != nil {
			return err
		}
		if profileBaseURL == "" || profileClientID == "" || profileClientSecret == "" {
			return fmt.Errorf("--base-url, --client-id, and --client-secret are all required")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		profile := config.Profile{
			BaseURL:      profileBaseURL,
			ClientID:     profileClientID,
			ClientSecret: profileClientSecret,
			RedirectPort: profileRedirectPort,
		}
		if profileScopesFlag != "" {
			profile.Scopes = strings.Split(profileScopesFlag, ",")
		}
		addProfile(cfg, alias, profile, profileActivate)
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("Profile %q saved. Run `gojira auth login --profile %s` to authorize it.\n", alias, alias)
		switch {
		case profileActivate:
			fmt.Printf("Profile %q is now active.\n", alias)
		case cfg.ActiveProfile == "":
			fmt.Printf("No active profile yet. Run `gojira profile use %s` (or pass --activate next time) once it's authorized.\n", alias)
		}
		return nil
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 {
			fmt.Println("No profiles configured. Run `gojira profile add <alias> --base-url ... --client-id ... --client-secret ...`.")
			return nil
		}
		for _, line := range profileListLines(cfg) {
			fmt.Println(line)
		}
		return nil
	},
}

var profileUseCmd = &cobra.Command{
	Use:   "use [alias]",
	Short: "Set the active profile (commands use it unless overridden with --profile)",
	Long: `Set the active profile. All commands default to it unless a single
invocation passes --profile to target a different one without changing what's
active.

Activating a profile immediately forces a token refresh for it, so a stale
access token doesn't linger unnoticed. If that refresh fails (e.g. the
refresh token itself has expired from long disuse), the switch still takes
effect, but the profile stays unauthenticated until you re-run
"gojira auth login --profile <alias>". Profiles you don't switch to are never
refreshed in the background.

With no alias, lists the configured profiles and prompts for one — this
requires interactive input; pass the alias directly in non-interactive
contexts.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		alias := ""
		if len(args) == 1 {
			alias = args[0]
		} else {
			if len(cfg.Profiles) == 0 {
				return fmt.Errorf("no profiles configured; run `gojira profile add` first")
			}
			alias, err = promptProfileChoice(cfg, cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
		}

		profile, err := activateProfile(cfg, alias)
		if err != nil {
			return err
		}
		if err := config.Save(cfg); err != nil {
			return err
		}

		tok, err := refreshToken(cmd.Context(), alias, profile)
		if err != nil {
			fmt.Printf("Switched to profile %q, but its token could not be refreshed: %v\nRun `gojira auth login --profile %s` to authenticate.\n", alias, err, alias)
			return nil
		}
		fmt.Printf("Switched to profile %q (token valid until %s).\n", alias, tok.Expiry.Format(time.RFC3339))
		return nil
	},
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <alias>",
	Short: "Remove a configured profile and its stored credentials",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := removeProfile(cfg, alias); err != nil {
			return err
		}
		if err := auth.DeleteToken(alias); err != nil {
			return err
		}
		return config.Save(cfg)
	},
}

// addProfile stores profile under alias in cfg, activating it only when
// activate is true — adding a profile never implicitly changes which one is
// active.
func addProfile(cfg *config.Config, alias string, profile config.Profile, activate bool) {
	cfg.Profiles[alias] = profile
	if activate {
		cfg.ActiveProfile = alias
	}
}

// activateProfile sets cfg.ActiveProfile to alias and returns its Profile,
// or an error if alias isn't configured. It mutates cfg but performs no I/O.
func activateProfile(cfg *config.Config, alias string) (config.Profile, error) {
	profile, ok := cfg.Profiles[alias]
	if !ok {
		return config.Profile{}, fmt.Errorf("unknown profile %q; run `gojira profile list`", alias)
	}
	cfg.ActiveProfile = alias
	return profile, nil
}

// removeProfile deletes alias from cfg.Profiles and clears ActiveProfile if
// it pointed at the removed profile. It mutates cfg but performs no I/O.
func removeProfile(cfg *config.Config, alias string) error {
	if _, ok := cfg.Profiles[alias]; !ok {
		return fmt.Errorf("unknown profile %q", alias)
	}
	delete(cfg.Profiles, alias)
	if cfg.ActiveProfile == alias {
		cfg.ActiveProfile = ""
	}
	return nil
}

// profileListLines formats cfg's profiles as one line per profile, sorted
// by alias, marking the active one.
func profileListLines(cfg *config.Config) []string {
	lines := make([]string, 0, len(cfg.Profiles))
	for _, alias := range sortedAliases(cfg.Profiles) {
		profile := cfg.Profiles[alias]
		marker := ""
		if alias == cfg.ActiveProfile {
			marker = " (active)"
		}
		cloud := "not resolved yet"
		if profile.CloudID != "" {
			cloud = profile.CloudID
		}
		lines = append(lines, fmt.Sprintf("%s%s\t%s\tcloud_id=%s", alias, marker, profile.BaseURL, cloud))
	}
	return lines
}

// sortedAliases returns profile aliases in a deterministic order, since Go
// map iteration order is randomized.
func sortedAliases(profiles map[string]config.Profile) []string {
	aliases := make([]string, 0, len(profiles))
	for alias := range profiles {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

// promptProfileChoice lists cfg's profiles on out and reads a 1-based
// selection from in. It errors instead of blocking forever when in has no
// input to give (e.g. a non-interactive invocation), the same way
// confirmDestructive declines rather than hanging.
func promptProfileChoice(cfg *config.Config, in io.Reader, out io.Writer) (string, error) {
	aliases := sortedAliases(cfg.Profiles)
	fmt.Fprintln(out, "Configured profiles:")
	for i, alias := range aliases {
		marker := ""
		if alias == cfg.ActiveProfile {
			marker = " (active)"
		}
		fmt.Fprintf(out, "  %d) %s%s\t%s\n", i+1, alias, marker, cfg.Profiles[alias].BaseURL)
	}
	fmt.Fprint(out, "Select a profile to activate [1-N]: ")

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		fmt.Fprintln(out, "\nno selection received; run `gojira profile use <alias>` directly")
		return "", fmt.Errorf("no selection received")
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(aliases) {
		return "", fmt.Errorf("invalid selection %q", strings.TrimSpace(line))
	}
	return aliases[n-1], nil
}

func init() {
	profileAddCmd.Flags().StringVar(&profileBaseURL, "base-url", "", "Jira Cloud site URL, e.g. https://yourteam.atlassian.net (required)")
	profileAddCmd.Flags().StringVar(&profileClientID, "client-id", "", "OAuth app client id (required)")
	profileAddCmd.Flags().StringVar(&profileClientSecret, "client-secret", "", "OAuth app client secret (required)")
	profileAddCmd.Flags().StringVar(&profileScopesFlag, "scopes", "", "comma-separated OAuth scopes (defaults to a broad read/write/admin set)")
	profileAddCmd.Flags().IntVar(&profileRedirectPort, "redirect-port", config.DefaultRedirectPort, "local port for the OAuth callback listener")
	profileAddCmd.Flags().BoolVar(&profileActivate, "activate", false, "make this the active profile immediately")

	profileCmd.AddCommand(profileAddCmd, profileListCmd, profileUseCmd, profileRemoveCmd)
}
