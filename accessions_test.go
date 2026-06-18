package ichsm

import "testing"

func TestIdentifyAccession(t *testing.T) {
	tests := []struct {
		accession string
		fixed     string
		typ       AccessionType
		ok        bool
	}{
		{"GCA_123456789", "GCA_123456789", AccessionTypeAssembly, true},
		{"GCA_123456789.1", "GCA_123456789", AccessionTypeAssembly, true},
		{"GCA_12345678.1", "", "", false},
		{"GCF_123456789.1", "GCF_123456789", AccessionTypeAssembly, true},
		{"AGQU00000000.1", "AGQU01", AccessionTypeContigSet, true},
		{"AGQU000000000.2", "AGQU02", AccessionTypeContigSet, true},
		{"ABCDEF000000000.3", "ABCDEF03", AccessionTypeContigSet, true},
		{"AGQU01", "AGQU01", AccessionTypeContigSet, true},
		{"AGQU01000000", "AGQU01", AccessionTypeContigSet, true},
		{"BAAHUD010000000", "BAAHUD01", AccessionTypeContigSet, true},
		{"AGQU00000000", "AGQU00000000", AccessionTypeSequence, true},
		{"AGQU01000001.1", "AGQU01000001", AccessionTypeSequence, true},
		{"G123456.1", "", "", false},
		{"U49845.1", "U49845", AccessionTypeSequence, true},
		{"AF086833.2", "AF086833", AccessionTypeSequence, true},
		{"AB12345678.1", "AB12345678", AccessionTypeSequence, true},
		{"NC_000001.11", "NC_000001", AccessionTypeSequence, true},
		{"AAA98665.1", "AAA98665", AccessionTypeCoding, true},
		{"ABC1234567.1", "ABC1234567", AccessionTypeCoding, true},
		{"WP_002248791.1", "WP_002248791", AccessionTypeCoding, true},
		{"PRJEB1787", "PRJEB1787", AccessionTypeStudy, true},
		{"PRJNA123456", "PRJNA123456", AccessionTypeStudy, true},
		{"ERP001736", "ERP001736", AccessionTypeStudy, true},
		{"DRP123456", "DRP123456", AccessionTypeStudy, true},
		{"SRP123456", "SRP123456", AccessionTypeStudy, true},
		{"ERP12345", "", "", false},
		{"PRJ123456", "", "", false},
		{"SAMN123456", "SAMN123456", AccessionTypeSample, true},
		{"ERS123456", "ERS123456", AccessionTypeSample, true},
		{"ERS12345", "", "", false},
		{"ERR123456", "ERR123456", AccessionTypeRun, true},
		{"ERR12345", "", "", false},
		{"ERX123456", "ERX123456", AccessionTypeExperiment, true},
	}

	for _, tt := range tests {
		fixed, typ, ok := IdentifyAccession(tt.accession)
		if fixed != tt.fixed || typ != tt.typ || ok != tt.ok {
			t.Fatalf("IdentifyAccession(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.accession, fixed, typ, ok, tt.fixed, tt.typ, tt.ok)
		}
	}
}
