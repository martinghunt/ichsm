package ichsm

import (
	"regexp"
	"strings"
)

// AccessionType is the metadata category inferred from an accession.
type AccessionType string

const (
	AccessionTypeAssembly   AccessionType = "assembly"
	AccessionTypeContigSet  AccessionType = "contig_set"
	AccessionTypeWGSSet     AccessionType = "wgs_set"
	AccessionTypeTSASet     AccessionType = "tsa_set"
	AccessionTypeTLSSet     AccessionType = "tls_set"
	AccessionTypeSequence   AccessionType = "sequence"
	AccessionTypeCoding     AccessionType = "coding"
	AccessionTypeStudy      AccessionType = "study"
	AccessionTypeSample     AccessionType = "sample"
	AccessionTypeRun        AccessionType = "run"
	AccessionTypeExperiment AccessionType = "experiment"
)

type accessionRegex struct {
	re        *regexp.Regexp
	typ       AccessionType
	normalize func([]string) string
}

var accessionRegexes = []accessionRegex{
	{re: regexp.MustCompile(`^(GC[AF]_[0-9]{9})(\.[0-9]*)*$`), typ: AccessionTypeAssembly, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^(?:([A-Z]{4})0{8,10}|([A-Z]{6})0{9,11})\.([0-9]+)$`), typ: AccessionTypeContigSet, normalize: normalizeWGSSetMasterAccession},
	{re: regexp.MustCompile(`^([A-Z]{4}(?:0[1-9]|[1-9][0-9]))0{6}$`), typ: AccessionTypeContigSet, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^([A-Z]{6}(?:0[1-9]|[1-9][0-9]))0{7}$`), typ: AccessionTypeContigSet, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^([A-Z]{4}[0-9]{2}|[A-Z]{6}[0-9]{2})$`), typ: AccessionTypeContigSet, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^(PRJ(?:E|D|N)[A-Z][0-9]+)$`), typ: AccessionTypeStudy, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^((?:E|D|S)RP[0-9]{6,})$`), typ: AccessionTypeStudy, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^(SAM(?:E|D|N)[A-Z]?[0-9]+)$`), typ: AccessionTypeSample, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^((?:E|D|S)RS[0-9]{6,})$`), typ: AccessionTypeSample, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^((?:E|D|S)RR[0-9]{6,})$`), typ: AccessionTypeRun, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^((?:E|D|S)RX[0-9]{6,})$`), typ: AccessionTypeExperiment, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^((?:WP|NP|XP|YP|AP|ZP)_[0-9]+)(\.[0-9]+)*$`), typ: AccessionTypeCoding, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^((?:NC|NG|NM|NR|NT|NW|NZ|XM|XR|AC|CM|CP)_[0-9]+)(\.[0-9]+)*$`), typ: AccessionTypeSequence, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^([A-Z]{3}(?:[0-9]{5}|[0-9]{7}))(\.[0-9]+)*$`), typ: AccessionTypeCoding, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^([A-Z][0-9]{5}|[A-Z]{2}[0-9]{6}|[A-Z]{2}[0-9]{8})(\.[0-9]+)*$`), typ: AccessionTypeSequence, normalize: firstAccessionMatch},
	{re: regexp.MustCompile(`^([A-Z]{4}[0-9]{8,}|[A-Z]{6}[0-9]{9,})(\.[0-9]+)*$`), typ: AccessionTypeSequence, normalize: firstAccessionMatch},
}

var primaryStudyAccessionRE = regexp.MustCompile(`^PRJ(?:E|D|N)[A-Z][0-9]+$`)

// IdentifyAccession returns the normalized accession, its type, and whether it
// was recognized. Version suffixes are stripped where the metadata APIs expect
// core accessions. WGS/TSA/TLS master accessions are normalized to their set id.
func IdentifyAccession(accession string) (string, AccessionType, bool) {
	accession = strings.ToUpper(strings.TrimSpace(accession))
	for _, candidate := range accessionRegexes {
		matches := candidate.re.FindStringSubmatch(accession)
		if matches != nil {
			if candidate.typ == AccessionTypeCoding && isReservedSRASecondaryPrefix(matches[1]) {
				continue
			}
			return candidate.normalize(matches), candidate.typ, true
		}
	}

	return "", "", false
}

func isPrimaryStudyAccession(accession string) bool {
	return primaryStudyAccessionRE.MatchString(strings.ToUpper(strings.TrimSpace(accession)))
}

func isReservedSRASecondaryPrefix(accession string) bool {
	core, _ := splitAccessionVersion(accession)
	if len(core) < 3 {
		return false
	}
	switch core[:3] {
	case "ERP", "DRP", "SRP", "ERS", "DRS", "SRS", "ERR", "DRR", "SRR", "ERX", "DRX", "SRX":
		return true
	default:
		return false
	}
}

func splitAccessionVersion(accession string) (string, string) {
	if dot := strings.LastIndexByte(accession, '.'); dot > 0 && allDigits(accession[dot+1:]) {
		return accession[:dot], accession[dot+1:]
	}
	return accession, ""
}

func contigSetMasterAccession(accession string) string {
	if len(accession) == 6 {
		return accession + "000000"
	}
	if len(accession) == 8 {
		return accession + "0000000"
	}
	return accession
}

func firstAccessionMatch(matches []string) string {
	return matches[1]
}

func normalizeWGSSetMasterAccession(matches []string) string {
	prefix := matches[1]
	if prefix == "" {
		prefix = matches[2]
	}

	version := matches[3]
	if len(version) == 1 {
		version = "0" + version
	}
	return prefix + version
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
