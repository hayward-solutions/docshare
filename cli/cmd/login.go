package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/config"
	"github.com/spf13/cobra"
)

var flagToken string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your DocShare server",
	Long: `Authenticate using either an API token or the device authorization flow.

API Token:
  docshare login --token dsh_abc123...

Device Flow (default):
  docshare login
  Opens your browser to approve the CLI.`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&flagToken, "token", "", "API token (dsh_...) for direct authentication")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	if flagToken != "" {
		return loginWithToken(flagToken)
	}
	return loginDeviceFlow()
}

func loginWithToken(token string) error {
	// Validate the token by calling /auth/me.
	client := api.NewClient(cfg.ServerURL, token)
	var resp api.Response[api.User]
	if err := client.Get("/auth/me", nil, &resp); err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) && apiErr.Status == 401 {
			return fmt.Errorf("invalid token — server returned 401")
		}
		return fmt.Errorf("validating token: %w", err)
	}

	cfg.Token = token
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Logged in as %s %s (%s)\n", resp.Data.FirstName, resp.Data.LastName, resp.Data.Email)
	return nil
}

func loginDeviceFlow() error {
	client := api.NewClient(cfg.ServerURL, "")

	// Step 1: Request a device code.
	var deviceResp api.DeviceCodeResponse
	values := url.Values{
		"client_id": {"docshare-cli"},
	}
	if err := client.PostForm("/auth/device/code", values, &deviceResp); err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}

	fmt.Printf("Opening browser to complete authentication...\n")
	fmt.Printf("If the browser doesn't open, visit:\n  %s\n\n", deviceResp.VerificationURIComplete)
	fmt.Printf("Your code: %s\n\n", deviceResp.UserCode)

	_ = openBrowser(deviceResp.VerificationURIComplete)

	// Step 2: Poll for the token.
	interval := time.Duration(deviceResp.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)

	fmt.Print("Waiting for approval...")

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		pollValues := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {deviceResp.DeviceCode},
			"client_id":   {"docshare-cli"},
		}

		var tokenResp api.DeviceTokenResponse
		err := client.PostForm("/auth/device/token", pollValues, &tokenResp)
		if err != nil {
			var apiErr *api.APIError
			if errors.As(err, &apiErr) {
				switch {
				case apiErr.Status == 400 && (apiErr.Message == "authorization_pending" || apiErr.Message == "the user has not yet approved"):
					fmt.Print(".")
					continue
				case apiErr.Status == 400 && apiErr.Message == "slow_down":
					interval += 5 * time.Second
					continue
				case apiErr.Message == "the device code has expired" || apiErr.Message == "expired_token":
					fmt.Println()
					return fmt.Errorf("device code expired — please try again")
				case apiErr.Message == "the user denied the request" || apiErr.Message == "access_denied":
					fmt.Println()
					return fmt.Errorf("authorization denied")
				}
			}
			fmt.Println()
			return fmt.Errorf("polling for token: %w", err)
		}

		if tokenResp.AccessToken != "" {
			fmt.Println(" approved!")

			cfg.Token = tokenResp.AccessToken
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			// Fetch and display user info.
			authClient := api.NewClient(cfg.ServerURL, tokenResp.AccessToken)
			var meResp api.Response[api.User]
			if err := authClient.Get("/auth/me", nil, &meResp); err == nil {
				fmt.Printf("Logged in as %s %s (%s)\n", meResp.Data.FirstName, meResp.Data.LastName, meResp.Data.Email)
			} else {
				fmt.Println("Logged in successfully.")
			}
			return nil
		}
	}

	fmt.Println()
	return fmt.Errorf("device code expired — please try again")
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}
