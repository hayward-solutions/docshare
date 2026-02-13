package cmd

import (
	"fmt"

	"github.com/docshare/cli/internal/api"
	"github.com/spf13/cobra"
)

var unshareCmd = &cobra.Command{
	Use:   "unshare <share-id>",
	Short: "Revoke a share",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		shareID := args[0]
		var resp api.Response[map[string]string]
		if err := apiClient.Delete("/shares/"+shareID, &resp); err != nil {
			return fmt.Errorf("revoking share: %w", err)
		}

		fmt.Println("Share revoked.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unshareCmd)
}
