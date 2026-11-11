package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func runUpdate(args []string) {
	fmt.Fprintln(os.Stderr, "Checking for updates...")

	client := &http.Client{Timeout: 30 * time.Second}
	rel, err := fetchLatestRelease(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(version, "v")

	if current == latest {
		fmt.Printf("Already up to date (v%s)\n", current)
		return
	}
	if current != "dev" {
		fmt.Printf("New version available: v%s → v%s\n", current, latest)
	} else {
		fmt.Printf("Installing release v%s\n", latest)
	}

	// Find the right asset for this platform.
	assetName := platformAssetName()
	var assetURL, checksumURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			assetURL = a.BrowserDownloadURL
		case "checksums.txt":
			checksumURL = a.BrowserDownloadURL
		}
	}
	if assetURL == "" {
		fmt.Fprintf(os.Stderr, "error: no release asset found for %s/%s (looking for %q)\n",
			runtime.GOOS, runtime.GOARCH, assetName)
		os.Exit(1)
	}

	// Download archive.
	fmt.Fprintf(os.Stderr, "Downloading %s...\n", assetName)
	archiveData, err := downloadBytes(client, assetURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: download archive: %v\n", err)
		os.Exit(1)
	}

	// Verify checksum if available.
	if checksumURL != "" {
		checksumData, err := downloadBytes(client, checksumURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not download checksums.txt: %v\n", err)
		} else {
			if err := verifyChecksum(archiveData, assetName, checksumData); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "Checksum verified.")
		}
	}

	// Extract binary from archive.
	binaryName := "poixe"
	if runtime.GOOS == "windows" {
		binaryName = "poixe.exe"
	}
	newBinary, err := extractBinary(archiveData, assetName, binaryName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: extract binary: %v\n", err)
		os.Exit(1)
	}

	// Replace current executable.
	if err := replaceSelf(newBinary, binaryName); err != nil {
		fmt.Fprintf(os.Stderr, "error: replace binary: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to v%s\n", latest)
}

func platformAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goos == "windows" {
		return fmt.Sprintf("poixe_%s_%s.zip", goos, goarch)
	}
	return fmt.Sprintf("poixe_%s_%s.tar.gz", goos, goarch)
}

func fetchLatestRelease(client *http.Client) (*ghRelease, error) {
	req, _ := http.NewRequest(http.MethodGet,
		"https://api.github.com/repos/poixeai/poixe-security/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "poixe-cli/"+version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no releases found")
	}
	return &rel, nil
}

func downloadBytes(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func verifyChecksum(data []byte, filename string, checksumFile []byte) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])

	for _, line := range strings.Split(string(checksumFile), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimPrefix(parts[1], "*")
		if name == filename {
			if parts[0] != got {
				return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filename, parts[0], got)
			}
			return nil
		}
	}
	return fmt.Errorf("checksum for %q not found in checksums.txt", filename)
}

func extractBinary(archiveData []byte, assetName, binaryName string) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZip(archiveData, binaryName)
	}
	return extractFromTarGz(archiveData, binaryName)
}

func extractFromTarGz(data []byte, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(hdr.Name, "./") == binaryName || hdr.Name == binaryName {
			return io.ReadAll(io.LimitReader(tr, 32<<20))
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", binaryName)
}

func extractFromZip(data []byte, binaryName string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, "./")
		if name == binaryName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 32<<20))
		}
	}
	return nil, fmt.Errorf("binary %q not found in zip", binaryName)
}

func replaceSelf(newBinary []byte, binaryName string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}

	// Write to a temp file in the same directory (for atomic rename).
	dir := dirOf(exe)
	tmp, err := os.CreateTemp(dir, ".poixe-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return fmt.Errorf("write new binary: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpName, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	if runtime.GOOS == "windows" {
		// On Windows, rename current binary out of the way first.
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return fmt.Errorf("rename current binary: %w", err)
		}
		fmt.Fprintf(os.Stderr, "note: old binary saved as %s — you may delete it\n", old)
	}

	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("install new binary: %w", err)
	}
	return nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
