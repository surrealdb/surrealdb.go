package codec

type Marshaler interface {
	Marshal(v any) ([]byte, error)
}

type Unmarshaler interface {
	Unmarshal(data []byte, dst any) error
}

// Codec both marshals and unmarshals.
type Codec interface {
	Marshaler
	Unmarshaler
}

// CBOR protocol versions, as negotiated with the server.
const (
	// CBORv1 is plain CBOR: WebSocket subprotocol "cbor", HTTP "application/cbor".
	CBORv1 = 1
	// CBORv2 is SurrealDB 4.0's CBOR with sized numbers: WebSocket subprotocol
	// "cbor.2", HTTP "application/cbor; v=2".
	CBORv2 = 2
)

// CBORVersioned is implemented by a Marshaler or Unmarshaler that can speak
// more than one CBOR protocol version. Connections use it to pick the codec
// for the version they negotiated.
type CBORVersioned interface {
	// ForCBORVersion returns a codec for CBOR protocol version v,
	// or nil when the codec does not speak it.
	ForCBORVersion(v int) Codec
}

// ForCBORVersion returns the marshaler and unmarshaler to use for CBOR
// protocol version v, derived from m and u. ok is false when either of them
// cannot speak v. Version 1 always succeeds: m and u are returned as they are
// unless they say how to speak version 1.
func ForCBORVersion(m Marshaler, u Unmarshaler, v int) (Marshaler, Unmarshaler, bool) {
	vm, mok := m.(CBORVersioned)
	vu, uok := u.(CBORVersioned)
	if v == CBORv1 {
		if mok {
			if c := vm.ForCBORVersion(CBORv1); c != nil {
				m = c
			}
		}
		if uok {
			if c := vu.ForCBORVersion(CBORv1); c != nil {
				u = c
			}
		}
		return m, u, true
	}
	if !mok || !uok {
		return nil, nil, false
	}
	cm := vm.ForCBORVersion(v)
	cu := vu.ForCBORVersion(v)
	if cm == nil || cu == nil {
		return nil, nil, false
	}
	return cm, cu, true
}
