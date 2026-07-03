package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateInstallsNewerRelease(t *testing.T) {
	archiveName := "ichsm-v1.2.0-linux-amd64.tar.gz"
	archiveData := tarGzipFile(t, "ichsm-v1.2.0-linux-amd64", []byte("new-binary\n"))
	client := releaseClient(t, "v1.2.0", map[string][]byte{
		archiveName: archiveData,
		"ichsm-v1.2.0-checksums.txt": []byte(fmt.Sprintf("%s  %s\n",
			sha256Hex(archiveData), archiveName)),
	}, nil)

	binaryPath := filepath.Join(t.TempDir(), "ichsm")
	if err := os.WriteFile(binaryPath, []byte("old-binary\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(current binary) error = %v", err)
	}

	result, err := Update(context.Background(), Options{
		CurrentVersion: "v1.1.0",
		BinaryPath:     binaryPath,
		APIBaseURL:     "https://example.test",
		HTTPClient:     client,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !result.Updated || result.UpToDate {
		t.Fatalf("result = %+v, want updated result", result)
	}
	if result.AssetName != archiveName {
		t.Fatalf("asset name = %q, want %q", result.AssetName, archiveName)
	}
	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile(updated binary) error = %v", err)
	}
	if string(got) != "new-binary\n" {
		t.Fatalf("updated binary = %q, want %q", string(got), "new-binary\n")
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("Stat(updated binary) error = %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("updated binary mode = %v, want executable bit set", info.Mode().Perm())
	}
}

func TestUpdateCheckOnlyDoesNotDownloadOrReplace(t *testing.T) {
	archiveName := "ichsm-v1.2.0-linux-amd64.tar.gz"
	downloads := 0
	client := releaseClient(t, "v1.2.0", map[string][]byte{
		archiveName: []byte("not requested"),
	}, func(name string) {
		if name == archiveName {
			downloads++
		}
	})

	binaryPath := filepath.Join(t.TempDir(), "ichsm")
	if err := os.WriteFile(binaryPath, []byte("old-binary\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(current binary) error = %v", err)
	}

	result, err := Update(context.Background(), Options{
		CurrentVersion: "v1.1.0",
		BinaryPath:     binaryPath,
		CheckOnly:      true,
		APIBaseURL:     "https://example.test",
		HTTPClient:     client,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !result.CheckOnly || result.Updated || result.UpToDate {
		t.Fatalf("result = %+v, want check-only availability result", result)
	}
	if downloads != 0 {
		t.Fatalf("asset downloads = %d, want 0", downloads)
	}
	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile(current binary) error = %v", err)
	}
	if string(got) != "old-binary\n" {
		t.Fatalf("current binary = %q, want unchanged old binary", string(got))
	}
}

func TestUpdateReturnsUpToDateBeforeAssetSelection(t *testing.T) {
	client := releaseClient(t, "v1.2.0", nil, nil)

	result, err := Update(context.Background(), Options{
		CurrentVersion: "v1.2.0",
		APIBaseURL:     "https://example.test",
		HTTPClient:     client,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !result.UpToDate || result.Updated {
		t.Fatalf("result = %+v, want up-to-date result", result)
	}
}

func TestUpdateRejectsDevVersionWithoutForce(t *testing.T) {
	client := releaseClient(t, "v1.2.0", nil, nil)

	_, err := Update(context.Background(), Options{
		CurrentVersion: "dev",
		APIBaseURL:     "https://example.test",
		HTTPClient:     client,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err == nil || !strings.Contains(err.Error(), `current version "dev" is not a release version`) {
		t.Fatalf("Update() error = %v, want dev version error", err)
	}
}

func TestUpdateRejectsChecksumMismatch(t *testing.T) {
	archiveName := "ichsm-v1.2.0-linux-amd64.tar.gz"
	archiveData := tarGzipFile(t, "ichsm-v1.2.0-linux-amd64", []byte("new-binary\n"))
	client := releaseClient(t, "v1.2.0", map[string][]byte{
		archiveName:                  archiveData,
		"ichsm-v1.2.0-checksums.txt": []byte(strings.Repeat("0", 64) + "  " + archiveName + "\n"),
	}, nil)

	binaryPath := filepath.Join(t.TempDir(), "ichsm")
	if err := os.WriteFile(binaryPath, []byte("old-binary\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(current binary) error = %v", err)
	}

	_, err := Update(context.Background(), Options{
		CurrentVersion: "v1.1.0",
		BinaryPath:     binaryPath,
		APIBaseURL:     "https://example.test",
		HTTPClient:     client,
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Update() error = %v, want checksum mismatch", err)
	}
	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile(current binary) error = %v", err)
	}
	if string(got) != "old-binary\n" {
		t.Fatalf("current binary = %q, want unchanged old binary", string(got))
	}
}

func TestExtractBinaryFromZipAsset(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "release.zip")
	zipFile(t, archivePath, "ichsm-v1.2.0-windows-amd64.exe", []byte("windows-binary\n"))

	gotPath, err := extractBinary(archivePath, "ichsm-v1.2.0-windows-amd64.exe.zip", dir)
	if err != nil {
		t.Fatalf("extractBinary() error = %v", err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("ReadFile(extracted binary) error = %v", err)
	}
	if string(got) != "windows-binary\n" {
		t.Fatalf("extracted binary = %q, want %q", string(got), "windows-binary\n")
	}
}

func TestSelectReleaseAsset(t *testing.T) {
	assets := []githubAsset{
		{Name: "ichsm-v1.2.0-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.test/linux"},
		{Name: "ichsm-v1.2.0-darwin-arm64.tar.gz", BrowserDownloadURL: "https://example.test/darwin"},
		{Name: "ichsm-v1.2.0-windows-amd64.exe.zip", BrowserDownloadURL: "https://example.test/windows"},
	}
	tests := []struct {
		name   string
		goos   string
		goarch string
		want   string
	}{
		{name: "linux", goos: "linux", goarch: "amd64", want: "ichsm-v1.2.0-linux-amd64.tar.gz"},
		{name: "darwin", goos: "darwin", goarch: "arm64", want: "ichsm-v1.2.0-darwin-arm64.tar.gz"},
		{name: "windows", goos: "windows", goarch: "amd64", want: "ichsm-v1.2.0-windows-amd64.exe.zip"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectReleaseAsset(assets, "v1.2.0", tt.goos, tt.goarch)
			if err != nil {
				t.Fatalf("selectReleaseAsset() error = %v", err)
			}
			if got.Name != tt.want {
				t.Fatalf("asset = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestParseAndCompareReleaseVersions(t *testing.T) {
	current, ok := parseReleaseVersion("v1.2.3")
	if !ok {
		t.Fatal("parseReleaseVersion(v1.2.3) failed")
	}
	older, ok := parseReleaseVersion("1.2.2")
	if !ok {
		t.Fatal("parseReleaseVersion(1.2.2) failed")
	}
	newer, ok := parseReleaseVersion("v1.3.0")
	if !ok {
		t.Fatal("parseReleaseVersion(v1.3.0) failed")
	}
	if compareReleaseVersions(current, older) <= 0 {
		t.Fatal("v1.2.3 should compare greater than 1.2.2")
	}
	if compareReleaseVersions(current, newer) >= 0 {
		t.Fatal("v1.2.3 should compare less than v1.3.0")
	}
	if _, ok := parseReleaseVersion("dev"); ok {
		t.Fatal("parseReleaseVersion(dev) succeeded, want failure")
	}
}

func releaseClient(t *testing.T, tag string, assets map[string][]byte, assetHook func(name string)) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/martinghunt/ichsm/releases/latest":
			var releaseAssets []githubAsset
			for name := range assets {
				releaseAssets = append(releaseAssets, githubAsset{
					Name:               name,
					BrowserDownloadURL: "https://example.test/assets/" + name,
				})
			}
			var body bytes.Buffer
			if err := json.NewEncoder(&body).Encode(githubRelease{
				TagName: tag,
				Assets:  releaseAssets,
			}); err != nil {
				t.Fatalf("Encode(release) error = %v", err)
			}
			return testHTTPResponse(http.StatusOK, body.Bytes()), nil
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			name := strings.TrimPrefix(r.URL.Path, "/assets/")
			if assetHook != nil {
				assetHook(name)
			}
			data, ok := assets[name]
			if !ok {
				return testHTTPResponse(http.StatusNotFound, nil), nil
			}
			return testHTTPResponse(http.StatusOK, data), nil
		default:
			return testHTTPResponse(http.StatusNotFound, nil), nil
		}
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testHTTPResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func tarGzipFile(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(data)),
	}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("Write(tar data) error = %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close(tar) error = %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("Close(gzip) error = %v", err)
	}
	return buf.Bytes()
}

func zipFile(t *testing.T, path, name string, data []byte) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(zip) error = %v", err)
	}
	zw := zip.NewWriter(out)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create(zip entry) error = %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write(zip entry) error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close(zip writer) error = %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close(zip file) error = %v", err)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
