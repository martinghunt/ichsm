package ichsm

const BasePortalURL = "https://www.ebi.ac.uk/ena/portal/api/"
const BaseBrowserXMLURL = "https://www.ebi.ac.uk/ena/browser/api/xml/"

type searchEndpoint struct {
	mainType string
	result   string
}

var urlSearchData = map[AccessionType]searchEndpoint{
	AccessionTypeAssembly:   {mainType: "search", result: "assembly"},
	AccessionTypeWGSSet:     {mainType: "search", result: "wgs_set"},
	AccessionTypeTSASet:     {mainType: "search", result: "tsa_set"},
	AccessionTypeTLSSet:     {mainType: "search", result: "tls_set"},
	AccessionTypeSequence:   {mainType: "search", result: "sequence"},
	AccessionTypeCoding:     {mainType: "search", result: "coding"},
	AccessionTypeStudy:      {mainType: "search", result: "study"},
	AccessionTypeSample:     {mainType: "search", result: "sample"},
	AccessionTypeRun:        {mainType: "search", result: "read_run"},
	AccessionTypeExperiment: {mainType: "search", result: "read_run"},
	AccessionTypeAnalysis:   {mainType: "search", result: "analysis"},
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
