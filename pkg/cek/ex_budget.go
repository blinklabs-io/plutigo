package cek

type ExBudget struct {
	mem int64
	cpu int64
}

func (ex *ExBudget) occurrences(n uint32) {
	ex.mem *= int64(n)
	ex.cpu *= int64(n)
}

var DefaultExBudget = ExBudget{
	mem: 14000000,
	cpu: 10000000000,
}
