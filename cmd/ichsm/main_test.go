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

	client := &ichsm.Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
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

	client := &ichsm.Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
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
		return run([]string{"identify", "SAMN05276490", "GCF_000001405.40", "--outfmt", "tsv"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tnormalized_accession\ttype\tdescription\tena_search\tncbi_search\n" +
		"SAMN05276490\tSAMN05276490\tsample\tSample accession\tyes\tno\n" +
		"GCF_000001405.40\tGCF_000001405\tassembly\tGenome assembly accession\tyes\tyes\n"
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
		"read_run\tRaw reads\trun_accession\tyes\n" +
		"sample\tSamples\tsample_accession\tyes\n" +
		"tls_set\tTargeted locus study contig sets\taccession\tyes\n" +
		"wgs_set\tGenome assembly contig set (WGS)\taccession\tyes\n" +
		"analysis\tAnalyses\tanalysis_accession\tno\n"
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
		_, _ = w.Write([]byte("columnId\tdescription\nrun_accession\taccession number\n"))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_fields", "read_run"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "columnId\tdescription\tichsm_columns\nrun_accession\taccession number\tSMALL\n"
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
		_, _ = w.Write([]byte("columnId\tdescription\nage\tAge when sampled\ncenter_name\tSubmitting center\nfastq_ftp\tFASTQ URLs\nrun_accession\taccession number\n"))
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"get_fields", "read_run", "--sort", "ichsm_columns"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "columnId\tdescription\tichsm_columns\n" +
		"run_accession\taccession number\tSMALL\n" +
		"fastq_ftp\tFASTQ URLs\tDEFAULT\n" +
		"center_name\tSubmitting center\tBIG\n" +
		"age\tAge when sampled\tALL\n"
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
		"sample    Samples      yes\n" +
		"analysis  Analyses     no\n"
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
			BaseURL:     server.URL,
			NCBIBaseURL: server.URL,
			HTTPClient:  server.Client(),
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
