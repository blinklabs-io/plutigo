package pkg

type Typ interface{}

type TBool struct{}

type TInteger struct{}

type TString struct{}

type TByteString struct{}

type TUnit struct{}

type TList struct {
	Typ
}

type TPair struct {
	First  Typ
	Second Typ
}

type TData struct{}
