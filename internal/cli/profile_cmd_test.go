package cli

import (
	"strings"
	"testing"

	"github.com/joaoreis13/gojira/internal/config"
)

func TestValidateAliasAcceptsSafeCharset(t *testing.T) {
	for _, alias := range []string{"work", "personal-2", "team_a", "A1"} {
		if err := validateAlias(alias); err != nil {
			t.Errorf("validateAlias(%q) = %v, want nil", alias, err)
		}
	}
}

func TestValidateAliasRejectsPathTraversal(t *testing.T) {
	for _, alias := range []string{
		"../../etc/cron.d/evil",
		"..",
		"a/b",
		"a\\b",
		"",
		"has space",
		".hidden",
	} {
		if err := validateAlias(alias); err == nil {
			t.Errorf("validateAlias(%q) = nil, want an error (would escape the credentials dir or keyring key namespace)", alias)
		}
	}
}

func TestAddProfileNeverActivatesWithoutFlag(t *testing.T) {
	cfg := &config.Config{Profiles: map[string]config.Profile{}}
	addProfile(cfg, "work", config.Profile{BaseURL: "https://work.atlassian.net"}, false)

	if _, ok := cfg.Profiles["work"]; !ok {
		t.Fatal("expected profile \"work\" to be stored")
	}
	if cfg.ActiveProfile != "" {
		t.Errorf("ActiveProfile = %q, want empty (adding must never auto-activate)", cfg.ActiveProfile)
	}
}

func TestAddProfileActivatesOnlyWhenRequested(t *testing.T) {
	cfg := &config.Config{Profiles: map[string]config.Profile{}}
	addProfile(cfg, "work", config.Profile{BaseURL: "https://work.atlassian.net"}, true)

	if cfg.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "work")
	}
}

func TestAddProfileDoesNotDisturbExistingActive(t *testing.T) {
	cfg := &config.Config{
		ActiveProfile: "work",
		Profiles: map[string]config.Profile{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}
	addProfile(cfg, "personal", config.Profile{BaseURL: "https://personal.atlassian.net"}, false)

	if cfg.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q, want unchanged %q", cfg.ActiveProfile, "work")
	}
}

func TestActivateProfileSwitchesActive(t *testing.T) {
	cfg := &config.Config{
		ActiveProfile: "work",
		Profiles: map[string]config.Profile{
			"work":     {BaseURL: "https://work.atlassian.net"},
			"personal": {BaseURL: "https://personal.atlassian.net"},
		},
	}
	profile, err := activateProfile(cfg, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "personal" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "personal")
	}
	if profile.BaseURL != "https://personal.atlassian.net" {
		t.Errorf("returned profile.BaseURL = %q", profile.BaseURL)
	}
}

func TestActivateProfileUnknownAlias(t *testing.T) {
	cfg := &config.Config{Profiles: map[string]config.Profile{}}
	if _, err := activateProfile(cfg, "missing"); err == nil {
		t.Error("expected error activating an unconfigured profile, got nil")
	}
}

func TestRemoveProfileClearsActive(t *testing.T) {
	cfg := &config.Config{
		ActiveProfile: "work",
		Profiles: map[string]config.Profile{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}
	if err := removeProfile(cfg, "work"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profiles["work"]; ok {
		t.Error("expected profile \"work\" to be removed")
	}
	if cfg.ActiveProfile != "" {
		t.Errorf("ActiveProfile = %q, want cleared after removing the active profile", cfg.ActiveProfile)
	}
}

func TestRemoveProfileLeavesOtherActiveAlone(t *testing.T) {
	cfg := &config.Config{
		ActiveProfile: "work",
		Profiles: map[string]config.Profile{
			"work":     {BaseURL: "https://work.atlassian.net"},
			"personal": {BaseURL: "https://personal.atlassian.net"},
		},
	}
	if err := removeProfile(cfg, "personal"); err != nil {
		t.Fatal(err)
	}
	if cfg.ActiveProfile != "work" {
		t.Errorf("ActiveProfile = %q, want unchanged %q", cfg.ActiveProfile, "work")
	}
}

func TestRemoveProfileUnknownAlias(t *testing.T) {
	cfg := &config.Config{Profiles: map[string]config.Profile{}}
	if err := removeProfile(cfg, "missing"); err == nil {
		t.Error("expected error removing an unconfigured profile, got nil")
	}
}

func TestProfileListLinesMarksActive(t *testing.T) {
	cfg := &config.Config{
		ActiveProfile: "work",
		Profiles: map[string]config.Profile{
			"work":     {BaseURL: "https://work.atlassian.net", CloudID: "cloud-1"},
			"personal": {BaseURL: "https://personal.atlassian.net"},
		},
	}
	lines := profileListLines(cfg)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	// sortedAliases orders alphabetically: personal, work.
	if !strings.Contains(lines[0], "personal") || strings.Contains(lines[0], "(active)") {
		t.Errorf("lines[0] = %q, want personal without active marker", lines[0])
	}
	if !strings.Contains(lines[1], "work (active)") {
		t.Errorf("lines[1] = %q, want work marked active", lines[1])
	}
	if !strings.Contains(lines[0], "not resolved yet") {
		t.Errorf("lines[0] = %q, want cloud_id=not resolved yet", lines[0])
	}
}

func TestPromptProfileChoiceValidSelection(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"personal": {BaseURL: "https://personal.atlassian.net"},
			"work":     {BaseURL: "https://work.atlassian.net"},
		},
	}
	in := strings.NewReader("2\n")
	var out strings.Builder

	got, err := promptProfileChoice(cfg, in, &out)
	if err != nil {
		t.Fatal(err)
	}
	// sorted order: personal(1), work(2)
	if got != "work" {
		t.Errorf("selection = %q, want %q", got, "work")
	}
	if !strings.Contains(out.String(), "1) personal") || !strings.Contains(out.String(), "2) work") {
		t.Errorf("prompt output missing expected listing: %q", out.String())
	}
}

func TestPromptProfileChoiceOutOfRange(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}
	in := strings.NewReader("9\n")
	var out strings.Builder

	if _, err := promptProfileChoice(cfg, in, &out); err == nil {
		t.Error("expected error for out-of-range selection, got nil")
	}
}

func TestPromptProfileChoiceNonNumeric(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}
	in := strings.NewReader("nope\n")
	var out strings.Builder

	if _, err := promptProfileChoice(cfg, in, &out); err == nil {
		t.Error("expected error for non-numeric selection, got nil")
	}
}

func TestPromptProfileChoiceNoInputDoesNotHang(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"work": {BaseURL: "https://work.atlassian.net"},
		},
	}
	in := strings.NewReader("") // immediate EOF, simulating non-interactive stdin
	var out strings.Builder

	if _, err := promptProfileChoice(cfg, in, &out); err == nil {
		t.Error("expected error on immediate EOF, got nil")
	}
}

func TestSortedAliasesDeterministic(t *testing.T) {
	profiles := map[string]config.Profile{
		"zeta":  {},
		"alpha": {},
		"mid":   {},
	}
	got := sortedAliases(profiles)
	want := []string{"alpha", "mid", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sortedAliases = %v, want %v", got, want)
		}
	}
}
