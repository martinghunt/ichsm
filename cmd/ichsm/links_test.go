package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRunLinksWritesRunTree(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "DRR510832"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

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
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMN02471593","secondary_sample_accession":"SRS123456"}]`))
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "SAMN02471593"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

	const want = "Project: PRJNA302362\n" +
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
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "DRX494734"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

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
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "SRP076676"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

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
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "DBIITB010000000"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

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
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "ERZ26912061"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

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
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
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
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMN02471593"}]`))
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
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"links", "GCA_000231155"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	assertNoRootSampleLines(t, stdout)

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

func TestWriteLinksTSVWritesEdgeTable(t *testing.T) {
	root := newLinkTreeNode("Project", "PRJNA123456")
	sample := root.child("Sample", "SAMN02471593")
	assembly := sample.child("Assembly", "GCA_000231155")
	assembly.child("ContigSet", "AGQU010000000")
	sample.child("Analysis", "ERZ26912061 (SEQUENCE_ASSEMBLY)")

	var out bytes.Buffer
	if err := writeLinks(&out, "GCA_000231155", []*linkTreeNode{root}, outputFormatTSV); err != nil {
		t.Fatal(err)
	}

	const want = "input_accession\tparent_type\tparent_accession\tparent_detail\tchild_type\tchild_accession\tchild_detail\n" +
		"GCA_000231155\tproject\tPRJNA123456\t.\tsample\tSAMN02471593\t.\n" +
		"GCA_000231155\tsample\tSAMN02471593\t.\tassembly\tGCA_000231155\t.\n" +
		"GCA_000231155\tassembly\tGCA_000231155\t.\tcontig_set\tAGQU010000000\t.\n" +
		"GCA_000231155\tsample\tSAMN02471593\t.\tanalysis\tERZ26912061\tSEQUENCE_ASSEMBLY\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}

func TestWriteLinksJSONWritesTree(t *testing.T) {
	root := newLinkTreeNode("Project", "PRJNA123456")
	sample := root.child("Sample", "SAMN02471593")
	sample.child("Analysis", "ERZ26912061 (SEQUENCE_ASSEMBLY)")

	var out bytes.Buffer
	if err := writeLinks(&out, "ERZ26912061", []*linkTreeNode{root}, outputFormatJSON); err != nil {
		t.Fatal(err)
	}

	var got linkJSONOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json output did not unmarshal: %v\n%s", err, out.String())
	}
	if got.InputAccession != "ERZ26912061" {
		t.Fatalf("input_accession = %q, want ERZ26912061", got.InputAccession)
	}
	if len(got.Links) != 1 || got.Links[0].Type != "project" || got.Links[0].Accession != "PRJNA123456" {
		t.Fatalf("root node = %#v", got.Links)
	}
	if len(got.Links[0].Children) != 1 || got.Links[0].Children[0].Type != "sample" {
		t.Fatalf("sample node = %#v", got.Links[0].Children)
	}
	analysis := got.Links[0].Children[0].Children[0]
	if analysis.Type != "analysis" || analysis.Accession != "ERZ26912061" || analysis.Detail != "SEQUENCE_ASSEMBLY" {
		t.Fatalf("analysis node = %#v", analysis)
	}
}

func assertNoRootSampleLines(t *testing.T, stdout string) {
	t.Helper()

	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if strings.HasPrefix(line, "Sample: ") {
			t.Fatalf("stdout has root-level sample line %q:\n%s", line, stdout)
		}
	}
}
