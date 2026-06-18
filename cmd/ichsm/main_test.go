package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/martinghunt/ichsm"
)

func TestRunSearchWritesTSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"search", "-a", "SAMN05276490"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output when no records are returned", stdout)
	}
}

func TestWarnLargeJSONSearchCountsForProjectRun(t *testing.T) {
	var sawCount bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "sample" {
			t.Fatalf("result = %q, want sample", got)
		}
		_, _ = w.Write([]byte(`[{"secondary_sample_accession":"SRS123456","collection_date":"2016-01-01","country":""}]`))
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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

func TestParseReadsOutfmtTransposedFormats(t *testing.T) {
	tests := map[string]string{
		"ttable": outputFormatTTable,
		"ttsv":   outputFormatTTSV,
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			got, err := parseReadsOutfmt(format)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("parseReadsOutfmt(%q) = %q, want %q", format, got, want)
			}
		})
	}
}

func TestRunSearchWithLevel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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

func TestRunLinksWritesRunTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "sample":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkSampleFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMD00654312","secondary_sample_accession":"SRS24913212","study_accession":"PRJEB90490;PRJDB16917"}]`))
		case "assembly":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "read_run":
			switch got := query.Get("query"); got {
			case "run_accession=DRR510832", "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312":
			default:
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"run_accession":"DRR510832","experiment_accession":"DRX494734","sample_accession":"SAMD00654312","study_accession":"PRJDB16917"}]`))
		case "analysis":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"analysis_accession":"ERZ26912061","analysis_type":"SEQUENCE_ASSEMBLY","sample_accession":"SAMD00654312","study_accession":"PRJEB90490"}]`))
		case "wgs_set":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[` +
				`{"accession":"BAAHUD010000000","sample_accession":"SAMD00654312","study_accession":"PRJDB16917","run_accession":"DRR510832"},` +
				`{"accession":"DBIITB010000000","sample_accession":"SAMD00654312","study_accession":"PRJNA514245","run_accession":"DRR510832"}` +
				`]`))
		case "tsa_set", "tls_set":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "DRR510832"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJEB90490\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 Analysis: ERZ26912061 (SEQUENCE_ASSEMBLY)\n" +
		"Project: PRJDB16917\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u251c\u2500\u2500 Experiment: DRX494734\n" +
		"    \u2502   \u2514\u2500\u2500 Run: DRR510832\n" +
		"    \u2514\u2500\u2500 ContigSet: BAAHUD010000000\n" +
		"Project: PRJNA514245\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 ContigSet: DBIITB010000000\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksWritesSampleTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "sample":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkSampleFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMN02471593","secondary_sample_accession":"SRS123456","study_accession":"PRJEB45982;PRJNA302362"}]`))
		case "assembly":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"accession":"GCA_000231155","sample_accession":"SAMN02471593","study_accession":"PRJNA302362"}]`))
		case "read_run":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"run_accession":"SRR3675520","experiment_accession":"SRX1850792","sample_accession":"SAMN02471593","study_accession":"PRJNA302362"}]`))
		case "analysis":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "wgs_set":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"accession":"AGQU01000000","assembly_accession":"GCA_000231155","sample_accession":"SAMN02471593","study_accession":"PRJNA302362"}]`))
		case "tsa_set", "tls_set":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "SAMN02471593"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJEB45982\n" +
		"\u2514\u2500\u2500 Sample: SAMN02471593\n" +
		"Project: PRJNA302362\n" +
		"\u2514\u2500\u2500 Sample: SAMN02471593\n" +
		"    \u251c\u2500\u2500 Assembly: GCA_000231155\n" +
		"    \u2502   \u2514\u2500\u2500 ContigSet: AGQU01000000\n" +
		"    \u2514\u2500\u2500 Experiment: SRX1850792\n" +
		"        \u2514\u2500\u2500 Run: SRR3675520\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksWritesExperimentTreeWithContigSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "sample":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkSampleFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMD00654312","secondary_sample_accession":"SRS24913212","study_accession":"PRJEB90490;PRJDB16917"}]`))
		case "assembly":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "read_run":
			switch got := query.Get("query"); got {
			case "experiment_accession=DRX494734", "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312":
			default:
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"run_accession":"DRR510832","experiment_accession":"DRX494734","sample_accession":"SAMD00654312","study_accession":"PRJDB16917"}]`))
		case "analysis":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"analysis_accession":"ERZ26912061","analysis_type":"SEQUENCE_ASSEMBLY","sample_accession":"SAMD00654312","study_accession":"PRJEB90490"}]`))
		case "wgs_set":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[` +
				`{"accession":"BAAHUD010000000","sample_accession":"SAMD00654312","study_accession":"PRJDB16917","run_accession":"DRR510832"},` +
				`{"accession":"DBIITB010000000","sample_accession":"SAMD00654312","study_accession":"PRJNA514245","run_accession":"DRR510832"}` +
				`]`))
		case "tsa_set", "tls_set":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "DRX494734"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJEB90490\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 Analysis: ERZ26912061 (SEQUENCE_ASSEMBLY)\n" +
		"Project: PRJDB16917\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u251c\u2500\u2500 Experiment: DRX494734\n" +
		"    \u2502   \u2514\u2500\u2500 Run: DRR510832\n" +
		"    \u2514\u2500\u2500 ContigSet: BAAHUD010000000\n" +
		"Project: PRJNA514245\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 ContigSet: DBIITB010000000\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksWritesProjectTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "study":
			if got := query.Get("query"); got != "study_accession=SRP076676 OR secondary_study_accession=SRP076676" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkStudyFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"study_accession":"PRJNA302362","secondary_study_accession":"SRP076676"}]`))
		case "read_run":
			if got := query.Get("query"); got != "study_accession=PRJNA302362" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[` +
				`{"run_accession":"SRR1","experiment_accession":"SRX1","sample_accession":"SAMN1","study_accession":"PRJNA302362"},` +
				`{"run_accession":"SRR2","experiment_accession":"SRX1","sample_accession":"SAMN1","study_accession":"PRJNA302362"},` +
				`{"run_accession":"SRR3","experiment_accession":"SRX2","sample_accession":"SAMN2","study_accession":"PRJNA302362"}` +
				`]`))
		case "wgs_set":
			if got := query.Get("query"); got != "study_accession=PRJNA302362" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"accession":"WGS1","sample_accession":"SAMN1","study_accession":"PRJNA302362"}]`))
		case "analysis":
			if got := query.Get("query"); got != "study_accession=PRJNA302362" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "tsa_set", "tls_set":
			if got := query.Get("query"); got != "study_accession=PRJNA302362" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "SRP076676"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJNA302362\n" +
		"\u251c\u2500\u2500 Sample: SAMN1\n" +
		"\u2502   \u251c\u2500\u2500 Experiment: SRX1\n" +
		"\u2502   \u2502   \u251c\u2500\u2500 Run: SRR1\n" +
		"\u2502   \u2502   \u2514\u2500\u2500 Run: SRR2\n" +
		"\u2502   \u2514\u2500\u2500 ContigSet: WGS1\n" +
		"\u2514\u2500\u2500 Sample: SAMN2\n" +
		"    \u2514\u2500\u2500 Experiment: SRX2\n" +
		"        \u2514\u2500\u2500 Run: SRR3\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksWritesContigSetTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "sample":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkSampleFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMD00654312","secondary_sample_accession":"SRS24913212","study_accession":"PRJEB90490;PRJDB16917"}]`))
		case "assembly":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "wgs_set":
			switch got := query.Get("query"); got {
			case "wgs_set=DBIITB01":
				if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
					t.Fatalf("fields = %q", got)
				}
				_, _ = w.Write([]byte(`[{"accession":"DBIITB010000000","sample_accession":"SAMD00654312","study_accession":"PRJNA514245","run_accession":"DRR510832"}]`))
			case "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312":
				if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
					t.Fatalf("fields = %q", got)
				}
				_, _ = w.Write([]byte(`[` +
					`{"accession":"BAAHUD010000000","sample_accession":"SAMD00654312","study_accession":"PRJDB16917","run_accession":"DRR510832"},` +
					`{"accession":"DBIITB010000000","sample_accession":"SAMD00654312","study_accession":"PRJNA514245","run_accession":"DRR510832"}` +
					`]`))
			default:
				t.Fatalf("query = %q", got)
			}
		case "tsa_set", "tls_set":
			switch got := query.Get("query"); got {
			case "accession=DBIITB010000000", "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312":
			default:
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "read_run":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"run_accession":"DRR510832","experiment_accession":"DRX494734","sample_accession":"SAMD00654312","study_accession":"PRJDB16917"}]`))
		case "analysis":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"analysis_accession":"ERZ26912061","analysis_type":"SEQUENCE_ASSEMBLY","sample_accession":"SAMD00654312","study_accession":"PRJEB90490"}]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "DBIITB010000000"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJEB90490\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 Analysis: ERZ26912061 (SEQUENCE_ASSEMBLY)\n" +
		"Project: PRJDB16917\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u251c\u2500\u2500 Experiment: DRX494734\n" +
		"    \u2502   \u2514\u2500\u2500 Run: DRR510832\n" +
		"    \u2514\u2500\u2500 ContigSet: BAAHUD010000000\n" +
		"Project: PRJNA514245\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 ContigSet: DBIITB010000000\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksWritesAnalysisTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "analysis":
			switch got := query.Get("query"); got {
			case "analysis_accession=ERZ26912061", "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312":
			default:
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"analysis_accession":"ERZ26912061","analysis_type":"SEQUENCE_ASSEMBLY","sample_accession":"SAMD00654312","study_accession":"PRJEB90490"}]`))
		case "sample":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkSampleFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMD00654312","secondary_sample_accession":"SRS24913212","study_accession":"PRJEB90490;PRJDB16917"}]`))
		case "assembly":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "read_run":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"run_accession":"DRR510832","experiment_accession":"DRX494734","sample_accession":"SAMD00654312","study_accession":"PRJDB16917"}]`))
		case "wgs_set":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[` +
				`{"accession":"BAAHUD010000000","sample_accession":"SAMD00654312","study_accession":"PRJDB16917","run_accession":"DRR510832"},` +
				`{"accession":"DBIITB010000000","sample_accession":"SAMD00654312","study_accession":"PRJNA514245","run_accession":"DRR510832"}` +
				`]`))
		case "tsa_set", "tls_set":
			if got := query.Get("query"); got != "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "ERZ26912061"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJEB90490\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 Analysis: ERZ26912061 (SEQUENCE_ASSEMBLY)\n" +
		"Project: PRJDB16917\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u251c\u2500\u2500 Experiment: DRX494734\n" +
		"    \u2502   \u2514\u2500\u2500 Run: DRR510832\n" +
		"    \u2514\u2500\u2500 ContigSet: BAAHUD010000000\n" +
		"Project: PRJNA514245\n" +
		"\u2514\u2500\u2500 Sample: SAMD00654312\n" +
		"    \u2514\u2500\u2500 ContigSet: DBIITB010000000\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksWritesAssemblyTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		switch query.Get("result") {
		case "assembly":
			switch got := query.Get("query"); got {
			case "accession=GCA_000231155":
				if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
					t.Fatalf("fields = %q", got)
				}
				_, _ = w.Write([]byte(`[{"accession":"GCA_000231155","sample_accession":"SAMN02471593","study_accession":"PRJNA123456","run_accession":"ERR123456"}]`))
			case "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593":
				if got := query.Get("fields"); got != strings.Join(linkAssemblyFields, ",") {
					t.Fatalf("fields = %q", got)
				}
				_, _ = w.Write([]byte(`[{"accession":"GCA_000231155","sample_accession":"SAMN02471593","study_accession":"PRJNA123456","run_accession":"ERR123456"}]`))
			default:
				t.Fatalf("query = %q", got)
			}
		case "sample":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkSampleFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMN02471593","study_accession":"PRJNA123456"}]`))
		case "read_run":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkRunFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","experiment_accession":"ERX123456","sample_accession":"SAMN02471593","study_accession":"PRJNA123456"}]`))
		case "analysis":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkAnalysisFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "wgs_set":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkWGSSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"accession":"AGQU010000000","assembly_accession":"GCA_000231155","sample_accession":"SAMN02471593","study_accession":"PRJNA123456","run_accession":"ERR123456"}]`))
		case "tsa_set", "tls_set":
			if got := query.Get("query"); got != "sample_accession=SAMN02471593 OR secondary_sample_accession=SAMN02471593" {
				t.Fatalf("query = %q", got)
			}
			if got := query.Get("fields"); got != strings.Join(linkContigSetFields, ",") {
				t.Fatalf("fields = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "GCA_000231155"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "Project: PRJNA123456\n" +
		"\u2514\u2500\u2500 Sample: SAMN02471593\n" +
		"    \u251c\u2500\u2500 Assembly: GCA_000231155\n" +
		"    \u2502   \u2514\u2500\u2500 ContigSet: AGQU010000000\n" +
		"    \u2514\u2500\u2500 Experiment: ERX123456\n" +
		"        \u2514\u2500\u2500 Run: ERR123456\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunLinksRejectsUnsupportedAccessionType(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "U49845.1"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}

func TestRunSearchCountWritesTSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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

func TestRunIdentifyWritesTSV(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"identify", "SAMN05276490", "GCF_000001405.40", "ERZ26912061", "--outfmt", "tsv"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tnormalized_accession\ttype\tdescription\tena_search\tncbi_search\n" +
		"SAMN05276490\tSAMN05276490\tsample\tSample accession\tyes\tno\n" +
		"GCF_000001405.40\tGCF_000001405\tassembly\tGenome assembly accession\tyes\tyes\n" +
		"ERZ26912061\tERZ26912061\tanalysis\tAnalysis accession\tyes\tno\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunIdentifyDefaultsToHumanReadableTable(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"identify", "WP_002248791.1"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Protein or coding sequence accession") {
		t.Fatalf("stdout = %q, want human-readable description", stdout)
	}
	if strings.Contains(stdout, "\t") {
		t.Fatalf("stdout = %q, did not expect tabs in default table output", stdout)
	}
}

func TestRunIdentifyReportsUnknownAccessions(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"identify", "not-an-accession", "--outfmt", "tsv"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}

	const want = "input_accession\tnormalized_accession\ttype\tdescription\tena_search\tncbi_search\n" +
		"not-an-accession\t.\tunknown\tUnrecognized accession format\tno\tno\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunSearchWGSSetWritesTSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

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

func TestRunReadsWritesManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if got := query.Get("fields"); got != "run_accession,fastq_ftp,fastq_md5,fastq_bytes" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"SRR3675520","fastq_ftp":"ftp.sra.ebi.ac.uk/read_1.fastq.gz;ftp.sra.ebi.ac.uk/read_2.fastq.gz","fastq_md5":"abc;def","fastq_bytes":"10;20"}]`))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"reads", "-a", "SAMN05276490"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\trun_accession\tfilename\turl\tmd5\tbytes\n" +
		"SAMN05276490\tSRR3675520\tread_1.fastq.gz\thttps://ftp.sra.ebi.ac.uk/read_1.fastq.gz\tabc\t10\n" +
		"SAMN05276490\tSRR3675520\tread_2.fastq.gz\thttps://ftp.sra.ebi.ac.uk/read_2.fastq.gz\tdef\t20\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunReadsWritesTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"SRR3675520","fastq_ftp":"f.gz","fastq_md5":"abc","fastq_bytes":"10"}]`))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"reads", "-a", "SAMN05276490", "--outfmt", "table"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession  run_accession  filename  url           md5  bytes\n" +
		"SAMN05276490     SRR3675520     f.gz      https://f.gz  abc  10\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunReadsWritesWget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/file.fastq.gz","fastq_md5":"abc","fastq_bytes":"10"}]`))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"reads", "-a", "ERR123456", "--outfmt", "wget", "--protocol", "ftp", "--output-dir", "reads"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "wget -c -O 'reads/file.fastq.gz' 'ftp://ftp.sra.ebi.ac.uk/file.fastq.gz'\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunReadsWritesMD5(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/file.fastq.gz","fastq_md5":"abc","fastq_bytes":"10"}]`))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"reads", "-a", "ERR123456", "--outfmt", "md5", "--output-dir", "reads"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "abc  reads/file.fastq.gz\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenPrintsURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "SRR3675520", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ebi.ac.uk/ena/browser/view/SRR3675520\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenPrintsWGSSetURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "AGQU00000000.1", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ebi.ac.uk/ena/browser/view/AGQU00000000.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenUsesBrowserOpener(t *testing.T) {
	var openedURL string
	withTestBrowserOpener(t, func(browserURL string) error {
		openedURL = browserURL
		return nil
	})

	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "-a", "GCA_000195955.2"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}

	const wantURL = "https://www.ebi.ac.uk/ena/browser/view/GCA_000195955.2"
	if openedURL != wantURL {
		t.Fatalf("openedURL = %q, want %q", openedURL, wantURL)
	}
}

func TestRunOpenRejectsInvalidAccession(t *testing.T) {
	code, _ := captureStdout(t, func() int {
		return run([]string{"open", "not-an-accession", "--print-url"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

func TestRunOpenPrintsNCBIProteinURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "WP_002248791.1", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/protein/WP_002248791.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenPrintsNCBIAssemblyURL(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "GCF_000001405.40", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/datasets/genome/GCF_000001405.40/\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenCanForceNCBIForSharedProteinAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "AAA98665.1", "--source", "ncbi", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/protein/AAA98665.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenCanForceNCBIForSharedNucleotideAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "U49845.1", "--source", "ncbi", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/nuccore/U49845.1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenCanForceNCBIForRunAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"open", "DRR013337", "--source", "ncbi", "--print-url"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "https://www.ncbi.nlm.nih.gov/sra/?term=DRR013337\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunOpenRejectsNCBIOnlyAccessionWithENASource(t *testing.T) {
	code, _ := captureStdout(t, func() int {
		return run([]string{"open", "WP_002248791.1", "--source", "ena", "--print-url"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

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

func TestWriteReadsCurl(t *testing.T) {
	files := []ichsm.ReadFile{
		{
			OutputPath: "reads/file.fastq.gz",
			URL:        "https://ftp.sra.ebi.ac.uk/file.fastq.gz",
		},
	}

	var out bytes.Buffer
	if err := writeReads(&out, files, readsFormatCurl); err != nil {
		t.Fatal(err)
	}

	const want = "curl -L --fail --continue-at - --output 'reads/file.fastq.gz' 'https://ftp.sra.ebi.ac.uk/file.fastq.gz'\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestWriteReadsTTable(t *testing.T) {
	files := []ichsm.ReadFile{
		{
			InputAccession: "SAMN05276490",
			RunAccession:   "SRR3675520",
			Filename:       "read.fastq.gz",
			URL:            "https://ftp.sra.ebi.ac.uk/read.fastq.gz",
			MD5:            "abc",
			Bytes:          "10",
		},
	}

	var out bytes.Buffer
	if err := writeReads(&out, files, outputFormatTTable); err != nil {
		t.Fatal(err)
	}

	const want = "input_accession  SAMN05276490\n" +
		"run_accession    SRR3675520\n" +
		"filename         read.fastq.gz\n" +
		"url              https://ftp.sra.ebi.ac.uk/read.fastq.gz\n" +
		"md5              abc\n" +
		"bytes            10\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestWriteReadsTTSV(t *testing.T) {
	files := []ichsm.ReadFile{
		{
			InputAccession: "SAMN05276490",
			RunAccession:   "SRR3675520",
			Filename:       "read.fastq.gz",
			URL:            "https://ftp.sra.ebi.ac.uk/read.fastq.gz",
			MD5:            "abc",
			Bytes:          "10",
		},
	}

	var out bytes.Buffer
	if err := writeReads(&out, files, outputFormatTTSV); err != nil {
		t.Fatal(err)
	}

	const want = "input_accession\tSAMN05276490\n" +
		"run_accession\tSRR3675520\n" +
		"filename\tread.fastq.gz\n" +
		"url\thttps://ftp.sra.ebi.ac.uk/read.fastq.gz\n" +
		"md5\tabc\n" +
		"bytes\t10\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
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

func TestWriteAlignedRows(t *testing.T) {
	var out bytes.Buffer
	err := writeAlignedRows(&out, [][]string{
		{"a", "long"},
		{"aa", "x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	const want = "a   long\n" +
		"aa  x\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func withTestClient(t *testing.T, server *httptest.Server) {
	t.Helper()

	previous := newClient
	newClient = func() *ichsm.Client {
		return &ichsm.Client{
			BaseURL:               server.URL,
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
