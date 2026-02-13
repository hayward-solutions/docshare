package cmd

import (
	"fmt"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/spf13/cobra"
)

var sharedCmd = &cobra.Command{
	Use:   "shared",
	Short: "List files shared with you",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		var resp api.Response[[]api.Share]
		if err := apiClient.Get("/shared", nil, &resp); err != nil {
			return fmt.Errorf("listing shared files: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		output.ShareTable(resp.Data)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sharedCmd)
}
