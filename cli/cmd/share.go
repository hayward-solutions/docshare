package cmd

import (
	"fmt"
	"net/url"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var flagPermission string

var shareCmd = &cobra.Command{
	Use:   "share <file-path> <user-email>",
	Short: "Share a file with a user",
	Long: `Share a file or directory with another user by email.

  docshare share /Documents/report.pdf alice@example.com
  docshare share /Documents/report.pdf alice@example.com --permission edit`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuth(); err != nil {
			return err
		}

		fileID, err := pathutil.Resolve(apiClient, args[0])
		if err != nil {
			return err
		}
		if fileID == "" {
			return fmt.Errorf("cannot share root directory")
		}

		// Search for user by email.
		email := args[1]
		params := url.Values{"q": {email}}
		var searchResp api.Response[[]api.User]
		if err := apiClient.Get("/users/search", params, &searchResp); err != nil {
			return fmt.Errorf("searching users: %w", err)
		}

		var targetUser *api.User
		for _, u := range searchResp.Data {
			if u.Email == email {
				targetUser = &u
				break
			}
		}
		if targetUser == nil {
			return fmt.Errorf("user not found: %s", email)
		}

		body := map[string]interface{}{
			"sharedWithUserID": targetUser.ID,
			"permission":       flagPermission,
		}

		var resp api.Response[api.Share]
		if err := apiClient.Post("/files/"+fileID+"/share", body, &resp); err != nil {
			return fmt.Errorf("sharing: %w", err)
		}

		if flagJSON {
			output.JSON(resp.Data)
			return nil
		}

		fmt.Printf("Shared with %s (%s permission)\n", email, resp.Data.Permission)
		return nil
	},
}

func init() {
	shareCmd.Flags().StringVar(&flagPermission, "permission", "download", "Permission level: view, download, edit")
	rootCmd.AddCommand(shareCmd)
}
