package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRunQueryWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "sample" {
			t.Fatalf("result = %q, want sample", got)
		}
		if got := query.Get("query"); got != "tax_tree(2)" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "sample_accession,scientific_name,tax_id" {
			t.Fatalf("fields = %q", got)
		}
		if got := query.Get("format"); got != "tsv" {
			t.Fatalf("format = %q, want tsv", got)
		}
		if got := query.Get("limit"); got != "2" {
			t.Fatalf("limit = %q, want 2", got)
		}
		_, _ = w.Write([]byte("sample_accession\tscientific_name\ttax_id\nSAMEA1\tEscherichia coli\t562\nSAMEA2\t\t573\n"))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"query", "--result", "sample", "--query", "tax_tree(2)", "--columns", "sample_accession,scientific_name,tax_id", "--limit", "2"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "sample_accession\tscientific_name\ttax_id\n" +
		"SAMEA1\tEscherichia coli\t562\n" +
		"SAMEA2\t.\t573\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunQueryStreamingTSVNormalizesEmbeddedNewline(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("format"); got != "tsv" {
			t.Fatalf("format = %q, want tsv", got)
		}
		if got := query.Get("fields"); got != "sample_accession,description" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte("sample_accession\tdescription\nSAMEA1\t\"project line one\nproject line two\"\n"))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"query", "--result", "sample", "--query", "tax_tree(2)", "--columns", "sample_accession,description"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "sample_accession\tdescription\n" +
		"SAMEA1\tproject line one project line two\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunQueryWritesJSON(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		if got := query.Get("query"); got != "tax_tree(2) AND instrument_platform=OXFORD_NANOPORE" {
			t.Fatalf("query = %q", got)
		}
		_, _ = w.Write([]byte(`[{"sample_accession":"SAMEA1","run_accession":"ERR1","instrument_platform":"OXFORD_NANOPORE"}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"query", "--result", "run", "--query", "tax_tree(2) AND instrument_platform=OXFORD_NANOPORE", "--columns", "sample_accession,run_accession,instrument_platform", "--outfmt", "json"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var got []map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json output did not unmarshal: %v\n%s", err, stdout)
	}
	if len(got) != 1 || got[0]["run_accession"] != "ERR1" || got[0]["source"] != "ena" {
		t.Fatalf("json output = %#v", got)
	}
}

func TestRunQueryCountWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/count" {
			t.Fatalf("path = %q, want /count", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		if got := query.Get("query"); got != "tax_tree(2)" {
			t.Fatalf("query = %q", got)
		}
		_, _ = w.Write([]byte(`{"count":"123"}`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"query", "--result", "run", "--query", "tax_tree(2)", "--count"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "result_type\tena_result\tquery\tcount\n" +
		"run\tread_run\ttax_tree(2)\t123\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
