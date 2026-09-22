package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	searchSiteFlag      string
	searchMaxResults    int
	searchPageToken     string
	searchFieldsAPIFlag string
	searchFieldsFlag    string
	searchOutputFlag    string
)

var searchCmd = &cobra.Command{
	Use:   "search <JQL>",
	Short: "Run a JQL search (convenience wrapper over POST /search/jql)",
	Long: `A thin convenience wrapper over Jira's current JQL search endpoint,
POST /rest/api/3/search/jql. (The older GET/POST /search is marked
"currently being removed" in Atlassian's own API reference, so gojira
targets the replacement directly: see the Issue search group at
https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/)

Results are paginated by an opaque token instead of an offset: when the
response's "isLast" is false, pass its "nextPageToken" back via --page-token
to fetch the next page.

JQL field/syntax reference: https://support.atlassian.com/jira-software-cloud/docs/jql-fields/`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]any{
			"jql":        args[0],
			"maxResults": searchMaxResults,
		}
		if searchFieldsAPIFlag != "" {
			body["fields"] = strings.Split(searchFieldsAPIFlag, ",")
		}
		if searchPageToken != "" {
			body["nextPageToken"] = searchPageToken
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}

		c, alias, err := siteClient(cmd.Context(), searchSiteFlag)
		if err != nil {
			return err
		}
		resp, err := c.Do(cmd.Context(), http.MethodPost, "/search/jql", nil, payload)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("POST /search/jql -> HTTP %d on site %q: %s", resp.StatusCode, alias, string(resp.Body))
		}
		return renderResponse(os.Stdout, resp.Body, searchFieldsFlag, searchOutputFlag, false)
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchSiteFlag, "site", "", "site alias (defaults to the configured default site)")
	searchCmd.Flags().IntVar(&searchMaxResults, "max-results", 50, "maximum number of issues to return in this page")
	searchCmd.Flags().StringVar(&searchPageToken, "page-token", "", "nextPageToken from a previous response, to fetch the following page")
	searchCmd.Flags().StringVar(&searchFieldsAPIFlag, "jira-fields", "", "comma-separated Jira issue fields to request from the API, e.g. summary,status,assignee")
	searchCmd.Flags().StringVar(&searchFieldsFlag, "fields", "", "comma-separated dot-paths to keep in the output, e.g. issues.key,issues.fields.summary")
	searchCmd.Flags().StringVar(&searchOutputFlag, "output", "text", "output format: json or text")
}
