package connection

import (
	"mime"
	"strings"

	"github.com/surrealdb/surrealdb.go/internal/codec"
)

// CBORVersion is the CBOR protocol version a connection negotiated with the server.
//
// SurrealDB 4.0 adds cbor.2, which is plain CBOR plus tags for its sized
// numbers (i8 to u64, f16 and f32). Connections offer cbor.2 when their codec
// speaks it, as surrealcbor's does, and fall back to plain CBOR when the
// server does not: servers before 4.0 only speak plain CBOR.
//
//   - WebSocket offers the subprotocols "cbor.2" and "cbor" and uses the one
//     the server picks in the handshake.
//   - HTTP sends "Accept: application/cbor; v=2" with plain CBOR requests
//     until a response comes back as "application/cbor; v=2", then sends
//     cbor.2 requests. Each response is decoded by its own Content-Type.
//
// See the CBOR tag table in https://github.com/surrealdb/surrealdb-protocol
// (spec/cbor-tags.yaml) for the wire format.
type CBORVersion int

const (
	// CBORv1 is plain CBOR: WebSocket subprotocol "cbor", HTTP "application/cbor".
	CBORv1 CBORVersion = codec.CBORv1
	// CBORv2 is cbor.2, SurrealDB 4.0's CBOR with sized numbers: WebSocket
	// subprotocol "cbor.2", HTTP "application/cbor; v=2".
	CBORv2 CBORVersion = codec.CBORv2
)

// String returns the version's WebSocket subprotocol name.
func (v CBORVersion) String() string {
	if v == CBORv2 {
		return SubprotocolCBOR2
	}
	return SubprotocolCBOR
}

// WebSocket subprotocols and HTTP media types for the CBOR protocol versions.
const (
	SubprotocolCBOR  = "cbor"
	SubprotocolCBOR2 = "cbor.2"

	MIMECBOR  = "application/cbor"
	MIMECBOR2 = "application/cbor; v=2"
)

// CBORVersioner is implemented by connections that report the CBOR protocol
// version they negotiated with the server.
//
// A WebSocket connection knows its version once Connect returns. An HTTP
// connection reports CBORv1 until the server has answered a request in cbor.2.
type CBORVersioner interface {
	CBORVersion() CBORVersion
}

// CBORCodecs returns the marshaler and unmarshaler to use for version v,
// derived from m and u. ok is false when they cannot speak v; for CBORv1 it is
// always true.
func CBORCodecs(m codec.Marshaler, u codec.Unmarshaler, v CBORVersion) (codec.Marshaler, codec.Unmarshaler, bool) {
	return codec.ForCBORVersion(m, u, int(v))
}

// CanSpeakCBORv2 reports whether m and u can both encode and decode cbor.2.
func CanSpeakCBORv2(m codec.Marshaler, u codec.Unmarshaler) bool {
	_, _, ok := CBORCodecs(m, u, CBORv2)
	return ok
}

// CBORSubprotocols returns the WebSocket subprotocols to offer, in order of
// preference: "cbor.2" and "cbor" when offerV2 is true, else just "cbor".
func CBORSubprotocols(offerV2 bool) []string {
	if offerV2 {
		return []string{SubprotocolCBOR2, SubprotocolCBOR}
	}
	return []string{SubprotocolCBOR}
}

// CBORVersionFromSubprotocol returns the CBOR version the server picked in a
// WebSocket handshake. Anything but "cbor.2" is plain CBOR.
func CBORVersionFromSubprotocol(subprotocol string) CBORVersion {
	if strings.TrimSpace(subprotocol) == SubprotocolCBOR2 {
		return CBORv2
	}
	return CBORv1
}

// CBORVersionFromContentType returns the CBOR version of an HTTP body with
// the given Content-Type. ok is false when it is not CBOR.
// The "v" parameter is matched case-insensitively and may be quoted.
func CBORVersionFromContentType(contentType string) (v CBORVersion, ok bool) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != MIMECBOR {
		return 0, false
	}
	switch strings.TrimSpace(params["v"]) {
	case "", "1":
		return CBORv1, true
	case "2":
		return CBORv2, true
	default:
		return 0, false
	}
}
