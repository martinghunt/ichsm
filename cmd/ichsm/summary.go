package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

type summaryOptions struct {
	outfmt string
	source string
	apiKey string
	email  string
}

type summaryResult struct {
	InputAccession       string              `json:"input_accession"`
	NormalizedAccession  string              `json:"normalized_accession"`
	InputType            ichsm.AccessionType `json:"input_type"`
	ResultType           ichsm.AccessionType `json:"result_type"`
	Source               ichsm.SearchSource  `json:"source"`
	Title                string              `json:"title"`
	Description          string              `json:"description"`
	ScientificNames      []string            `json:"scientific_names"`
	TaxIDs               []string            `json:"tax_ids"`
	ProjectAccessions    []string            `json:"project_accessions"`
	SampleAccessions     []string            `json:"sample_accessions"`
	ExperimentAccessions []string            `json:"experiment_accessions"`
	RunAccessions        []string            `json:"run_accessions"`
	AssemblyAccessions   []string            `json:"assembly_accessions"`
	ContigSetAccessions  []string            `json:"contig_set_accessions"`
	AnalysisAccessions   []string            `json:"analysis_accessions"`
	Platforms            []string            `json:"platforms"`
	PlatformCounts       map[string]int      `json:"platform_counts"`
	LibraryLayouts       []string            `json:"library_layouts"`
	FirstPublic          string              `json:"first_public"`
	LastUpdated          string              `json:"last_updated"`
	SampleCount          *int                `json:"sample_count"`
	RunCount             *int                `json:"run_count"`
	AssemblyCount        *int                `json:"assembly_count"`
	AnalysisCount        *int                `json:"analysis_count"`
	ContigSetCount       *int                `json:"contig_set_count"`
	PublicationCount     *int                `json:"publication_count"`
}

var summaryColumns = []string{
	"input_accession",
	"normalized_accession",
	"input_type",
	"result_type",
	"source",
	"title",
	"description",
	"scientific_names",
	"tax_ids",
	"project_accessions",
	"sample_accessions",
	"experiment_accessions",
	"run_accessions",
	"assembly_accessions",
	"contig_set_accessions",
	"analysis_accessions",
	"platforms",
	"platform_counts",
	"library_layouts",
	"first_public",
	"last_updated",
	"sample_count",
	"run_count",
	"assembly_count",
	"analysis_count",
	"contig_set_count",
	"publication_count",
}

var summaryRunPlatforms = []string{
	"ABI_SOLID",
	"BGISEQ",
	"CAPILLARY",
	"COMPLETE_GENOMICS",
	"DNBSEQ",
	"HELICOS",
	"ILLUMINA",
	"ION_TORRENT",
	"LS454",
	"OXFORD_NANOPORE",
	"PACBIO_SMRT",
	"ULTIMA",
	"UNSPECIFIED",
}

func newSummaryCommand() *cobra.Command {
	opts := summaryOptions{
		outfmt: outputFormatTTable,
		source: string(ichsm.SearchSourceAuto),
	}
	cmd := &cobra.Command{
		Use:   "summary [accession]",
		Short: "Summarize an accession with linked IDs and counts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSummary(cmd, args[0], opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	flags.StringVar(&opts.source, "source", opts.source, "Metadata source: auto, ena, or ncbi")
	flags.StringVar(&opts.apiKey, "api-key", "", "NCBI API key; defaults to NCBI_API_KEY")
	flags.StringVar(&opts.email, "email", "", "Email address sent to NCBI; defaults to NCBI_EMAIL")
	return cmd
}

func executeSummary(cmd *cobra.Command, accession string, opts summaryOptions) error {
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}
	source, err := parseSearchSource(opts.source)
	if err != nil {
		return err
	}

	client := newClient()
	if opts.apiKey == "" {
		opts.apiKey = os.Getenv("NCBI_API_KEY")
	}
	if opts.email == "" {
		opts.email = os.Getenv("NCBI_EMAIL")
	}
	client.NCBIAPIKey = opts.apiKey
	client.NCBIEmail = opts.email

	summary, err := summarizeAccession(cmd.Context(), client, accession, source)
	if err != nil {
		return err
	}
	return writeSummaryResults(cmd.OutOrStdout(), []summaryResult{summary}, outfmt)
}

func summarizeAccession(ctx context.Context, client *ichsm.Client, accession string, source ichsm.SearchSource) (summaryResult, error) {
	inputAccession := strings.TrimSpace(accession)
	fixedAccession, accessionType, ok := ichsm.IdentifyAccession(inputAccession)
	if !ok {
		return summaryResult{}, fmt.Errorf("accession format not recognised: %s", accession)
	}

	resultSource, resultType, _, records, err := client.QueryWithSource(ctx, inputAccession, fixedAccession, accessionType, []string{"BIG"}, "", source)
	if err != nil {
		return summaryResult{}, fmt.Errorf("error getting summary data for accession %s: %w", inputAccession, err)
	}
	if len(records) == 0 {
		return summaryResult{}, fmt.Errorf("no results returned for accession %s", inputAccession)
	}

	summary := summaryResult{
		InputAccession:      inputAccession,
		NormalizedAccession: fixedAccession,
		InputType:           accessionType,
		ResultType:          resultType,
		Source:              resultSource,
	}
	addSummaryRecords(&summary, records, resultType)

	if resultSource != ichsm.SearchSourceNCBI {
		summary.SampleCount = summaryCount(ctx, client, fixedAccession, accessionType, ichsm.AccessionTypeSample)
		summary.RunCount = summaryCount(ctx, client, fixedAccession, accessionType, ichsm.AccessionTypeRun)
		summary.AssemblyCount = summaryCount(ctx, client, fixedAccession, accessionType, ichsm.AccessionTypeAssembly)
		summary.AnalysisCount = summaryCount(ctx, client, fixedAccession, accessionType, ichsm.AccessionTypeAnalysis)
		summary.ContigSetCount = summaryContigSetCount(ctx, client, fixedAccession, accessionType)
		summary.PlatformCounts = summaryPlatformCounts(ctx, client, fixedAccession, accessionType, summary.RunCount)
		for _, platform := range orderedSummaryCountKeys(summary.PlatformCounts) {
			if platform != "OTHER" {
				summary.Platforms = appendUniqueStringValue(summary.Platforms, platform)
			}
		}
	}
	if accessionType == ichsm.AccessionTypeStudy {
		summary.PublicationCount = summaryPublicationCount(ctx, client, fixedAccession)
	}

	return summary, nil
}

func addSummaryRecords(summary *summaryResult, records []ichsm.Record, resultType ichsm.AccessionType) {
	for _, record := range records {
		if summary.Title == "" {
			summary.Title = firstSummaryString(record, "study_title", "project_name", "assembly_name", "analysis_title", "description", "product", "title")
		}
		if summary.Description == "" {
			summary.Description = firstSummaryString(record, "study_description", "description", "analysis_description", "assemblydescription", "title")
		}
		if summary.FirstPublic == "" {
			summary.FirstPublic = firstSummaryString(record, "first_public")
		}
		if summary.LastUpdated == "" {
			summary.LastUpdated = firstSummaryString(record, "last_updated")
		}

		summary.ScientificNames = appendUniqueSummaryValues(summary.ScientificNames, record, "scientific_name", "organism", "speciesname")
		summary.TaxIDs = appendUniqueSummaryValues(summary.TaxIDs, record, "tax_id", "taxid", "speciestaxid")
		summary.ProjectAccessions = appendUniqueSummaryValues(summary.ProjectAccessions, record, "study_accession", "secondary_study_accession", "project_accession")
		summary.SampleAccessions = appendUniqueSummaryValues(summary.SampleAccessions, record, "sample_accession", "secondary_sample_accession", "biosampleaccn")
		summary.ExperimentAccessions = appendUniqueSummaryValues(summary.ExperimentAccessions, record, "experiment_accession")
		summary.RunAccessions = appendUniqueSummaryValues(summary.RunAccessions, record, "run_accession")
		summary.AssemblyAccessions = appendUniqueSummaryValues(summary.AssemblyAccessions, record, "assembly_accession")
		summary.ContigSetAccessions = appendUniqueSummaryValues(summary.ContigSetAccessions, record, "wgs_set")
		summary.AnalysisAccessions = appendUniqueSummaryValues(summary.AnalysisAccessions, record, "analysis_accession")
		summary.Platforms = appendUniqueSummaryValues(summary.Platforms, record, "instrument_platform")
		summary.LibraryLayouts = appendUniqueSummaryValues(summary.LibraryLayouts, record, "library_layout")

		accession := recordLinkString(record, "accession")
		switch resultType {
		case ichsm.AccessionTypeAssembly:
			summary.AssemblyAccessions = appendUniqueStringValue(summary.AssemblyAccessions, accession)
		case ichsm.AccessionTypeContigSet, ichsm.AccessionTypeWGSSet, ichsm.AccessionTypeTSASet, ichsm.AccessionTypeTLSSet:
			summary.ContigSetAccessions = appendUniqueStringValue(summary.ContigSetAccessions, accession)
		case ichsm.AccessionTypeAnalysis:
			summary.AnalysisAccessions = appendUniqueStringValue(summary.AnalysisAccessions, accession)
		}
	}
}

func firstSummaryString(record ichsm.Record, keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(formatValue(record[key]))
		if value != "" && value != "." && value != "null" {
			return value
		}
	}
	return ""
}

func appendUniqueSummaryValues(values []string, record ichsm.Record, keys ...string) []string {
	for _, key := range keys {
		for _, value := range recordLinkValues(record, key) {
			values = appendUniqueStringValue(values, value)
		}
	}
	return values
}

func appendUniqueStringValue(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == "null" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func summaryCount(ctx context.Context, client *ichsm.Client, accession string, accessionType ichsm.AccessionType, level ichsm.AccessionType) *int {
	_, count, err := client.CountENA(ctx, accession, accessionType, level)
	if err != nil {
		return nil
	}
	return &count
}

func summaryContigSetCount(ctx context.Context, client *ichsm.Client, accession string, accessionType ichsm.AccessionType) *int {
	levels := []ichsm.AccessionType{ichsm.AccessionTypeWGSSet, ichsm.AccessionTypeTSASet, ichsm.AccessionTypeTLSSet}
	var total int
	var ok bool
	for _, level := range levels {
		_, count, err := client.CountENA(ctx, accession, accessionType, level)
		if err != nil {
			continue
		}
		total += count
		ok = true
	}
	if !ok {
		return nil
	}
	return &total
}

func summaryPublicationCount(ctx context.Context, client *ichsm.Client, accession string) *int {
	publications, err := client.Publications(ctx, accession)
	if err != nil {
		return nil
	}
	count := len(publications)
	return &count
}

func summaryPlatformCounts(ctx context.Context, client *ichsm.Client, accession string, accessionType ichsm.AccessionType, runCount *int) map[string]int {
	counts := map[string]int{}
	total := 0
	for _, platform := range summaryRunPlatforms {
		_, count, err := client.CountENAFiltered(ctx, accession, accessionType, ichsm.AccessionTypeRun, map[string]string{
			"instrument_platform": platform,
		})
		if err != nil || count == 0 {
			continue
		}
		counts[platform] = count
		total += count
	}
	if runCount != nil && total < *runCount {
		counts["OTHER"] = *runCount - total
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func writeSummaryResults(out io.Writer, summaries []summaryResult, outfmt string) error {
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, summaries)
	}

	return writeRowsForOutputFormat(out, summaryRows(summaries), outfmt)
}

func summaryRows(summaries []summaryResult) [][]string {
	rows := [][]string{append([]string(nil), summaryColumns...)}
	for _, summary := range summaries {
		rows = append(rows, []string{
			summary.InputAccession,
			summary.NormalizedAccession,
			string(summary.InputType),
			string(summary.ResultType),
			string(summary.Source),
			formatSummaryCell(summary.Title),
			formatSummaryCell(summary.Description),
			formatSummaryValues(summary.ScientificNames),
			formatSummaryValues(summary.TaxIDs),
			formatSummaryValues(summary.ProjectAccessions),
			formatSummaryValues(summary.SampleAccessions),
			formatSummaryValues(summary.ExperimentAccessions),
			formatSummaryValues(summary.RunAccessions),
			formatSummaryValues(summary.AssemblyAccessions),
			formatSummaryValues(summary.ContigSetAccessions),
			formatSummaryValues(summary.AnalysisAccessions),
			formatSummaryValues(summary.Platforms),
			formatSummaryCountMap(summary.PlatformCounts),
			formatSummaryValues(summary.LibraryLayouts),
			formatSummaryCell(summary.FirstPublic),
			formatSummaryCell(summary.LastUpdated),
			formatSummaryCount(summary.SampleCount),
			formatSummaryCount(summary.RunCount),
			formatSummaryCount(summary.AssemblyCount),
			formatSummaryCount(summary.AnalysisCount),
			formatSummaryCount(summary.ContigSetCount),
			formatSummaryCount(summary.PublicationCount),
		})
	}
	return rows
}

func formatSummaryCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "."
	}
	return value
}

func formatSummaryValues(values []string) string {
	if len(values) == 0 {
		return "."
	}
	return strings.Join(values, ";")
}

func formatSummaryCount(count *int) string {
	if count == nil {
		return "."
	}
	return fmt.Sprint(*count)
}

func formatSummaryCountMap(counts map[string]int) string {
	keys := orderedSummaryCountKeys(counts)
	if len(keys) == 0 {
		return "."
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, counts[key]))
	}
	return strings.Join(parts, ";")
}

func orderedSummaryCountKeys(counts map[string]int) []string {
	if len(counts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(counts))
	seen := map[string]bool{}
	for _, platform := range summaryRunPlatforms {
		if _, ok := counts[platform]; ok {
			keys = append(keys, platform)
			seen[platform] = true
		}
	}
	if _, ok := counts["OTHER"]; ok {
		keys = append(keys, "OTHER")
		seen["OTHER"] = true
	}
	for key := range counts {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}
