package ichsm

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// Publication is one PubMed-linked publication for a project/study accession.
type Publication struct {
	InputAccession   string   `json:"input_accession"`
	ProjectAccession string   `json:"project_accession"`
	Relation         string   `json:"relation"`
	Sources          []string `json:"sources"`
	PubMedID         string   `json:"pubmed_id"`
	Year             string   `json:"year"`
	Journal          string   `json:"journal"`
	DOI              string   `json:"doi"`
	Title            string   `json:"title"`
}

type publicationProject struct {
	accession string
	relation  string
}

type publicationLink struct {
	inputAccession   string
	projectAccession string
	relation         string
	pubMedID         string
	sources          []string
}

type publicationCollector struct {
	input string
	links []publicationLink
	index map[string]int
}

type enaBrowserPublicationInfo struct {
	Accession        string
	ParentAccessions []string
	PubMedIDs        []string
}

type pubMedSummary struct {
	UID             string `json:"uid"`
	PubDate         string `json:"pubdate"`
	Source          string `json:"source"`
	FullJournalName string `json:"fulljournalname"`
	Title           string `json:"title"`
	ElocationID     string `json:"elocationid"`
	ArticleIDs      []struct {
		IDType string `json:"idtype"`
		Value  string `json:"value"`
	} `json:"articleids"`
}

const (
	publicationRelationSelf   = "self"
	publicationRelationParent = "parent"
	publicationSourceENA      = "ena"
	publicationSourceNCBI     = "ncbi"
)

var publicationYearRE = regexp.MustCompile(`\b(?:19|20)[0-9]{2}\b`)

// Publications returns PubMed-linked publications for a study/project
// accession. It checks the accession itself and its immediate parent projects.
func (c *Client) Publications(ctx context.Context, accession string) ([]Publication, error) {
	inputAccession := strings.TrimSpace(accession)
	fixedAccession, accessionType, ok := IdentifyAccession(inputAccession)
	if !ok {
		return nil, fmt.Errorf("accession format not recognised: %s", accession)
	}
	if accessionType != AccessionTypeStudy {
		return nil, fmt.Errorf("pubs supports study/project accessions")
	}

	projects := make([]publicationProject, 0, 3)
	projectIndex := map[string]int{}
	addProject := func(accession string, relation string) {
		accession = strings.ToUpper(strings.TrimSpace(accession))
		if accession == "" {
			return
		}
		if existing, ok := projectIndex[accession]; ok {
			if projects[existing].relation != publicationRelationSelf && relation == publicationRelationSelf {
				projects[existing].relation = publicationRelationSelf
			}
			return
		}
		projectIndex[accession] = len(projects)
		projects = append(projects, publicationProject{accession: accession, relation: relation})
	}

	addProject(fixedAccession, publicationRelationSelf)
	if !isPrimaryStudyAccession(fixedAccession) {
		primaryAccession, err := c.resolvePrimaryStudyAccession(ctx, fixedAccession)
		if err != nil {
			return nil, fmt.Errorf("error resolving primary project accession for %s: %w", fixedAccession, err)
		}
		addProject(primaryAccession, publicationRelationSelf)
	}

	collector := newPublicationCollector(inputAccession)
	for i := 0; i < len(projects); i++ {
		project := projects[i]
		body, err := c.requestENABrowserXML(ctx, project.accession)
		if err != nil {
			return nil, fmt.Errorf("error getting ENA Browser XML for project %s: %w", project.accession, err)
		}
		info, err := parseENABrowserPublications(body)
		if err != nil {
			return nil, fmt.Errorf("error parsing ENA Browser XML for project %s: %w", project.accession, err)
		}
		projectAccession := project.accession
		if info.Accession != "" {
			projectAccession = info.Accession
		}
		for _, pubMedID := range info.PubMedIDs {
			collector.add(projectAccession, project.relation, pubMedID, publicationSourceENA)
		}
		if project.relation == publicationRelationSelf {
			for _, parentAccession := range info.ParentAccessions {
				addProject(parentAccession, publicationRelationParent)
			}
		}
	}

	for _, project := range projects {
		uid, err := c.ncbiBioProjectID(ctx, project.accession)
		if err != nil {
			return nil, fmt.Errorf("error getting NCBI BioProject id for %s: %w", project.accession, err)
		}
		if uid == "" {
			continue
		}
		pubMedIDs, err := c.ncbiBioProjectPubMedIDs(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("error getting NCBI PubMed links for %s: %w", project.accession, err)
		}
		for _, pubMedID := range pubMedIDs {
			collector.add(project.accession, project.relation, pubMedID, publicationSourceNCBI)
		}
	}

	links := collector.orderedLinks()
	pubMedIDs := make([]string, 0, len(links))
	for _, link := range links {
		pubMedIDs = append(pubMedIDs, link.pubMedID)
	}
	summaries, err := c.ncbiPubMedSummaries(ctx, pubMedIDs)
	if err != nil {
		return nil, err
	}

	publications := make([]Publication, 0, len(links))
	for _, link := range links {
		summary := summaries[link.pubMedID]
		publications = append(publications, Publication{
			InputAccession:   link.inputAccession,
			ProjectAccession: link.projectAccession,
			Relation:         link.relation,
			Sources:          append([]string(nil), link.sources...),
			PubMedID:         link.pubMedID,
			Year:             publicationYear(summary.PubDate),
			Journal:          publicationJournal(summary),
			DOI:              publicationDOI(summary),
			Title:            cleanPublicationText(summary.Title),
		})
	}
	return publications, nil
}

func newPublicationCollector(input string) *publicationCollector {
	return &publicationCollector{
		input: input,
		index: map[string]int{},
	}
}

func (c *publicationCollector) add(projectAccession string, relation string, pubMedID string, source string) {
	projectAccession = strings.ToUpper(strings.TrimSpace(projectAccession))
	relation = strings.TrimSpace(relation)
	pubMedID = strings.TrimSpace(pubMedID)
	source = strings.TrimSpace(source)
	if projectAccession == "" || relation == "" || pubMedID == "" || source == "" {
		return
	}

	key := projectAccession + "\x00" + relation + "\x00" + pubMedID
	if index, ok := c.index[key]; ok {
		c.links[index].sources = appendSource(c.links[index].sources, source)
		return
	}

	c.index[key] = len(c.links)
	c.links = append(c.links, publicationLink{
		inputAccession:   c.input,
		projectAccession: projectAccession,
		relation:         relation,
		pubMedID:         pubMedID,
		sources:          []string{source},
	})
}

func (c *publicationCollector) orderedLinks() []publicationLink {
	out := make([]publicationLink, len(c.links))
	copy(out, c.links)
	return out
}

func appendSource(sources []string, source string) []string {
	for _, existing := range sources {
		if existing == source {
			return sources
		}
	}
	return append(sources, source)
}

func (c *Client) requestENABrowserXML(ctx context.Context, accession string) ([]byte, error) {
	baseURL := BaseBrowserXMLURL
	if c != nil && c.BrowserBaseURL != "" {
		baseURL = c.BrowserBaseURL
	}
	return c.requestWithBase(ctx, baseURL, accession, nil, "ENA Browser", &enaRequestLimiter, c.enaRateLimitInterval())
}

func parseENABrowserPublications(body []byte) (enaBrowserPublicationInfo, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var info enaBrowserPublicationInfo
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return info, nil
			}
			return info, err
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "PROJECT", "STUDY":
			if accession := xmlAttr(start, "accession"); accession != "" && info.Accession == "" {
				info.Accession = accession
			}
		case "PARENT_PROJECT":
			info.ParentAccessions = appendUniqueString(info.ParentAccessions, xmlAttr(start, "accession"))
		case "XREF_LINK":
			var xref struct {
				DB string `xml:"DB"`
				ID string `xml:"ID"`
			}
			if err := decoder.DecodeElement(&xref, &start); err != nil {
				return info, err
			}
			if strings.EqualFold(strings.TrimSpace(xref.DB), "PUBMED") {
				info.PubMedIDs = appendUniqueString(info.PubMedIDs, xref.ID)
			}
		}
	}
}

func xmlAttr(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (c *Client) ncbiBioProjectID(ctx context.Context, accession string) (string, error) {
	params := url.Values{}
	params.Set("db", "bioproject")
	params.Set("term", strings.ToUpper(strings.TrimSpace(accession))+"[Project Accession]")
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
		return "", fmt.Errorf("error parsing NCBI bioproject esearch json: %w", err)
	}
	if response.Error != "" {
		return "", fmt.Errorf("NCBI bioproject esearch error: %s", response.Error)
	}
	if len(response.ESearchResult.IDList) == 0 {
		return "", nil
	}
	return response.ESearchResult.IDList[0], nil
}

func (c *Client) ncbiBioProjectPubMedIDs(ctx context.Context, bioprojectUID string) ([]string, error) {
	params := url.Values{}
	params.Set("dbfrom", "bioproject")
	params.Set("db", "pubmed")
	params.Set("id", strings.TrimSpace(bioprojectUID))
	params.Set("retmode", "json")
	c.addNCBIParams(params)

	body, err := c.requestNCBI(ctx, "elink.fcgi", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Error    string `json:"error"`
		LinkSets []struct {
			LinkSetDBs []struct {
				DBTo  string   `json:"dbto"`
				Links []string `json:"links"`
			} `json:"linksetdbs"`
		} `json:"linksets"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error parsing NCBI bioproject elink json: %w", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("NCBI bioproject elink error: %s", response.Error)
	}

	var pubMedIDs []string
	for _, linkSet := range response.LinkSets {
		for _, linkSetDB := range linkSet.LinkSetDBs {
			if linkSetDB.DBTo != "" && !strings.EqualFold(linkSetDB.DBTo, "pubmed") {
				continue
			}
			for _, pubMedID := range linkSetDB.Links {
				pubMedIDs = appendUniqueString(pubMedIDs, pubMedID)
			}
		}
	}
	return pubMedIDs, nil
}

func (c *Client) ncbiPubMedSummaries(ctx context.Context, pubMedIDs []string) (map[string]pubMedSummary, error) {
	out := make(map[string]pubMedSummary, len(pubMedIDs))
	if len(pubMedIDs) == 0 {
		return out, nil
	}

	for start := 0; start < len(pubMedIDs); start += 200 {
		end := start + 200
		if end > len(pubMedIDs) {
			end = len(pubMedIDs)
		}
		if err := c.addNCBIPubMedSummaryBatch(ctx, out, pubMedIDs[start:end]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (c *Client) addNCBIPubMedSummaryBatch(ctx context.Context, summaries map[string]pubMedSummary, pubMedIDs []string) error {
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("id", strings.Join(pubMedIDs, ","))
	params.Set("retmode", "json")
	c.addNCBIParams(params)

	body, err := c.requestNCBI(ctx, "esummary.fcgi", params)
	if err != nil {
		return err
	}

	var envelope struct {
		Error  string                     `json:"error"`
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("error parsing NCBI pubmed esummary json: %w", err)
	}
	if envelope.Error != "" {
		return fmt.Errorf("NCBI pubmed esummary error: %s", envelope.Error)
	}

	for _, pubMedID := range pubMedIDs {
		raw, ok := envelope.Result[pubMedID]
		if !ok {
			continue
		}
		var summary pubMedSummary
		if err := json.Unmarshal(raw, &summary); err != nil {
			return fmt.Errorf("error parsing NCBI pubmed summary for %s: %w", pubMedID, err)
		}
		summaries[pubMedID] = summary
	}
	return nil
}

func publicationYear(pubDate string) string {
	return publicationYearRE.FindString(pubDate)
}

func publicationJournal(summary pubMedSummary) string {
	if summary.Source != "" {
		return cleanPublicationText(summary.Source)
	}
	return cleanPublicationText(summary.FullJournalName)
}

func publicationDOI(summary pubMedSummary) string {
	for _, articleID := range summary.ArticleIDs {
		if strings.EqualFold(articleID.IDType, "doi") {
			return cleanPublicationText(articleID.Value)
		}
	}
	if doi, ok := strings.CutPrefix(strings.TrimSpace(summary.ElocationID), "doi:"); ok {
		return cleanPublicationText(doi)
	}
	return ""
}

func cleanPublicationText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
