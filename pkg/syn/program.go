package syn

type Program[T Binder] struct {
	Version [3]uint32
	Term    Term[T]
}
