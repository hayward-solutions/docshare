package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var flagForce bool

var rmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Delete a file or directory",
	Long: `Delete a file or directory from the server.

  docshare rm /Documents/old-report.pdf
  docshare rm /Temp --force                    Skip confirmation

Warning: Deleting a directory removes all contents recursively. This cannot be undone.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		fileID, err := pathutil.Resolve(apiClient, args[0])
		if err != nil {
			return err
		}
		if fileID == "" {
			return fmt.Errorf("cannot delete root directory")
		}

		// Get file info for confirmation message.
		var infoResp api.Response[api.File]
		if err := apiClient.Get("/files/"+fileID, nil, &infoResp); err != nil {
			return fmt.Errorf("fetching file info: %w", err)
		}

		f := infoResp.Data

		if !flagForce {
			kind := "file"
			if f.IsDirectory {
				kind = "directory (and all contents)"
			}
			fmt.Printf("Delete %s %q? This cannot be undone. [y/N] ", kind, f.Name)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		var resp api.Response[map[string]string]
		if err := apiClient.Delete("/files/"+fileID, &resp); err != nil {
			return fmt.Errorf("deleting: %w", err)
		}

		fmt.Printf("Deleted: %s\n", f.Name)
		return nil
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&flagForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}
