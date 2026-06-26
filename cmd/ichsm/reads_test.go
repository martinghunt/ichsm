package main

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/martinghunt/ichsm"
)

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

func TestRunReadsWritesManifest(t *testing.T) {
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
		if got := query.Get("fields"); got != "run_accession,fastq_ftp,fastq_md5,fastq_bytes" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"SRR3675520","fastq_ftp":"ftp.sra.ebi.ac.uk/read_1.fastq.gz;ftp.sra.ebi.ac.uk/read_2.fastq.gz","fastq_md5":"abc;def","fastq_bytes":"10;20"}]`))
	})

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

func TestRunReadsSkipsNoRecordAccessionsAndReturnsNonZero(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"run_accession":"SRR3675520","fastq_ftp":"ftp.sra.ebi.ac.uk/read.fastq.gz","fastq_md5":"abc","fastq_bytes":"10"}]`))
		case strings.Contains(query, "SAMN15052188"):
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("query = %q", query)
		}
	})

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"reads", "-f", accessionFile.Name()})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	const want = "input_accession\trun_accession\tfilename\turl\tmd5\tbytes\n" +
		"SAMN05276490\tSRR3675520\tread.fastq.gz\thttps://ftp.sra.ebi.ac.uk/read.fastq.gz\tabc\t10\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if !strings.Contains(stderr, "warning: no results returned for accession SAMN15052188; skipping") {
		t.Fatalf("stderr = %q, want no-results warning", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN15052188") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestRunReadsFailModeStopsWithoutPartialOutput(t *testing.T) {
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
			_, _ = w.Write([]byte(`[{"run_accession":"SRR3675520","fastq_ftp":"ftp.sra.ebi.ac.uk/read.fastq.gz","fastq_md5":"abc","fastq_bytes":"10"}]`))
		case strings.Contains(query, "SAMN15052188"):
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("query = %q", query)
		}
	})

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"reads", "-f", accessionFile.Name(), "--on-no-results", "fail"})
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

func TestRunReadsEmptyModeWritesEmptyManifestRow(t *testing.T) {
	server := withPathResponseServer(t, "/search", `[]`)

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"reads", "-a", "SAMN15052188", "--on-no-results", "empty"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	const want = "input_accession\trun_accession\tfilename\turl\tmd5\tbytes\n" +
		"SAMN15052188\t.\t.\t.\t.\t.\n"
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

func TestRunReadsReportModeWritesDiagnosticManifestRow(t *testing.T) {
	server := withPathResponseServer(t, "/search", `[]`)

	withTestClient(t, server)
	code, stdout, stderr := captureStdoutStderr(t, func() int {
		return run([]string{"reads", "-a", "SAMN15052188", "--on-no-results", "report"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	const want = "input_accession\trun_accession\tfilename\turl\tmd5\tbytes\tichsm_status\tichsm_error\n" +
		"SAMN15052188\t.\t.\t.\t.\t.\tno_results\tno results returned\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if !strings.Contains(stderr, "warning: no results returned for accession SAMN15052188; writing report record") {
		t.Fatalf("stderr = %q, want no-results warning", stderr)
	}
	if !strings.Contains(stderr, "Error: no results returned for accession SAMN15052188") {
		t.Fatalf("stderr = %q, want final no-results error", stderr)
	}
}

func TestRunReadsWritesTable(t *testing.T) {
	server := withPathResponseServer(t, "/search", `[{"run_accession":"SRR3675520","fastq_ftp":"f.gz","fastq_md5":"abc","fastq_bytes":"10"}]`)

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
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/file.fastq.gz","fastq_md5":"abc","fastq_bytes":"10"}]`))
	})

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
	server := withPathResponseServer(t, "/search", `[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/file.fastq.gz","fastq_md5":"abc","fastq_bytes":"10"}]`)

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
