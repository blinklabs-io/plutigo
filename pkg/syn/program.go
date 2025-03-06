package syn

type Program[T Binder] struct {
	version [3]uint32
	term    Term[T]
}
