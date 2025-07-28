package codec

import "io"

type Encoder interface {
	Encode(v any) error
}

type Decoder interface {
	Decode(v any) error
}

type Marshaler interface {
	Marshal(v any) ([]byte, error)
	NewEncoder(w io.Writer) Encoder
}

type Unmarshaler interface {
	Unmarshal(data []byte, dst any) error
	NewDecoder(r io.Reader) Decoder
}
