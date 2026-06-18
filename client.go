package ichsm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	BaseURL     string
	NCBIBaseURL string
	NCBIAPIKey  string
	NCBIEmail   string
	NCBITool    string
	HTTPClient  *http.Client
}

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
	case AccessionTypeStudy:
		switch resultType {
		case AccessionTypeStudy:
			return "query", "study_accession=" + accession + " OR secondary_study_accession=" + accession, nil
		case AccessionTypeSample, AccessionTypeRun, AccessionTypeAssembly, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet:
			return "query", "study_accession=" + accession, nil
		default:
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
	case AccessionTypeSample:
		switch resultType {
		case AccessionTypeSample, AccessionTypeRun, AccessionTypeAssembly, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet:
			return "query", "sample_accession=" + accession + " OR secondary_sample_accession=" + accession, nil
		default:
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
	case AccessionTypeRun:
		if resultType != AccessionTypeRun && resultType != AccessionTypeAssembly && resultType != AccessionTypeWGSSet {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "run_accession=" + accession, nil
	case AccessionTypeExperiment:
		if resultType != AccessionTypeRun {
			return "", "", unsupportedSearchLevel(queryType, resultType)
		}
		return "query", "experiment_accession=" + accession, nil
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

	searchKey, searchValue, err := SearchKeyValue(accessionType, resultType, accession)
	if err != nil {
		return "", nil, nil, err
	}

	endpoint, ok := urlSearchData[resultType]
	if !ok {
		return "", nil, nil, fmt.Errorf("unsupported accession type %q", resultType)
	}

	resolvedFields, err := ResolveFields(resultType, fields)
	if err != nil {
		return "", nil, nil, err
	}

	params := url.Values{}
	params.Set("result", endpoint.result)
	params.Set(searchKey, searchValue)
	params.Set("format", "json")
	params.Set("fields", strings.Join(resolvedFields, ","))

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
		searchKey, searchValue, err := SearchKeyValue(accessionType, resultType, accession)
		if err != nil {
			lastErr = err
			continue
		}
		endpoint := urlSearchData[resultType]
		resolvedFields, err := ResolveFields(resultType, fields)
		if err != nil {
			lastErr = err
			continue
		}

		params := url.Values{}
		params.Set("result", endpoint.result)
		params.Set(searchKey, searchValue)
		params.Set("format", "json")
		params.Set("fields", strings.Join(resolvedFields, ","))

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

// CountENA returns the number of ENA records matching one normalized accession
// at a requested output level.
func (c *Client) CountENA(ctx context.Context, accession string, accessionType AccessionType, level AccessionType) (AccessionType, int, error) {
	return c.countENA(ctx, accession, accessionType, level)
}

func (c *Client) countENA(ctx context.Context, accession string, accessionType AccessionType, level AccessionType) (AccessionType, int, error) {
	resultType, err := ResolveSearchLevel(accessionType, level)
	if err != nil {
		return "", 0, err
	}
	if resultType == AccessionTypeContigSet {
		return c.countENAContigSet(ctx, accession, accessionType)
	}

	if accessionType == AccessionTypeStudy && resultType != AccessionTypeStudy && !isPrimaryStudyAccession(accession) {
		accession, err = c.resolvePrimaryStudyAccession(ctx, accession)
		if err != nil {
			return "", 0, err
		}
	}

	count, err := c.countENAResultType(ctx, accession, accessionType, resultType)
	if err != nil {
		return "", 0, err
	}
	return resultType, count, nil
}

func (c *Client) countENAContigSet(ctx context.Context, accession string, accessionType AccessionType) (AccessionType, int, error) {
	candidates := []AccessionType{AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet}
	var lastResultType AccessionType
	var lastErr error
	for _, resultType := range candidates {
		count, err := c.countENAResultType(ctx, accession, accessionType, resultType)
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
	searchKey, searchValue, err := SearchKeyValue(accessionType, resultType, accession)
	if err != nil {
		return 0, err
	}

	endpoint, ok := urlSearchData[resultType]
	if !ok {
		return 0, fmt.Errorf("unsupported accession type %q", resultType)
	}

	params := url.Values{}
	params.Set("result", endpoint.result)
	params.Set(searchKey, searchValue)
	params.Set("format", "json")
	return c.requestCount(ctx, params)
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
	case AccessionTypeAssembly, AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeSequence, AccessionTypeCoding, AccessionTypeStudy, AccessionTypeSample, AccessionTypeRun:
	default:
		return "", fmt.Errorf("unsupported search level %q; expected study, sample, run, assembly, sequence, coding, contig_set, wgs_set, tsa_set, or tls_set", level)
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

func supportsENA(accessionType AccessionType) bool {
	switch accessionType {
	case AccessionTypeAssembly, AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeSequence, AccessionTypeCoding, AccessionTypeStudy, AccessionTypeSample, AccessionTypeRun, AccessionTypeExperiment:
		return true
	default:
		return false
	}
}

// SupportsENA reports whether ichsm has an ENA search route for an accession type.
func SupportsENA(accessionType AccessionType) bool {
	return supportsENA(accessionType)
}

func supportsNCBI(accessionType AccessionType) bool {
	switch accessionType {
	case AccessionTypeAssembly, AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeSequence, AccessionTypeCoding:
		return true
	default:
		return false
	}
}

// SupportsNCBI reports whether ichsm has an NCBI search route for an accession type.
func SupportsNCBI(accessionType AccessionType) bool {
	return supportsNCBI(accessionType)
}

func (c *Client) resolvePrimaryStudyAccession(ctx context.Context, accession string) (string, error) {
	searchKey, searchValue, err := SearchKeyValue(AccessionTypeStudy, AccessionTypeStudy, accession)
	if err != nil {
		return "", err
	}

	endpoint := urlSearchData[AccessionTypeStudy]
	params := url.Values{}
	params.Set("result", endpoint.result)
	params.Set(searchKey, searchValue)
	params.Set("format", "json")
	params.Set("fields", "study_accession")

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
	return c.requestWithBase(ctx, baseURL, path, params)
}

func (c *Client) requestWithBase(ctx context.Context, baseURL string, path string, params url.Values) ([]byte, error) {
	requestURL, err := requestURL(baseURL, path, params)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("error requesting data: status=%d url=%s body=%s", resp.StatusCode, requestURL, strings.TrimSpace(string(body)))
	}

	return body, nil
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
