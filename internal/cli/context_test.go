package cli

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/joaoreis13/gojira/internal/config"
)

// fakeTokenSource lets tests stand in for auth.TokenSource without touching
// the OS keyring or the network.
type fakeTokenSource struct {
	tok *oauth2.Token
	err error
}

func (f fakeTokenSource) Token() (*oauth2.Token, error) { return f.tok, f.err }

func withFakeTokenSource(t *testing.T, fn func(ctx context.Context, alias string, profile config.Profile) (oauth2.TokenSource, error)) {
	t.Helper()
	orig := newTokenSource
	newTokenSource = fn
	t.Cleanup(func() { newTokenSource = orig })
}

func TestRefreshTokenSuccess(t *testing.T) {
	want := &oauth2.Token{AccessToken: "abc", Expiry: time.Now().Add(time.Hour)}
	withFakeTokenSource(t, func(ctx context.Context, alias string, profile config.Profile) (oauth2.TokenSource, error) {
		return fakeTokenSource{tok: want}, nil
	})

	got, err := refreshToken(context.Background(), "work", config.Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "abc" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "abc")
	}
}

func TestRefreshTokenPropagatesConstructionError(t *testing.T) {
	withFakeTokenSource(t, func(ctx context.Context, alias string, profile config.Profile) (oauth2.TokenSource, error) {
		return nil, fmt.Errorf("not logged in to profile %q", alias)
	})

	if _, err := refreshToken(context.Background(), "work", config.Profile{}); err == nil {
		t.Error("expected error when token source construction fails, got nil")
	}
}

func TestRefreshTokenPropagatesRefreshError(t *testing.T) {
	withFakeTokenSource(t, func(ctx context.Context, alias string, profile config.Profile) (oauth2.TokenSource, error) {
		return fakeTokenSource{err: fmt.Errorf("refresh token expired")}, nil
	})

	if _, err := refreshToken(context.Background(), "work", config.Profile{}); err == nil {
		t.Error("expected error when the refresh itself fails, got nil")
	}
}
