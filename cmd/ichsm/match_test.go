package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/martinghunt/ichsm"
)

func TestRunMatchWritesGroupRows(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		if got := query.Get("query"); got != "tax_tree(2)" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "sample_accession,instrument_platform" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[
{"sample_accession":"SAMEA1","run_accession":"ERR1","instrument_platform":"ILLUMINA"},
{"sample_accession":"SAMEA1","run_accession":"ERR2","instrument_platform":"OXFORD_NANOPORE"},
{"sample_accession":"SAMEA2","run_accession":"ERR3","instrument_platform":"ILLUMINA"},
{"sample_accession":"SAMEA3","run_accession":"ERR4","instrument_platform":"PACBIO_SMRT"},
{"sample_accession":"SAMEA4","run_accession":"ERR5","instrument_platform":"ILLUMINA"},
{"sample_accession":"SAMEA4","run_accession":"ERR6","instrument_platform":"PACBIO_SMRT"}
]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
			"--has", "instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE",
			"--strategy", "local",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "sample_accession\trecord_count\tinstrument_platform\n" +
		"SAMEA1\t2\tILLUMINA;OXFORD_NANOPORE\n" +
		"SAMEA4\t2\tILLUMINA;PACBIO_SMRT\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunMatchWritesRecordRows(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("fields"); got != "sample_accession,instrument_platform,library_layout,run_accession" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[
{"sample_accession":"SAMEA1","run_accession":"ERR1","instrument_platform":"ILLUMINA","library_layout":"PAIRED"},
{"sample_accession":"SAMEA1","run_accession":"ERR2","instrument_platform":"OXFORD_NANOPORE","library_layout":"SINGLE"},
{"sample_accession":"SAMEA2","run_accession":"ERR3","instrument_platform":"ILLUMINA","library_layout":"SINGLE"}
]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA;library_layout=PAIRED",
			"--output", "records",
			"--record-scope", "all",
			"--columns", "sample_accession,run_accession,instrument_platform,library_layout",
			"--strategy", "local",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "sample_accession\trun_accession\tinstrument_platform\tlibrary_layout\n" +
		"SAMEA1\tERR1\tILLUMINA\tPAIRED\n" +
		"SAMEA1\tERR2\tOXFORD_NANOPORE\tSINGLE\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunMatchWritesOnlyMatchingRecordRowsByDefault(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
{"sample_accession":"SAMEA1","run_accession":"ERR1","instrument_platform":"ILLUMINA","library_layout":"PAIRED"},
{"sample_accession":"SAMEA1","run_accession":"ERR2","instrument_platform":"OXFORD_NANOPORE","library_layout":"SINGLE"},
{"sample_accession":"SAMEA1","run_accession":"ERR3","instrument_platform":"ION_TORRENT","library_layout":"SINGLE"}
]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
			"--has", "instrument_platform=OXFORD_NANOPORE",
			"--output", "records",
			"--columns", "sample_accession,run_accession,instrument_platform,library_layout",
			"--strategy", "local",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "sample_accession\trun_accession\tinstrument_platform\tlibrary_layout\n" +
		"SAMEA1\tERR1\tILLUMINA\tPAIRED\n" +
		"SAMEA1\tERR2\tOXFORD_NANOPORE\tSINGLE\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunMatchWritesGroupJSON(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
{"sample_accession":"SAMEA1","run_accession":"ERR1","instrument_platform":"ILLUMINA"},
{"sample_accession":"SAMEA1","run_accession":"ERR2","instrument_platform":"OXFORD_NANOPORE"}
]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
			"--has", "instrument_platform=OXFORD_NANOPORE",
			"--outfmt", "json",
			"--strategy", "local",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var got []matchGroupJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json output did not unmarshal: %v\n%s", err, stdout)
	}
	if len(got) != 1 || got[0].Group != "SAMEA1" || got[0].RecordCount != 2 {
		t.Fatalf("json output = %#v", got)
	}
	if values := got[0].Values["instrument_platform"]; len(values) != 2 || values[0] != "ILLUMINA" || values[1] != "OXFORD_NANOPORE" {
		t.Fatalf("instrument_platform values = %#v", values)
	}
}

func TestRunMatchAutoCountsAndIntersectsSeedQueries(t *testing.T) {
	var searchQueries []string
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch r.URL.Path {
		case "/count":
			switch query.Get("query") {
			case "(tax_tree(2)) AND (instrument_platform=ILLUMINA)":
				_, _ = w.Write([]byte(`{"count":"1000"}`))
			case "(tax_tree(2)) AND ((instrument_platform=OXFORD_NANOPORE) OR (instrument_platform=PACBIO_SMRT))":
				_, _ = w.Write([]byte(`{"count":"3"}`))
			default:
				t.Fatalf("unexpected count query = %q", query.Get("query"))
			}
		case "/search":
			searchQueries = append(searchQueries, query.Get("query"))
			if got := query.Get("result"); got != "read_run" {
				t.Fatalf("result = %q, want read_run", got)
			}
			if got := query.Get("format"); got != "tsv" {
				t.Fatalf("format = %q, want tsv", got)
			}
			switch len(searchQueries) {
			case 1:
				if got := query.Get("fields"); got != "sample_accession" {
					t.Fatalf("seed fields = %q, want sample_accession", got)
				}
				_, _ = w.Write([]byte("sample_accession\nSAMEA1\nSAMEA4\nSAMEA5\n"))
			case 2:
				if got := query.Get("fields"); got != "sample_accession" {
					t.Fatalf("filtered seed fields = %q, want sample_accession", got)
				}
				_, _ = w.Write([]byte("sample_accession\nSAMEA1\nSAMEA4\n"))
			case 3:
				if got := query.Get("fields"); got != "sample_accession,instrument_platform" {
					t.Fatalf("record fields = %q, want sample_accession,instrument_platform", got)
				}
				_, _ = w.Write([]byte("sample_accession\tinstrument_platform\nSAMEA1\tILLUMINA\nSAMEA1\tOXFORD_NANOPORE\nSAMEA4\tILLUMINA\nSAMEA4\tPACBIO_SMRT\n"))
			default:
				t.Fatalf("unexpected search query %d: %q", len(searchQueries), query.Get("query"))
			}
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
			"--has", "instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	wantQueries := []string{
		"(tax_tree(2)) AND ((instrument_platform=OXFORD_NANOPORE) OR (instrument_platform=PACBIO_SMRT))",
		"((tax_tree(2)) AND (instrument_platform=ILLUMINA)) AND ((sample_accession=SAMEA1) OR (sample_accession=SAMEA4) OR (sample_accession=SAMEA5))",
		"(tax_tree(2)) AND ((sample_accession=SAMEA1) OR (sample_accession=SAMEA4))",
	}
	if len(searchQueries) != len(wantQueries) {
		t.Fatalf("searchQueries = %#v, want %#v", searchQueries, wantQueries)
	}
	for i := range wantQueries {
		if searchQueries[i] != wantQueries[i] {
			t.Fatalf("search query %d = %q, want %q", i, searchQueries[i], wantQueries[i])
		}
	}

	const want = "sample_accession\trecord_count\tinstrument_platform\n" +
		"SAMEA1\t2\tILLUMINA;OXFORD_NANOPORE\n" +
		"SAMEA4\t2\tILLUMINA;PACBIO_SMRT\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunMatchAutoVerboseReportsSeedQueries(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch r.URL.Path {
		case "/count":
			if got := query.Get("query"); got != "(tax_tree(2)) AND (instrument_platform=ILLUMINA)" {
				t.Fatalf("count query = %q", got)
			}
			_, _ = w.Write([]byte(`{"count":"1"}`))
		case "/search":
			if got := query.Get("format"); got != "tsv" {
				t.Fatalf("format = %q, want tsv", got)
			}
			switch query.Get("fields") {
			case "sample_accession":
				_, _ = w.Write([]byte("sample_accession\nSAMEA1\n"))
			case "sample_accession,instrument_platform":
				_, _ = w.Write([]byte("sample_accession\tinstrument_platform\nSAMEA1\tILLUMINA\n"))
			default:
				t.Fatalf("fields = %q", query.Get("fields"))
			}
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	withTestClient(t, server)
	code, _, stderr := captureStdoutStderr(t, func() int {
		return run([]string{
			"match",
			"--verbose",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
			"--output", "records",
			"--columns", "sample_accession,instrument_platform",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	wants := []string{
		"seed 1 count: 1 records for --has \"instrument_platform=ILLUMINA\"",
		"seed 1 query: (tax_tree(2)) AND (instrument_platform=ILLUMINA)",
		"requirement 1 --has \"instrument_platform=ILLUMINA\": fetching 1 batch(es) for query: (tax_tree(2)) AND (instrument_platform=ILLUMINA)",
		"final records query: (tax_tree(2)) AND (instrument_platform=ILLUMINA)",
	}
	for _, want := range wants {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want substring %q", stderr, want)
		}
	}
}

func TestFinalMatchRecordQueryNarrowsMatchingRecordOutput(t *testing.T) {
	requirement1, err := parseMatchRequirement("instrument_platform=ILLUMINA")
	if err != nil {
		t.Fatal(err)
	}
	requirement2, err := parseMatchRequirement("instrument_platform=PACBIO_SMRT,OXFORD_NANOPORE")
	if err != nil {
		t.Fatal(err)
	}

	got := finalMatchRecordQuery("tax_tree(2)", []matchRequirement{requirement1, requirement2}, matchOutputRecords, matchRecordScopeMatching)
	const want = "(tax_tree(2)) AND ((instrument_platform=ILLUMINA) OR ((instrument_platform=OXFORD_NANOPORE) OR (instrument_platform=PACBIO_SMRT)))"
	if got != want {
		t.Fatalf("final query = %q, want %q", got, want)
	}

	got = finalMatchRecordQuery("tax_tree(2)", []matchRequirement{requirement1}, matchOutputRecords, matchRecordScopeAll)
	if got != "tax_tree(2)" {
		t.Fatalf("all-scope final query = %q, want base query", got)
	}
}

func TestRunMatchAutoStopsWhenASeedCountIsZero(t *testing.T) {
	var searched bool
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/count":
			_, _ = w.Write([]byte(`{"count":"0"}`))
		case "/search":
			searched = true
			t.Fatalf("did not expect search request")
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
		})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if searched {
		t.Fatal("unexpected search request")
	}
	const want = "sample_accession\trecord_count\tinstrument_platform\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestMatchingGroupsSplitsENAListValues(t *testing.T) {
	records := []ichsm.Record{
		{
			"sample_accession":    "SAMEA1;SAMEA2",
			"instrument_platform": "ILLUMINA;OXFORD_NANOPORE",
		},
	}
	requirement, err := parseMatchRequirement("instrument_platform=OXFORD_NANOPORE")
	if err != nil {
		t.Fatal(err)
	}

	groups := matchingGroups(records, "sample_accession", []matchRequirement{requirement})
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].key != "SAMEA1" || groups[1].key != "SAMEA2" {
		t.Fatalf("group keys = %q, %q; want SAMEA1, SAMEA2", groups[0].key, groups[1].key)
	}
}

func TestFetchMatchGroupRecordsDeduplicatesRowsAcrossBatches(t *testing.T) {
	requests := 0
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("format"); got != "tsv" {
			t.Fatalf("format = %q, want tsv", got)
		}
		if got := query.Get("fields"); got != "sample_accession,instrument_platform" {
			t.Fatalf("fields = %q, want sample_accession,instrument_platform", got)
		}
		_, _ = w.Write([]byte("sample_accession\tinstrument_platform\nSAMEA000;SAMEA100\tILLUMINA\n"))
	})

	client := &ichsm.Client{
		BaseURL:              server.URL,
		HTTPClient:           server.Client(),
		ENARequestsPerSecond: -1,
		MaxRequestRetries:    -1,
	}
	groups := map[string]bool{}
	for i := 0; i <= matchGroupBatchSize; i++ {
		groups[fmt.Sprintf("SAMEA%03d", i)] = true
	}

	records, err := fetchMatchGroupRecords(context.Background(), client, "run", "tax_tree(2)", "sample_accession", groups, []string{"sample_accession", "instrument_platform"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
}

func TestRunMatchAutoRejectsLimitAndOffset(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{
			"match",
			"--result", "run",
			"--query", "tax_tree(2)",
			"--group-by", "sample_accession",
			"--has", "instrument_platform=ILLUMINA",
			"--limit", "10",
		})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestParseMatchRequirementRejectsInvalidSyntax(t *testing.T) {
	tests := []string{
		"",
		"instrument_platform",
		"=ILLUMINA",
		"instrument_platform=",
		"instrument_platform=ILLUMINA;",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			if _, err := parseMatchRequirement(test); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMatchProgressRequestEstimates(t *testing.T) {
	progress := matchProgress{requestsPerSecond: 5}

	if got := progress.requestEstimate(979); got != "; minimum time 3m16s at 5 req/s" {
		t.Fatalf("requestEstimate = %q", got)
	}
	if got := progress.remainingRequestEstimate(879); got != "; minimum time remaining 2m56s" {
		t.Fatalf("remainingRequestEstimate = %q", got)
	}
	if got := formatDurationEstimate(90 * time.Second); got != "1m30s" {
		t.Fatalf("formatDurationEstimate = %q", got)
	}
}
