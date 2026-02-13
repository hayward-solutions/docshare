package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagTransferTimeout string
)

var transferCmd = &cobra.Command{
	Use:   "transfer <send/receive> [path]",
	Short: "Transfer files between users",
	Long: `Send or receive files using secure transfer codes.

Send a file:
  docshare transfer send report.pdf

Receive a file:
  docshare transfer receive ABC123`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var transferSendCmd = &cobra.Command{
	Use:   "send <file>",
	Short: "Send a file and wait for receiver",
	Long: `Send a file and wait for someone to receive it.

  docshare transfer send report.pdf
  docshare transfer send ./folder/file.txt --timeout 10m`,
	Args: cobra.ExactArgs(1),
	RunE: runTransferSend,
}

var transferReceiveCmd = &cobra.Command{
	Use:   "receive <code>",
	Short: "Receive a file using a transfer code",
	Long: `Connect to a transfer using a code and receive the file.

  docshare transfer receive ABC123
  docshare transfer receive ABC123 --output ./Downloads`,
	Args: cobra.ExactArgs(1),
	RunE: runTransferReceive,
}

var transferListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending transfers you've initiated",
	RunE:  runTransferList,
}

var transferCancelCmd = &cobra.Command{
	Use:   "cancel <code>",
	Short: "Cancel a pending transfer",
	Args:  cobra.ExactArgs(1),
	RunE:  runTransferCancel,
}

func init() {
	transferSendCmd.Flags().StringVar(&flagTransferTimeout, "timeout", "5m", "How long to wait for receiver (e.g., 5m, 10m)")
	transferReceiveCmd.Flags().StringVarP(&flagOutput, "output", "o", ".", "Output directory for received file")

	transferCmd.AddCommand(transferSendCmd)
	transferCmd.AddCommand(transferReceiveCmd)
	transferCmd.AddCommand(transferListCmd)
	transferCmd.AddCommand(transferCancelCmd)
	rootCmd.AddCommand(transferCmd)
}

func parseTimeout(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	if strings.HasSuffix(s, "m") {
		var mins int
		if _, err := fmt.Sscanf(s, "%dm", &mins); err != nil {
			return 0, fmt.Errorf("invalid timeout format: %s", s)
		}
		return mins * 60, nil
	}
	if strings.HasSuffix(s, "s") {
		var secs int
		if _, err := fmt.Sscanf(s, "%ds", &secs); err != nil {
			return 0, fmt.Errorf("invalid timeout format: %s", s)
		}
		return secs, nil
	}
	if strings.HasSuffix(s, "h") {
		var hours int
		if _, err := fmt.Sscanf(s, "%dh", &hours); err != nil {
			return 0, fmt.Errorf("invalid timeout format: %s", s)
		}
		return hours * 3600, nil
	}

	return 0, fmt.Errorf("timeout must end in s, m, or h (e.g., 5m)")
}

func runTransferSend(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	localPath := args[0]

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", localPath, err)
	}

	if info.IsDir() {
		return fmt.Errorf("cannot transfer directories")
	}

	timeoutSecs, err := parseTimeout(flagTransferTimeout)
	if err != nil {
		return err
	}

	fileName := filepath.Base(localPath)
	fileSize := info.Size()

	fmt.Printf("Preparing to send %s (%s)...\n", fileName, output.FormatSize(fileSize))

	req := api.TransferCreateRequest{
		FileName: fileName,
		FileSize: fileSize,
		Timeout:  &timeoutSecs,
	}

	var resp api.Response[api.TransferCreateResponse]
	if err := apiClient.Post("/transfers", req, &resp); err != nil {
		return fmt.Errorf("creating transfer: %w", err)
	}

	code := resp.Data.Code
	fmt.Printf("\nShare this code with the recipient:\n\n")
	fmt.Printf("  \033[1m%s\033[0m\n\n", code)
	fmt.Printf("Waiting for receiver... (timeout: %s)\n", flagTransferTimeout)

	pollInterval := 2 * time.Second
	timeout := time.Duration(timeoutSecs) * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		var statusResp api.Response[api.TransferStatusResponse]
		if err := apiClient.Get("/transfers/"+code, nil, &statusResp); err != nil {
			return fmt.Errorf("checking status: %w", err)
		}

		status := statusResp.Data

		if status.Status == "receiver_connected" || status.Status == "active" {
			fmt.Println("\nReceiver connected! Starting transfer...")
			return uploadAndCompleteTransfer(code, localPath)
		}

		if status.Status == "cancelled" || status.Status == "expired" {
			return fmt.Errorf("transfer %s", status.Status)
		}

		time.Sleep(pollInterval)
	}

	apiClient.Delete("/transfers/"+code, nil)
	return fmt.Errorf("transfer timed out after %s", flagTransferTimeout)
}

func uploadAndCompleteTransfer(code, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	if err := apiClient.UploadTransferFile("/transfers/"+code+"/upload", file, info.Size()); err != nil {
		return fmt.Errorf("uploading: %w", err)
	}

	fmt.Println("Upload complete!")

	var completeResp api.Response[map[string]string]
	if err := apiClient.Post("/transfers/"+code+"/complete", nil, &completeResp); err != nil {
		return fmt.Errorf("completing transfer: %w", err)
	}

	return nil
}

func runTransferReceive(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	code := strings.ToUpper(args[0])

	fmt.Printf("Connecting to transfer %s...\n", code)

	var connectResp api.Response[map[string]string]
	if err := apiClient.Post("/transfers/"+code+"/connect", nil, &connectResp); err != nil {
		return fmt.Errorf("connecting: %w", err)
	}

	fileName := connectResp.Data["fileName"]
	fileSize, _ := strconv.ParseInt(connectResp.Data["fileSize"], 10, 64)

	fmt.Printf("Receiving: %s (%s)\n", fileName, output.FormatSize(fileSize))

	destDir := flagOutput
	if destDir == "." {
		cwd, _ := os.Getwd()
		destDir = cwd
	}

	destPath := filepath.Join(destDir, fileName)

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	if err := apiClient.DownloadTransferFile("/transfers/"+code+"/download", file); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("downloading: %w", err)
	}

	var completeResp api.Response[map[string]string]
	if err := apiClient.Post("/transfers/"+code+"/complete", nil, &completeResp); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not confirm completion: %v\n", err)
	}

	fmt.Printf("Received: %s → %s\n", fileName, destPath)
	return nil
}

func runTransferList(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	var resp api.Response[[]api.Transfer]
	if err := apiClient.Get("/transfers", nil, &resp); err != nil {
		return fmt.Errorf("listing transfers: %w", err)
	}

	if len(resp.Data) == 0 {
		fmt.Println("No pending transfers")
		return nil
	}

	fmt.Println("Pending transfers:")
	for _, t := range resp.Data {
		fmt.Printf("  %s  %s  (%s)  expires %s\n", t.Code, t.FileName, output.FormatSize(t.FileSize), t.ExpiresAt)
	}

	return nil
}

func runTransferCancel(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	code := strings.ToUpper(args[0])

	var resp api.Response[map[string]string]
	if err := apiClient.Delete("/transfers/"+code, &resp); err != nil {
		return fmt.Errorf("cancelling transfer: %w", err)
	}

	fmt.Printf("Transfer %s cancelled\n", code)
	return nil
}
