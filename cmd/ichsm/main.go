package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

var version = "local"
var newClient = ichsm.NewClient

const (
	outputFormatJSON   = "json"
	outputFormatTable  = "table"
	outputFormatTTable = "ttable"
	outputFormatTSV    = "tsv"
	outputFormatTTSV   = "ttsv"
)

const largeJSONRecordWarningThreshold = 1000

func main() {
	log.SetPrefix("[ichsm] ")
	log.SetFlags(0)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cmd := newRootCommand(os.Stdout, os.Stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}

func newRootCommand(out io.Writer, errOut io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ichsm",
		Short:         "Find sequence metadata from ENA and NCBI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.AddCommand(newSearchCommand(), newSummaryCommand(), newReadsCommand(), newLinksCommand(), newPubsCommand(), newOpenCommand(), newIdentifyCommand(), newGetFieldsCommand())
	return cmd
}

type searchOptions struct {
	accession string
	accFile   string
	columns   string
	level     string
	source    string
	apiKey    string
	email     string
	outfmt    string
	count     bool
	debug     bool
}

func newSearchCommand() *cobra.Command {
	opts := searchOptions{
		columns: "DEFAULT",
		outfmt:  outputFormatTSV,
	}
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search ENA and NCBI metadata for accessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeSearch(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.debug, "debug", false, "More verbose logging")
	flags.StringVarP(&opts.accession, "accession", "a", "", "Accession to search for")
	flags.StringVarP(&opts.accFile, "acc-file", "f", "", "File of accessions to search for, one per line")
	flags.StringVar(&opts.accFile, "acc_file", "", "File of accessions to search for, one per line")
	flags.StringVarP(&opts.columns, "columns", "c", opts.columns, "Columns/fields to output, comma-separated, or SMALL, DEFAULT, BIG, ALL")
	flags.StringVar(&opts.columns, "fields", opts.columns, "Columns/fields to output, comma-separated, or SMALL, DEFAULT, BIG, ALL")
	flags.StringVar(&opts.level, "level", "", "Output level: study, sample, run, assembly, sequence, coding, contig_set, wgs_set, tsa_set, or tls_set. Default is the input accession level")
	flags.StringVar(&opts.source, "source", string(ichsm.SearchSourceAuto), "Metadata source: auto, ena, or ncbi")
	flags.StringVar(&opts.apiKey, "api-key", "", "NCBI API key; defaults to NCBI_API_KEY")
	flags.StringVar(&opts.email, "email", "", "Email address sent to NCBI; defaults to NCBI_EMAIL")
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	flags.BoolVar(&opts.count, "count", false, "Only count matching ENA records; do not fetch metadata")
	_ = flags.MarkHidden("acc_file")

	return cmd
}

func executeSearch(cmd *cobra.Command, opts searchOptions) error {
	if (opts.accession == "") == (opts.accFile == "") {
		return fmt.Errorf("exactly one of -a/--accession or -f/--acc_file is required")
	}
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}

	accessions, err := accessionsFromInputs(opts.accession, opts.accFile)
	if err != nil {
		return err
	}

	fields := strings.Split(opts.columns, ",")
	level, err := parseSearchLevel(opts.level)
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

	if opts.count {
		counts, err := countAccessions(cmd.Context(), client, accessions, level, source, cmd.ErrOrStderr())
		if err != nil {
			return err
		}
		return writeCountResults(cmd.OutOrStdout(), counts, outfmt)
	}

	results, err := searchAccessions(cmd.Context(), client, accessions, fields, level, source, opts.debug, cmd.ErrOrStderr(), outfmt == outputFormatJSON)
	if err != nil {
		return err
	}

	if outfmt == outputFormatJSON {
		return writeJSON(cmd.OutOrStdout(), results)
	}
	if outfmt == outputFormatTable {
		return writeTable(cmd.OutOrStdout(), results, fields)
	}
	if outfmt == outputFormatTTable {
		return writeTransposedSearchTable(cmd.OutOrStdout(), results, fields)
	}
	if outfmt == outputFormatTTSV {
		return writeTransposedSearchTSV(cmd.OutOrStdout(), results, fields)
	}

	return writeTSV(cmd.OutOrStdout(), results, fields)
}

func parseOutputFormat(format string, allowJSON bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatTSV:
		return outputFormatTSV, nil
	case outputFormatTable, "human":
		return outputFormatTable, nil
	case outputFormatTTable:
		return outputFormatTTable, nil
	case outputFormatTTSV:
		return outputFormatTTSV, nil
	case outputFormatJSON:
		if allowJSON {
			return outputFormatJSON, nil
		}
	}

	if allowJSON {
		return "", fmt.Errorf("unsupported --outfmt %q; expected json, table, tsv, ttable, or ttsv", format)
	}
	return "", fmt.Errorf("unsupported --outfmt %q; expected table, tsv, ttable, or ttsv", format)
}

func parseSearchLevel(level string) (ichsm.AccessionType, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "":
		return "", nil
	case string(ichsm.AccessionTypeStudy):
		return ichsm.AccessionTypeStudy, nil
	case string(ichsm.AccessionTypeSample):
		return ichsm.AccessionTypeSample, nil
	case string(ichsm.AccessionTypeRun):
		return ichsm.AccessionTypeRun, nil
	case string(ichsm.AccessionTypeAssembly):
		return ichsm.AccessionTypeAssembly, nil
	case string(ichsm.AccessionTypeSequence):
		return ichsm.AccessionTypeSequence, nil
	case string(ichsm.AccessionTypeCoding):
		return ichsm.AccessionTypeCoding, nil
	case string(ichsm.AccessionTypeAnalysis):
		return ichsm.AccessionTypeAnalysis, nil
	case string(ichsm.AccessionTypeContigSet):
		return ichsm.AccessionTypeContigSet, nil
	case string(ichsm.AccessionTypeWGSSet):
		return ichsm.AccessionTypeWGSSet, nil
	case string(ichsm.AccessionTypeTSASet):
		return ichsm.AccessionTypeTSASet, nil
	case string(ichsm.AccessionTypeTLSSet):
		return ichsm.AccessionTypeTLSSet, nil
	default:
		return "", fmt.Errorf("unsupported --level %q; expected study, sample, run, assembly, sequence, coding, analysis, contig_set, wgs_set, tsa_set, or tls_set", level)
	}
}

func parseSearchSource(source string) (ichsm.SearchSource, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", string(ichsm.SearchSourceAuto):
		return ichsm.SearchSourceAuto, nil
	case string(ichsm.SearchSourceENA):
		return ichsm.SearchSourceENA, nil
	case string(ichsm.SearchSourceNCBI):
		return ichsm.SearchSourceNCBI, nil
	default:
		return "", fmt.Errorf("unsupported --source %q; expected auto, ena, or ncbi", source)
	}
}

func newGetFieldsCommand() *cobra.Command {
	var debug bool
	outfmt := outputFormatTSV
	sortBy := ""

	cmd := &cobra.Command{
		Use:   "get_fields [data_type]",
		Short: "List ENA data types or fields for a given data type",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedOutfmt, err := parseOutputFormat(outfmt, false)
			if err != nil {
				return err
			}
			parsedSort, err := parseGetFieldsSort(sortBy)
			if err != nil {
				return err
			}
			if parsedSort != "" && len(args) == 0 {
				return fmt.Errorf("--sort is supported only with get_fields <data_type>")
			}

			client := newClient()
			var text string
			if len(args) == 0 {
				if debug {
					log.Printf("getting available data types")
				}
				text, err = client.GetResultTypes(cmd.Context())
				if err == nil {
					text = appendICHSMSearchColumn(text)
				}
			} else {
				if debug {
					log.Printf("getting fields for %s", args[0])
				}
				text, err = client.GetAllowedFields(cmd.Context(), args[0])
				if err == nil {
					text = appendICHSMColumnsColumn(text, args[0], parsedSort)
				}
			}
			if err != nil {
				return err
			}
			if parsedOutfmt == outputFormatTable {
				return writeAlignedRows(cmd.OutOrStdout(), tsvTextRows(text))
			}
			if parsedOutfmt == outputFormatTTable {
				return writeTransposedTable(cmd.OutOrStdout(), tsvTextRows(text))
			}
			if parsedOutfmt == outputFormatTTSV {
				return writeTransposedDelimitedRows(cmd.OutOrStdout(), tsvTextRows(text), "\t")
			}
			return writeTextWithTrailingNewline(cmd.OutOrStdout(), text)
		},
	}
	cmd.Flags().BoolVar(&debug, "debug", false, "More verbose logging")
	cmd.Flags().StringVar(&outfmt, "outfmt", outfmt, "Output format: table, tsv, ttable, or ttsv")
	cmd.Flags().StringVar(&sortBy, "sort", sortBy, "Sort field rows: ichsm_columns")
	return cmd
}

func parseGetFieldsSort(sortBy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "", "none":
		return "", nil
	case "ichsm_columns", "preset", "columns":
		return "ichsm_columns", nil
	default:
		return "", fmt.Errorf("unsupported --sort %q; expected ichsm_columns", sortBy)
	}
}

func writeTextWithTrailingNewline(out io.Writer, text string) error {
	fmt.Fprint(out, text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprintln(out)
	}
	return nil
}

func appendICHSMSearchColumn(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return text
	}

	type resultTypeRow struct {
		resultType string
		supported  bool
		line       string
	}

	rows := make([]resultTypeRow, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		rows = append(rows, resultTypeRow{
			resultType: fields[0],
			supported:  ichsmSearchSupportsResult(fields[0]),
			line:       line,
		})
	}
	sort.Slice(rows, func(i int, j int) bool {
		if rows[i].supported != rows[j].supported {
			return rows[i].supported
		}
		return rows[i].resultType < rows[j].resultType
	})

	out := make([]string, 0, len(lines))
	out = append(out, lines[0]+"\tichsm_search")
	for _, row := range rows {
		out = append(out, row.line+"\t"+yesNo(row.supported))
	}
	return strings.Join(out, "\n") + "\n"
}

func appendICHSMColumnsColumn(text string, resultType string, sortBy string) string {
	rows := tsvTextRows(text)
	if len(rows) == 0 {
		return text
	}

	type fieldRow struct {
		fields []string
		level  string
		rank   int
	}

	fieldRows := make([]fieldRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		row = normalizeGetFieldsRow(rows[0], row)
		level := "."
		rank := ichsmColumnPresetRank(level)
		if len(row) > 0 {
			if presetLevel, ok := ichsm.FieldPresetLevelForResult(resultType, row[0]); ok {
				level = presetLevel
				rank = ichsmColumnPresetRank(level)
			}
		}
		fieldRows = append(fieldRows, fieldRow{
			fields: append([]string(nil), row...),
			level:  level,
			rank:   rank,
		})
	}

	if sortBy == "ichsm_columns" {
		sort.SliceStable(fieldRows, func(i int, j int) bool {
			if fieldRows[i].rank != fieldRows[j].rank {
				return fieldRows[i].rank < fieldRows[j].rank
			}
			return fieldRows[i].fields[0] < fieldRows[j].fields[0]
		})
	}

	outRows := make([][]string, 0, len(rows))
	outRows = append(outRows, getFieldsOutputHeader(rows[0]))
	for _, row := range fieldRows {
		outRows = append(outRows, getFieldsOutputRow(rows[0], row.fields, row.level))
	}

	var out strings.Builder
	_ = writeDelimitedRows(&out, outRows, "\t")
	return out.String()
}

func getFieldsOutputHeader(header []string) []string {
	if isENASearchFieldsHeader(header) {
		return []string{"columnId", "type", "ichsm_columns", "description"}
	}
	out := append([]string(nil), header...)
	out = append(out, "ichsm_columns")
	return out
}

func getFieldsOutputRow(header []string, row []string, level string) []string {
	if isENASearchFieldsHeader(header) {
		return []string{row[0], row[2], level, row[1]}
	}
	out := append([]string(nil), row...)
	out = append(out, level)
	return out
}

func isENASearchFieldsHeader(header []string) bool {
	return len(header) == 3 && header[0] == "columnId" && header[1] == "description" && header[2] == "type"
}

func normalizeGetFieldsRow(header []string, row []string) []string {
	out := append([]string(nil), row...)
	if isENASearchFieldsHeader(header) && len(out) == 2 && looksLikeENAFieldType(out[1]) {
		out = []string{out[0], "", out[1]}
	}
	for len(out) < len(header) {
		out = append(out, "")
	}
	return out
}

func looksLikeENAFieldType(value string) bool {
	switch value {
	case "boolean", "controlled value", "date", "indexed", "latlon", "list", "number", "taxonomy", "text":
		return true
	default:
		return false
	}
}

func ichsmColumnPresetRank(level string) int {
	switch level {
	case ichsm.FieldPresetSmall:
		return 0
	case ichsm.FieldPresetDefault:
		return 1
	case ichsm.FieldPresetBig:
		return 2
	case ichsm.FieldPresetAll:
		return 3
	default:
		return 4
	}
}

func ichsmSearchSupportsResult(resultType string) bool {
	switch resultType {
	case "analysis", "assembly", "coding", "read_run", "sample", "sequence", "study", "tls_set", "tsa_set", "wgs_set":
		return true
	default:
		return false
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func accessionsFromInputs(accession string, accFile string) ([]string, error) {
	if accession != "" {
		return []string{accession}, nil
	}
	return ichsm.ReadAccessionsFile(accFile)
}

type accessionSearch struct {
	input string
	fixed string
	typ   ichsm.AccessionType
}

type countResult struct {
	InputAccession string              `json:"input_accession"`
	ResultType     ichsm.AccessionType `json:"result_type"`
	Count          int                 `json:"count"`
}

func prepareAccessionSearches(accessions []string, level ichsm.AccessionType, errOut io.Writer) ([]accessionSearch, error) {
	if len(accessions) == 0 {
		return nil, errors.New("no accessions provided")
	}
	if errOut == nil {
		errOut = io.Discard
	}

	toSearch := make([]accessionSearch, 0, len(accessions))
	var firstType ichsm.AccessionType
	for _, accession := range accessions {
		fixedAccession, accessionType, ok := ichsm.IdentifyAccession(accession)
		if !ok {
			fmt.Fprintf(errOut, "%s\t%s\n", accession, "")
			return nil, fmt.Errorf("error getting result types from accessions")
		}
		if firstType == "" {
			firstType = accessionType
		} else if accessionType != firstType {
			for _, searched := range toSearch {
				fmt.Fprintf(errOut, "%s\t%s\n", searched.input, searched.typ)
			}
			fmt.Fprintf(errOut, "%s\t%s\n", accession, accessionType)
			return nil, fmt.Errorf("error getting result types from accessions")
		}
		if _, err := ichsm.ResolveSearchLevel(accessionType, level); err != nil {
			return nil, err
		}

		toSearch = append(toSearch, accessionSearch{input: accession, fixed: fixedAccession, typ: accessionType})
	}

	return toSearch, nil
}

func searchAccessions(ctx context.Context, client *ichsm.Client, accessions []string, fields []string, level ichsm.AccessionType, source ichsm.SearchSource, debug bool, errOut io.Writer, preflightLargeJSON bool) ([]ichsm.SearchResult, error) {
	toSearch, err := prepareAccessionSearches(accessions, level, errOut)
	if err != nil {
		return nil, err
	}

	if preflightLargeJSON {
		warnLargeJSONSearchCounts(ctx, client, toSearch, level, source, debug, errOut)
	}

	results := make([]ichsm.SearchResult, 0, len(toSearch))
	for _, accession := range toSearch {
		if debug {
			if level == "" {
				log.Printf("search for %s", accession.input)
			} else {
				log.Printf("search for %s at %s level", accession.input, level)
			}
		}

		resultSource, resultType, newFields, records, err := client.QueryWithSource(ctx, accession.input, accession.fixed, accession.typ, fields, level, source)
		if err != nil {
			return nil, fmt.Errorf("error getting data for accession %s: %w", accession.input, err)
		}
		if len(records) == 0 {
			return nil, fmt.Errorf("no results returned for accession %s", accession.input)
		}

		results = append(results, ichsm.SearchResult{
			InputAccession: accession.input,
			FixedAccession: accession.fixed,
			InputType:      accession.typ,
			ResultType:     resultType,
			Source:         resultSource,
			Fields:         newFields,
			Records:        records,
		})
	}

	return results, nil
}

func countAccessions(ctx context.Context, client *ichsm.Client, accessions []string, level ichsm.AccessionType, source ichsm.SearchSource, errOut io.Writer) ([]countResult, error) {
	if source == ichsm.SearchSourceNCBI {
		return nil, fmt.Errorf("--count is currently supported only for ENA-backed searches")
	}

	toSearch, err := prepareAccessionSearches(accessions, level, errOut)
	if err != nil {
		return nil, err
	}

	counts := make([]countResult, 0, len(toSearch))
	for _, accession := range toSearch {
		resultType, count, err := client.CountENA(ctx, accession.fixed, accession.typ, level)
		if err != nil {
			return nil, fmt.Errorf("error counting accession %s: %w", accession.input, err)
		}
		counts = append(counts, countResult{
			InputAccession: accession.input,
			ResultType:     resultType,
			Count:          count,
		})
	}
	return counts, nil
}

func warnLargeJSONSearchCounts(ctx context.Context, client *ichsm.Client, searches []accessionSearch, level ichsm.AccessionType, source ichsm.SearchSource, debug bool, errOut io.Writer) {
	if errOut == nil {
		errOut = io.Discard
	}

	for _, accession := range searches {
		resultType, err := ichsm.ResolveSearchLevel(accession.typ, level)
		if err != nil {
			continue
		}
		if !needsJSONCountPreflight(source, accession.typ, resultType) {
			continue
		}

		countResultType, count, err := client.CountENA(ctx, accession.fixed, accession.typ, level)
		if err != nil {
			if debug {
				log.Printf("warning: could not check result count for accession %s: %v", accession.input, err)
			}
			continue
		}
		if count >= largeJSONRecordWarningThreshold {
			fmt.Fprintf(errOut, "warning: JSON search for %s at %s level will return %d records; JSON output may use a lot of memory. Use --outfmt tsv for large tabular output.\n", accession.input, countResultType, count)
		}
	}
}

func needsJSONCountPreflight(source ichsm.SearchSource, inputType ichsm.AccessionType, resultType ichsm.AccessionType) bool {
	if source == ichsm.SearchSourceNCBI {
		return false
	}

	switch inputType {
	case ichsm.AccessionTypeStudy:
		return resultType != ichsm.AccessionTypeStudy
	case ichsm.AccessionTypeContigSet, ichsm.AccessionTypeWGSSet, ichsm.AccessionTypeTSASet, ichsm.AccessionTypeTLSSet:
		return true
	default:
		return false
	}
}

func writeCountResults(out io.Writer, counts []countResult, outfmt string) error {
	if outfmt == outputFormatJSON {
		encoded, err := json.MarshalIndent(counts, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(encoded))
		return nil
	}

	rows := countRows(counts)
	if outfmt == outputFormatTable {
		return writeAlignedRows(out, rows)
	}
	if outfmt == outputFormatTTable {
		return writeTransposedTable(out, rows)
	}
	if outfmt == outputFormatTTSV {
		return writeTransposedDelimitedRows(out, rows, "\t")
	}
	return writeDelimitedRows(out, rows, "\t")
}

func countRows(counts []countResult) [][]string {
	rows := [][]string{{"input_accession", "result_type", "count"}}
	for _, count := range counts {
		rows = append(rows, []string{
			count.InputAccession,
			string(count.ResultType),
			fmt.Sprint(count.Count),
		})
	}
	return rows
}

func writeJSON(out io.Writer, results []ichsm.SearchResult) error {
	byAccession := make(map[string][]ichsm.Record, len(results))
	for _, result := range results {
		byAccession[result.InputAccession] = result.Records
	}

	encoded, err := json.MarshalIndent(byAccession, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(encoded))
	return nil
}

func writeTSV(out io.Writer, results []ichsm.SearchResult, requestedFields []string) error {
	rows, err := searchRows(results, requestedFields)
	if err != nil {
		return err
	}
	return writeDelimitedRows(out, rows, "\t")
}

func writeTable(out io.Writer, results []ichsm.SearchResult, requestedFields []string) error {
	rows, err := searchRows(results, requestedFields)
	if err != nil {
		return err
	}
	return writeAlignedRows(out, rows)
}

func writeTransposedSearchTable(out io.Writer, results []ichsm.SearchResult, requestedFields []string) error {
	rows, err := searchRows(results, requestedFields)
	if err != nil {
		return err
	}
	return writeTransposedTable(out, rows)
}

func writeTransposedSearchTSV(out io.Writer, results []ichsm.SearchResult, requestedFields []string) error {
	rows, err := searchRows(results, requestedFields)
	if err != nil {
		return err
	}
	return writeTransposedDelimitedRows(out, rows, "\t")
}

func searchRows(results []ichsm.SearchResult, requestedFields []string) ([][]string, error) {
	var columns []string
	var rows [][]string
	allFields := requestedAllFields(requestedFields)
	if allFields {
		columns = allRecordKeys(results)
	}

	for _, result := range results {
		if len(result.Records) == 0 {
			continue
		}

		if rows == nil {
			if !allFields {
				columns = result.Fields
			}
			rows = append(rows, append([]string{"input_accession"}, columns...))
		} else if !allFields && !sameStringSet(columns, result.Fields) {
			return nil, fmt.Errorf("field set changed between results")
		}

		for _, record := range result.Records {
			row := make([]string, 0, len(columns)+1)
			row = append(row, result.InputAccession)
			for _, column := range columns {
				row = append(row, formatRecordColumn(record, column, allFields))
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func allRecordKeys(results []ichsm.SearchResult) []string {
	keySet := map[string]bool{}
	for _, result := range results {
		for _, record := range result.Records {
			for key := range record {
				keySet[key] = true
			}
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func requestedAllFields(fields []string) bool {
	return len(fields) == 1 && fields[0] == "ALL"
}

func sameStringSet(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func formatValue(value any) string {
	if value == nil {
		return "."
	}
	return fmt.Sprint(value)
}

func formatRecordColumn(record ichsm.Record, column string, nullMissing bool) string {
	value, ok := record[column]
	if !ok && nullMissing {
		return "null"
	}
	return formatValue(value)
}
