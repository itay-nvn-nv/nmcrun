package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"nmcrun/internal/version"
)

type Updater struct {
	repoOwner string
	repoName  string
	client    *http.Client
}

type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
		Size        int64  `json:"size"`
	} `json:"assets"`
	PublishedAt time.Time `json:"published_at"`
}

// New creates a new updater instance
func New() *Updater {
	return &Updater{
		repoOwner: "itay-nvn-nv", // Your GitHub username
		repoName:  "nmcrun",      // Your repo name
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetRepository allows customizing the repository
func (u *Updater) SetRepository(owner, name string) {
	u.repoOwner = owner
	u.repoName = name
}

// CheckAndUpgrade checks for updates and upgrades if available
func (u *Updater) CheckAndUpgrade() error {
	fmt.Println("🔍 Checking for updates...")
	
	currentVersion := version.Get()
	fmt.Printf("Current version: %s\n", currentVersion)
	
	// Get latest release
	release, err := u.getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	
	if release == nil {
		fmt.Println("ℹ️  No releases found")
		return nil
	}
	
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	fmt.Printf("Latest version: %s\n", latestVersion)
	
	// Compare versions
	if currentVersion == latestVersion || currentVersion == "dev" {
		fmt.Println("✅ You are already running the latest version!")
		return nil
	}
	
	fmt.Printf("🆕 New version available: %s\n", latestVersion)
	fmt.Printf("Released: %s\n", release.PublishedAt.Format("2006-01-02 15:04:05"))
	
	if release.Body != "" {
		fmt.Printf("\nRelease notes:\n%s\n", release.Body)
	}
	
	// Find appropriate asset for current platform
	assetURL, assetName, err := u.findAssetForPlatform(release)
	if err != nil {
		return fmt.Errorf("no compatible binary found for your platform (%s/%s): %w", runtime.GOOS, runtime.GOARCH, err)
	}
	
	fmt.Printf("\n📥 Downloading %s...\n", assetName)
	
	// Download and install
	if err := u.downloadAndInstall(assetURL, assetName); err != nil {
		return fmt.Errorf("failed to download and install update: %w", err)
	}
	
	fmt.Printf("🎉 Successfully upgraded to version %s!\n", latestVersion)
	fmt.Println("💡 Please restart nmcrun to use the new version.")
	
	return nil
}

// getLatestRelease fetches the latest release from GitHub
func (u *Updater) getLatestRelease() (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", u.repoOwner, u.repoName)
	
	resp, err := u.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 404 {
		return nil, nil // No releases found
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	
	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	
	return &release, nil
}

// findAssetForPlatform finds the appropriate binary asset for the current platform
func (u *Updater) findAssetForPlatform(release *GitHubRelease) (string, string, error) {
	platform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	
	// Common platform mappings
	platformMappings := map[string][]string{
		"darwin_amd64": {"darwin_amd64", "macos_amd64", "mac_amd64"},
		"darwin_arm64": {"darwin_arm64", "macos_arm64", "mac_arm64", "macos_m1"},
		"linux_amd64":  {"linux_amd64", "linux_x86_64"},
		"linux_arm64":  {"linux_arm64", "linux_aarch64"},
		"windows_amd64": {"windows_amd64", "windows_x86_64", "win_amd64"},
	}
	
	possibleNames := platformMappings[platform]
	if possibleNames == nil {
		possibleNames = []string{platform}
	}
	
	for _, asset := range release.Assets {
		assetLower := strings.ToLower(asset.Name)
		
		for _, possibleName := range possibleNames {
			if strings.Contains(assetLower, possibleName) {
				return asset.DownloadURL, asset.Name, nil
			}
		}
	}
	
	return "", "", fmt.Errorf("no asset found for platform %s", platform)
}

// downloadAndInstall downloads the binary and replaces the current executable
func (u *Updater) downloadAndInstall(url, assetName string) error {
	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}
	
	// Create temporary file
	tempFile, err := os.CreateTemp("", "nmcrun_update_*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())
	
	// Download file
	resp, err := u.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	
	// Extract binary from archive if needed
	var binaryReader io.Reader = resp.Body
	
	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		binaryReader, err = u.extractBinaryFromTarGz(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to extract binary from archive: %w", err)
		}
	} else if strings.HasSuffix(assetName, ".gz") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to decompress gzip: %w", err)
		}
		defer gzReader.Close()
		binaryReader = gzReader
	}
	
	// Copy to temp file
	if _, err := io.Copy(tempFile, binaryReader); err != nil {
		return fmt.Errorf("failed to write downloaded file: %w", err)
	}
	
	// Make executable
	if err := os.Chmod(tempFile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to make file executable: %w", err)
	}
	
	// Replace current executable
	if err := u.replaceExecutable(currentExe, tempFile.Name()); err != nil {
		return fmt.Errorf("failed to replace executable: %w", err)
	}
	
	return nil
}

// extractBinaryFromTarGz extracts the binary from a tar.gz archive
func (u *Updater) extractBinaryFromTarGz(reader io.Reader) (io.Reader, error) {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()
	
	tarReader := tar.NewReader(gzReader)
	
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		// Look for the binary file (usually named nmcrun or similar)
		if strings.Contains(header.Name, "nmcrun") && !strings.Contains(header.Name, ".") {
			// Create a buffer to hold the binary content
			var buf strings.Builder
			if _, err := io.Copy(&buf, tarReader); err != nil {
				return nil, err
			}
			return strings.NewReader(buf.String()), nil
		}
	}
	
	return nil, fmt.Errorf("binary not found in archive")
}

// replaceExecutable replaces the current executable with the new one
func (u *Updater) replaceExecutable(currentPath, newPath string) error {
	// On Windows, we can't replace a running executable directly
	if runtime.GOOS == "windows" {
		backupPath := currentPath + ".old"
		
		// Move current executable to backup
		if err := os.Rename(currentPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup current executable: %w", err)
		}
		
		// Move new executable to current location
		if err := os.Rename(newPath, currentPath); err != nil {
			// Try to restore backup
			os.Rename(backupPath, currentPath)
			return fmt.Errorf("failed to move new executable: %w", err)
		}
		
		// Schedule backup deletion (best effort)
		go func() {
			time.Sleep(1 * time.Second)
			os.Remove(backupPath)
		}()
		
		return nil
	}
	
	// On Unix-like systems, we can replace the file directly
	// Copy the new file over the current executable
	sourceFile, err := os.Open(newPath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	destFile, err := os.OpenFile(currentPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	
	return nil
}

// CheckVersion checks if a new version is available without upgrading
func (u *Updater) CheckVersion() (*GitHubRelease, bool, error) {
	release, err := u.getLatestRelease()
	if err != nil {
		return nil, false, err
	}
	
	if release == nil {
		return nil, false, nil
	}
	
	currentVersion := version.Get()
	latestVersion := strings.TrimPrefix(release.TagName, "v")
	
	updateAvailable := currentVersion != latestVersion && currentVersion != "dev"
	
	return release, updateAvailable, nil
} 