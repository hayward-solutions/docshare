package cmd

import (
	"fmt"
	"strings"

	"github.com/docshare/cli/internal/upgrade"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the CLI to the latest version",
	Long: `Check for a newer version and upgrade the CLI binary in-place.

This command will:
  - Check GitHub for the latest release
  - Download and verify the binary
  - Replace the current executable
  - Preserve your configuration

Example:
  docshare upgrade`,
	RunE: func(cmd *cobra.Command, args []string) error {
		currentVersion := Version
		if currentVersion == "" {
			currentVersion = "dev"
		}

		fmt.Printf("Current version: %s\n", currentVersion)

		if currentVersion == "dev" {
			return fmt.Errorf("cannot upgrade dev builds — please use a release version or build from source")
		}

		fmt.Println("Checking for updates...")
		latestVersion, err := upgrade.GetLatestVersion()
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}

		fmt.Printf("Latest version:  %s\n", latestVersion)

		if compareVersions(currentVersion, latestVersion) >= 0 {
			fmt.Println("\nYou're already running the latest version!")
			return nil
		}

		fmt.Printf("\nUpgrading to %s...\n", latestVersion)
		if err := upgrade.DownloadAndInstall(latestVersion); err != nil {
			return fmt.Errorf("upgrade failed: %w", err)
		}

		fmt.Printf("\nSuccessfully upgraded to %s\n", latestVersion)
		fmt.Println("Run 'docshare version' to verify.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func compareVersions(current, latest string) int {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	for i := 0; i < len(currentParts) && i < len(latestParts); i++ {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}

	if len(currentParts) < len(latestParts) {
		return -1
	}
	if len(currentParts) > len(latestParts) {
		return 1
	}

	return 0
}
