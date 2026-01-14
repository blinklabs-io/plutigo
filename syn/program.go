package syn

import (
	"github.com/blinklabs-io/plutigo/lang"
)

// (program 1.0.0 (con integer 1))
type Program[T any] struct {
	Version lang.LanguageVersion
	Term    Term[T]
}
