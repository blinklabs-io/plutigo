package cek

import (
	"github.com/blinklabs-io/plutigo/lang"
)

type SemanticsVariant int

const (
	SemanticsVariantA SemanticsVariant = 1
	SemanticsVariantB SemanticsVariant = 2
	SemanticsVariantC SemanticsVariant = 3
	SemanticsVariantD SemanticsVariant = 4
	SemanticsVariantE SemanticsVariant = 5
)

const (
	changProtoMajorVersion     = 9
	vanRossemProtoMajorVersion = 11
)

type ProtoVersion struct {
	Major uint
	Minor uint
}

func GetSemantics(
	version lang.LanguageVersion,
	protoVersion ProtoVersion,
) SemanticsVariant {
	switch version {
	case lang.LanguageVersionV1, lang.LanguageVersionV2:
		if protoVersion.Major < changProtoMajorVersion {
			return SemanticsVariantA
		}
		if protoVersion.Major < vanRossemProtoMajorVersion {
			return SemanticsVariantB
		}
		return SemanticsVariantD
	default:
		if protoVersion.Major >= vanRossemProtoMajorVersion {
			return SemanticsVariantE
		}
		return SemanticsVariantC
	}
}
