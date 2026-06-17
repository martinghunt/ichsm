package ichsm

const (
	FieldPresetSmall   = "SMALL"
	FieldPresetDefault = "DEFAULT"
	FieldPresetBig     = "BIG"
	FieldPresetAll     = "ALL"
)

// FieldPresetLevelForResult reports the lowest ichsm -c preset that includes
// field for an ENA result type. Fields outside BIG are available only through
// ALL. The boolean is false when ichsm has no search preset for resultType.
func FieldPresetLevelForResult(resultType string, field string) (string, bool) {
	accessionType, ok := accessionTypeForResult(resultType)
	if !ok {
		return "", false
	}
	return fieldPresetLevel(accessionType, field), true
}

func accessionTypeForResult(resultType string) (AccessionType, bool) {
	switch resultType {
	case "assembly":
		return AccessionTypeAssembly, true
	case "wgs_set":
		return AccessionTypeWGSSet, true
	case "tsa_set":
		return AccessionTypeTSASet, true
	case "tls_set":
		return AccessionTypeTLSSet, true
	case "sequence":
		return AccessionTypeSequence, true
	case "coding":
		return AccessionTypeCoding, true
	case "study":
		return AccessionTypeStudy, true
	case "sample":
		return AccessionTypeSample, true
	case "read_run":
		return AccessionTypeRun, true
	default:
		return "", false
	}
}

func fieldPresetLevel(accessionType AccessionType, field string) string {
	presets, ok := fieldPresets[accessionType]
	if !ok {
		return ""
	}
	for _, level := range []string{FieldPresetSmall, FieldPresetDefault, FieldPresetBig} {
		if stringSliceContains(presets[level], field) {
			return level
		}
	}
	return FieldPresetAll
}

func stringSliceContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
