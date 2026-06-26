package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/martinghunt/ichsm"
)

func TestRunSearchWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "sample" {
			t.Fatalf("result = %q, want sample", got)
		}
		if got := query.Get("query"); got != "sample_accession=SAMN05276490 OR secondary_sample_accession=SAMN05276490" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "sample_accession,description,secondary_sample_accession,study_accession,scientific_name,tax_id,collection_date,country" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"sample_accession":"SAMN05276490","description":"sample description","secondary_sample_accession":"SRS123456","study_accession":"PRJNA302362","scientific_name":"Mycobacterium tuberculosis","tax_id":"1773","collection_date":"2016-01-01","country":""}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tsample_accession\tdescription\tsecondary_sample_accession\tstudy_accession\tscientific_name\ttax_id\tcollection_date\tcountry\n" +
		"SAMN05276490\tSAMN05276490\tsample description\tSRS123456\tPRJNA302362\tMycobacterium tuberculosis\t1773\t2016-01-01\t.\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchFailsInsteadOfWritingPartialOutput(t *testing.T) {
	accessionFile, err := os.CreateTemp(t.TempDir(), "accessions-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accessionFile.WriteString("SAMN05276490\nSAMN00000001\n"); err != nil {
		t.Fatal(err)
	}
	if err := accessionFile.Close(); err != nil {
		t.Fatal(err)
	}

	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query().Get("query")
		switch {
		case strings.Contains(query, "SAMN05276490"):
			_, _ = w.Write([]byte(`[{"secondary_sample_accession":"SRS123456","collection_date":"2016-01-01","country":"France"}]`))
		case strings.Contains(query, "SAMN00000001"):
			http.Error(w, "upstream error", http.StatusBadGateway)
		default:
			t.Fatalf("query = %q", query)
		}
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-f", accessionFile.Name()})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output on partial failure", stdout)
	}
}

func TestRunSearchFailsWhenNoRecordsReturned(t *testing.T) {
	server := withPathResponseServer(t, "/search", `[]`)

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output when no records are returned", stdout)
	}
	if !strings.Contains(stderr, "warning: no results returned for accession SAMN05276490; skipping") {
		t.Fatalf("stderr = %q, want no-results warning", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN05276490") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestRunSearchSkipsNoRecordAccessionsAndReturnsNonZero(t *testing.T) {
	accessionFile, err := os.CreateTemp(t.TempDir(), "accessions-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accessionFile.WriteString("SAMN05276490\nSAMN15052188\n"); err != nil {
		t.Fatal(err)
	}
	if err := accessionFile.Close(); err != nil {
		t.Fatal(err)
	}

	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query().Get("query")
		switch {
		case strings.Contains(query, "SAMN05276490"):
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMN05276490","study_accession":"PRJNA302362"}]`))
		case strings.Contains(query, "SAMN15052188"):
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("query = %q", query)
		}
	})

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"search", "-f", accessionFile.Name(), "--outfmt", "json"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}

	var got map[string][]map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("error parsing stdout JSON: %v\nstdout = %s", err, stdout)
	}
	if _, ok := got["SAMN15052188"]; ok {
		t.Fatalf("stdout includes skipped accession: %s", stdout)
	}
	records := got["SAMN05276490"]
	if len(records) != 1 {
		t.Fatalf("SAMN05276490 records = %d, want 1; stdout = %s", len(records), stdout)
	}
	if gotSample, _ := records[0]["sample_accession"].(string); gotSample != "SAMN05276490" {
		t.Fatalf("sample_accession = %q, want SAMN05276490", gotSample)
	}
	if !strings.Contains(stderr, "warning: no results returned for accession SAMN15052188; skipping") {
		t.Fatalf("stderr = %q, want no-results warning", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN15052188") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestRunSearchFailModeStopsWithoutPartialOutput(t *testing.T) {
	accessionFile, err := os.CreateTemp(t.TempDir(), "accessions-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accessionFile.WriteString("SAMN05276490\nSAMN15052188\n"); err != nil {
		t.Fatal(err)
	}
	if err := accessionFile.Close(); err != nil {
		t.Fatal(err)
	}

	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query().Get("query")
		switch {
		case strings.Contains(query, "SAMN05276490"):
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMN05276490","study_accession":"PRJNA302362"}]`))
		case strings.Contains(query, "SAMN15052188"):
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("query = %q", query)
		}
	})

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"search", "-f", accessionFile.Name(), "--outfmt", "json", "--on-no-results", "fail"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output in fail mode", stdout)
	}
	if strings.Contains(stderr, "warning:") {
		t.Fatalf("stderr = %q, want no warning in fail mode", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN15052188") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestRunSearchEmptyModeWritesEmptyRecord(t *testing.T) {
	server := withPathResponseServer(t, "/search", `[]`)

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"search", "-a", "SAMN15052188", "--columns", "sample_accession,study_accession", "--on-no-results", "empty"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	const want = "input_accession\tsample_accession\tstudy_accession\n" +
		"SAMN15052188\t.\t.\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if !strings.Contains(stderr, "warning: no results returned for accession SAMN15052188; writing empty record") {
		t.Fatalf("stderr = %q, want no-results warning", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN15052188") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestRunSearchErrorModeWritesDiagnosticRecord(t *testing.T) {
	server := withPathResponseServer(t, "/search", `[]`)

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"search", "-a", "SAMN15052188", "--columns", "sample_accession,study_accession", "--outfmt", "json", "--on-no-results", "error"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	var got map[string][]map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("error parsing stdout JSON: %v\nstdout = %s", err, stdout)
	}
	records := got["SAMN15052188"]
	if len(records) != 1 {
		t.Fatalf("SAMN15052188 records = %d, want 1; stdout = %s", len(records), stdout)
	}
	if value, ok := records[0]["sample_accession"]; !ok || value != nil {
		t.Fatalf("sample_accession = %#v, want nil", value)
	}
	if gotStatus, _ := records[0]["ichsm_status"].(string); gotStatus != "no_results" {
		t.Fatalf("ichsm_status = %q, want no_results", gotStatus)
	}
	if gotError, _ := records[0]["ichsm_error"].(string); gotError != "no results returned" {
		t.Fatalf("ichsm_error = %q, want no results returned", gotError)
	}
	if !strings.Contains(stderr, "warning: no results returned for accession SAMN15052188; writing error record") {
		t.Fatalf("stderr = %q, want no-results warning", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN15052188") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestWarnLargeJSONSearchCountsForProjectRun(t *testing.T) {
	var sawCount bool
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`[{"study_accession":"PRJEB1787"}]`))
		case "/count":
			sawCount = true
			query := r.URL.Query()
			if got := query.Get("result"); got != "read_run" {
				t.Fatalf("count result = %q, want read_run", got)
			}
			if got := query.Get("query"); got != "study_accession=PRJEB1787" {
				t.Fatalf("count query = %q", got)
			}
			_, _ = w.Write([]byte(`{"count":"1000"}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	client := &ichsm.Client{
		BaseURL:               server.URL + "/",
		HTTPClient:            server.Client(),
		ENARequestsPerSecond:  -1,
		NCBIRequestsPerSecond: -1,
		MaxRequestRetries:     -1,
	}
	var stderr bytes.Buffer
	warnLargeJSONSearchCounts(context.Background(), client, []accessionSearch{
		{input: "ERP001736", fixed: "ERP001736", typ: ichsm.AccessionTypeStudy},
	}, ichsm.AccessionTypeRun, ichsm.SearchSourceAuto, false, &stderr)

	if !sawCount {
		t.Fatal("expected count preflight request")
	}
	if got := stderr.String(); !strings.Contains(got, "JSON search for ERP001736 at run level will return 1000 records") {
		t.Fatalf("stderr = %q, want large JSON warning", got)
	}
}

func TestWarnLargeJSONSearchCountsSkipsSampleRun(t *testing.T) {
	var requested bool
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requested = true
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})

	client := &ichsm.Client{
		BaseURL:               server.URL + "/",
		HTTPClient:            server.Client(),
		ENARequestsPerSecond:  -1,
		NCBIRequestsPerSecond: -1,
		MaxRequestRetries:     -1,
	}
	var stderr bytes.Buffer
	warnLargeJSONSearchCounts(context.Background(), client, []accessionSearch{
		{input: "SAMN05276490", fixed: "SAMN05276490", typ: ichsm.AccessionTypeSample},
	}, ichsm.AccessionTypeRun, ichsm.SearchSourceAuto, false, &stderr)

	if requested {
		t.Fatal("did not expect count preflight request")
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want no warning", stderr.String())
	}
}

func TestNeedsJSONCountPreflight(t *testing.T) {
	tests := []struct {
		name       string
		source     ichsm.SearchSource
		inputType  ichsm.AccessionType
		resultType ichsm.AccessionType
		want       bool
	}{
		{
			name:       "study fanout",
			source:     ichsm.SearchSourceAuto,
			inputType:  ichsm.AccessionTypeStudy,
			resultType: ichsm.AccessionTypeRun,
			want:       true,
		},
		{
			name:       "study self lookup",
			source:     ichsm.SearchSourceAuto,
			inputType:  ichsm.AccessionTypeStudy,
			resultType: ichsm.AccessionTypeStudy,
		},
		{
			name:       "sample to run intentionally skipped",
			source:     ichsm.SearchSourceAuto,
			inputType:  ichsm.AccessionTypeSample,
			resultType: ichsm.AccessionTypeRun,
		},
		{
			name:       "contig set",
			source:     ichsm.SearchSourceENA,
			inputType:  ichsm.AccessionTypeContigSet,
			resultType: ichsm.AccessionTypeContigSet,
			want:       true,
		},
		{
			name:       "forced ncbi",
			source:     ichsm.SearchSourceNCBI,
			inputType:  ichsm.AccessionTypeContigSet,
			resultType: ichsm.AccessionTypeContigSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsJSONCountPreflight(tt.source, tt.inputType, tt.resultType); got != tt.want {
				t.Fatalf("needsJSONCountPreflight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunSearchWritesTable(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "sample" {
			t.Fatalf("result = %q, want sample", got)
		}
		_, _ = w.Write([]byte(`[{"secondary_sample_accession":"SRS123456","collection_date":"2016-01-01","country":""}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490", "--outfmt", "table"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession  sample_accession  description  secondary_sample_accession  study_accession  scientific_name  tax_id  collection_date  country\n" +
		"SAMN05276490     .                 .            SRS123456                   .                .                .       2016-01-01       .\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchWritesTTable(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "sample" {
			t.Fatalf("result = %q, want sample", got)
		}
		if got := query.Get("fields"); got != "sample_accession,description" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"sample_accession":"SAMN05276490","description":"sample description"}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490", "--columns", "sample_accession,description", "--outfmt", "ttable"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession   SAMN05276490\n" +
		"sample_accession  SAMN05276490\n" +
		"description       sample description\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchWritesTTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "sample" {
			t.Fatalf("result = %q, want sample", got)
		}
		if got := query.Get("fields"); got != "sample_accession,description" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"sample_accession":"SAMN05276490","description":"sample description"}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490", "--columns", "sample_accession,description", "--outfmt", "ttsv"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tSAMN05276490\n" +
		"sample_accession\tSAMN05276490\n" +
		"description\tsample description\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestParseOutputFormatTransposedFormats(t *testing.T) {
	tests := map[string]string{
		"ttable": outputFormatTTable,
		"ttsv":   outputFormatTTSV,
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			got, err := parseOutputFormat(format, true)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("parseOutputFormat(%q) = %q, want %q", format, got, want)
			}
		})
	}
}

func TestRunSearchWithLevel(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		if got := query.Get("query"); got != "sample_accession=SAMN05276490 OR secondary_sample_accession=SAMN05276490" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "study_accession,secondary_study_accession,sample_accession,secondary_sample_accession,run_accession,description,instrument_platform,library_layout,fastq_ftp" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/file.fastq.gz"}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490", "--level", "run"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tstudy_accession\tsecondary_study_accession\tsample_accession\tsecondary_sample_accession\trun_accession\tdescription\tinstrument_platform\tlibrary_layout\tfastq_ftp\n" +
		"SAMN05276490\t.\t.\t.\t.\tERR123456\t.\t.\t.\tftp.sra.ebi.ac.uk/file.fastq.gz\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchWritesJSON(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		if got := query.Get("query"); got != "run_accession=ERR123456" {
			t.Fatalf("query = %q", got)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/file.fastq.gz"}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "ERR123456", "--outfmt", "json"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var got map[string][]map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json output did not unmarshal: %v\n%s", err, stdout)
	}
	if got["ERR123456"][0]["run_accession"] != "ERR123456" {
		t.Fatalf("run_accession = %q", got["ERR123456"][0]["run_accession"])
	}
	if got["ERR123456"][0]["fastq_ftp"] != "ftp.sra.ebi.ac.uk/file.fastq.gz" {
		t.Fatalf("fastq_ftp = %q", got["ERR123456"][0]["fastq_ftp"])
	}
}

func TestRunSearchCountWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			query := r.URL.Query()
			if got := query.Get("result"); got != "study" {
				t.Fatalf("study result = %q, want study", got)
			}
			if got := query.Get("query"); got != "study_accession=PRJEB1787 OR secondary_study_accession=PRJEB1787" {
				t.Fatalf("study query = %q", got)
			}
			if got := query.Get("fields"); got != "study_accession" {
				t.Fatalf("study fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"study_accession":"PRJEB1787"}]`))
		case "/count":
			query := r.URL.Query()
			if got := query.Get("result"); got != "read_run" {
				t.Fatalf("count result = %q, want read_run", got)
			}
			if got := query.Get("query"); got != "study_accession=PRJEB1787" {
				t.Fatalf("count query = %q", got)
			}
			_, _ = w.Write([]byte(`{"count":"249"}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "PRJEB1787", "--level", "run", "--count"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tresult_type\tcount\n" +
		"PRJEB1787\trun\t249\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchCountWritesJSON(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/count" {
			t.Fatalf("path = %q, want /count", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("count result = %q, want read_run", got)
		}
		if got := query.Get("query"); got != "run_accession=ERR123456" {
			t.Fatalf("count query = %q", got)
		}
		_, _ = w.Write([]byte(`{"count":"1"}`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "ERR123456", "--count", "--outfmt", "json"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	var got []countResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json output did not unmarshal: %v\n%s", err, stdout)
	}
	if len(got) != 1 || got[0].InputAccession != "ERR123456" || got[0].ResultType != ichsm.AccessionTypeRun || got[0].Count != 1 {
		t.Fatalf("count json = %#v", got)
	}
}

func TestRunSearchCountRejectsNCBISource(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "GCF_000001405.40", "--source", "ncbi", "--count"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestRunSearchWGSSetWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "wgs_set" {
			t.Fatalf("result = %q, want wgs_set", got)
		}
		if got := query.Get("query"); got != "wgs_set=AGQU01" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "accession,wgs_set,assembly_accession,sample_accession,run_accession,sequence_version,description,study_accession,scientific_name,tax_id" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"accession":"AGQU01000000","wgs_set":"AGQU01","assembly_accession":"GCA_000231155","sequence_version":"1","description":"test contig set","study_accession":"PRJNA123456","scientific_name":"Mycobacteroides abscessus 47J26","tax_id":"1087483","sample_accession":"SAMN02471593","run_accession":""}]`))
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "AGQU00000000.1", "--level", "assembly"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\taccession\twgs_set\tassembly_accession\tsample_accession\trun_accession\tsequence_version\tdescription\tstudy_accession\tscientific_name\ttax_id\n" +
		"AGQU00000000.1\tAGQU01000000\tAGQU01\tGCA_000231155\tSAMN02471593\t.\t1\ttest contig set\tPRJNA123456\tMycobacteroides abscessus 47J26\t1087483\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchFallsBackToNCBIAssembly(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if got := r.URL.Query().Get("result"); got != "assembly" {
				t.Fatalf("ENA result = %q, want assembly", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "/esearch.fcgi":
			if got := r.URL.Query().Get("term"); got != "GCF_000001405.40[Assembly Accession]" {
				t.Fatalf("NCBI term = %q", got)
			}
			_, _ = w.Write([]byte(`{"esearchresult":{"idlist":["11968211"]}}`))
		case "/esummary.fcgi":
			_, _ = w.Write([]byte(`{"result":{"uids":["11968211"],"11968211":{"assemblyaccession":"GCF_000001405.40","assemblydescription":"Genome Reference Consortium Human Build 38 patch release 14 (GRCh38.p14)","speciesname":"Homo sapiens","taxid":9606,"biosampleaccn":"SAMN1","rs_bioprojects":[{"bioprojectaccn":"PRJNA168"}]}}}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "GCF_000001405.40"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\taccession\tsample_accession\trun_accession\tversion\tdescription\tstudy_accession\tscientific_name\ttax_id\n" +
		"GCF_000001405.40\tGCF_000001405\tSAMN1\t.\t40\tGenome Reference Consortium Human Build 38 patch release 14 (GRCh38.p14)\tPRJNA168\tHomo sapiens\t9606\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestWriteTSVAllFieldsUsesUnionOfRecordColumns(t *testing.T) {
	results := []ichsm.SearchResult{
		{
			InputAccession: "SAMN05276490",
			Records: []ichsm.Record{
				{
					"z_field": "last",
					"a_field": "first",
					"m_field": nil,
				},
				{
					"b_field": "later",
					"z_field": "last again",
				},
			},
		},
		{
			InputAccession: "SAMN00000002",
			Records: []ichsm.Record{
				{
					"c_field": "second accession",
				},
			},
		},
	}

	var out bytes.Buffer
	if err := writeTSV(&out, results, []string{"ALL"}); err != nil {
		t.Fatal(err)
	}

	const want = "input_accession\ta_field\tb_field\tc_field\tm_field\tz_field\n" +
		"SAMN05276490\tfirst\tnull\tnull\t.\tlast\n" +
		"SAMN05276490\tnull\tlater\tnull\tnull\tlast again\n" +
		"SAMN00000002\tnull\tnull\tsecond accession\tnull\tnull\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}
