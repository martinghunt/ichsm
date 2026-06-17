package ichsm

const BasePortalURL = "https://www.ebi.ac.uk/ena/portal/api/"

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
}

var assemblySmall = []string{"accession", "sample_accession", "run_accession", "version"}
var assemblyDefault = append(copyStrings(assemblySmall), "scientific_name", "tax_id")

var wgsSetSmall = []string{"accession", "wgs_set", "assembly_accession", "sample_accession", "run_accession", "sequence_version"}
var wgsSetDefault = append(copyStrings(wgsSetSmall), "scientific_name", "tax_id")

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
var runDefault = append(copyStrings(runSmall), "instrument_platform", "library_layout", "fastq_ftp")
var runBig = append(copyStrings(runDefault),
	"center_name",
	"broker_name",
	"read_count",
	"base_count",
	"collection_date",
	"scientific_name",
)

var fieldPresets = map[AccessionType]map[string][]string{
	AccessionTypeAssembly: {
		"SMALL":   assemblySmall,
		"DEFAULT": assemblyDefault,
		"BIG":     assemblyDefault,
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
}

func copyStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}
