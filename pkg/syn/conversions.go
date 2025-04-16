package syn

func (p *Program[Name]) NamedDeBruijn() (*Program[NamedDeBruijn], error) {
	panic("hi")
}

func (p *Program[Name]) DeBruijn() (*Program[DeBruijn], error) {
	panic("hi")
}
