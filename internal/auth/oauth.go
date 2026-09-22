// Package auth implements Atlassian's OAuth 2.0 (3LO) authorization-code
// flow for Jira Cloud (no PKCE — see Login), plus token storage and
// transparent refresh.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/joaoreis13/gojira/internal/config"
)

const (
	authorizeURL           = "https://auth.atlassian.com/authorize"
	tokenURL               = "https://auth.atlassian.com/oauth/token"
	accessibleResourcesURL = "https://api.atlassian.com/oauth/token/accessible-resources"
	loginCallbackTimeout   = 3 * time.Minute
)

func redirectPort(site config.Site) int {
	if site.RedirectPort != 0 {
		return site.RedirectPort
	}
	return config.DefaultRedirectPort
}

func scopes(site config.Site) []string {
	if len(site.Scopes) > 0 {
		return site.Scopes
	}
	return config.DefaultScopes
}

func oauthConfig(site config.Site) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     site.ClientID,
		ClientSecret: site.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: authorizeURL, TokenURL: tokenURL},
		RedirectURL:  fmt.Sprintf("http://localhost:%d/callback", redirectPort(site)),
		Scopes:       scopes(site),
	}
}

// AccessibleResource is one Jira site the authorizing user granted access to.
type AccessibleResource struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// Login runs the interactive browser + local-callback OAuth flow, exchanges
// the resulting code for tokens, resolves the Jira Cloud id matching
// site.BaseURL, and persists the token. It returns the resolved cloud id.
//
// This uses the plain authorization-code grant with a client secret, not
// PKCE: as of this writing, Atlassian's OAuth 2.0 (3LO) apps don't support
// PKCE for apps created in the developer console (confirmed by Atlassian
// staff: https://community.developer.atlassian.com/t/oauth-2-0-with-proof-key-for-code-exchange-pkce/80173),
// so a confidential client (client id + secret) is required.
func Login(ctx context.Context, alias string, site config.Site, autoOpenBrowser bool) (cloudID string, err error) {
	state, err := randomString(24)
	if err != nil {
		return "", err
	}
	conf := oauthConfig(site)

	code, err := runCallbackServer(ctx, redirectPort(site), state, conf, autoOpenBrowser)
	if err != nil {
		return "", err
	}

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange authorization code: %w", err)
	}

	// Save the token before resolving the cloud id: the exchange above is the
	// hard-won, non-retryable step (it consumes a single-use auth code and
	// needs a fresh browser consent to redo), while the cloud id is a cheap
	// lookup that EnsureCloudID already re-resolves lazily on next use. Losing
	// the token to a transient failure in the latter would be far worse than
	// losing the (already-cached) cloud id.
	if _, err := SaveToken(alias, tok); err != nil {
		return "", fmt.Errorf("save token: %w", err)
	}

	cloudID, err = ResolveCloudID(ctx, tok, site.BaseURL)
	if err != nil {
		return "", fmt.Errorf("token saved, but resolving the Jira cloud id failed (it will be retried automatically on your next gojira command): %w", err)
	}
	return cloudID, nil
}

func runCallbackServer(ctx context.Context, port int, state string, conf *oauth2.Config, autoOpenBrowser bool) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if authErr := q.Get("error"); authErr != "" {
			fmt.Fprintf(w, "<html><body>gojira: authorization failed (%s). You can close this tab.</body></html>", authErr)
			errCh <- fmt.Errorf("authorization denied: %s", authErr)
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth state mismatch (possible CSRF); aborting login")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no authorization code in callback")
			return
		}
		fmt.Fprint(w, "<html><body>gojira: authorization complete. You can close this tab.</body></html>")
		codeCh <- code
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", fmt.Errorf("listen on 127.0.0.1:%d for OAuth callback (must match the redirect URL registered on the Atlassian app): %w", port, err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(ln) }()
	defer server.Close()

	authURL := conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("audience", "api.atlassian.com"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	if autoOpenBrowser {
		fmt.Printf("Opening your browser to authorize gojira:\n\n%s\n\n", authURL)
		if err := openBrowser(authURL); err != nil {
			fmt.Printf("(couldn't open a browser automatically: %v; open the URL above manually)\n", err)
		}
	} else {
		fmt.Printf("Open this URL in your browser to authorize gojira:\n\n%s\n\n", authURL)
	}

	select {
	case code := <-codeCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-time.After(loginCallbackTimeout):
		return "", fmt.Errorf("timed out waiting for browser authorization")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

const maxCloudIDRetries = 3

// ResolveCloudID looks up the Jira Cloud id for the site matching baseURL
// among the resources the current token has access to. If baseURL is empty
// and exactly one resource is accessible, that one is used.
func ResolveCloudID(ctx context.Context, tok *oauth2.Token, baseURL string) (string, error) {
	body, err := listAccessibleResources(ctx, tok)
	if err != nil {
		return "", err
	}

	var resources []AccessibleResource
	if err := json.Unmarshal(body, &resources); err != nil {
		return "", fmt.Errorf("parse accessible resources: %w", err)
	}
	if len(resources) == 0 {
		return "", fmt.Errorf("the authorizing account has no accessible Jira sites for this app's granted scopes")
	}

	norm := func(u string) string { return strings.TrimSuffix(strings.ToLower(u), "/") }
	if baseURL != "" {
		for _, r := range resources {
			if norm(r.URL) == norm(baseURL) {
				return r.ID, nil
			}
		}
		var available []string
		for _, r := range resources {
			available = append(available, r.URL)
		}
		return "", fmt.Errorf("no accessible site matches base_url %q; accessible sites: %s", baseURL, strings.Join(available, ", "))
	}

	if len(resources) > 1 {
		var available []string
		for _, r := range resources {
			available = append(available, r.URL)
		}
		return "", fmt.Errorf("multiple accessible sites and no base_url configured to disambiguate: %s", strings.Join(available, ", "))
	}
	return resources[0].ID, nil
}

// listAccessibleResources calls accessibleResourcesURL, retrying on 429 and
// transient 5xx responses with backoff, the same way internal/client.Do does
// for regular API calls (this endpoint is reached directly rather than
// through that client, since it's not scoped to a resolved cloud id yet).
func listAccessibleResources(ctx context.Context, tok *oauth2.Token) ([]byte, error) {
	var lastErr error
	var wait time.Duration
	for attempt := 0; attempt <= maxCloudIDRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, accessibleResourcesURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("list accessible resources: %w", err)
			wait = time.Duration(1<<uint(attempt+1)) * 500 * time.Millisecond
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read accessible resources response: %w", readErr)
			wait = time.Duration(1<<uint(attempt+1)) * 500 * time.Millisecond
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			lastErr = fmt.Errorf("list accessible resources: %s: %s", resp.Status, string(body))
			if attempt < maxCloudIDRetries {
				wait = time.Duration(1<<uint(attempt+1)) * 500 * time.Millisecond
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("list accessible resources: %s: %s", resp.Status, string(body))
		}
		return body, nil
	}
	return nil, lastErr
}

// TokenSource returns an oauth2.TokenSource that transparently refreshes the
// access token as needed and persists any refreshed token back to storage.
func TokenSource(ctx context.Context, alias string, site config.Site) (oauth2.TokenSource, error) {
	tok, err := LoadToken(alias)
	if err != nil {
		return nil, err
	}
	inner := oauthConfig(site).TokenSource(ctx, tok)
	return &persistingTokenSource{alias: alias, inner: inner, last: tok.AccessToken}, nil
}

type persistingTokenSource struct {
	alias string
	inner oauth2.TokenSource
	last  string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.inner.Token()
	if err != nil {
		return nil, fmt.Errorf("refresh access token for site %q (try `gojira auth login --site %s`): %w", p.alias, p.alias, err)
	}
	if tok.AccessToken != p.last {
		if _, err := SaveToken(p.alias, tok); err != nil {
			return nil, fmt.Errorf("persist refreshed token: %w", err)
		}
		p.last = tok.AccessToken
	}
	return tok, nil
}

// EnsureCloudID resolves and returns site.CloudID, resolving it via the
// accessible-resources endpoint and reporting it as changed (so the caller
// can persist config) when it wasn't already cached.
func EnsureCloudID(ctx context.Context, ts oauth2.TokenSource, site config.Site) (cloudID string, changed bool, err error) {
	if site.CloudID != "" {
		return site.CloudID, false, nil
	}
	tok, err := ts.Token()
	if err != nil {
		return "", false, fmt.Errorf("get access token: %w", err)
	}
	cloudID, err = ResolveCloudID(ctx, tok, site.BaseURL)
	if err != nil {
		return "", false, err
	}
	return cloudID, true, nil
}

// Status describes the current stored credential for a site without
// exposing the token values themselves.
type Status struct {
	LoggedIn   bool
	ExpiresAt  time.Time
	HasRefresh bool
}

func GetStatus(alias string) (Status, error) {
	tok, err := LoadToken(alias)
	if err != nil {
		return Status{}, err
	}
	return Status{
		LoggedIn:   true,
		ExpiresAt:  tok.Expiry,
		HasRefresh: tok.RefreshToken != "",
	}, nil
}

// Logout deletes locally stored credentials for a site. It does not revoke
// the grant on Atlassian's side; that must be done from
// https://id.atlassian.com/manage-profile/apps.
func Logout(alias string) error {
	return DeleteToken(alias)
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
