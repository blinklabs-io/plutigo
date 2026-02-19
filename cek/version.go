package cek

type LanguageVersion = [3]uint32

var (
	LanguageVersionV1 = [3]uint32{1, 0, 0}
	LanguageVersionV2 = [3]uint32{1, 1, 0}
	LanguageVersionV3 = [3]uint32{1, 2, 0}
	LanguageVersionV4 = [3]uint32{1, 3, 0}
)

// VersionLessThan returns true if v is less than other
func VersionLessThan(v, other LanguageVersion) bool {
	for i := range 3 {
		if v[i] < other[i] {
			return true
		}
		if v[i] > other[i] {
			return false
		}
	}
	return false
}
