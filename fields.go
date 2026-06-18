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
