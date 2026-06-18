package main

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/martinghunt/ichsm"
)

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
