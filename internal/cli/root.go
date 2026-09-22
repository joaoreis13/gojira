// Package cli wires up gojira's cobra command tree.
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gojira",
	Short: "An OAuth-managed, full-coverage CLI for the Jira Cloud REST API",
	Long: `gojira authenticates to Jira Cloud with OAuth 2.0 (3LO) + PKCE and
manages token refresh automatically. "gojira api" reaches any REST API v3
endpoint directly (including admin and destructive ones), and a small set
of convenience commands (whoami, search, ...) sit on top of it.`,
	SilenceUsage:  true,
	SilenceErrors: false,
}

// Execute runs the CLI, returning any error from the invoked command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(authCmd, siteCmd, apiCmd, whoamiCmd, searchCmd)
}
