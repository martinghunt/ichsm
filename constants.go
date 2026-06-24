package ichsm

import "strings"

const BasePortalURL = "https://www.ebi.ac.uk/ena/portal/api/"
const BaseBrowserXMLURL = "https://www.ebi.ac.uk/ena/browser/api/xml/"

type searchEndpoint struct {
	mainType string
	result   string
}

type accessionTypeInfo struct {
	accessionType AccessionType
	enaResult     string
	enaSearch     bool
	ncbiSearch    bool
	ncbiDatabase  string
	ncbiBrowser   bool
}

var accessionTypeData = []accessionTypeInfo{
	{accessionType: AccessionTypeAssembly, enaResult: "assembly", enaSearch: true, ncbiSearch: true, ncbiDatabase: "assembly", ncbiBrowser: true},
	{accessionType: AccessionTypeContigSet, enaSearch: true, ncbiSearch: true, ncbiBrowser: true},
	{accessionType: AccessionTypeWGSSet, enaResult: "wgs_set", enaSearch: true, ncbiSearch: true, ncbiBrowser: true},
	{accessionType: AccessionTypeTSASet, enaResult: "tsa_set", enaSearch: true, ncbiSearch: true, ncbiBrowser: true},
	{accessionType: AccessionTypeTLSSet, enaResult: "tls_set", enaSearch: true, ncbiSearch: true, ncbiBrowser: true},
	{accessionType: AccessionTypeSequence, enaResult: "sequence", enaSearch: true, ncbiSearch: true, ncbiDatabase: "nuccore", ncbiBrowser: true},
	{accessionType: AccessionTypeCoding, enaResult: "coding", enaSearch: true, ncbiSearch: true, ncbiDatabase: "protein", ncbiBrowser: true},
	{accessionType: AccessionTypeStudy, enaResult: "study", enaSearch: true},
	{accessionType: AccessionTypeSample, enaResult: "sample", enaSearch: true},
	{accessionType: AccessionTypeRun, enaResult: "read_run", enaSearch: true, ncbiBrowser: true},
	{accessionType: AccessionTypeExperiment, enaResult: "read_run", enaSearch: true},
	{accessionType: AccessionTypeAnalysis, enaResult: "analysis", enaSearch: true},
}

var accessionTypeIndex = indexAccessionTypes(accessionTypeData)
var resultTypeIndex = indexResultTypes(accessionTypeData)
var urlSearchData = makeURLSearchData(accessionTypeData)

func indexAccessionTypes(data []accessionTypeInfo) map[AccessionType]accessionTypeInfo {
	out := make(map[AccessionType]accessionTypeInfo, len(data))
	for _, info := range data {
		out[info.accessionType] = info
	}
	return out
}

func indexResultTypes(data []accessionTypeInfo) map[string]accessionTypeInfo {
	out := make(map[string]accessionTypeInfo)
	for _, info := range data {
		if info.enaResult == "" {
			continue
		}
		if _, ok := out[info.enaResult]; ok {
			continue
		}
		out[info.enaResult] = info
	}
	return out
}

func makeURLSearchData(data []accessionTypeInfo) map[AccessionType]searchEndpoint {
	out := make(map[AccessionType]searchEndpoint)
	for _, info := range data {
		if info.enaResult == "" {
			continue
		}
		out[info.accessionType] = searchEndpoint{mainType: "search", result: info.enaResult}
	}
	return out
}

func accessionTypeForResult(resultType string) (AccessionType, bool) {
	info, ok := resultTypeIndex[resultType]
	return info.accessionType, ok
}

func supportsENA(accessionType AccessionType) bool {
	info, ok := accessionTypeIndex[accessionType]
	return ok && info.enaSearch
}

func supportsENAResult(resultType string) bool {
	info, ok := resultTypeIndex[resultType]
	return ok && info.enaSearch
}

// SupportsENAResult reports whether ichsm has an ENA search route for an ENA result type.
func SupportsENAResult(resultType string) bool {
	return supportsENAResult(resultType)
}

// NormalizeENAResult returns the ichsm accession type and concrete ENA result
// id for an ichsm result alias such as "run" or an ENA result id such as
// "read_run".
func NormalizeENAResult(result string) (AccessionType, string, bool) {
	result = strings.ToLower(strings.TrimSpace(result))
	if result == "" {
		return "", "", false
	}
	if info, ok := resultTypeIndex[result]; ok && info.enaSearch {
		return info.accessionType, info.enaResult, true
	}
	accessionType := AccessionType(result)
	if info, ok := accessionTypeIndex[accessionType]; ok && info.enaSearch && info.enaResult != "" {
		return info.accessionType, info.enaResult, true
	}
	return "", "", false
}

func supportsNCBI(accessionType AccessionType) bool {
	info, ok := accessionTypeIndex[accessionType]
	return ok && info.ncbiSearch
}

// SupportsNCBIBrowser reports whether ichsm can build an NCBI browser URL for an accession type.
func SupportsNCBIBrowser(accessionType AccessionType) bool {
	info, ok := accessionTypeIndex[accessionType]
	return ok && info.ncbiBrowser
}

func ncbiDatabase(resultType AccessionType) (string, bool) {
	info, ok := accessionTypeIndex[resultType]
	return info.ncbiDatabase, ok && info.ncbiDatabase != ""
}

var assemblySmall = []string{"accession", "sample_accession", "run_accession", "version"}
var assemblyDefault = append(copyStrings(assemblySmall), "description", "study_accession", "scientific_name", "tax_id")
var assemblyBig = append(copyStrings(assemblyDefault),
	"wgs_set",
	"assembly_name",
	"assembly_level",
	"assembly_type",
	"assembly_software",
	"coverage",
	"genome_representation",
	"center_name",
	"country",
	"strain",
	"isolate",
	"last_updated",
)

var wgsSetSmall = []string{"accession", "wgs_set", "assembly_accession", "sample_accession", "run_accession", "sequence_version"}
var wgsSetDefault = append(copyStrings(wgsSetSmall), "description", "study_accession", "scientific_name", "tax_id")

var contigSetSmall = []string{"accession", "sample_accession", "sequence_version"}
var contigSetDefault = append(copyStrings(contigSetSmall), "description", "scientific_name", "tax_id", "study_accession")

var sequenceSmall = []string{"accession", "sequence_version"}
var sequenceDefault = append(copyStrings(sequenceSmall), "description", "scientific_name", "tax_id")
var sequenceBig = append(copyStrings(sequenceDefault),
	"sample_accession",
	"study_accession",
	"assembly_accession",
	"base_count",
	"mol_type",
)

var codingSmall = []string{"accession", "protein_id", "parent_accession", "sequence_version"}
var codingDefault = append(copyStrings(codingSmall), "description", "product", "scientific_name", "tax_id")
var codingBig = append(copyStrings(codingDefault),
	"sample_accession",
	"study_accession",
	"gene",
	"locus_tag",
	"transl_table",
)

var studySmall = []string{
	"study_accession",
	"secondary_study_accession",
}
var studyDefault = append(copyStrings(studySmall),
	"description",
	"study_title",
	"project_name",
)
var studyBig = append(copyStrings(studyDefault),
	"study_description",
	"center_name",
	"broker_name",
	"first_public",
	"last_updated",
	"scientific_name",
	"tax_id",
)

var sampleSmall = []string{
	"study_accession",
	"sample_accession",
}
var sampleDefault = []string{
	"sample_accession",
	"description",
	"secondary_sample_accession",
	"study_accession",
	"scientific_name",
	"tax_id",
	"collection_date",
	"country",
}
var sampleBig = append(copyStrings(sampleDefault),
	"center_name",
	"broker_name",
)

var runSmall = []string{
	"study_accession",
	"secondary_study_accession",
	"sample_accession",
	"secondary_sample_accession",
	"run_accession",
}
var runDefault = append(copyStrings(runSmall), "description", "instrument_platform", "library_layout", "fastq_ftp")
var runBig = append(copyStrings(runDefault),
	"center_name",
	"broker_name",
	"read_count",
	"base_count",
	"collection_date",
	"scientific_name",
)

var analysisSmall = []string{
	"study_accession",
	"sample_accession",
	"analysis_accession",
	"analysis_type",
}
var analysisDefault = append(copyStrings(analysisSmall),
	"analysis_title",
	"analysis_description",
)
var analysisBig = append(copyStrings(analysisDefault),
	"secondary_study_accession",
	"secondary_sample_accession",
	"experiment_accession",
	"run_accession",
	"assembly_software",
	"pipeline_name",
	"pipeline_version",
	"center_name",
	"broker_name",
	"first_public",
	"last_updated",
	"scientific_name",
	"tax_id",
)

var fieldPresets = map[AccessionType]map[string][]string{
	AccessionTypeAssembly: {
		"SMALL":   assemblySmall,
		"DEFAULT": assemblyDefault,
		"BIG":     assemblyBig,
	},
	AccessionTypeContigSet: {
		"SMALL":   contigSetSmall,
		"DEFAULT": contigSetDefault,
		"BIG":     contigSetDefault,
	},
	AccessionTypeWGSSet: {
		"SMALL":   wgsSetSmall,
		"DEFAULT": wgsSetDefault,
		"BIG":     wgsSetDefault,
	},
	AccessionTypeTSASet: {
		"SMALL":   contigSetSmall,
		"DEFAULT": contigSetDefault,
		"BIG":     contigSetDefault,
	},
	AccessionTypeTLSSet: {
		"SMALL":   contigSetSmall,
		"DEFAULT": contigSetDefault,
		"BIG":     contigSetDefault,
	},
	AccessionTypeSequence: {
		"SMALL":   sequenceSmall,
		"DEFAULT": sequenceDefault,
		"BIG":     sequenceBig,
	},
	AccessionTypeCoding: {
		"SMALL":   codingSmall,
		"DEFAULT": codingDefault,
		"BIG":     codingBig,
	},
	AccessionTypeStudy: {
		"SMALL":   studySmall,
		"DEFAULT": studyDefault,
		"BIG":     studyBig,
	},
	AccessionTypeSample: {
		"SMALL":   sampleSmall,
		"DEFAULT": sampleDefault,
		"BIG":     sampleBig,
	},
	AccessionTypeRun: {
		"SMALL":   runSmall,
		"DEFAULT": runDefault,
		"BIG":     runBig,
	},
	AccessionTypeExperiment: {
		"SMALL":   runSmall,
		"DEFAULT": runDefault,
		"BIG":     runBig,
	},
	AccessionTypeAnalysis: {
		"SMALL":   analysisSmall,
		"DEFAULT": analysisDefault,
		"BIG":     analysisBig,
	},
}

func copyStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}
