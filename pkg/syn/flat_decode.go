package syn

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	d := newDecoder()

	return nil, nil
}

type decoder struct{}

func newDecoder() *decoder {
	return &decoder{}
}
