package syn

// (program 1.0.0 (con integer 1))
type Program[T any] struct {
	Version [3]uint32
	Term    Term[T]
}
