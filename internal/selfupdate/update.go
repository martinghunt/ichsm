package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.github.com"
	defaultRepoPath   = "martinghunt/ichsm"
	defaultBinaryName = "ichsm"
	defaultHTTPTime   = 60 * time.Second
)

// Options controls a self-update check or install.
type Options struct {
	// CurrentVersion is the version of the running binary. Release versions may
	// include or omit the leading "v".
	CurrentVersion string
	// BinaryPath is the executable to replace. Empty means the current process
	// executable.
	BinaryPath string
	// CheckOnly reports availability without downloading or replacing the binary.
	CheckOnly bool
	// Force installs the latest release even when CurrentVersion is not older or
	// cannot be compared.
	Force bool
	// APIBaseURL overrides the GitHub API base URL for tests.
	APIBaseURL string
	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client
	// Token is sent as a GitHub bearer token when set.
	Token string
	// GOOS and GOARCH override runtime target selection for tests.
	GOOS   string
	GOARCH string
}

// Result describes the outcome of an update check.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	AssetName      string
	BinaryPath     string
	Updated        bool
	UpToDate       bool
	CheckOnly      bool
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Update checks GitHub releases and replaces the current binary when a newer
// release exists.
func Update(ctx context.Context, opts Options) (Result, error) {
	opts = normalizeOptions(opts)
	release, err := latestRelease(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	if release.TagName == "" {
		return Result{}, fmt.Errorf("latest release has no tag name")
	}

	result := Result{
		CurrentVersion: opts.CurrentVersion,
		LatestVersion:  release.TagName,
		CheckOnly:      opts.CheckOnly,
	}

	currentVersion, currentOK := parseReleaseVersion(opts.CurrentVersion)
	latestVersion, latestOK := parseReleaseVersion(release.TagName)
	if !latestOK {
		return result, fmt.Errorf("latest release tag %q is not a semantic version", release.TagName)
	}

	if currentOK && compareReleaseVersions(currentVersion, latestVersion) >= 0 && !opts.Force {
		result.UpToDate = true
		return result, nil
	}
	if !currentOK && !opts.Force && !opts.CheckOnly {
		return result, fmt.Errorf("current version %q is not a release version; use --force to install %s", opts.CurrentVersion, displayVersion(release.TagName))
	}

	asset, err := selectReleaseAsset(release.Assets, release.TagName, opts.GOOS, opts.GOARCH)
	if err != nil {
		return result, err
	}
	result.AssetName = asset.Name
	if opts.CheckOnly {
		return result, nil
	}

	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		binaryPath, err = executablePath()
		if err != nil {
			return result, err
		}
	}
	result.BinaryPath = binaryPath

	targetDir := filepath.Dir(binaryPath)
	archivePath, archiveSum, err := downloadAsset(ctx, opts, asset, targetDir, ".ichsm-update-archive-*")
	if err != nil {
		return result, err
	}
	defer os.Remove(archivePath)

	checksumAsset, err := selectChecksumAsset(release.Assets, release.TagName)
	if err != nil {
		return result, err
	}
	if err := verifyChecksum(ctx, opts, checksumAsset, asset.Name, archiveSum); err != nil {
		return result, err
	}

	newBinary, err := extractBinary(archivePath, asset.Name, targetDir)
	if err != nil {
		return result, err
	}
	if err := chmodLikeCurrent(newBinary, binaryPath); err != nil {
		_ = os.Remove(newBinary)
		return result, err
	}
	if err := os.Rename(newBinary, binaryPath); err != nil {
		_ = os.Remove(newBinary)
		return result, fmt.Errorf("replace %s: %w", binaryPath, err)
	}

	result.Updated = true
	return result, nil
}

func normalizeOptions(opts Options) Options {
	if opts.CurrentVersion == "" {
		opts.CurrentVersion = "dev"
	}
	if opts.APIBaseURL == "" {
		opts.APIBaseURL = defaultAPIBaseURL
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: defaultHTTPTime}
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}
	return opts
}

func latestRelease(ctx context.Context, opts Options) (githubRelease, error) {
	url := strings.TrimRight(opts.APIBaseURL, "/") + "/repos/" + defaultRepoPath + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return githubRelease{}, err
	}
	setGitHubHeaders(req, opts.Token)

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return githubRelease{}, fmt.Errorf("check latest release: %s", resp.Status)
	}

	var release githubRelease
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode latest release: %w", err)
	}
	return release, nil
}

func setGitHubHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ichsm")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func selectReleaseAsset(assets []githubAsset, tag, goos, goarch string) (githubAsset, error) {
	binary := releaseBinaryName(tag, goos, goarch)
	candidates := []string{binary}
	switch goos {
	case "windows":
		candidates = append([]string{binary + ".zip"}, candidates...)
	case "darwin", "linux":
		candidates = append([]string{binary + ".tar.gz"}, candidates...)
	}

	for _, name := range candidates {
		for _, asset := range assets {
			if asset.Name == name && asset.BrowserDownloadURL != "" {
				return asset, nil
			}
		}
	}
	return githubAsset{}, fmt.Errorf("latest release %s has no ichsm asset for %s/%s", tag, goos, goarch)
}

func selectChecksumAsset(assets []githubAsset, tag string) (githubAsset, error) {
	name := fmt.Sprintf("%s-%s-checksums.txt", defaultBinaryName, tag)
	for _, asset := range assets {
		if asset.Name == name && asset.BrowserDownloadURL != "" {
			return asset, nil
		}
	}
	return githubAsset{}, fmt.Errorf("latest release %s has no checksum asset %s", tag, name)
}

func releaseBinaryName(tag, goos, goarch string) string {
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("%s-%s-%s-%s%s", defaultBinaryName, tag, goos, goarch, ext)
}

func downloadAsset(ctx context.Context, opts Options, asset githubAsset, dir, pattern string) (string, string, error) {
	out, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", "", fmt.Errorf("create update temp file: %w", err)
	}
	path := out.Name()
	remove := true
	defer func() {
		if remove {
			_ = out.Close()
			_ = os.Remove(path)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", "", err
	}
	setGitHubHeaders(req, opts.Token)

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", "", fmt.Errorf("download %s: %s", asset.Name, resp.Status)
	}

	hasher := sha256.New()
	if _, err := copyHashing(out, resp.Body, hasher); err != nil {
		return "", "", fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if err := out.Close(); err != nil {
		return "", "", fmt.Errorf("write %s: %w", path, err)
	}
	remove = false
	return path, hex.EncodeToString(hasher.Sum(nil)), nil
}

func copyHashing(dst io.Writer, src io.Reader, h hash.Hash) (int64, error) {
	return io.Copy(io.MultiWriter(dst, h), src)
}

func verifyChecksum(ctx context.Context, opts Options, checksumAsset githubAsset, assetName, got string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumAsset.BrowserDownloadURL, nil)
	if err != nil {
		return err
	}
	setGitHubHeaders(req, opts.Token)

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", checksumAsset.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download %s: %s", checksumAsset.Name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read %s: %w", checksumAsset.Name, err)
	}
	want, ok := checksumForAsset(string(data), assetName)
	if !ok {
		return fmt.Errorf("%s has no checksum for %s", checksumAsset.Name, assetName)
	}
	if !strings.EqualFold(want, got) {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", assetName, got, want)
	}
	return nil
}

func checksumForAsset(data, assetName string) (string, bool) {
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == assetName && isSHA256(fields[0]) {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

func isSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func extractBinary(archivePath, assetName, dir string) (string, error) {
	targetName := binaryNameInAsset(assetName)
	out, err := os.CreateTemp(dir, ".ichsm-update-bin-*")
	if err != nil {
		return "", fmt.Errorf("create update binary temp file: %w", err)
	}
	outPath := out.Name()
	remove := true
	defer func() {
		if remove {
			_ = out.Close()
			_ = os.Remove(outPath)
		}
	}()

	var extractErr error
	switch {
	case strings.HasSuffix(assetName, ".tar.gz"):
		extractErr = extractTarGzipBinary(archivePath, targetName, out)
	case strings.HasSuffix(assetName, ".zip"):
		extractErr = extractZipBinary(archivePath, targetName, out)
	default:
		extractErr = copyFileToWriter(archivePath, out)
	}
	if extractErr != nil {
		return "", extractErr
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("write %s: %w", outPath, err)
	}
	remove = false
	return outPath, nil
}

func binaryNameInAsset(assetName string) string {
	name := assetName
	name = strings.TrimSuffix(name, ".tar.gz")
	name = strings.TrimSuffix(name, ".zip")
	return name
}

func extractTarGzipBinary(archivePath, targetName string, dst io.Writer) error {
	in, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if hdr.FileInfo().Mode().IsRegular() && filepath.Base(hdr.Name) == targetName {
			if _, err := io.Copy(dst, tr); err != nil {
				return fmt.Errorf("extract %s: %w", targetName, err)
			}
			return nil
		}
	}
	return fmt.Errorf("archive does not contain %s", targetName)
}

func extractZipBinary(archivePath, targetName string, dst io.Writer) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer zr.Close()
	for _, file := range zr.File {
		if !file.FileInfo().Mode().IsRegular() || filepath.Base(file.Name) != targetName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open %s in zip archive: %w", targetName, err)
		}
		_, copyErr := io.Copy(dst, rc)
		closeErr := rc.Close()
		if copyErr != nil {
			return fmt.Errorf("extract %s: %w", targetName, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("extract %s: %w", targetName, closeErr)
		}
		return nil
	}
	return fmt.Errorf("archive does not contain %s", targetName)
}

func copyFileToWriter(path string, dst io.Writer) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err := io.Copy(dst, in); err != nil {
		return err
	}
	return nil
}

func chmodLikeCurrent(newPath, currentPath string) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(currentPath); err == nil {
		mode = info.Mode().Perm()
		if mode&0o111 == 0 {
			mode |= 0o111
		}
	}
	if err := os.Chmod(newPath, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", newPath, err)
	}
	return nil
}

func executablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

type releaseVersion struct {
	Major int
	Minor int
	Patch int
}

func parseReleaseVersion(s string) (releaseVersion, bool) {
	s = strings.TrimSpace(s)
	if len(s) > 1 && (s[0] == 'v' || s[0] == 'V') {
		s = s[1:]
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return releaseVersion{}, false
	}
	values := make([]int, 3)
	for i, part := range parts {
		if part == "" {
			return releaseVersion{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return releaseVersion{}, false
		}
		values[i] = n
	}
	return releaseVersion{Major: values[0], Minor: values[1], Patch: values[2]}, true
}

func compareReleaseVersions(a, b releaseVersion) int {
	switch {
	case a.Major != b.Major:
		return compareInt(a.Major, b.Major)
	case a.Minor != b.Minor:
		return compareInt(a.Minor, b.Minor)
	default:
		return compareInt(a.Patch, b.Patch)
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func displayVersion(raw string) string {
	if len(raw) > 1 && (raw[0] == 'v' || raw[0] == 'V') && raw[1] >= '0' && raw[1] <= '9' {
		return raw[1:]
	}
	return raw
}
