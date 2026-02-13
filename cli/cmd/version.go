package cmd

import (
	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/spf13/cobra"
)

// Version is the CLI version, injected at build time:
//
//	go build -ldflags "-X github.com/docshare/cli/cmd.Version=1.2.3"
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI and server version",
	RunE: func(cmd *cobra.Command, args []string) error {
		var resp api.Response[api.VersionInfo]
		serverErr := apiClient.Get("/version", nil, &resp)

		var serverInfo *api.VersionInfo
		if serverErr == nil {
			serverInfo = &resp.Data
		}

		if flagJSON {
			type jsonOut struct {
				CLIVersion    string `json:"cliVersion"`
				ServerVersion string `json:"serverVersion,omitempty"`
				APIVersion    string `json:"apiVersion,omitempty"`
				ServerError   string `json:"serverError,omitempty"`
			}
			out := jsonOut{CLIVersion: Version}
			if serverInfo != nil {
				out.ServerVersion = serverInfo.Version
				out.APIVersion = serverInfo.APIVersion
			} else {
				out.ServerError = serverErr.Error()
			}
			output.JSON(out)
			return nil
		}

		output.VersionInfo(Version, serverInfo)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
