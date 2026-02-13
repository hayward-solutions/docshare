package cmd

import (
	"fmt"
	"os"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagJSON      bool
	flagServerURL string

	cfg       *config.Config
	apiClient *api.Client
)

var rootCmd = &cobra.Command{
	Use:   "docshare",
	Short: "DocShare CLI — manage your files from the terminal",
	Long: `DocShare CLI lets you upload, download, share, and manage files
on your DocShare server without leaving the terminal.

Get started:
  docshare login              Authenticate via browser (device flow)
  docshare login --token X    Authenticate with an API token
  docshare ls                 List files in your root directory
  docshare upload file.pdf    Upload a file`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if flagServerURL != "" {
			cfg.ServerURL = flagServerURL
		}
		apiClient = api.NewClient(cfg.ServerURL, cfg.Token)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().StringVar(&flagServerURL, "server", "", "Override server URL (default: from config or http://localhost:8080)")
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return err
	}
	return nil
}

// requireAuth is a helper that returns an error if no token is configured.
func requireAuth() error {
	if cfg == nil || !cfg.HasToken() {
		return fmt.Errorf("not authenticated — run \"docshare login\" first")
	}
	return nil
}
