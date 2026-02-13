package cmd

import (
	"fmt"
	"path"
	"strings"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var mkdirCmd = &cobra.Command{
	Use:   "mkdir <name> [parent-path]",
	Short: "Create a new directory",
	Long: `Create a new directory on the server.

  docshare mkdir "My Documents"                    Create in root
  docshare mkdir Reports /Documents                Create inside a folder
  docshare mkdir Reports --parent <uuid>           Create inside a folder by ID`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		name := args[0]

		// Support creating nested path like "mkdir /Documents/Reports/Q1"
		// by resolving the parent path portion.
		parentID := flagParent
		if parentID == "" && len(args) > 1 {
			resolved, err := pathutil.Resolve(apiClient, args[1])
			if err != nil {
				return fmt.Errorf("resolving parent: %w", err)
			}
			parentID = resolved
		}

		// If name contains slashes, treat the prefix as the parent path.
		if parentID == "" && strings.Contains(name, "/") {
			dir := path.Dir(name)
			name = path.Base(name)
			if dir != "." && dir != "/" {
				resolved, err := pathutil.Resolve(apiClient, dir)
				if err != nil {
					return fmt.Errorf("resolving parent path: %w", err)
				}
				parentID = resolved
			}
		}

		body := map[string]interface{}{"name": name}
		if parentID != "" {
			body["parentID"] = parentID
		}

		var resp api.Response[api.File]
		if err := apiClient.Post("/files/directory", body, &resp); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		fmt.Printf("Created directory: %s (id: %s)\n", resp.Data.Name, resp.Data.ID)
		return nil
	},
}

func init() {
	mkdirCmd.Flags().StringVar(&flagParent, "parent", "", "Parent folder ID")
	rootCmd.AddCommand(mkdirCmd)
}
