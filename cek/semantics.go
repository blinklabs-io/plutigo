package cek

import (
	"github.com/blinklabs-io/plutigo/lang"
)

type SemanticsVariant int

const (
	SemanticsVariantA SemanticsVariant = 1
	SemanticsVariantB SemanticsVariant = 2
	SemanticsVariantC SemanticsVariant = 3
)

func GetSemantics(version lang.LanguageVersion) SemanticsVariant {
	switch version {
	case lang.LanguageVersionV1:
		return SemanticsVariantA
	case lang.LanguageVersionV2:
		return SemanticsVariantB
	default:
		// V3 or later
		return SemanticsVariantC
	}
}
