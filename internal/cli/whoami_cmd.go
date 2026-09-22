package cli

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var whoamiSiteFlag string

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated Jira user (convenience wrapper over GET /myself)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, alias, err := siteClient(cmd.Context(), whoamiSiteFlag)
		if err != nil {
			return err
		}
		resp, err := c.Do(cmd.Context(), http.MethodGet, "/myself", nil, nil)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("GET /myself -> HTTP %d on site %q: %s", resp.StatusCode, alias, string(resp.Body))
		}
		return renderResponse(os.Stdout, resp.Body, "displayName,emailAddress,accountId", "json", false)
	},
}

func init() {
	whoamiCmd.Flags().StringVar(&whoamiSiteFlag, "site", "", "site alias (defaults to the configured default site)")
}
