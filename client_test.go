package ichsm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestQuerySampleAtRunLevel(t *testing.T) {
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
		if got := query.Get("format"); got != "json" {
			t.Fatalf("format = %q, want json", got)
		}
		if got := query.Get("fields"); got != "study_accession,secondary_study_accession,sample_accession,secondary_sample_accession,run_accession,description,instrument_platform,library_layout,fastq_ftp" {
			t.Fatalf("fields = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456","fastq_ftp":""}]`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "SAMN05276490", AccessionTypeSample, []string{"DEFAULT"}, AccessionTypeRun)
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeRun {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeRun)
	}
	if !reflect.DeepEqual(fields, runDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, runDefault)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0]["fastq_ftp"] != nil {
		t.Fatalf("empty string was not normalized to nil: %#v", records[0]["fastq_ftp"])
	}
}

func TestQueryStudy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "study" {
			t.Fatalf("result = %q, want study", got)
		}
		if got := query.Get("query"); got != "study_accession=PRJEB1787 OR secondary_study_accession=PRJEB1787" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("format"); got != "json" {
			t.Fatalf("format = %q, want json", got)
		}
		if got := query.Get("fields"); got != "study_accession,secondary_study_accession,description,study_title,project_name" {
			t.Fatalf("fields = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"study_accession":"PRJEB1787","secondary_study_accession":"ERP001736","study_title":"Tara Oceans"}]`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "PRJEB1787", AccessionTypeStudy, []string{"DEFAULT"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeStudy {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeStudy)
	}
	if !reflect.DeepEqual(fields, studyDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, studyDefault)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0]["secondary_study_accession"] != "ERP001736" {
		t.Fatalf("secondary_study_accession = %q", records[0]["secondary_study_accession"])
	}
}

func TestQueryPrimaryStudyAtRunLevelSkipsResolution(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run; primary study accessions should not be resolved first", got)
		}
		if got := query.Get("query"); got != "study_accession=PRJEB1787" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "run_accession" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"run_accession":"ERR123456"}]`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, _, records, err := client.Query(context.Background(), "PRJEB1787", AccessionTypeStudy, []string{"run_accession"}, AccessionTypeRun)
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeRun {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeRun)
	}
	if len(records) != 1 || records[0]["run_accession"] != "ERR123456" {
		t.Fatalf("records = %#v", records)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestCountPrimaryStudyAtRunLevelSkipsResolution(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/count" {
			t.Fatalf("path = %q, want /count", r.URL.Path)
		}
		query := r.URL.Query()
		if got := query.Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run; primary study accessions should not be resolved first", got)
		}
		if got := query.Get("query"); got != "study_accession=PRJEB1787" {
			t.Fatalf("query = %q", got)
		}
		_, _ = w.Write([]byte(`{"count":"2"}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, count, err := client.CountENA(context.Background(), "PRJEB1787", AccessionTypeStudy, AccessionTypeRun)
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeRun {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeRun)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestQueryWGSSet(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"accession":"AGQU01000000","wgs_set":"AGQU01","assembly_accession":"GCA_000231155"}]`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "AGQU01", AccessionTypeContigSet, []string{"DEFAULT"}, AccessionTypeAssembly)
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeWGSSet {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeWGSSet)
	}
	if !reflect.DeepEqual(fields, wgsSetDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, wgsSetDefault)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
	if records[0]["assembly_accession"] != "GCA_000231155" {
		t.Fatalf("assembly_accession = %q", records[0]["assembly_accession"])
	}
}

func TestQuerySequence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if got := query.Get("result"); got != "sequence" {
			t.Fatalf("result = %q, want sequence", got)
		}
		if got := query.Get("query"); got != "accession=U49845" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "accession,sequence_version,description,scientific_name,tax_id" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"accession":"U49845","sequence_version":"1","description":"test sequence","scientific_name":"Saccharomyces cerevisiae","tax_id":"4932"}]`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "U49845", AccessionTypeSequence, []string{"DEFAULT"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeSequence {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeSequence)
	}
	if !reflect.DeepEqual(fields, sequenceDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, sequenceDefault)
	}
	if records[0]["source"] != "ena" {
		t.Fatalf("source = %q, want ena", records[0]["source"])
	}
}

func TestQueryCoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if got := query.Get("result"); got != "coding" {
			t.Fatalf("result = %q, want coding", got)
		}
		if got := query.Get("query"); got != "accession=AAA98665" {
			t.Fatalf("query = %q", got)
		}
		if got := query.Get("fields"); got != "accession,protein_id,parent_accession,sequence_version,description,product,scientific_name,tax_id" {
			t.Fatalf("fields = %q", got)
		}
		_, _ = w.Write([]byte(`[{"accession":"AAA98665","protein_id":"AAA98665.1","parent_accession":"U49845","sequence_version":"1","description":"test protein","product":"TCP1-beta"}]`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "AAA98665", AccessionTypeCoding, []string{"DEFAULT"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if resultType != AccessionTypeCoding {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeCoding)
	}
	if !reflect.DeepEqual(fields, codingDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, codingDefault)
	}
	if records[0]["protein_id"] != "AAA98665.1" {
		t.Fatalf("protein_id = %q", records[0]["protein_id"])
	}
}

func TestQueryContigSetFallsBackToTSASet(t *testing.T) {
	var sawWGS bool
	var sawTSA bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("result") {
		case "wgs_set":
			sawWGS = true
			if got := query.Get("query"); got != "wgs_set=GHIQ01" {
				t.Fatalf("wgs query = %q", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "tsa_set":
			sawTSA = true
			if got := query.Get("query"); got != "accession=GHIQ01000000" {
				t.Fatalf("tsa query = %q", got)
			}
			if got := query.Get("fields"); got != "accession,sample_accession,sequence_version,description,scientific_name,tax_id,study_accession" {
				t.Fatalf("tsa fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"accession":"GHIQ01000000","sequence_version":"1","description":"test tsa"}]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "GHIQ01", AccessionTypeContigSet, []string{"DEFAULT"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !sawWGS || !sawTSA {
		t.Fatalf("sawWGS=%v sawTSA=%v", sawWGS, sawTSA)
	}
	if resultType != AccessionTypeTSASet {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeTSASet)
	}
	if !reflect.DeepEqual(fields, contigSetDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, contigSetDefault)
	}
	if records[0]["accession"] != "GHIQ01000000" {
		t.Fatalf("accession = %q", records[0]["accession"])
	}
}

func TestQueryWithSourceAutoFallsBackToNCBIAssembly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if got := r.URL.Query().Get("result"); got != "assembly" {
				t.Fatalf("ENA result = %q, want assembly", got)
			}
			_, _ = w.Write([]byte(`[]`))
		case "/esearch.fcgi":
			query := r.URL.Query()
			if got := query.Get("db"); got != "assembly" {
				t.Fatalf("NCBI db = %q, want assembly", got)
			}
			if got := query.Get("term"); got != "GCF_000001405.40[Assembly Accession]" {
				t.Fatalf("NCBI term = %q", got)
			}
			_, _ = w.Write([]byte(`{"esearchresult":{"idlist":["11968211"]}}`))
		case "/esummary.fcgi":
			if got := r.URL.Query().Get("id"); got != "11968211" {
				t.Fatalf("NCBI id = %q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"uids":["11968211"],"11968211":{"assemblyaccession":"GCF_000001405.40","speciesname":"Homo sapiens","taxid":9606,"biosampleaccn":"SAMN1","rs_bioprojects":[{"bioprojectaccn":"PRJNA168"}]}}}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", NCBIBaseURL: server.URL + "/", HTTPClient: server.Client()}
	source, resultType, fields, records, err := client.QueryWithSource(context.Background(), "GCF_000001405.40", "GCF_000001405", AccessionTypeAssembly, []string{"DEFAULT"}, "", SearchSourceAuto)
	if err != nil {
		t.Fatal(err)
	}
	if source != SearchSourceNCBI {
		t.Fatalf("source = %q, want %q", source, SearchSourceNCBI)
	}
	if resultType != AccessionTypeAssembly {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeAssembly)
	}
	if !reflect.DeepEqual(fields, assemblyDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, assemblyDefault)
	}
	if records[0]["accession"] != "GCF_000001405" || records[0]["version"] != "40" {
		t.Fatalf("record accession/version = %q/%q", records[0]["accession"], records[0]["version"])
	}
	if records[0]["source"] != "ncbi" {
		t.Fatalf("source field = %q", records[0]["source"])
	}
}

func TestQuerySecondaryStudyAtSampleLevel(t *testing.T) {
	var sawStudyLookup bool
	var sawSampleSearch bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}

		query := r.URL.Query()
		switch query.Get("result") {
		case "study":
			sawStudyLookup = true
			if got := query.Get("query"); got != "study_accession=ERP001736 OR secondary_study_accession=ERP001736" {
				t.Fatalf("study lookup query = %q", got)
			}
			if got := query.Get("fields"); got != "study_accession" {
				t.Fatalf("study lookup fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"study_accession":"PRJEB1787"}]`))
		case "sample":
			sawSampleSearch = true
			if got := query.Get("query"); got != "study_accession=PRJEB1787" {
				t.Fatalf("sample query = %q", got)
			}
			if got := query.Get("fields"); got != "sample_accession,description,secondary_sample_accession,study_accession,scientific_name,tax_id,collection_date,country" {
				t.Fatalf("sample fields = %q", got)
			}
			_, _ = w.Write([]byte(`[{"sample_accession":"SAMEA123","description":"sample description","secondary_sample_accession":"ERS478017","study_accession":"PRJEB1787","scientific_name":"marine metagenome","tax_id":"408172","country":"France"}]`))
		default:
			t.Fatalf("result = %q", query.Get("result"))
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, fields, records, err := client.Query(context.Background(), "ERP001736", AccessionTypeStudy, []string{"DEFAULT"}, AccessionTypeSample)
	if err != nil {
		t.Fatal(err)
	}
	if !sawStudyLookup || !sawSampleSearch {
		t.Fatalf("sawStudyLookup=%v sawSampleSearch=%v", sawStudyLookup, sawSampleSearch)
	}
	if resultType != AccessionTypeSample {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeSample)
	}
	if !reflect.DeepEqual(fields, sampleDefault) {
		t.Fatalf("fields = %#v, want %#v", fields, sampleDefault)
	}
	if len(records) != 1 {
		t.Fatalf("len(records) = %d, want 1", len(records))
	}
}

func TestCountENASecondaryStudyAtRunLevel(t *testing.T) {
	var sawStudyLookup bool
	var sawCount bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			sawStudyLookup = true
			query := r.URL.Query()
			if got := query.Get("result"); got != "study" {
				t.Fatalf("study result = %q, want study", got)
			}
			if got := query.Get("query"); got != "study_accession=ERP001736 OR secondary_study_accession=ERP001736" {
				t.Fatalf("study query = %q", got)
			}
			if got := query.Get("fields"); got != "study_accession" {
				t.Fatalf("study fields = %q", got)
			}
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
			if got := query.Get("format"); got != "json" {
				t.Fatalf("count format = %q, want json", got)
			}
			_, _ = w.Write([]byte(`{"count":"249"}`))
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	resultType, count, err := client.CountENA(context.Background(), "ERP001736", AccessionTypeStudy, AccessionTypeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !sawStudyLookup || !sawCount {
		t.Fatalf("sawStudyLookup=%v sawCount=%v", sawStudyLookup, sawCount)
	}
	if resultType != AccessionTypeRun {
		t.Fatalf("resultType = %q, want %q", resultType, AccessionTypeRun)
	}
	if count != 249 {
		t.Fatalf("count = %d, want 249", count)
	}
}

func TestResolveSearchLevelRejectsUnsupportedCombination(t *testing.T) {
	_, err := ResolveSearchLevel(AccessionTypeRun, AccessionTypeSample)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSearchKeyValueSupportsContigSetLinks(t *testing.T) {
	tests := []struct {
		name       string
		queryType  AccessionType
		resultType AccessionType
		accession  string
		wantValue  string
	}{
		{
			name:       "sample to wgs set",
			queryType:  AccessionTypeSample,
			resultType: AccessionTypeWGSSet,
			accession:  "SAMD00654312",
			wantValue:  "sample_accession=SAMD00654312 OR secondary_sample_accession=SAMD00654312",
		},
		{
			name:       "study to tsa set",
			queryType:  AccessionTypeStudy,
			resultType: AccessionTypeTSASet,
			accession:  "PRJEB123",
			wantValue:  "study_accession=PRJEB123",
		},
		{
			name:       "run to wgs set",
			queryType:  AccessionTypeRun,
			resultType: AccessionTypeWGSSet,
			accession:  "DRR510832",
			wantValue:  "run_accession=DRR510832",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, err := SearchKeyValue(tt.queryType, tt.resultType, tt.accession)
			if err != nil {
				t.Fatalf("SearchKeyValue() error = %v", err)
			}
			if key != "query" || value != tt.wantValue {
				t.Fatalf("SearchKeyValue() = %q, %q; want query, %q", key, value, tt.wantValue)
			}
			if got, err := ResolveSearchLevel(tt.queryType, tt.resultType); err != nil || got != tt.resultType {
				t.Fatalf("ResolveSearchLevel() = %q, %v; want %q, nil", got, err, tt.resultType)
			}
		})
	}
}

func TestFieldPresetsAreNested(t *testing.T) {
	for accessionType, presets := range fieldPresets {
		for _, field := range presets[FieldPresetSmall] {
			if !stringSliceContains(presets[FieldPresetDefault], field) {
				t.Fatalf("%s SMALL field %q is not in DEFAULT", accessionType, field)
			}
		}
		for _, field := range presets[FieldPresetDefault] {
			if !stringSliceContains(presets[FieldPresetBig], field) {
				t.Fatalf("%s DEFAULT field %q is not in BIG", accessionType, field)
			}
		}
	}
}

func TestFieldPresetLevelForResult(t *testing.T) {
	tests := []struct {
		resultType string
		field      string
		wantLevel  string
		wantOK     bool
	}{
		{resultType: "read_run", field: "run_accession", wantLevel: FieldPresetSmall, wantOK: true},
		{resultType: "read_run", field: "fastq_ftp", wantLevel: FieldPresetDefault, wantOK: true},
		{resultType: "read_run", field: "center_name", wantLevel: FieldPresetBig, wantOK: true},
		{resultType: "read_run", field: "age", wantLevel: FieldPresetAll, wantOK: true},
		{resultType: "analysis", field: "analysis_accession"},
	}

	for _, tt := range tests {
		t.Run(tt.resultType+"/"+tt.field, func(t *testing.T) {
			gotLevel, gotOK := FieldPresetLevelForResult(tt.resultType, tt.field)
			if gotLevel != tt.wantLevel || gotOK != tt.wantOK {
				t.Fatalf("FieldPresetLevelForResult() = %q, %v; want %q, %v", gotLevel, gotOK, tt.wantLevel, tt.wantOK)
			}
		})
	}
}

func TestGetAllowedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/searchFields" {
			t.Fatalf("path = %q, want /searchFields", r.URL.Path)
		}
		if got := r.URL.Query().Get("result"); got != "read_run" {
			t.Fatalf("result = %q, want read_run", got)
		}
		_, _ = w.Write([]byte("field\tdescription\nrun_accession\tRun accession\n"))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	text, err := client.GetAllowedFields(context.Background(), "read_run")
	if err != nil {
		t.Fatal(err)
	}
	if text != "field\tdescription\nrun_accession\tRun accession\n" {
		t.Fatalf("text = %q", text)
	}
}

func TestGetResultTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/results" {
			t.Fatalf("path = %q, want /results", r.URL.Path)
		}
		_, _ = w.Write([]byte("resultId\tdescription\nread_run\tRaw reads\n"))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL + "/", HTTPClient: server.Client()}
	text, err := client.GetResultTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if text != "resultId\tdescription\nread_run\tRaw reads\n" {
		t.Fatalf("text = %q", text)
	}
}

func TestSearchRejectsMixedAccessionTypes(t *testing.T) {
	client := &Client{}
	_, err := client.Search(context.Background(), SearchOptions{
		Accessions: []string{"SAMN123456", "ERR123456"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
