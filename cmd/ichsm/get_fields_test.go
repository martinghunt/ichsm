package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunGetFieldsListsDataTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/results" {
			t.Fatalf("path = %q, want /results", r.URL.Path)
		}
		_, _ = w.Write([]byte("resultId\tdescription\tprimaryAccessionType\ntls_set\tTargeted locus study contig sets\taccession\nsample\tSamples\tsample_accession\nanalysis\tAnalyses\tanalysis_accession\nwgs_set\tGenome assembly contig set (WGS)\taccession\nread_run\tRaw reads\trun_accession\n"))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_fields"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "resultId\tdescription\tprimaryAccessionType\tichsm_search\n" +
		"analysis\tAnalyses\tanalysis_accession\tyes\n" +
		"read_run\tRaw reads\trun_accession\tyes\n" +
		"sample\tSamples\tsample_accession\tyes\n" +
		"tls_set\tTargeted locus study contig sets\taccession\tyes\n" +
		"wgs_set\tGenome assembly contig set (WGS)\taccession\tyes\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunGetFieldsForDataType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/searchFields" {
			t.Fatalf("path = %q, want /searchFields", r.URL.Path)
		}
		if got := r.URL.Query().Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		_, _ = w.Write([]byte("columnId\tdescription\ttype\nrun_accession\taccession number\ttext\n"))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_fields", "read_run"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "columnId\ttype\tichsm_columns\tdescription\nrun_accession\ttext\tSMALL\taccession number\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunGetFieldsForDataTypeSortsByICHSMColumns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/searchFields" {
			t.Fatalf("path = %q, want /searchFields", r.URL.Path)
		}
		if got := r.URL.Query().Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		_, _ = w.Write([]byte("columnId\tdescription\ttype\nage\tAge when sampled\ttext\ncenter_name\tSubmitting center\ttext\nfastq_ftp\tFASTQ URLs\ttext\nlocation_end\tlatlon\nrun_accession\taccession number\ttext\n"))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_fields", "read_run", "--sort", "ichsm_columns"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "columnId\ttype\tichsm_columns\tdescription\n" +
		"run_accession\ttext\tSMALL\taccession number\n" +
		"fastq_ftp\ttext\tDEFAULT\tFASTQ URLs\n" +
		"center_name\ttext\tBIG\tSubmitting center\n" +
		"age\ttext\tALL\tAge when sampled\n" +
		"location_end\tlatlon\tALL\t\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunGetFieldsWritesTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/results" {
			t.Fatalf("path = %q, want /results", r.URL.Path)
		}
		_, _ = w.Write([]byte("resultId\tdescription\nsample\tSamples\nanalysis\tAnalyses\n"))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_fields", "--outfmt", "table"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "resultId  description  ichsm_search\n" +
		"analysis  Analyses     yes\n" +
		"sample    Samples      yes\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
