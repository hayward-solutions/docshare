package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	repo       = "hayward-solutions/docshare"
	binaryName = "docshare"
)

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

func GetLatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decoding release info: %w", err)
	}

	return release.TagName, nil
}

func DownloadAndInstall(version string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if goarch == "amd64" {
		goarch = "amd64"
	} else if goarch == "arm64" {
		goarch = "arm64"
	} else {
		return fmt.Errorf("unsupported architecture: %s", goarch)
	}

	if goos != "linux" && goos != "darwin" && goos != "windows" {
		return fmt.Errorf("unsupported OS: %s", goos)
	}

	versionStripped := strings.TrimPrefix(version, "v")
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	archive := fmt.Sprintf("%s_%s_%s_%s.%s", binaryName, versionStripped, goos, goarch, ext)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, archive)

	tmpDir, err := os.MkdirTemp("", "docshare-upgrade-")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpArchive := filepath.Join(tmpDir, archive)
	if err := downloadFile(url, tmpArchive); err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}

	var binaryPath string
	if ext == "zip" {
		binaryPath, err = extractZip(tmpArchive, tmpDir)
	} else {
		binaryPath, err = extractTarGz(tmpArchive, tmpDir)
	}
	if err != nil {
		return fmt.Errorf("extracting archive: %w", err)
	}

	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}

	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Errorf("making binary executable: %w", err)
	}

	backupPath := currentBinary + ".backup"
	if err := os.Rename(currentBinary, backupPath); err != nil {
		return fmt.Errorf("backing up current binary: %w", err)
	}

	if err := copyFile(binaryPath, currentBinary); err != nil {
		_ = os.Rename(backupPath, currentBinary)
		return fmt.Errorf("installing new binary: %w", err)
	}

	_ = os.Remove(backupPath)

	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGz(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var binaryPath string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag == tar.TypeReg {
			name := filepath.Base(header.Name)
			if name == binaryName || name == binaryName+".exe" {
				target := filepath.Join(destDir, name)
				outFile, err := os.Create(target)
				if err != nil {
					return "", err
				}
				if _, err := io.Copy(outFile, tr); err != nil {
					outFile.Close()
					return "", err
				}
				outFile.Close()
				binaryPath = target
				break
			}
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("binary not found in archive")
	}

	return binaryPath, nil
}

func extractZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var binaryPath string
	for _, f := range r.File {
		// Skip entries with potentially unsafe paths.
		if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
			continue
		}

		name := filepath.Base(f.Name)
		if name == binaryName || name == binaryName+".exe" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			target := filepath.Join(destDir, name)
			outFile, err := os.Create(target)
			if err != nil {
				rc.Close()
				return "", err
			}

			_, err = io.Copy(outFile, rc)
			rc.Close()
			outFile.Close()
			if err != nil {
				return "", err
			}

			binaryPath = target
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("binary not found in archive")
	}

	return binaryPath, nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}
