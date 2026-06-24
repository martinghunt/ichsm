package ichsm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Record is one metadata record returned by a metadata provider.
type Record map[string]any

// SearchSource is the metadata provider to query.
type SearchSource string

const (
	SearchSourceAuto SearchSource = "auto"
	SearchSourceENA  SearchSource = "ena"
	SearchSourceNCBI SearchSource = "ncbi"
)

// Client queries ENA and NCBI metadata services.
type Client struct {
	BaseURL        string
	BrowserBaseURL string
	NCBIBaseURL    string
	NCBIAPIKey     string
	NCBIEmail      string
	NCBITool       string
	HTTPClient     *http.Client

	// ENARequestsPerSecond limits ENA Portal API requests per process. Zero uses
	// the default of 25 requests per second; negative values disable the limiter.
	ENARequestsPerSecond int
	// NCBIRequestsPerSecond limits NCBI E-utilities requests per process. Zero
	// uses the default of 3 requests per second, or 10 with an API key; negative
	// values disable the limiter.
	NCBIRequestsPerSecond int
	// MaxRequestRetries controls retries for HTTP 429 and transient 5xx responses.
	// Zero uses the default; negative values disable retries.
	MaxRequestRetries int
	// RequestRetryBaseDelay controls exponential retry backoff when Retry-After is
	// not supplied. Zero uses the default.
	RequestRetryBaseDelay time.Duration
	// RequestRetryMaxDelay caps exponential retry backoff. Zero uses the default.
	RequestRetryMaxDelay time.Duration
}

const (
	defaultENARequestsPerSecond        = 25
	defaultNCBIRequestsPerSecond       = 3
	defaultNCBIAPIKeyRequestsPerSecond = 10
	defaultMaxRequestRetries           = 5
	defaultRetryBaseDelay              = 250 * time.Millisecond
	defaultRetryMaxDelay               = 5 * time.Second
)

var enaRequestLimiter requestRateLimiter
var ncbiRequestLimiter requestRateLimiter

// SearchOptions configures a multi-accession search.
type SearchOptions struct {
	Accessions []string
	Fields     []string
	Level      AccessionType
	Source     SearchSource
}

// SearchResult contains records for one input accession.
type SearchResult struct {
	InputAccession string        `json:"input_accession"`
	FixedAccession string        `json:"fixed_accession"`
	InputType      AccessionType `json:"input_type"`
	ResultType     AccessionType `json:"result_type"`
	Source         SearchSource  `json:"source"`
	Fields         []string      `json:"fields"`
	Records        []Record      `json:"records"`
}

// ENAQueryOptions configures a raw ENA Portal API search query.
type ENAQueryOptions struct {
	Result string
	Query  string
	Fields []string
	Limit  int
	Offset int
}

// ENAQueryResult contains records returned by a raw ENA Portal API query.
type ENAQueryResult struct {
	ResultType AccessionType `json:"result_type"`
	ENAResult  string        `json:"ena_result"`
	Query      string        `json:"query"`
	Fields     []string      `json:"fields"`
	Records    []Record      `json:"records"`
}

// ENAControlledVocabValue is one value from an ENA controlled vocabulary field.
type ENAControlledVocabValue struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// NewClient returns a client configured for the public ENA and NCBI metadata services.
func NewClient() *Client {
	return &Client{
		BaseURL: BasePortalURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchKeyValue returns the ENA search parameter for an input accession type
// at a requested output level.
func SearchKeyValue(queryType AccessionType, resultType AccessionType, accession string) (string, string, error) {
	switch queryType {
	case AccessionTypeAssembly:
		if resultType != AccessionTypeAssembly {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "accession=" + accession, nil
	case AccessionTypeContigSet:
		switch resultType {
		case AccessionTypeWGSSet:
			return "query", "wgs_set=" + accession, nil
		case AccessionTypeTSASet, AccessionTypeTLSSet:
			return "query", "accession=" + contigSetMasterAccession(accession), nil
		default:
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
	case AccessionTypeWGSSet:
		if resultType != AccessionTypeWGSSet {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "wgs_set=" + accession, nil
	case AccessionTypeTSASet, AccessionTypeTLSSet:
		if resultType != queryType {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "accession=" + contigSetMasterAccession(accession), nil
	case AccessionTypeSequence:
		if resultType != AccessionTypeSequence {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "accession=" + accession, nil
	case AccessionTypeCoding:
		if resultType != AccessionTypeCoding {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "accession=" + accession, nil
	case AccessionTypeAnalysis:
		if resultType != AccessionTypeAnalysis {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "analysis_accession=" + accession, nil
	case AccessionTypeStudy:
		switch resultType {
		case AccessionTypeStudy:
			return "query", "study_accession=" + accession + " OR secondary_study_accession=" + accession, nil
		case AccessionTypeSample, AccessionTypeRun, AccessionTypeAssembly, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeAnalysis:
			return "query", "study_accession=" + accession, nil
		default:
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
	case AccessionTypeSample:
		switch resultType {
		case AccessionTypeSample, AccessionTypeRun, AccessionTypeAssembly, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeAnalysis:
			return "query", "sample_accession=" + accession + " OR secondary_sample_accession=" + accession, nil
		default:
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
	case AccessionTypeRun:
		if resultType != AccessionTypeRun && resultType != AccessionTypeAssembly && resultType != AccessionTypeWGSSet && resultType != AccessionTypeAnalysis {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "run_accession=" + accession, nil
	case AccessionTypeExperiment:
		switch resultType {
		case AccessionTypeRun:
			return "query", "experiment_accession=" + accession, nil
		case AccessionTypeAnalysis:
			return "query", "experiment_accession=" + accession, nil
		default:
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
	default:
		return "", "", fmt.Errorf("unsupported accession type %q", queryType)
	}
}

// ResolveFields expands SMALL, DEFAULT, and BIG field presets for an accession
// type. Unknown presets, including ALL, are passed through unchanged.
func ResolveFields(accessionType AccessionType, fields []string) ([]string, error) {
	if len(fields) == 0 {
		fields = []string{"DEFAULT"}
	}

	presets, ok := fieldPresets[accessionType]
	if !ok {
		return nil, fmt.Errorf("unsupported accession type %q", accessionType)
	}

	if preset, ok := presets[fields[0]]; ok {
		return copyStrings(preset), nil
	}

	return copyStrings(fields), nil
}

// Query searches ENA for one normalized accession at a requested output level.
func (c *Client) Query(ctx context.Context, accession string, accessionType AccessionType, fields []string, level AccessionType) (AccessionType, []string, []Record, error) {
	return c.queryENA(ctx, accession, accessionType, fields, level)
}

// QueryENA searches ENA using a raw ENA Portal API query string. Result accepts
// either an ichsm result alias such as "run" or a concrete ENA result id such as
// "read_run".
func (c *Client) QueryENA(ctx context.Context, opts ENAQueryOptions) (ENAQueryResult, error) {
	resultType, enaResult, resolvedFields, params, err := enaRawQueryParams(opts, true)
	if err != nil {
		return ENAQueryResult{}, err
	}

	records, err := c.requestJSON(ctx, "search", params)
	if err != nil {
		return ENAQueryResult{}, err
	}
	addSourceToRecords(records, SearchSourceENA)

	return ENAQueryResult{
		ResultType: resultType,
		ENAResult:  enaResult,
		Query:      strings.TrimSpace(opts.Query),
		Fields:     resolvedFields,
		Records:    records,
	}, nil
}

// CountENAQuery returns the number of ENA records matching a raw ENA Portal API
// query string.
func (c *Client) CountENAQuery(ctx context.Context, opts ENAQueryOptions) (AccessionType, string, int, error) {
	opts.Limit = 0
	opts.Offset = 0
	resultType, enaResult, _, params, err := enaRawQueryParams(opts, false)
	if err != nil {
		return "", "", 0, err
	}

	count, err := c.requestCount(ctx, params)
	if err != nil {
		return "", "", 0, err
	}
	return resultType, enaResult, count, nil
}

// QueryWithSource searches for one accession using the requested source. Auto
// source queries ENA first, then falls back to NCBI when ENA returns no rows and
// the accession has an NCBI route.
func (c *Client) QueryWithSource(ctx context.Context, inputAccession string, accession string, accessionType AccessionType, fields []string, level AccessionType, source SearchSource) (SearchSource, AccessionType, []string, []Record, error) {
	source, err := normalizeSearchSource(source)
	if err != nil {
		return "", "", nil, nil, err
	}

	switch source {
	case SearchSourceENA:
		resultType, resolvedFields, records, err := c.queryENA(ctx, accession, accessionType, fields, level)
		return SearchSourceENA, resultType, resolvedFields, records, err
	case SearchSourceNCBI:
		resultType, resolvedFields, records, err := c.queryNCBI(ctx, inputAccession, accession, accessionType, fields, level)
		return SearchSourceNCBI, resultType, resolvedFields, records, err
	case SearchSourceAuto:
		resultType, resolvedFields, records, err := c.queryENA(ctx, accession, accessionType, fields, level)
		if err == nil && len(records) > 0 {
			return SearchSourceENA, resultType, resolvedFields, records, nil
		}
		if !supportsNCBI(accessionType) {
			return SearchSourceENA, resultType, resolvedFields, records, err
		}
		if err != nil && supportsENA(accessionType) {
			return SearchSourceENA, resultType, resolvedFields, records, err
		}
		resultType, resolvedFields, records, err = c.queryNCBI(ctx, inputAccession, accession, accessionType, fields, level)
		return SearchSourceNCBI, resultType, resolvedFields, records, err
	default:
		return "", "", nil, nil, fmt.Errorf("unsupported source %q", source)
	}
}

func (c *Client) queryENA(ctx context.Context, accession string, accessionType AccessionType, fields []string, level AccessionType) (AccessionType, []string, []Record, error) {
	resultType, err := ResolveSearchLevel(accessionType, level)
	if err != nil {
		return "", nil, nil, err
	}
	if resultType == AccessionTypeContigSet {
		return c.queryENAContigSet(ctx, accession, accessionType, fields)
	}

	if accessionType == AccessionTypeStudy && resultType != AccessionTypeStudy && !isPrimaryStudyAccession(accession) {
		accession, err = c.resolvePrimaryStudyAccession(ctx, accession)
		if err != nil {
			return "", nil, nil, err
		}
	}

	resolvedFields, err := ResolveFields(resultType, fields)
	if err != nil {
		return "", nil, nil, err
	}

	endpoint, params, err := enaSearchParams(accession, accessionType, resultType, resolvedFields, nil)
	if err != nil {
		return "", nil, nil, err
	}

	results, err := c.requestJSON(ctx, endpoint.mainType, params)
	if err != nil {
		return "", nil, nil, err
	}
	addSourceToRecords(results, SearchSourceENA)

	return resultType, resolvedFields, results, nil
}

func (c *Client) queryENAContigSet(ctx context.Context, accession string, accessionType AccessionType, fields []string) (AccessionType, []string, []Record, error) {
	candidates := []AccessionType{AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet}
	var lastResultType AccessionType
	var lastFields []string
	var lastErr error
	for _, resultType := range candidates {
		resolvedFields, err := ResolveFields(resultType, fields)
		if err != nil {
			lastErr = err
			continue
		}

		endpoint, params, err := enaSearchParams(accession, accessionType, resultType, resolvedFields, nil)
		if err != nil {
			lastErr = err
			continue
		}

		records, err := c.requestJSON(ctx, endpoint.mainType, params)
		if err != nil {
			lastErr = err
			continue
		}
		lastResultType = resultType
		lastFields = resolvedFields
		if len(records) > 0 {
			addSourceToRecords(records, SearchSourceENA)
			return resultType, resolvedFields, records, nil
		}
	}

	if lastErr != nil {
		return "", nil, nil, lastErr
	}
	if lastResultType == "" {
		lastResultType = AccessionTypeContigSet
		lastFields, _ = ResolveFields(AccessionTypeContigSet, fields)
	}
	return lastResultType, lastFields, nil, nil
}

func enaSearchParams(accession string, accessionType AccessionType, resultType AccessionType, fields []string, filters map[string]string) (searchEndpoint, url.Values, error) {
	searchKey, searchValue, err := SearchKeyValue(accessionType, resultType, accession)
	if err != nil {
		return searchEndpoint{}, nil, err
	}

	endpoint, ok := urlSearchData[resultType]
	if !ok {
		return searchEndpoint{}, nil, fmt.Errorf("unsupported accession type %q", resultType)
	}

	params := url.Values{}
	params.Set("result", endpoint.result)
	params.Set(searchKey, filteredENAQuery(searchValue, filters))
	params.Set("format", "json")
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}
	return endpoint, params, nil
}

func enaRawQueryParams(opts ENAQueryOptions, includeFields bool) (AccessionType, string, []string, url.Values, error) {
	resultType, enaResult, ok := NormalizeENAResult(opts.Result)
	if !ok {
		return "", "", nil, nil, fmt.Errorf("unsupported ENA result %q; expected study, sample, run/read_run, assembly, sequence, coding, analysis, wgs_set, tsa_set, or tls_set", opts.Result)
	}

	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return "", "", nil, nil, errors.New("ENA query is required")
	}
	if opts.Limit < 0 {
		return "", "", nil, nil, fmt.Errorf("limit must be non-negative")
	}
	if opts.Offset < 0 {
		return "", "", nil, nil, fmt.Errorf("offset must be non-negative")
	}

	resolvedFields := []string(nil)
	if includeFields {
		var err error
		resolvedFields, err = ResolveFields(resultType, opts.Fields)
		if err != nil {
			return "", "", nil, nil, err
		}
	}

	params := url.Values{}
	params.Set("result", enaResult)
	params.Set("query", query)
	params.Set("format", "json")
	if includeFields && len(resolvedFields) > 0 {
		params.Set("fields", strings.Join(resolvedFields, ","))
	}
	if opts.Limit > 0 {
		params.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		params.Set("offset", strconv.Itoa(opts.Offset))
	}

	return resultType, enaResult, resolvedFields, params, nil
}

// CountENA returns the number of ENA records matching one normalized accession
// at a requested output level.
func (c *Client) CountENA(ctx context.Context, accession string, accessionType AccessionType, level AccessionType) (AccessionType, int, error) {
	return c.countENA(ctx, accession, accessionType, level)
}

// CountENAFiltered returns the number of ENA records matching one normalized
// accession at a requested output level, with additional ENA query filters.
func (c *Client) CountENAFiltered(ctx context.Context, accession string, accessionType AccessionType, level AccessionType, filters map[string]string) (AccessionType, int, error) {
	return c.countENAFiltered(ctx, accession, accessionType, level, filters)
}

func (c *Client) countENA(ctx context.Context, accession string, accessionType AccessionType, level AccessionType) (AccessionType, int, error) {
	return c.countENAFiltered(ctx, accession, accessionType, level, nil)
}

func (c *Client) countENAFiltered(ctx context.Context, accession string, accessionType AccessionType, level AccessionType, filters map[string]string) (AccessionType, int, error) {
	resultType, err := ResolveSearchLevel(accessionType, level)
	if err != nil {
		return "", 0, err
	}
	if resultType == AccessionTypeContigSet {
		return c.countENAContigSetFiltered(ctx, accession, accessionType, filters)
	}

	if accessionType == AccessionTypeStudy && resultType != AccessionTypeStudy && !isPrimaryStudyAccession(accession) {
		accession, err = c.resolvePrimaryStudyAccession(ctx, accession)
		if err != nil {
			return "", 0, err
		}
	}

	count, err := c.countENAResultTypeFiltered(ctx, accession, accessionType, resultType, filters)
	if err != nil {
		return "", 0, err
	}
	return resultType, count, nil
}

func (c *Client) countENAContigSet(ctx context.Context, accession string, accessionType AccessionType) (AccessionType, int, error) {
	return c.countENAContigSetFiltered(ctx, accession, accessionType, nil)
}

func (c *Client) countENAContigSetFiltered(ctx context.Context, accession string, accessionType AccessionType, filters map[string]string) (AccessionType, int, error) {
	candidates := []AccessionType{AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet}
	var lastResultType AccessionType
	var lastErr error
	for _, resultType := range candidates {
		count, err := c.countENAResultTypeFiltered(ctx, accession, accessionType, resultType, filters)
		if err != nil {
			lastErr = err
			continue
		}
		lastResultType = resultType
		if count > 0 {
			return resultType, count, nil
		}
	}

	if lastErr != nil {
		return "", 0, lastErr
	}
	if lastResultType == "" {
		lastResultType = AccessionTypeContigSet
	}
	return lastResultType, 0, nil
}

func (c *Client) countENAResultType(ctx context.Context, accession string, accessionType AccessionType, resultType AccessionType) (int, error) {
	return c.countENAResultTypeFiltered(ctx, accession, accessionType, resultType, nil)
}

func (c *Client) countENAResultTypeFiltered(ctx context.Context, accession string, accessionType AccessionType, resultType AccessionType, filters map[string]string) (int, error) {
	_, params, err := enaSearchParams(accession, accessionType, resultType, nil, filters)
	if err != nil {
		return 0, err
	}
	return c.requestCount(ctx, params)
}

func filteredENAQuery(baseQuery string, filters map[string]string) string {
	keys := make([]string, 0, len(filters))
	for key, value := range filters {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) > 0 && strings.Contains(baseQuery, " OR ") {
		baseQuery = "(" + baseQuery + ")"
	}
	sort.Strings(keys)
	for _, key := range keys {
		baseQuery += " AND " + strings.TrimSpace(key) + "=" + strings.TrimSpace(filters[key])
	}
	return baseQuery
}

// Search identifies and queries a set of accessions. As in the original CLI,
// all accessions must have the same inferred type.
func (c *Client) Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if len(opts.Accessions) == 0 {
		return nil, fmt.Errorf("no accessions provided")
	}

	type accessionSearch struct {
		input string
		fixed string
		typ   AccessionType
	}

	toSearch := make([]accessionSearch, 0, len(opts.Accessions))
	var firstType AccessionType
	for _, accession := range opts.Accessions {
		fixedAccession, accessionType, ok := IdentifyAccession(accession)
		if !ok {
			return nil, fmt.Errorf("accession format not recognised: %s", accession)
		}
		if firstType == "" {
			firstType = accessionType
		} else if accessionType != firstType {
			return nil, fmt.Errorf("accessions must all be the same type: got %s and %s", firstType, accessionType)
		}
		toSearch = append(toSearch, accessionSearch{input: accession, fixed: fixedAccession, typ: accessionType})
	}

	results := make([]SearchResult, 0, len(toSearch))
	for _, accession := range toSearch {
		source, resultType, fields, records, err := c.QueryWithSource(ctx, accession.input, accession.fixed, accession.typ, opts.Fields, opts.Level, opts.Source)
		if err != nil {
			return nil, err
		}
		results = append(results, SearchResult{
			InputAccession: accession.input,
			FixedAccession: accession.fixed,
			InputType:      accession.typ,
			ResultType:     resultType,
			Source:         source,
			Fields:         fields,
			Records:        records,
		})
	}

	return results, nil
}

// ResolveSearchLevel returns the ENA result level to search. A zero level means
// the closest report level for the input accession type.
func ResolveSearchLevel(inputType AccessionType, level AccessionType) (AccessionType, error) {
	if level == "" {
		if inputType == AccessionTypeExperiment {
			return AccessionTypeRun, nil
		}
		if inputType == AccessionTypeWGSSet || inputType == AccessionTypeTSASet || inputType == AccessionTypeTLSSet {
			return inputType, nil
		}
		return inputType, nil
	}

	if inputType == AccessionTypeContigSet && level == AccessionTypeAssembly {
		return AccessionTypeContigSet, nil
	}

	switch level {
	case AccessionTypeAssembly, AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeSequence, AccessionTypeCoding, AccessionTypeStudy, AccessionTypeSample, AccessionTypeRun, AccessionTypeAnalysis:
	default:
		return "", fmt.Errorf("unsupported search level %q; expected study, sample, run, assembly, sequence, coding, analysis, contig_set, wgs_set, tsa_set, or tls_set", level)
	}

	if inputType == AccessionTypeContigSet {
		switch level {
		case AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet:
			return level, nil
		}
	}

	if _, _, err := SearchKeyValue(inputType, level, ""); err != nil {
		return "", err
	}
	return level, nil
}

func unsupportedSearchLevel(inputType AccessionType, level AccessionType) error {
	return fmt.Errorf("cannot search %s accessions at %s level", inputType, level)
}

func normalizeSearchSource(source SearchSource) (SearchSource, error) {
	switch SearchSource(strings.ToLower(strings.TrimSpace(string(source)))) {
	case "", SearchSourceAuto:
		return SearchSourceAuto, nil
	case SearchSourceENA:
		return SearchSourceENA, nil
	case SearchSourceNCBI:
		return SearchSourceNCBI, nil
	default:
		return "", fmt.Errorf("unsupported source %q; expected auto, ena, or ncbi", source)
	}
}

func addSourceToRecords(records []Record, source SearchSource) {
	for _, record := range records {
		record["source"] = string(source)
	}
}

// SupportsENA reports whether ichsm has an ENA search route for an accession type.
func SupportsENA(accessionType AccessionType) bool {
	return supportsENA(accessionType)
}

// SupportsNCBI reports whether ichsm has an NCBI search route for an accession type.
func SupportsNCBI(accessionType AccessionType) bool {
	return supportsNCBI(accessionType)
}

func (c *Client) resolvePrimaryStudyAccession(ctx context.Context, accession string) (string, error) {
	endpoint, params, err := enaSearchParams(accession, AccessionTypeStudy, AccessionTypeStudy, []string{"study_accession"}, nil)
	if err != nil {
		return "", err
	}

	results, err := c.requestJSON(ctx, endpoint.mainType, params)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", fmt.Errorf("no study found for accession %s", accession)
	}

	studyAccession, ok := results[0]["study_accession"].(string)
	if !ok || studyAccession == "" {
		return "", fmt.Errorf("no primary study accession found for accession %s", accession)
	}
	return studyAccession, nil
}

// GetAllowedFields returns the ENA searchFields response for a result type,
// such as read_run.
func (c *Client) GetAllowedFields(ctx context.Context, dataType string) (string, error) {
	params := url.Values{}
	params.Set("result", dataType)
	return c.requestText(ctx, "searchFields", params)
}

// GetControlledVocabulary returns the allowed values for an ENA controlled
// vocabulary field, such as instrument_platform or library_layout.
func (c *Client) GetControlledVocabulary(ctx context.Context, field string) ([]ENAControlledVocabValue, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil, errors.New("controlled vocabulary field is required")
	}

	params := url.Values{}
	params.Set("field", field)
	body, err := c.request(ctx, "controlledVocab", params)
	if err != nil {
		return nil, err
	}

	var values []ENAControlledVocabValue
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("error parsing ENA controlled vocabulary json: %w", err)
	}
	return values, nil
}

// GetResultTypes returns the ENA results response listing available data types.
func (c *Client) GetResultTypes(ctx context.Context) (string, error) {
	return c.requestText(ctx, "results", url.Values{})
}

// SortedRecordKeys returns record keys in deterministic order. It is useful
// when ENA's ALL field preset is requested and the output columns come from the
// returned JSON object.
func SortedRecordKeys(record Record) []string {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *Client) requestJSON(ctx context.Context, path string, params url.Values) ([]Record, error) {
	body, err := c.request(ctx, path, params)
	if err != nil {
		return nil, err
	}

	var results []Record
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&results); err != nil {
		return nil, fmt.Errorf("error parsing json from query: %w", err)
	}

	for _, result := range results {
		for key, value := range result {
			if value == "" {
				result[key] = nil
			}
		}
	}

	return results, nil
}

func (c *Client) requestText(ctx context.Context, path string, params url.Values) (string, error) {
	body, err := c.request(ctx, path, params)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) requestCount(ctx context.Context, params url.Values) (int, error) {
	body, err := c.request(ctx, "count", params)
	if err != nil {
		return 0, err
	}

	var response struct {
		Count string `json:"count"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("error parsing ENA count json: %w", err)
	}
	count, err := strconv.Atoi(response.Count)
	if err != nil {
		return 0, fmt.Errorf("error parsing ENA count value %q: %w", response.Count, err)
	}
	return count, nil
}

func (c *Client) request(ctx context.Context, path string, params url.Values) ([]byte, error) {
	baseURL := BasePortalURL
	if c != nil && c.BaseURL != "" {
		baseURL = c.BaseURL
	}
	return c.requestWithBase(ctx, baseURL, path, params, "ENA", &enaRequestLimiter, c.enaRateLimitInterval())
}

func (c *Client) requestWithBase(ctx context.Context, baseURL string, path string, params url.Values, serviceName string, limiter *requestRateLimiter, rateLimitInterval time.Duration) ([]byte, error) {
	requestURL, err := requestURL(baseURL, path, params)
	if err != nil {
		return nil, err
	}

	maxRetries := c.maxRequestRetries()
	for attempt := 0; ; attempt++ {
		if limiter != nil {
			if err := limiter.wait(ctx, rateLimitInterval); err != nil {
				return nil, fmt.Errorf("error waiting for %s rate limit: %w", serviceName, err)
			}
		}

		body, err := c.requestOnce(ctx, requestURL)
		if err == nil {
			return body, nil
		}
		if !isRetryableRequestError(err) || attempt >= maxRetries {
			if attempt > 0 {
				return nil, fmt.Errorf("error requesting data after %d attempts: %w", attempt+1, err)
			}
			return nil, err
		}

		delay := c.requestRetryDelay(attempt, err)
		if err := sleepContext(ctx, delay); err != nil {
			return nil, fmt.Errorf("error waiting to retry request: %w", err)
		}
	}
}

func (c *Client) requestOnce(ctx context.Context, requestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("error requesting data from %s: %w", requestURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &requestStatusError{
			statusCode: resp.StatusCode,
			url:        requestURL,
			body:       trimResponseBody(body),
			retryAfter: resp.Header.Get("Retry-After"),
		}
	}

	return body, nil
}

type requestStatusError struct {
	statusCode int
	url        string
	body       string
	retryAfter string
}

func (e *requestStatusError) Error() string {
	return fmt.Sprintf("error requesting data: status=%d url=%s body=%s", e.statusCode, e.url, e.body)
}

func isRetryableRequestError(err error) bool {
	statusErr, ok := err.(*requestStatusError)
	if !ok {
		return false
	}
	if statusErr.statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusErr.statusCode == http.StatusInternalServerError ||
		statusErr.statusCode == http.StatusBadGateway ||
		statusErr.statusCode == http.StatusServiceUnavailable ||
		statusErr.statusCode == http.StatusGatewayTimeout
}

func (c *Client) requestRetryDelay(attempt int, err error) time.Duration {
	if statusErr, ok := err.(*requestStatusError); ok {
		if delay, ok := parseRetryAfter(statusErr.retryAfter, time.Now()); ok {
			return delay
		}
	}

	delay := c.retryBaseDelay()
	for i := 0; i < attempt; i++ {
		if delay >= c.retryMaxDelay()/2 {
			delay = c.retryMaxDelay()
			break
		}
		delay *= 2
	}
	if maxDelay := c.retryMaxDelay(); delay > maxDelay {
		delay = maxDelay
	}
	if delay <= 0 {
		return 0
	}

	jitterLimit := delay / 2
	if jitterLimit <= 0 {
		return delay
	}
	delay += time.Duration(rand.Int63n(int64(jitterLimit) + 1))
	if maxDelay := c.retryMaxDelay(); delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0, true
		}
		return time.Duration(seconds) * time.Second, true
	}
	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := retryAt.Sub(now)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func trimResponseBody(body []byte) string {
	const maxErrorBodyBytes = 1000
	text := strings.TrimSpace(string(body))
	if len(text) <= maxErrorBodyBytes {
		return text
	}
	return text[:maxErrorBodyBytes] + "..."
}

func (c *Client) maxRequestRetries() int {
	if c != nil && c.MaxRequestRetries < 0 {
		return 0
	}
	if c != nil && c.MaxRequestRetries > 0 {
		return c.MaxRequestRetries
	}
	return defaultMaxRequestRetries
}

func (c *Client) retryBaseDelay() time.Duration {
	if c != nil && c.RequestRetryBaseDelay > 0 {
		return c.RequestRetryBaseDelay
	}
	return defaultRetryBaseDelay
}

func (c *Client) retryMaxDelay() time.Duration {
	if c != nil && c.RequestRetryMaxDelay > 0 {
		return c.RequestRetryMaxDelay
	}
	return defaultRetryMaxDelay
}

func (c *Client) enaRateLimitInterval() time.Duration {
	requestsPerSecond := defaultENARequestsPerSecond
	if c != nil && c.ENARequestsPerSecond != 0 {
		requestsPerSecond = c.ENARequestsPerSecond
	}
	return requestRateLimitInterval(requestsPerSecond)
}

func (c *Client) ncbiRateLimitInterval() time.Duration {
	requestsPerSecond := defaultNCBIRequestsPerSecond
	if c != nil && strings.TrimSpace(c.NCBIAPIKey) != "" {
		requestsPerSecond = defaultNCBIAPIKeyRequestsPerSecond
	}
	if c != nil && c.NCBIRequestsPerSecond != 0 {
		requestsPerSecond = c.NCBIRequestsPerSecond
	}
	return requestRateLimitInterval(requestsPerSecond)
}

func requestRateLimitInterval(requestsPerSecond int) time.Duration {
	if requestsPerSecond <= 0 {
		return 0
	}
	interval := time.Second / time.Duration(requestsPerSecond)
	if interval <= 0 {
		return time.Nanosecond
	}
	return interval
}

type requestRateLimiter struct {
	mu   sync.Mutex
	next time.Time
}

func (l *requestRateLimiter) wait(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	if l.next.IsZero() || now.After(l.next) {
		l.next = now.Add(interval)
		l.mu.Unlock()
		return nil
	}
	delay := l.next.Sub(now)
	l.next = l.next.Add(interval)
	l.mu.Unlock()

	return sleepContext(ctx, delay)
}

func (c *Client) requestURL(path string, params url.Values) (string, error) {
	baseURL := BasePortalURL
	if c != nil && c.BaseURL != "" {
		baseURL = c.BaseURL
	}
	return requestURL(baseURL, path, params)
}

func requestURL(baseURL string, path string, params url.Values) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return "", err
	}
	parsed.RawQuery = params.Encode()
	return parsed.String(), nil
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}

	return &http.Client{Timeout: 30 * time.Second}
}
