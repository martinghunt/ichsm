package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

const (
	matchOutputGroups  = "groups"
	matchOutputRecords = "records"

	matchRecordScopeMatching = "matching"
	matchRecordScopeAll      = "all"

	matchStrategyAuto  = "auto"
	matchStrategyLocal = "local"

	matchGroupBatchSize           = 100
	matchAutoENARequestsPerSecond = 5
	matchRecordProgressEvery      = 100000
	matchBatchProgressEvery       = 100
)

type matchOptions struct {
	result      string
	query       string
	groupBy     string
	has         []string
	columns     string
	output      string
	recordScope string
	outfmt      string
	strategy    string
	noResults   string
	limit       int
	offset      int
	debug       bool
	verbose     bool
}

type matchRequirement struct {
	raw   string
	terms []matchTerm
}

type matchTerm struct {
	field  string
	values map[string]bool
}

type matchGroup struct {
	key     string
	records []ichsm.Record
}

type matchGroupJSON struct {
	GroupBy     string              `json:"group_by"`
	Group       string              `json:"group"`
	RecordCount int                 `json:"record_count"`
	Values      map[string][]string `json:"values,omitempty"`
}

func newMatchCommand() *cobra.Command {
	opts := matchOptions{
		columns:     "DEFAULT",
		output:      matchOutputGroups,
		recordScope: matchRecordScopeMatching,
		outfmt:      outputFormatTSV,
		strategy:    matchStrategyAuto,
		noResults:   noResultsModeSkip,
	}
	cmd := &cobra.Command{
		Use:   "match",
		Short: "Find ENA record groups matching row-level requirements",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeMatch(cmd, opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.debug, "debug", false, "More verbose logging")
	flags.BoolVar(&opts.verbose, "verbose", false, "Print match progress to stderr")
	flags.StringVarP(&opts.result, "result", "r", "", "ENA result type to query, such as sample or read_run")
	flags.StringVarP(&opts.query, "query", "q", "", "ENA Portal API base query string, such as tax_tree(2)")
	flags.StringVar(&opts.groupBy, "group-by", "", "Field used to group records, such as sample_accession")
	flags.StringArrayVar(&opts.has, "has", nil, "Group requirement: field=value[,value...][;field=value...]. Repeat for AND between requirements")
	flags.StringVarP(&opts.columns, "columns", "c", opts.columns, "Record columns for --output records, comma-separated, or SMALL, DEFAULT, BIG, ALL")
	flags.StringVar(&opts.columns, "fields", opts.columns, "Record columns for --output records, comma-separated, or SMALL, DEFAULT, BIG, ALL")
	flags.StringVar(&opts.output, "output", opts.output, "Output mode: groups or records")
	flags.StringVar(&opts.recordScope, "record-scope", opts.recordScope, "Records to write with --output records: matching or all")
	flags.StringVar(&opts.strategy, "strategy", opts.strategy, "Matching strategy: auto or local")
	flags.StringVar(&opts.outfmt, "outfmt", opts.outfmt, "Output format: json, table, tsv, ttable, or ttsv")
	flags.StringVar(&opts.noResults, "on-no-results", opts.noResults, "How to handle no matching groups: skip, empty, error, or fail")
	flags.IntVar(&opts.limit, "limit", 0, "Maximum number of ENA records to fetch before grouping with --strategy local; 0 means no explicit limit")
	flags.IntVar(&opts.offset, "offset", 0, "ENA result offset for paging with --strategy local")
	return cmd
}

func executeMatch(cmd *cobra.Command, opts matchOptions) error {
	outfmt, err := parseOutputFormat(opts.outfmt, true)
	if err != nil {
		return err
	}
	output, err := parseMatchOutput(opts.output)
	if err != nil {
		return err
	}
	recordScope, err := parseMatchRecordScope(opts.recordScope)
	if err != nil {
		return err
	}
	opts.output = output
	opts.recordScope = recordScope
	strategy, err := parseMatchStrategy(opts.strategy)
	if err != nil {
		return err
	}
	if strategy == matchStrategyAuto && (opts.limit > 0 || opts.offset > 0) {
		return fmt.Errorf("--limit and --offset are supported only with --strategy local")
	}
	noResultsMode, err := parseNoResultsMode(opts.noResults)
	if err != nil {
		return err
	}
	groupBy := strings.TrimSpace(opts.groupBy)
	if groupBy == "" {
		return fmt.Errorf("--group-by is required")
	}
	if len(opts.has) == 0 {
		return fmt.Errorf("at least one --has requirement is required")
	}

	requirements, err := parseMatchRequirements(opts.has)
	if err != nil {
		return err
	}

	resultType, _, ok := ichsm.NormalizeENAResult(opts.result)
	if !ok {
		return fmt.Errorf("unsupported ENA result %q; expected study, sample, run/read_run, assembly, sequence, coding, analysis, wgs_set, tsa_set, or tls_set", opts.result)
	}

	outputFields, err := resolveMatchOutputFields(resultType, opts.columns, output)
	if err != nil {
		return err
	}
	queryFields := matchQueryFields(groupBy, requirements, outputFields)

	if opts.debug {
		fmt.Fprintf(cmd.ErrOrStderr(), "matching ENA result %s grouped by %s with %d requirement(s)\n", opts.result, groupBy, len(requirements))
	}

	client := newClient()
	if strategy == matchStrategyAuto && client.ENARequestsPerSecond == 0 {
		client.ENARequestsPerSecond = matchAutoENARequestsPerSecond
	}
	var groups []matchGroup
	switch strategy {
	case matchStrategyAuto:
		groups, err = autoMatchingGroups(cmd.Context(), client, opts, groupBy, requirements, queryFields, cmd.ErrOrStderr())
	case matchStrategyLocal:
		groups, err = localMatchingGroups(cmd.Context(), client, opts, groupBy, requirements, queryFields)
	default:
		err = fmt.Errorf("unsupported match strategy %q", strategy)
	}
	if err != nil {
		return err
	}
	if output == matchOutputRecords {
		if recordScope == matchRecordScopeMatching {
			groups = matchingRecordGroups(groups, requirements)
		}
	}
	if len(groups) == 0 {
		return writeNoMatchResults(cmd.OutOrStdout(), cmd.ErrOrStderr(), groupBy, requirements, outputFields, output, outfmt, noResultsMode)
	}
	switch output {
	case matchOutputGroups:
		return writeMatchGroups(cmd.OutOrStdout(), groupBy, groups, requirements, outfmt)
	case matchOutputRecords:
		return writeMatchRecords(cmd.OutOrStdout(), groups, outputFields, outfmt)
	default:
		return fmt.Errorf("unsupported match output %q", output)
	}
}

type noResultsMatchError struct{}

func (e *noResultsMatchError) Error() string {
	return matchNoResultsMessage()
}

func parseMatchRecordScope(recordScope string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(recordScope)) {
	case "", matchRecordScopeMatching:
		return matchRecordScopeMatching, nil
	case matchRecordScopeAll:
		return matchRecordScopeAll, nil
	default:
		return "", fmt.Errorf("unsupported --record-scope %q; expected matching or all", recordScope)
	}
}

func parseMatchStrategy(strategy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", matchStrategyAuto:
		return matchStrategyAuto, nil
	case matchStrategyLocal:
		return matchStrategyLocal, nil
	default:
		return "", fmt.Errorf("unsupported --strategy %q; expected auto or local", strategy)
	}
}

func parseMatchOutput(output string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "", matchOutputGroups:
		return matchOutputGroups, nil
	case matchOutputRecords:
		return matchOutputRecords, nil
	default:
		return "", fmt.Errorf("unsupported --output %q; expected groups or records", output)
	}
}

func localMatchingGroups(ctx context.Context, client *ichsm.Client, opts matchOptions, groupBy string, requirements []matchRequirement, queryFields []string) ([]matchGroup, error) {
	result, err := client.QueryENA(ctx, ichsm.ENAQueryOptions{
		Result: opts.result,
		Query:  opts.query,
		Fields: queryFields,
		Limit:  opts.limit,
		Offset: opts.offset,
	})
	if err != nil {
		return nil, err
	}
	return matchingGroups(result.Records, groupBy, requirements), nil
}

func autoMatchingGroups(ctx context.Context, client *ichsm.Client, opts matchOptions, groupBy string, requirements []matchRequirement, queryFields []string, errOut io.Writer) ([]matchGroup, error) {
	progress := matchProgress{
		enabled:           opts.verbose,
		errOut:            errOut,
		requestsPerSecond: matchENARequestsPerSecond(client),
	}
	progress.printf("counting %d match requirements\n", len(requirements))

	seeds, err := countMatchSeeds(ctx, client, opts, requirements)
	if err != nil {
		return nil, err
	}
	for _, seed := range seeds {
		if seed.count == 0 {
			return nil, nil
		}
	}

	sort.SliceStable(seeds, func(i int, j int) bool {
		return seeds[i].count < seeds[j].count
	})
	if opts.debug {
		for _, seed := range seeds {
			fmt.Fprintf(errOut, "seed count %d for %s\n", seed.count, seed.query)
		}
	}
	for i, seed := range seeds {
		progress.printf("seed %d count: %d records for --has %q\n", i+1, seed.count, seed.requirement.raw)
		progress.printf("seed %d query: %s\n", i+1, seed.query)
	}

	var candidates map[string]bool
	for i, seed := range seeds {
		var filterValues []string
		if i > 0 {
			filterValues = sortedMapKeys(candidates)
			if len(filterValues) == 0 {
				return nil, nil
			}
		}

		label := matchSeedProgressLabel(i, seed)
		seedGroups, err := fetchMatchGroupKeys(ctx, client, opts.result, seed.query, groupBy, filterValues, &progress, label)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			candidates = seedGroups
		} else {
			candidates = intersectStringSets(candidates, seedGroups)
		}
		progress.printf("%s candidates: %d %s groups\n", label, len(candidates), groupBy)
		if len(candidates) == 0 {
			return nil, nil
		}
	}

	finalQuery := finalMatchRecordQuery(opts.query, requirements, opts.output, opts.recordScope)
	progress.printf("final records query: %s\n", strings.TrimSpace(finalQuery))
	records, err := fetchMatchGroupRecords(ctx, client, opts.result, finalQuery, groupBy, candidates, queryFields, &progress)
	if err != nil {
		return nil, err
	}
	progress.printf("fetched %d final records for %d candidate groups\n", len(records), len(candidates))
	return matchingGroups(records, groupBy, requirements), nil
}

type matchProgress struct {
	enabled           bool
	errOut            io.Writer
	requestsPerSecond int
}

func (p *matchProgress) printf(format string, args ...any) {
	if p == nil || !p.enabled {
		return
	}
	out := p.errOut
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintf(out, format, args...)
}

func (p *matchProgress) requestEstimate(batchCount int) string {
	if p == nil || p.requestsPerSecond <= 0 || batchCount <= 0 {
		return ""
	}
	duration := time.Duration(batchCount) * time.Second / time.Duration(p.requestsPerSecond)
	if duration <= 0 {
		return ""
	}
	return fmt.Sprintf("; minimum time %s at %d req/s", formatDurationEstimate(duration), p.requestsPerSecond)
}

func (p *matchProgress) remainingRequestEstimate(remainingRequests int) string {
	if p == nil || p.requestsPerSecond <= 0 || remainingRequests <= 0 {
		return ""
	}
	duration := time.Duration(remainingRequests) * time.Second / time.Duration(p.requestsPerSecond)
	if duration <= 0 {
		return ""
	}
	return fmt.Sprintf("; minimum time remaining %s", formatDurationEstimate(duration))
}

func matchENARequestsPerSecond(client *ichsm.Client) int {
	if client != nil && client.ENARequestsPerSecond > 0 {
		return client.ENARequestsPerSecond
	}
	if client != nil && client.ENARequestsPerSecond < 0 {
		return 0
	}
	return matchAutoENARequestsPerSecond
}

func formatDurationEstimate(duration time.Duration) string {
	if duration < time.Minute {
		seconds := int(duration.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		return fmt.Sprintf("%ds", seconds)
	}
	duration = duration.Round(time.Second)
	minutes := int(duration / time.Minute)
	seconds := int((duration % time.Minute) / time.Second)
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}

type matchSeed struct {
	requirement matchRequirement
	query       string
	count       int
}

func matchSeedProgressLabel(index int, seed matchSeed) string {
	if strings.TrimSpace(seed.requirement.raw) == "" {
		return fmt.Sprintf("requirement %d", index+1)
	}
	return fmt.Sprintf("requirement %d --has %q", index+1, seed.requirement.raw)
}

func countMatchSeeds(ctx context.Context, client *ichsm.Client, opts matchOptions, requirements []matchRequirement) ([]matchSeed, error) {
	seeds := make([]matchSeed, 0, len(requirements))
	for _, requirement := range requirements {
		query := andENAQueries(opts.query, requirement.enaQuery())
		_, _, count, err := client.CountENAQuery(ctx, ichsm.ENAQueryOptions{
			Result: opts.result,
			Query:  query,
		})
		if err != nil {
			return nil, err
		}
		seeds = append(seeds, matchSeed{requirement: requirement, query: query, count: count})
	}
	return seeds, nil
}

func finalMatchRecordQuery(baseQuery string, requirements []matchRequirement, output string, recordScope string) string {
	if output != matchOutputRecords || recordScope != matchRecordScopeMatching {
		return baseQuery
	}

	queries := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		queries = append(queries, requirement.enaQuery())
	}
	return andENAQueries(baseQuery, orENAQueries(queries...))
}

func fetchMatchGroupKeys(ctx context.Context, client *ichsm.Client, result string, query string, groupBy string, filterValues []string, progress *matchProgress, label string) (map[string]bool, error) {
	groups := map[string]bool{}
	batches := matchGroupBatches(filterValues)
	totalRecords := 0
	nextRecordProgress := matchRecordProgressEvery
	progress.printf("%s: fetching %d batch(es)%s for query: %s\n", label, len(batches), progress.requestEstimate(len(batches)), query)
	for batchIndex, batch := range batches {
		batchQuery := query
		if len(batch) > 0 {
			batchQuery = andENAQueries(batchQuery, groupFilterQuery(groupBy, batch))
		}
		_, err := client.StreamENATSV(ctx, ichsm.ENAQueryOptions{
			Result: result,
			Query:  batchQuery,
			Fields: []string{groupBy},
		}, nil, func(record ichsm.Record) error {
			totalRecords++
			if progress != nil && totalRecords >= nextRecordProgress {
				progress.printf("%s: downloaded %d records\n", label, totalRecords)
				nextRecordProgress += matchRecordProgressEvery
			}
			for _, groupKey := range recordValues(record, groupBy) {
				groups[groupKey] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if progress != nil && len(batches) > 1 && (batchIndex+1)%matchBatchProgressEvery == 0 {
			progress.printf("%s: checked %d/%d batches; %d groups seen%s\n", label, batchIndex+1, len(batches), len(groups), progress.remainingRequestEstimate(len(batches)-batchIndex-1))
		}
	}
	progress.printf("%s: downloaded %d records; saw %d groups\n", label, totalRecords, len(groups))
	return groups, nil
}

func fetchMatchGroupRecords(ctx context.Context, client *ichsm.Client, result string, baseQuery string, groupBy string, groups map[string]bool, fields []string, progress *matchProgress) ([]ichsm.Record, error) {
	groupKeys := sortedMapKeys(groups)
	if len(groupKeys) == 0 {
		return nil, nil
	}

	var records []ichsm.Record
	seenRecords := map[string]bool{}
	batches := matchGroupBatches(groupKeys)
	totalRecords := 0
	nextRecordProgress := matchRecordProgressEvery
	progress.printf("final records: fetching %d batch(es)%s\n", len(batches), progress.requestEstimate(len(batches)))
	for batchIndex, batch := range batches {
		query := andENAQueries(baseQuery, groupFilterQuery(groupBy, batch))
		_, err := client.StreamENATSV(ctx, ichsm.ENAQueryOptions{
			Result: result,
			Query:  query,
			Fields: fields,
		}, nil, func(record ichsm.Record) error {
			totalRecords++
			if progress != nil && totalRecords >= nextRecordProgress {
				progress.printf("final records: downloaded %d records\n", totalRecords)
				nextRecordProgress += matchRecordProgressEvery
			}
			if recordHasAnyGroup(record, groupBy, groups) {
				recordKey := recordIdentity(record)
				if seenRecords[recordKey] {
					return nil
				}
				seenRecords[recordKey] = true
				records = append(records, record)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if progress != nil && len(batches) > 1 && (batchIndex+1)%matchBatchProgressEvery == 0 {
			progress.printf("final records: checked %d/%d batches; kept %d records%s\n", batchIndex+1, len(batches), len(records), progress.remainingRequestEstimate(len(batches)-batchIndex-1))
		}
	}
	progress.printf("final records: downloaded %d records; kept %d records\n", totalRecords, len(records))
	return records, nil
}

func matchGroupBatches(values []string) [][]string {
	if values == nil {
		return [][]string{nil}
	}
	if len(values) == 0 {
		return nil
	}
	batches := [][]string{}
	for start := 0; start < len(values); start += matchGroupBatchSize {
		end := start + matchGroupBatchSize
		if end > len(values) {
			end = len(values)
		}
		batches = append(batches, values[start:end])
	}
	return batches
}

func parseMatchRequirements(rawRequirements []string) ([]matchRequirement, error) {
	requirements := make([]matchRequirement, 0, len(rawRequirements))
	for _, raw := range rawRequirements {
		requirement, err := parseMatchRequirement(raw)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, requirement)
	}
	return requirements, nil
}

func parseMatchRequirement(raw string) (matchRequirement, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return matchRequirement{}, fmt.Errorf("empty --has requirement")
	}

	parts := strings.Split(raw, ";")
	requirement := matchRequirement{raw: raw, terms: make([]matchTerm, 0, len(parts))}
	for _, part := range parts {
		term, err := parseMatchTerm(part)
		if err != nil {
			return matchRequirement{}, fmt.Errorf("invalid --has requirement %q: %w", raw, err)
		}
		requirement.terms = append(requirement.terms, term)
	}
	return requirement, nil
}

func parseMatchTerm(raw string) (matchTerm, error) {
	raw = strings.TrimSpace(raw)
	field, valuesText, ok := strings.Cut(raw, "=")
	if !ok {
		return matchTerm{}, fmt.Errorf("expected field=value")
	}
	field = strings.TrimSpace(field)
	if field == "" {
		return matchTerm{}, fmt.Errorf("field is empty")
	}

	values := map[string]bool{}
	for _, value := range strings.Split(valuesText, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		values[value] = true
	}
	if len(values) == 0 {
		return matchTerm{}, fmt.Errorf("value is empty for field %s", field)
	}

	return matchTerm{field: field, values: values}, nil
}

func resolveMatchOutputFields(resultType ichsm.AccessionType, columns string, output string) ([]string, error) {
	if output != matchOutputRecords {
		return nil, nil
	}
	return ichsm.ResolveFields(resultType, parseColumnList(columns))
}

func matchQueryFields(groupBy string, requirements []matchRequirement, outputFields []string) []string {
	if requestedAllFields(outputFields) {
		return outputFields
	}

	seen := map[string]bool{}
	fields := []string{}
	appendField := func(field string) {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			return
		}
		seen[field] = true
		fields = append(fields, field)
	}

	appendField(groupBy)
	for _, requirement := range requirements {
		for _, term := range requirement.terms {
			appendField(term.field)
		}
	}
	for _, field := range outputFields {
		appendField(field)
	}
	return fields
}

func matchingGroups(records []ichsm.Record, groupBy string, requirements []matchRequirement) []matchGroup {
	grouped := map[string][]ichsm.Record{}
	for _, record := range records {
		for _, groupKey := range recordValues(record, groupBy) {
			grouped[groupKey] = append(grouped[groupKey], record)
		}
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	groups := make([]matchGroup, 0, len(keys))
	for _, key := range keys {
		groupRecords := grouped[key]
		if groupMatches(groupRecords, requirements) {
			groups = append(groups, matchGroup{key: key, records: groupRecords})
		}
	}
	return groups
}

func matchingRecordGroups(groups []matchGroup, requirements []matchRequirement) []matchGroup {
	filtered := make([]matchGroup, 0, len(groups))
	for _, group := range groups {
		records := make([]ichsm.Record, 0, len(group.records))
		for _, record := range group.records {
			if recordMatchesAnyRequirement(record, requirements) {
				records = append(records, record)
			}
		}
		if len(records) > 0 {
			filtered = append(filtered, matchGroup{key: group.key, records: records})
		}
	}
	return filtered
}

func recordMatchesAnyRequirement(record ichsm.Record, requirements []matchRequirement) bool {
	for _, requirement := range requirements {
		if requirement.matches(record) {
			return true
		}
	}
	return false
}

func groupMatches(records []ichsm.Record, requirements []matchRequirement) bool {
	for _, requirement := range requirements {
		if !requirementMatches(records, requirement) {
			return false
		}
	}
	return true
}

func requirementMatches(records []ichsm.Record, requirement matchRequirement) bool {
	for _, record := range records {
		if requirement.matches(record) {
			return true
		}
	}
	return false
}

func (r matchRequirement) matches(record ichsm.Record) bool {
	for _, term := range r.terms {
		if !term.matches(record) {
			return false
		}
	}
	return true
}

func (r matchRequirement) enaQuery() string {
	parts := make([]string, 0, len(r.terms))
	for _, term := range r.terms {
		parts = append(parts, term.enaQuery())
	}
	return andENAQueries(parts...)
}

func (t matchTerm) matches(record ichsm.Record) bool {
	for _, value := range recordValues(record, t.field) {
		if t.values[value] {
			return true
		}
	}
	return false
}

func (t matchTerm) enaQuery() string {
	values := make([]string, 0, len(t.values))
	for value := range t.values {
		values = append(values, value)
	}
	sort.Strings(values)

	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, t.field+"="+value)
	}
	return orENAQueries(parts...)
}

func normalizedRecordString(record ichsm.Record, field string) string {
	value := record[field]
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func recordValues(record ichsm.Record, field string) []string {
	value := normalizedRecordString(record, field)
	if value == "" || value == "." || value == "null" {
		return nil
	}

	parts := strings.Split(value, ";")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && part != "." && part != "null" {
			values = append(values, part)
		}
	}
	return values
}

func recordHasAnyGroup(record ichsm.Record, groupBy string, groups map[string]bool) bool {
	for _, groupKey := range recordValues(record, groupBy) {
		if groups[groupKey] {
			return true
		}
	}
	return false
}

func recordIdentity(record ichsm.Record) string {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		value := normalizedRecordString(record, key)
		builder.WriteString(strconv.Itoa(len(key)))
		builder.WriteByte(':')
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(strconv.Itoa(len(value)))
		builder.WriteByte(':')
		builder.WriteString(value)
		builder.WriteByte(';')
	}
	return builder.String()
}

func writeNoMatchResults(out io.Writer, errOut io.Writer, groupBy string, requirements []matchRequirement, outputFields []string, output string, outfmt string, noResultsMode string) error {
	if noResultsMode == noResultsModeFail {
		return &noResultsMatchError{}
	}

	if errOut == nil {
		errOut = io.Discard
	}

	var err error
	switch noResultsMode {
	case noResultsModeSkip:
		fmt.Fprintf(errOut, "warning: %s; writing empty output\n", matchNoResultsMessage())
		err = writeNoMatchEmptyOutput(out, groupBy, requirements, outputFields, output, outfmt)
	case noResultsModeEmpty:
		fmt.Fprintf(errOut, "warning: %s; writing empty record\n", matchNoResultsMessage())
		err = writeNoMatchPlaceholder(out, groupBy, requirements, outputFields, output, outfmt, false)
	case noResultsModeError:
		fmt.Fprintf(errOut, "warning: %s; writing error record\n", matchNoResultsMessage())
		err = writeNoMatchPlaceholder(out, groupBy, requirements, outputFields, output, outfmt, true)
	default:
		return fmt.Errorf("unsupported no-results mode %q", noResultsMode)
	}
	if err != nil {
		return err
	}
	return &noResultsMatchError{}
}

func writeNoMatchEmptyOutput(out io.Writer, groupBy string, requirements []matchRequirement, outputFields []string, output string, outfmt string) error {
	switch output {
	case matchOutputGroups:
		return writeMatchGroups(out, groupBy, nil, requirements, outfmt)
	case matchOutputRecords:
		if outfmt == outputFormatJSON {
			return writeJSONValue(out, []ichsm.Record{})
		}
		return writeRowsForOutputFormat(out, matchRecordRows(nil, outputFields), outfmt)
	default:
		return fmt.Errorf("unsupported match output %q", output)
	}
}

func writeNoMatchPlaceholder(out io.Writer, groupBy string, requirements []matchRequirement, outputFields []string, output string, outfmt string, includeDiagnostics bool) error {
	switch output {
	case matchOutputGroups:
		return writeNoMatchGroupPlaceholder(out, groupBy, requirements, outfmt, includeDiagnostics)
	case matchOutputRecords:
		return writeNoMatchRecordPlaceholder(out, outputFields, outfmt, includeDiagnostics)
	default:
		return fmt.Errorf("unsupported match output %q", output)
	}
}

func writeNoMatchGroupPlaceholder(out io.Writer, groupBy string, requirements []matchRequirement, outfmt string, includeDiagnostics bool) error {
	valueFields := matchValueFields(requirements)
	if !includeDiagnostics {
		return writeMatchGroups(out, groupBy, []matchGroup{{key: "."}}, requirements, outfmt)
	}

	if outfmt == outputFormatJSON {
		return writeJSONValue(out, []map[string]any{{
			"group_by":           groupBy,
			"group":              ".",
			"record_count":       0,
			noResultsStatusField: "no_results",
			noResultsErrorField:  matchNoResultsMessage(),
		}})
	}

	header := append([]string{groupBy, "record_count"}, valueFields...)
	header = append(header, noResultsStatusField, noResultsErrorField)
	row := []string{".", "0"}
	for range valueFields {
		row = append(row, ".")
	}
	row = append(row, "no_results", matchNoResultsMessage())
	return writeRowsForOutputFormat(out, [][]string{header, row}, outfmt)
}

func writeNoMatchRecordPlaceholder(out io.Writer, outputFields []string, outfmt string, includeDiagnostics bool) error {
	record := matchNoResultsRecord(outputFields, includeDiagnostics)
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, []ichsm.Record{record})
	}
	return writeRowsForOutputFormat(out, matchRecordRows([]ichsm.Record{record}, matchNoResultsRecordFields(outputFields, includeDiagnostics)), outfmt)
}

func matchNoResultsRecordFields(outputFields []string, includeDiagnostics bool) []string {
	if requestedAllFields(outputFields) {
		if includeDiagnostics {
			return []string{noResultsStatusField, noResultsErrorField}
		}
		return nil
	}
	out := append([]string(nil), outputFields...)
	if includeDiagnostics {
		out = appendStringIfMissing(out, noResultsStatusField)
		out = appendStringIfMissing(out, noResultsErrorField)
	}
	return out
}

func matchNoResultsRecord(outputFields []string, includeDiagnostics bool) ichsm.Record {
	record := ichsm.Record{}
	if !requestedAllFields(outputFields) {
		for _, field := range outputFields {
			if strings.TrimSpace(field) == "" {
				continue
			}
			record[field] = nil
		}
	}
	if includeDiagnostics {
		record[noResultsStatusField] = "no_results"
		record[noResultsErrorField] = matchNoResultsMessage()
	}
	return record
}

func matchNoResultsMessage() string {
	return "no matching groups returned"
}

func andENAQueries(parts ...string) string {
	return joinENAQueries("AND", parts...)
}

func orENAQueries(parts ...string) string {
	return joinENAQueries("OR", parts...)
}

func joinENAQueries(operator string, parts ...string) string {
	queries := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			queries = append(queries, part)
		}
	}
	if len(queries) == 0 {
		return ""
	}
	if len(queries) == 1 {
		return queries[0]
	}

	wrapped := make([]string, 0, len(queries))
	for _, query := range queries {
		wrapped = append(wrapped, "("+query+")")
	}
	return strings.Join(wrapped, " "+operator+" ")
}

func groupFilterQuery(groupBy string, values []string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, groupBy+"="+value)
	}
	return orENAQueries(parts...)
}

func sortedMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func intersectStringSets(a map[string]bool, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	if len(a) > len(b) {
		a, b = b, a
	}
	for key := range a {
		if b[key] {
			out[key] = true
		}
	}
	return out
}

func writeMatchGroups(out io.Writer, groupBy string, groups []matchGroup, requirements []matchRequirement, outfmt string) error {
	valueFields := matchValueFields(requirements)
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, matchGroupJSONRows(groupBy, groups, valueFields))
	}
	return writeRowsForOutputFormat(out, matchGroupRows(groupBy, groups, valueFields), outfmt)
}

func matchValueFields(requirements []matchRequirement) []string {
	seen := map[string]bool{}
	fields := []string{}
	for _, requirement := range requirements {
		for _, term := range requirement.terms {
			if !seen[term.field] {
				seen[term.field] = true
				fields = append(fields, term.field)
			}
		}
	}
	return fields
}

func matchGroupRows(groupBy string, groups []matchGroup, valueFields []string) [][]string {
	header := append([]string{groupBy, "record_count"}, valueFields...)
	rows := [][]string{header}
	for _, group := range groups {
		row := []string{group.key, fmt.Sprint(len(group.records))}
		for _, field := range valueFields {
			row = append(row, formatSummaryValues(distinctRecordValues(group.records, field)))
		}
		rows = append(rows, row)
	}
	return rows
}

func matchGroupJSONRows(groupBy string, groups []matchGroup, valueFields []string) []matchGroupJSON {
	rows := make([]matchGroupJSON, 0, len(groups))
	for _, group := range groups {
		values := map[string][]string{}
		for _, field := range valueFields {
			fieldValues := distinctRecordValues(group.records, field)
			if len(fieldValues) > 0 {
				values[field] = fieldValues
			}
		}
		rows = append(rows, matchGroupJSON{
			GroupBy:     groupBy,
			Group:       group.key,
			RecordCount: len(group.records),
			Values:      values,
		})
	}
	return rows
}

func writeMatchRecords(out io.Writer, groups []matchGroup, outputFields []string, outfmt string) error {
	records := matchGroupRecords(groups)
	if outfmt == outputFormatJSON {
		return writeJSONValue(out, records)
	}
	return writeRowsForOutputFormat(out, matchRecordRows(records, outputFields), outfmt)
}

func matchGroupRecords(groups []matchGroup) []ichsm.Record {
	var records []ichsm.Record
	for _, group := range groups {
		records = append(records, group.records...)
	}
	return records
}

func matchRecordRows(records []ichsm.Record, outputFields []string) [][]string {
	columns := outputFields
	allFields := requestedAllFields(outputFields)
	if allFields {
		columns = queryRecordKeys(records)
	}
	if len(columns) == 0 {
		return nil
	}

	rows := [][]string{append([]string(nil), columns...)}
	for _, record := range records {
		row := make([]string, 0, len(columns))
		for _, column := range columns {
			row = append(row, formatRecordColumn(record, column, allFields))
		}
		rows = append(rows, row)
	}
	return rows
}

func distinctRecordValues(records []ichsm.Record, field string) []string {
	seen := map[string]bool{}
	values := []string{}
	for _, record := range records {
		for _, value := range recordValues(record, field) {
			if seen[value] {
				continue
			}
			seen[value] = true
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}
