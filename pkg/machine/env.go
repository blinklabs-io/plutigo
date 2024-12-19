package machine

type Env []Value

func (e *Env) lookup(name uint) (*Value, bool) {
	idx := len(*e) - int(name)

	if idx < 0 || idx >= len(*e) {
		return nil, false
	}

	return &(*e)[idx], true
}
