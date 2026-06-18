package ichsm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const BaseNCBIEUtilsURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/"

func (c *Client) queryNCBI(ctx context.Context, inputAccession string, accession string, accessionType AccessionType, fields []string, level AccessionType) (AccessionType, []string, []Record, error) {
	resultType, err := ResolveNCBIResultLevel(accessionType, level)
	if err != nil {
		return "", nil, nil, err
	}

	db, ok := ncbiDatabase(resultType)
	if !ok {
		return "", nil, nil, fmt.Errorf("cannot search %s accessions in NCBI", accessionType)
	}

	resolvedFields, err := ResolveFields(resultType, fields)
	if err != nil {
		return "", nil, nil, err
	}

	id, err := c.ncbiSearchID(ctx, db, resultType, ncbiAccessionCandidates(inputAccession, accession, accessionType))
	if err != nil {
		return "", nil, nil, err
	}
	if id == "" {
		return resultType, resolvedFields, nil, nil
	}

	summary, err := c.ncbiSummary(ctx, db, id)
	if err != nil {
		return "", nil, nil, err
	}

	record := normalizeNCBISummary(summary, resultType)
	record["source"] = string(SearchSourceNCBI)
	return resultType, resolvedFields, []Record{record}, nil
}

// ResolveNCBIResultLevel returns the normalized ichsm result level supplied by
// NCBI for an accession type.
func ResolveNCBIResultLevel(inputType AccessionType, level AccessionType) (AccessionType, error) {
	if level == "" {
		switch inputType {
		case AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet:
			return AccessionTypeSequence, nil
		default:
			return inputType, nil
		}
	}

	switch inputType {
	case AccessionTypeAssembly:
		if level == AccessionTypeAssembly {
			return AccessionTypeAssembly, nil
		}
	case AccessionTypeSequence:
		if level == AccessionTypeSequence {
			return AccessionTypeSequence, nil
		}
	case AccessionTypeCoding:
		if level == AccessionTypeCoding {
			return AccessionTypeCoding, nil
		}
	case AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet:
		switch level {
		case AccessionTypeAssembly, AccessionTypeContigSet, AccessionTypeWGSSet, AccessionTypeTSASet, AccessionTypeTLSSet, AccessionTypeSequence:
			return AccessionTypeSequence, nil
		}
	}

	return "", unsupportedSearchLevel(inputType, level)
}

func ncbiAccessionCandidates(inputAccession string, accession string, accessionType AccessionType) []string {
	candidates := make([]string, 0, 4)
	add := func(candidate string) {
		candidate = strings.ToUpper(strings.TrimSpace(candidate))
		if candidate == "" {
			return
		}
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	add(inputAccession)
	add(accession)
	core, _ := splitAccessionVersion(inputAccession)
	add(core)
	if accessionType == AccessionTypeContigSet || accessionType == AccessionTypeWGSSet || accessionType == AccessionTypeTSASet || accessionType == AccessionTypeTLSSet {
		add(contigSetMasterAccession(accession))
	}
	return candidates
}

func (c *Client) ncbiSearchID(ctx context.Context, db string, resultType AccessionType, accessions []string) (string, error) {
	for _, accession := range accessions {
		params := url.Values{}
		params.Set("db", db)
		params.Set("term", ncbiSearchTerm(accession, resultType))
		params.Set("retmode", "json")
		params.Set("retmax", "1")
		c.addNCBIParams(params)

		body, err := c.requestNCBI(ctx, "esearch.fcgi", params)
		if err != nil {
			return "", err
		}

		var response struct {
			Error         string `json:"error"`
			ESearchResult struct {
				IDList []string `json:"idlist"`
			} `json:"esearchresult"`
		}
		if err := json.Unmarshal(body, &response); err != nil {
			return "", fmt.Errorf("error parsing NCBI esearch json: %w", err)
		}
		if response.Error != "" {
			return "", fmt.Errorf("NCBI esearch error: %s", response.Error)
		}
		if len(response.ESearchResult.IDList) > 0 {
			return response.ESearchResult.IDList[0], nil
		}
	}
	return "", nil
}

func ncbiSearchTerm(accession string, resultType AccessionType) string {
	if resultType == AccessionTypeAssembly {
		return accession + "[Assembly Accession]"
	}
	return accession + "[Accession]"
}

func (c *Client) ncbiSummary(ctx context.Context, db string, id string) (Record, error) {
	params := url.Values{}
	params.Set("db", db)
	params.Set("id", id)
	params.Set("retmode", "json")
	c.addNCBIParams(params)

	body, err := c.requestNCBI(ctx, "esummary.fcgi", params)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Error  string                     `json:"error"`
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("error parsing NCBI esummary json: %w", err)
	}
	if envelope.Error != "" {
		return nil, fmt.Errorf("NCBI esummary error: %s", envelope.Error)
	}

	raw, ok := envelope.Result[id]
	if !ok {
		return nil, fmt.Errorf("NCBI esummary returned no record for uid %s", id)
	}

	var summary Record
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&summary); err != nil {
		return nil, fmt.Errorf("error parsing NCBI esummary record: %w", err)
	}
	return summary, nil
}

func normalizeNCBISummary(summary Record, resultType AccessionType) Record {
	record := copyRecord(summary)
	switch resultType {
	case AccessionTypeAssembly:
		accessionVersion := ncbiString(summary, "assemblyaccession")
		accession, version := splitAccessionVersion(accessionVersion)
		record["accession"] = accession
		record["version"] = version
		record["description"] = firstNCBIString(summary, "assemblydescription", "assemblyname")
		record["wgs_set"] = ncbiString(summary, "wgs")
		record["assembly_name"] = ncbiString(summary, "assemblyname")
		record["assembly_level"] = ncbiString(summary, "assemblystatus")
		record["assembly_type"] = ncbiString(summary, "assemblytype")
		record["coverage"] = ncbiString(summary, "coverage")
		record["center_name"] = ncbiString(summary, "submitterorganization")
		record["last_updated"] = firstNCBIString(summary, "lastupdatedate", "asmupdatedate")
		record["scientific_name"] = firstNCBIString(summary, "speciesname", "organism")
		record["tax_id"] = firstNCBIString(summary, "taxid", "speciestaxid")
		record["sample_accession"] = ncbiString(summary, "biosampleaccn")
		record["study_accession"] = ncbiAssemblyBioProject(summary)
	case AccessionTypeSequence:
		accessionVersion := firstNCBIString(summary, "accessionversion", "caption")
		accession, version := splitAccessionVersion(accessionVersion)
		record["accession"] = accession
		record["sequence_version"] = version
		record["description"] = ncbiString(summary, "title")
		record["scientific_name"] = ncbiString(summary, "organism")
		record["tax_id"] = ncbiString(summary, "taxid")
		record["base_count"] = ncbiString(summary, "slen")
	case AccessionTypeCoding:
		accessionVersion := firstNCBIString(summary, "accessionversion", "caption")
		accession, version := splitAccessionVersion(accessionVersion)
		record["accession"] = accession
		record["protein_id"] = accessionVersion
		record["sequence_version"] = version
		record["description"] = ncbiString(summary, "title")
		record["product"] = ncbiString(summary, "title")
		record["scientific_name"] = ncbiString(summary, "organism")
		record["tax_id"] = ncbiString(summary, "taxid")
	}
	normalizeEmptyRecordStrings(record)
	return record
}

func copyRecord(in Record) Record {
	out := make(Record, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func normalizeEmptyRecordStrings(record Record) {
	for key, value := range record {
		if value == "" {
			record[key] = nil
		}
	}
}

func firstNCBIString(record Record, keys ...string) string {
	for _, key := range keys {
		value := ncbiString(record, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func ncbiString(record Record, key string) string {
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}

func ncbiAssemblyBioProject(summary Record) string {
	for _, key := range []string{"rs_bioprojects", "gb_bioprojects"} {
		projects, ok := summary[key].([]any)
		if !ok {
			continue
		}
		for _, project := range projects {
			projectMap, ok := project.(map[string]any)
			if !ok {
				continue
			}
			if accession, ok := projectMap["bioprojectaccn"].(string); ok && accession != "" {
				return accession
			}
		}
	}
	return ""
}

func (c *Client) addNCBIParams(params url.Values) {
	tool := strings.TrimSpace(c.NCBITool)
	if tool == "" {
		tool = "ichsm"
	}
	params.Set("tool", tool)
	if c.NCBIAPIKey != "" {
		params.Set("api_key", c.NCBIAPIKey)
	}
	if c.NCBIEmail != "" {
		params.Set("email", c.NCBIEmail)
	}
}

func (c *Client) requestNCBI(ctx context.Context, path string, params url.Values) ([]byte, error) {
	baseURL := BaseNCBIEUtilsURL
	if c != nil && c.NCBIBaseURL != "" {
		baseURL = c.NCBIBaseURL
	}
	return c.requestWithBase(ctx, baseURL, path, params, "NCBI", &ncbiRequestLimiter, c.ncbiRateLimitInterval())
}
