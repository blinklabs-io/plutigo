package cek

type ExBudget struct {
	Mem int64
	Cpu int64
}

var DefaultExBudget = ExBudget{
	// Use mainnet-like limits by default; tests needing more should override explicitly
	Mem: 14_000_000,
	Cpu: 10_000_000_000,
}

func (ex *ExBudget) occurrences(n uint32) {
	ex.Mem *= int64(n)
	ex.Cpu *= int64(n)
}

func (ex *ExBudget) Sub(other *ExBudget) ExBudget {
	return ExBudget{
		Mem: ex.Mem - other.Mem,
		Cpu: ex.Cpu - other.Cpu,
	}
}
