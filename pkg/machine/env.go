package machine

type Env []Value

func (e *Env) lookup(name uint) (*Value, bool) {
	idx := len(*e) - int(name)

	if !indexExists(*e, idx) {
		return nil, false
	}

	return &(*e)[idx], true
}
