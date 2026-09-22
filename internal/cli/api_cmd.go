package cli

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	apiSiteFlag   string
	apiDataFlag   string
	apiQueryFlags []string
	apiFieldsFlag string
	apiOutputFlag string
	apiPrettyFlag bool
	apiYesFlag    bool
)

var apiCmd = &cobra.Command{
	Use:   "api <METHOD> <path>",
	Short: "Call any Jira Cloud REST API v3 endpoint directly",
	Long: `A generic passthrough to https://api.atlassian.com/ex/jira/{cloudId}/rest/api/3{path},
authenticated with the resolved site's OAuth token. This is the escape
hatch that makes every endpoint reachable, including admin and destructive
ones, without gojira having to hand-implement each one.

Examples:
  gojira api GET /issue/PROJ-123
  gojira api GET /search --query jql="project = PROJ" --fields "issues.key,issues.fields.summary"
  gojira api POST /issue --data '{"fields":{"project":{"key":"PROJ"},"summary":"New issue","issuetype":{"name":"Task"}}}'
  gojira api DELETE /issue/PROJ-123 --yes`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		method := strings.ToUpper(args[0])
		path := args[1]
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		if method == http.MethodDelete && !apiYesFlag {
			if !confirmDestructive(method, path) {
				return fmt.Errorf("aborted")
			}
		}

		body, err := readBody(apiDataFlag)
		if err != nil {
			return err
		}
		query, err := parseQuery(apiQueryFlags)
		if err != nil {
			return err
		}

		c, _, err := siteClient(cmd.Context(), apiSiteFlag)
		if err != nil {
			return err
		}

		resp, err := c.Do(cmd.Context(), method, path, query, body)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("%s %s -> HTTP %d: %s", method, path, resp.StatusCode, string(resp.Body))
		}

		return renderResponse(os.Stdout, resp.Body, apiFieldsFlag, apiOutputFlag, apiPrettyFlag)
	},
}

func init() {
	apiCmd.Flags().StringVar(&apiSiteFlag, "site", "", "site alias (defaults to the configured default site)")
	apiCmd.Flags().StringVar(&apiDataFlag, "data", "", "request body: inline JSON, @file, or - for stdin")
	apiCmd.Flags().StringArrayVar(&apiQueryFlags, "query", nil, "query parameter key=value (repeatable)")
	apiCmd.Flags().StringVar(&apiFieldsFlag, "fields", "", "comma-separated dot-paths to keep in the output, e.g. issues.key,issues.fields.summary")
	apiCmd.Flags().StringVar(&apiOutputFlag, "output", "json", "output format: json or text")
	apiCmd.Flags().BoolVar(&apiPrettyFlag, "pretty", false, "pretty-print JSON output")
	apiCmd.Flags().BoolVarP(&apiYesFlag, "yes", "y", false, "skip the confirmation prompt for destructive methods (DELETE)")
}
