package ichsm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestClientReadFilesQueriesENAReadRuns(t *testing.T) {
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
		if got := query.Get("fields"); got != "run_accession,fastq_ftp,fastq_md5,fastq_bytes" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":"ftp.sra.ebi.ac.uk/vol1/fastq/ERR123/ERR123456/ERR123456_1.fastq.gz;ftp.sra.ebi.ac.uk/vol1/fastq/ERR123/ERR123456/ERR123456_2.fastq.gz","fastq_md5":"abc;def","fastq_bytes":"10;20"}]`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:               server.URL + "/",
		HTTPClient:            server.Client(),
		ENARequestsPerSecond:  -1,
		NCBIRequestsPerSecond: -1,
		MaxRequestRetries:     -1,
	}
	files, err := client.ReadFiles(context.Background(), ReadFileOptions{
		Accessions: []string{"ERR123456"},
		OutputDir:  "reads",
	})
	if err != nil {
		t.Fatalf("ReadFiles() error = %v", err)
	}

	want := []ReadFile{
		{
			InputAccession: "ERR123456",
			RunAccession:   "ERR123456",
			Filename:       "ERR123456_1.fastq.gz",
			OutputPath:     "reads/ERR123456_1.fastq.gz",
			URL:            "https://ftp.sra.ebi.ac.uk/vol1/fastq/ERR123/ERR123456/ERR123456_1.fastq.gz",
			MD5:            "abc",
			Bytes:          "10",
		},
		{
			InputAccession: "ERR123456",
			RunAccession:   "ERR123456",
			Filename:       "ERR123456_2.fastq.gz",
			OutputPath:     "reads/ERR123456_2.fastq.gz",
			URL:            "https://ftp.sra.ebi.ac.uk/vol1/fastq/ERR123/ERR123456/ERR123456_2.fastq.gz",
			MD5:            "def",
			Bytes:          "20",
		},
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %#v, want %#v", files, want)
	}
}

func TestReadFilesFromRecordAllowsFTPProtocol(t *testing.T) {
	files, err := ReadFilesFromRecord("SAMN05276490", Record{
		"run_accession": "SRR3675520",
		"fastq_ftp":     "https://ftp.sra.ebi.ac.uk/read_1.fastq.gz",
		"fastq_md5":     "abc",
		"fastq_bytes":   "10",
	}, ReadFileOptions{Protocol: ReadProtocolFTP})
	if err != nil {
		t.Fatalf("ReadFilesFromRecord() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if files[0].URL != "ftp://ftp.sra.ebi.ac.uk/read_1.fastq.gz" {
		t.Fatalf("url = %q", files[0].URL)
	}
	if files[0].OutputPath != "read_1.fastq.gz" {
		t.Fatalf("output path = %q", files[0].OutputPath)
	}
}

func TestNormalizeReadDownloadURLRejectsEmptyURL(t *testing.T) {
	_, err := NormalizeReadDownloadURL("", ReadProtocolHTTPS)
	if err == nil || err.Error() != "empty FASTQ URL" {
		t.Fatalf("NormalizeReadDownloadURL() error = %v, want empty URL error", err)
	}
}

func TestNormalizeReadFileProtocolRejectsUnsupportedProtocol(t *testing.T) {
	_, err := NormalizeReadFileProtocol("http")
	if err == nil || err.Error() != `unsupported read file protocol "http"; expected https or ftp` {
		t.Fatalf("NormalizeReadFileProtocol() error = %v", err)
	}
}
