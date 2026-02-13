package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/docshare/cli/internal/api"
	"github.com/docshare/cli/internal/output"
	"github.com/docshare/cli/internal/pathutil"
	"github.com/spf13/cobra"
)

var (
	flagParent  string
	flagWorkers int
)

var uploadCmd = &cobra.Command{
	Use:   "upload <path> [remote-parent]",
	Short: "Upload a file or directory",
	Long: `Upload a local file or directory to DocShare.

  docshare upload report.pdf                     Upload to root
  docshare upload report.pdf /Documents           Upload to a folder
  docshare upload ./project/ /Documents           Upload directory recursively
  docshare upload report.pdf --parent <uuid>      Upload to folder by ID`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runUpload,
}

func init() {
	uploadCmd.Flags().StringVar(&flagParent, "parent", "", "Parent folder ID (alternative to positional arg)")
	uploadCmd.Flags().IntVarP(&flagWorkers, "workers", "w", 4, "Number of concurrent upload workers (for directories)")
	rootCmd.AddCommand(uploadCmd)
}

func runUpload(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	localPath := args[0]

	// Determine parent folder ID.
	parentID := flagParent
	if len(args) > 1 && parentID == "" {
		resolved, err := pathutil.Resolve(apiClient, args[1])
		if err != nil {
			return fmt.Errorf("resolving remote parent: %w", err)
		}
		parentID = resolved
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", localPath, err)
	}

	if !info.IsDir() {
		return uploadSingleFile(localPath, parentID)
	}

	return uploadDirectory(localPath, parentID)
}

func uploadSingleFile(path, parentID string) error {
	extra := map[string]string{}
	if parentID != "" {
		extra["parentID"] = parentID
	}

	var resp api.Response[api.File]
	if err := apiClient.Upload("/files/upload", "file", path, extra, &resp); err != nil {
		return fmt.Errorf("uploading %s: %w", filepath.Base(path), err)
	}

	if flagJSON {
		output.JSON(resp.Data)
		return nil
	}

	fmt.Printf("Uploaded %s (%s)\n", resp.Data.Name, output.FormatSize(resp.Data.Size))
	return nil
}

type uploadJob struct {
	localPath string
	parentID  string
}

func uploadDirectory(dirPath, parentID string) error {
	// Create the top-level directory on the server.
	dirName := filepath.Base(dirPath)
	topDirID, err := createRemoteDir(dirName, parentID)
	if err != nil {
		return fmt.Errorf("creating remote directory %s: %w", dirName, err)
	}
	fmt.Printf("Created directory: %s\n", dirName)

	// Walk the local directory tree and collect files + create remote directories.
	jobs := make(chan uploadJob, 64)
	var walkErr error

	var uploaded atomic.Int64
	var failed atomic.Int64

	// Producer: walk the tree, create directories, enqueue files.
	go func() {
		defer close(jobs)
		walkErr = walkTree(dirPath, topDirID, jobs)
	}()

	// Consumer: worker pool uploads files concurrently.
	workers := flagWorkers
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				extra := map[string]string{}
				if job.parentID != "" {
					extra["parentID"] = job.parentID
				}
				var resp api.Response[api.File]
				if err := apiClient.Upload("/files/upload", "file", job.localPath, extra, &resp); err != nil {
					fmt.Fprintf(os.Stderr, "  Failed: %s — %v\n", filepath.Base(job.localPath), err)
					failed.Add(1)
				} else {
					fmt.Printf("  Uploaded: %s (%s)\n", resp.Data.Name, output.FormatSize(resp.Data.Size))
					uploaded.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	if walkErr != nil {
		return fmt.Errorf("walking directory: %w", walkErr)
	}

	fmt.Printf("\nDone: %d uploaded, %d failed\n", uploaded.Load(), failed.Load())
	if failed.Load() > 0 {
		return fmt.Errorf("%d file(s) failed to upload", failed.Load())
	}
	return nil
}

// walkTree recursively walks a local directory, creating remote directories and enqueuing file uploads.
func walkTree(localDir, remoteParentID string, jobs chan<- uploadJob) error {
	entries, err := os.ReadDir(localDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		localPath := filepath.Join(localDir, entry.Name())

		if entry.IsDir() {
			// Create remote directory and recurse.
			childID, err := createRemoteDir(entry.Name(), remoteParentID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Failed to create directory: %s — %v\n", entry.Name(), err)
				continue
			}
			fmt.Printf("  Created directory: %s\n", entry.Name())
			if err := walkTree(localPath, childID, jobs); err != nil {
				return err
			}
		} else {
			jobs <- uploadJob{localPath: localPath, parentID: remoteParentID}
		}
	}
	return nil
}

func createRemoteDir(name, parentID string) (string, error) {
	body := map[string]interface{}{"name": name}
	if parentID != "" {
		body["parentID"] = parentID
	}
	var resp api.Response[api.File]
	if err := apiClient.Post("/files/directory", body, &resp); err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}
