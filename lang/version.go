package lang

type LanguageVersion = [3]uint32

var (
	LanguageVersionV1 = [3]uint32{1, 0, 0}
	LanguageVersionV2 = [3]uint32{1, 1, 0}
	LanguageVersionV3 = [3]uint32{1, 2, 0}
)

func GetParamNamesForVersion(version LanguageVersion) []string {
	switch version {
	case LanguageVersionV1:
		return CostModelParamNamesV1
	case LanguageVersionV2:
		return CostModelParamNamesV2
	case LanguageVersionV3:
		return CostModelParamNamesV3
	default:
		return nil
	}
}
