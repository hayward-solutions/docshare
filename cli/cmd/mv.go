package cmd

import (
	"fmt"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var mvCmd = &cobra.Command{
	Use:   "mv <source> <destination>",
	Short: "Move or rename a file/directory",
	Long: `Move a file to a different folder or rename it.

  docshare mv /Documents/report.pdf /Archive       Move to a different folder
  docshare mv /Documents/old.pdf new-name.pdf      Rename (destination without / = rename)`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		srcID, err := pathutil.Resolve(apiClient, args[0])
		if err != nil {
			return fmt.Errorf("resolving source: %w", err)
		}
		if srcID == "" {
			return fmt.Errorf("cannot move root directory")
		}

		dest := args[1]
		body := map[string]interface{}{}

		// If destination looks like a path (contains /), try to resolve it as a folder to move into.
		// If it's just a name (no /), treat it as a rename.
		destID, resolveErr := pathutil.Resolve(apiClient, dest)
		if resolveErr == nil && destID != "" {
			// Destination is an existing folder — move into it.
			body["parentID"] = destID
		} else {
			// Destination is not an existing path — treat as rename.
			body["name"] = dest
		}

		var resp api.Response[api.File]
		if err := apiClient.Put("/files/"+srcID, body, &resp); err != nil {
			return fmt.Errorf("moving/renaming: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		fmt.Printf("Updated: %s\n", resp.Data.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mvCmd)
}
