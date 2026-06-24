package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRunGetValuesWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/controlledVocab" {
			t.Fatalf("path = %q, want /controlledVocab", r.URL.Path)
		}
		if got := r.URL.Query().Get("field"); got != "instrument_platform" {
			t.Fatalf("field = %q, want instrument_platform", got)
		}
		_, _ = w.Write([]byte(`[{"value":"ILLUMINA","description":""},{"value":"OXFORD_NANOPORE","description":"Oxford Nanopore Technologies"}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_values", "instrument_platform"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "value\tdescription\n" +
		"ILLUMINA\t.\n" +
		"OXFORD_NANOPORE\tOxford Nanopore Technologies\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunGetValuesWritesJSON(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/controlledVocab" {
			t.Fatalf("path = %q, want /controlledVocab", r.URL.Path)
		}
		if got := r.URL.Query().Get("field"); got != "library_layout" {
			t.Fatalf("field = %q, want library_layout", got)
		}
		_, _ = w.Write([]byte(`[{"value":"PAIRED","description":""},{"value":"SINGLE","description":""}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_values", "library_layout", "--outfmt", "json"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var got []map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json output did not unmarshal: %v\n%s", err, stdout)
	}
	if len(got) != 2 || got[0]["value"] != "PAIRED" || got[1]["value"] != "SINGLE" {
		t.Fatalf("json output = %#v", got)
	}
}
