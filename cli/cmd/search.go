package cmd

import (
	"fmt"
	"net/url"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for files by name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		params := url.Values{"q": {args[0]}}
		var resp api.Response[[]api.File]
		if err := apiClient.Get("/files/search", params, &resp); err != nil {
			return fmt.Errorf("searching: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		output.FileTable(resp.Data)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
}
