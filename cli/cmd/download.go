package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var flagOutput string

var downloadCmd = &cobra.Command{
	Use:   "download <remote-path> [local-dir]",
	Short: "Download a file or directory",
	Long: `Download a file or directory from DocShare to your local machine.

  docshare download /Documents/report.pdf          Download to current directory
  docshare download /Documents/report.pdf ./out     Download to specific directory
  docshare download /Projects                       Download directory recursively
  docshare download <uuid>                          Download by file ID`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDownload,
}

func init() {
	downloadCmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output file path (overrides default naming)")
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	fileID, err := pathutil.Resolve(apiClient, args[0])
	if err != nil {
		return err
	}
	if fileID == "" {
		return fmt.Errorf("cannot download root — specify a file or folder path")
	}

	// Determine destination directory.
	destDir := "."
	if len(args) > 1 {
		destDir = args[1]
	}

	// Get file metadata to determine if it's a file or directory.
	var resp api.Response[api.File]
	if err := apiClient.Get("/files/"+fileID, nil, &resp); err != nil {
		return fmt.Errorf("fetching file info: %w", err)
	}

	f := resp.Data

	if f.IsDirectory {
		return downloadDirectory(f, destDir)
	}
	return downloadFile(f, destDir)
}

func downloadFile(f api.File, destDir string) error {
	// Get presigned download URL for efficiency.
	var dlResp api.Response[api.DownloadURLResponse]
	if err := apiClient.Get("/files/"+f.ID+"/download-url", nil, &dlResp); err != nil {
		return fmt.Errorf("getting download URL: %w", err)
	}

	dest := filepath.Join(destDir, f.Name)
	if flagOutput != "" {
		dest = flagOutput
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := apiClient.DownloadToFile(dlResp.Data.URL, dest); err != nil {
		return fmt.Errorf("downloading: %w", err)
	}

	fmt.Printf("Downloaded %s → %s\n", f.Name, dest)
	return nil
}

func downloadDirectory(f api.File, destDir string) error {
	localDir := filepath.Join(destDir, f.Name)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	fmt.Printf("Created directory: %s\n", localDir)

	// List children and download recursively.
	var resp api.Response[[]api.File]
	if err := apiClient.Get("/files/"+f.ID+"/children", nil, &resp); err != nil {
		return fmt.Errorf("listing children: %w", err)
	}

	for _, child := range resp.Data {
		if child.IsDirectory {
			if err := downloadDirectory(child, localDir); err != nil {
				fmt.Fprintf(os.Stderr, "  Failed: %s — %v\n", child.Name, err)
			}
		} else {
			if err := downloadFile(child, localDir); err != nil {
				fmt.Fprintf(os.Stderr, "  Failed: %s — %v\n", child.Name, err)
			}
		}
	}

	return nil
}
