package cek

type SemanticsVariant int

const (
	SemanticsVariantA SemanticsVariant = 1
	SemanticsVariantB SemanticsVariant = 2
	SemanticsVariantC SemanticsVariant = 3
)

func GetSemantics(version LanguageVersion) SemanticsVariant {
	switch version {
	case LanguageVersionV1:
		return SemanticsVariantA
	case LanguageVersionV2:
		return SemanticsVariantB
	default:
		// V3 or later
		return SemanticsVariantC
	}
}
