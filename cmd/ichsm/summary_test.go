package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestRunSummaryWritesTSV(t *testing.T) {
	server := withHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			query := r.URL.Query()
			if got := query.Get("result"); got != "study" {
				t.Fatalf("summary result = %q, want study", got)
			}
			if got := query.Get("query"); got != "study_accession=PRJEB1787 OR secondary_study_accession=PRJEB1787" {
				t.Fatalf("summary query = %q", got)
			}
			if got := query.Get("fields"); got != "study_accession,secondary_study_accession,description,study_title,project_name,study_description,center_name,broker_name,first_public,last_updated,scientific_name,tax_id" {
				t.Fatalf("summary fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"study_accession":"PRJEB1787","secondary_study_accession":"ERP001736","description":"Short project description","study_title":"Tara Oceans prokaryotes","study_description":"Long project description","first_public":"2013-04-12","last_updated":"2025-03-11","scientific_name":"marine metagenome","tax_id":"408172"}]`))
		case "/count":
			query := r.URL.Query()
			countQuery := query.Get("query")
			switch query.Get("result") {
			case "sample":
				if countQuery != "study_accession=PRJEB1787" {
					t.Fatalf("sample count query = %q", countQuery)
				}
				_, _ = w.Write([]byte(`{"count":"2"}`))
			case "read_run":
				if countQuery == "study_accession=PRJEB1787" {
					_, _ = w.Write([]byte(`{"count":"3"}`))
					return
				}
				switch countQuery {
				case "study_accession=PRJEB1787 AND instrument_platform=ILLUMINA":
					_, _ = w.Write([]byte(`{"count":"2"}`))
				case "study_accession=PRJEB1787 AND instrument_platform=LS454":
					_, _ = w.Write([]byte(`{"count":"1"}`))
				default:
					if !strings.HasPrefix(countQuery, "study_accession=PRJEB1787 AND instrument_platform=") {
						t.Fatalf("platform count query = %q", countQuery)
					}
					_, _ = w.Write([]byte(`{"count":"0"}`))
				}
			case "assembly":
				if countQuery != "study_accession=PRJEB1787" {
					t.Fatalf("assembly count query = %q", countQuery)
				}
				_, _ = w.Write([]byte(`{"count":"1"}`))
			case "analysis":
				if countQuery != "study_accession=PRJEB1787" {
					t.Fatalf("analysis count query = %q", countQuery)
				}
				_, _ = w.Write([]byte(`{"count":"0"}`))
			case "wgs_set":
				if countQuery != "study_accession=PRJEB1787" {
					t.Fatalf("wgs count query = %q", countQuery)
				}
				_, _ = w.Write([]byte(`{"count":"1"}`))
			case "tsa_set", "tls_set":
				if countQuery != "study_accession=PRJEB1787" {
					t.Fatalf("contig set count query = %q", countQuery)
				}
				_, _ = w.Write([]byte(`{"count":"0"}`))
			default:
				t.Fatalf("count result = %q", query.Get("result"))
			}
		case "/PRJEB1787":
			_, _ = w.Write([]byte(`<PROJECT_SET>
<PROJECT accession="PRJEB1787">
  <PROJECT_LINKS>
    <PROJECT_LINK>
      <XREF_LINK>
        <DB>PUBMED</DB>
        <ID>26863193</ID>
      </XREF_LINK>
    </PROJECT_LINK>
  </PROJECT_LINKS>
</PROJECT>
</PROJECT_SET>`))
		case "/esearch.fcgi":
			if got := r.URL.Query().Get("term"); got != "PRJEB1787[Project Accession]" {
				t.Fatalf("esearch term = %q", got)
			}
			_, _ = w.Write([]byte(`{"esearchresult":{"idlist":["196960"]}}`))
		case "/elink.fcgi":
			if got := r.URL.Query().Get("id"); got != "196960" {
				t.Fatalf("elink id = %q", got)
			}
			_, _ = w.Write([]byte(`{"linksets":[{"dbfrom":"bioproject","ids":["196960"]}]}`))
		case "/esummary.fcgi":
			if got := r.URL.Query().Get("db"); got != "pubmed" {
				t.Fatalf("esummary db = %q, want pubmed", got)
			}
			if got := r.URL.Query().Get("id"); got != "26863193" {
				t.Fatalf("esummary id = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"uids":["26863193"],"26863193":{"uid":"26863193","pubdate":"2016 Apr 28","source":"Nature","title":"Plankton networks driving carbon export in the oligotrophic ocean."}}}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	})

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"summary", "PRJEB1787", "--outfmt", "tsv"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tnormalized_accession\tinput_type\tresult_type\tsource\ttitle\tdescription\tscientific_names\ttax_ids\tproject_accessions\tsample_accessions\texperiment_accessions\trun_accessions\tassembly_accessions\tcontig_set_accessions\tanalysis_accessions\tplatforms\tplatform_counts\tlibrary_layouts\tfirst_public\tlast_updated\tsample_count\trun_count\tassembly_count\tanalysis_count\tcontig_set_count\tpublication_count\n" +
		"PRJEB1787\tPRJEB1787\tstudy\tstudy\tena\tTara Oceans prokaryotes\tLong project description\tmarine metagenome\t408172\tPRJEB1787;ERP001736\t.\t.\t.\t.\t.\t.\tILLUMINA;LS454\tILLUMINA:2;LS454:1\t.\t2013-04-12\t2025-03-11\t2\t3\t1\t0\t1\t1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
