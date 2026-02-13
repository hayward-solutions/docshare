package cmd

import (
	"fmt"
	"net/url"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var (
	flagSort  string
	flagOrder string
)

var lsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List files and directories",
	Long: `List files in your root directory or inside a folder.

  docshare ls                       List root
  docshare ls /Documents            List by path
  docshare ls 550e8400-...          List by folder ID`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		path := ""
		if len(args) > 0 {
			path = args[0]
		}

		folderID, err := pathutil.Resolve(apiClient, path)
		if err != nil {
			return err
		}

		params := url.Values{}
		if flagSort != "" {
			params.Set("sort", flagSort)
		}
		if flagOrder != "" {
			params.Set("order", flagOrder)
		}

		var resp api.Response[[]api.File]
		if folderID == "" {
			err = apiClient.Get("/files", params, &resp)
		} else {
			err = apiClient.Get("/files/"+folderID+"/children", params, &resp)
		}
		if err != nil {
			return fmt.Errorf("listing files: %w", err)
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
	lsCmd.Flags().StringVar(&flagSort, "sort", "", "Sort field: name, createdAt, size")
	lsCmd.Flags().StringVar(&flagOrder, "order", "", "Sort order: asc, desc")
	rootCmd.AddCommand(lsCmd)
}
