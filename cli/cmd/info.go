package cmd

import (
	"fmt"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show details for a file or directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		fileID, err := pathutil.Resolve(apiClient, args[0])
		if err != nil {
			return err
		}
		if fileID == "" {
			return fmt.Errorf("cannot get info for root directory")
		}

		var resp api.Response[api.File]
		if err := apiClient.Get("/files/"+fileID, nil, &resp); err != nil {
			return fmt.Errorf("fetching file info: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		output.FileDetail(resp.Data)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
