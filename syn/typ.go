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

type TBls12_381G1Element struct{}

func (TBls12_381G1Element) isTyp() {}

type TBls12_381G2Element struct{}

func (TBls12_381G2Element) isTyp() {}

type TBls12_381MlResult struct{}

func (TBls12_381MlResult) isTyp() {}

type TValue struct{}

func (TValue) isTyp() {}

func EqualType(a, b Typ) bool {
	switch ta := a.(type) {
	case nil:
		return b == nil
	case *TBool:
		_, ok := b.(*TBool)
		return ok
	case *TInteger:
		_, ok := b.(*TInteger)
		return ok
	case *TString:
		_, ok := b.(*TString)
		return ok
	case *TByteString:
		_, ok := b.(*TByteString)
		return ok
	case *TUnit:
		_, ok := b.(*TUnit)
		return ok
	case *TData:
		_, ok := b.(*TData)
		return ok
	case *TBls12_381G1Element:
		_, ok := b.(*TBls12_381G1Element)
		return ok
	case *TBls12_381G2Element:
		_, ok := b.(*TBls12_381G2Element)
		return ok
	case *TBls12_381MlResult:
		_, ok := b.(*TBls12_381MlResult)
		return ok
	case *TValue:
		_, ok := b.(*TValue)
		return ok
	case *TList:
		tb, ok := b.(*TList)
		return ok && EqualType(ta.Typ, tb.Typ)
	case *TPair:
		tb, ok := b.(*TPair)
		return ok && EqualType(ta.First, tb.First) &&
			EqualType(ta.Second, tb.Second)
	default:
		return false
	}
}
