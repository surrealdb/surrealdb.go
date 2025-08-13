package codec

type Marshaler interface {
	Marshal(v any) ([]byte, error)
}

type Unmarshaler interface {
	Unmarshal(data []byte, dst any) error
}
