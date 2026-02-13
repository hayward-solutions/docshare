package cmd

import (
	"fmt"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		var resp api.Response[api.User]
		if err := apiClient.Get("/auth/me", nil, &resp); err != nil {
			return fmt.Errorf("fetching user: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		output.UserInfo(resp.Data)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
