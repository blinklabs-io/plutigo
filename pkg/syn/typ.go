package syn

type Typ interface {
	isTyp()
}

type TBool struct{}

func (t TBool) isTyp() {}

type TInteger struct{}

func (t TInteger) isTyp() {}

type TString struct{}

func (t TString) isTyp() {}

type TByteString struct{}

func (t TByteString) isTyp() {}

type TUnit struct{}

func (t TUnit) isTyp() {}

type TList struct {
	Typ
}

func (t TList) isTyp() {}

type TPair struct {
	First  Typ
	Second Typ
}

func (t TPair) isTyp() {}

type TData struct{}

func (t TData) isTyp() {}
