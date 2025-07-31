package syn

type biMap struct {
	left  map[Unique]uint
	right map[uint]Unique
}

func (b *biMap) insert(unique Unique, level uint) {
	b.left[unique] = level
	b.right[level] = unique
}

func (b *biMap) remove(unique Unique, level uint) {
	delete(b.left, unique)
	delete(b.right, level)
}

func (b *biMap) getByUnique(unique Unique) (uint, bool) {
	level, ok := b.left[unique]

	return level, ok
}

//nolint:unused
func (b *biMap) getByLevel(level uint) (Unique, bool) {
	unique, ok := b.right[level]

	return unique, ok
}
