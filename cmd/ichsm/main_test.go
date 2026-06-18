package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/martinghunt/ichsm"
)

func withHTTPTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func withPathResponseServer(t *testing.T, path string, body string) *httptest.Server {
	t.Helper()

	return withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Fatalf("path = %q, want %s", r.URL.Path, path)
		}
		_, _ = w.Write([]byte(body))
	})
}

func withTestClient(t *testing.T, server *httptest.Server) {
	t.Helper()

	previous := newClient
	newClient = func() *ichsm.Client {
		return &ichsm.Client{
			BaseURL:               server.URL,
			BrowserBaseURL:        server.URL,
			NCBIBaseURL:           server.URL,
			HTTPClient:            server.Client(),
			ENARequestsPerSecond:  -1,
			NCBIRequestsPerSecond: -1,
			MaxRequestRetries:     -1,
		}
	}
	t.Cleanup(func() {
		newClient = previous
	})
}

func withTestBrowserOpener(t *testing.T, opener func(string) error) {
	t.Helper()

	previous := openBrowser
	openBrowser = opener
	t.Cleanup(func() {
		openBrowser = previous
	})
}

func captureStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()

	oldStdout := os.Stdout
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writeEnd
	defer func() {
		os.Stdout = oldStdout
	}()

	code := fn()

	if err := writeEnd.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if _, err := io.Copy(&stdout, readEnd); err != nil {
		t.Fatal(err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatal(err)
	}

	return code, stdout.String()
}
