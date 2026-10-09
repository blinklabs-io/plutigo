// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
