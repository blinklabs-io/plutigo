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

const changProtoMajorVersion = 9

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
		} else {
			return SemanticsVariantB
		}
	default:
		return SemanticsVariantC
	}
}
