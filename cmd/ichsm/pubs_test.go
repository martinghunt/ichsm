package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunPubsWritesParentProjectPublications(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/PRJEB1787":
			_, _ = w.Write([]byte(`<PROJECT_SET>
<PROJECT accession="PRJEB1787">
  <RELATED_PROJECTS>
    <RELATED_PROJECT>
      <PARENT_PROJECT accession="PRJEB402"/>
    </RELATED_PROJECT>
  </RELATED_PROJECTS>
</PROJECT>
</PROJECT_SET>`))
		case "/PRJEB402":
			_, _ = w.Write([]byte(`<PROJECT_SET>
<PROJECT accession="PRJEB402">
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
			if got := r.URL.Query().Get("db"); got != "bioproject" {
				t.Fatalf("esearch db = %q, want bioproject", got)
			}
			switch got := r.URL.Query().Get("term"); got {
			case "PRJEB1787[Project Accession]":
				_, _ = w.Write([]byte(`{"esearchresult":{"idlist":["196960"]}}`))
			case "PRJEB402[Project Accession]":
				_, _ = w.Write([]byte(`{"esearchresult":{"idlist":["173486"]}}`))
			default:
				t.Fatalf("esearch term = %q", got)
			}
		case "/elink.fcgi":
			if got := r.URL.Query().Get("dbfrom"); got != "bioproject" {
				t.Fatalf("elink dbfrom = %q, want bioproject", got)
			}
			if got := r.URL.Query().Get("db"); got != "pubmed" {
				t.Fatalf("elink db = %q, want pubmed", got)
			}
			switch got := r.URL.Query().Get("id"); got {
			case "196960":
				_, _ = w.Write([]byte(`{"linksets":[{"dbfrom":"bioproject","ids":["196960"]}]}`))
			case "173486":
				_, _ = w.Write([]byte(`{"linksets":[{"dbfrom":"bioproject","ids":["173486"],"linksetdbs":[{"dbto":"pubmed","links":["27654921","26863193"]}]}]}`))
			default:
				t.Fatalf("elink id = %q", got)
			}
		case "/esummary.fcgi":
			if got := r.URL.Query().Get("db"); got != "pubmed" {
				t.Fatalf("esummary db = %q, want pubmed", got)
			}
			if got := r.URL.Query().Get("id"); got != "26863193,27654921" {
				t.Fatalf("esummary id = %q, want 26863193,27654921", got)
			}
			_, _ = w.Write([]byte(`{"result":{"uids":["26863193","27654921"],"26863193":{"uid":"26863193","pubdate":"2016 Apr 28","source":"Nature","title":"Plankton networks driving carbon export in the oligotrophic ocean.","articleids":[{"idtype":"doi","value":"10.1038/nature16942"}]},"27654921":{"uid":"27654921","pubdate":"2016 Sep 9","source":"Science","title":"A global ocean atlas of eukaryotic genes.","articleids":[{"idtype":"doi","value":"10.1126/science.aaf4387"}]}}}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	withTestClient(t, server)
	code, stdout := captureStdout(t, func() int {
		return run([]string{"pubs", "PRJEB1787", "--outfmt", "tsv"})
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	const want = "input_accession\tproject_accession\trelation\tsources\tpubmed_id\tyear\tjournal\tdoi\ttitle\n" +
		"PRJEB1787\tPRJEB402\tparent\tena,ncbi\t26863193\t2016\tNature\t10.1038/nature16942\tPlankton networks driving carbon export in the oligotrophic ocean.\n" +
		"PRJEB1787\tPRJEB402\tparent\tncbi\t27654921\t2016\tScience\t10.1126/science.aaf4387\tA global ocean atlas of eukaryotic genes.\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunPubsRejectsNonStudyAccession(t *testing.T) {
	code, stdout := captureStdout(t, func() int {
		return run([]string{"pubs", "SAMN05276490"})
	})

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}
