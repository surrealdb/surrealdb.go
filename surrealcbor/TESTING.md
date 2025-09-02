# Testing Strategy for surrealcbor Package

The testing strategy for `surrealcbor` is three-fold:

1. **Compatibility Testing** (`decode*_test.go`): By using the standard cbor library for encoding, we ensure that surrealcbor can correctly decode CBOR data from other implementations, not just its own output.

2. **Round-trip Testing** (`encode*_test.go`): By using surrealcbor for both encoding and decoding, we ensure that our implementation is internally consistent and can correctly handle its own output.

3. **Comprehensive Coverage**: Together, these two approaches provide comprehensive test coverage:
   - External compatibility (can we decode standard CBOR?)
   - Internal consistency (can we round-trip our own data?)
   - Edge cases and error conditions in both encoding and decoding paths

## Test File Naming Convention

The test files in this package follow a specific naming pattern that indicates their testing approach:

### `decode*_test.go` Files

These files test the **unmarshaling (decoding)** functionality of surrealcbor.

**Testing approach:**
- **Encoding**: Uses the standard `github.com/fxamacker/cbor/v2` library
- **Decoding**: Uses our `surrealcbor` implementation

This approach ensures that our decoder can correctly unmarshal CBOR data encoded by a well-established, standard CBOR library. It validates that surrealcbor is compatible with standard CBOR encoding.

**Example:**
```go
// In decode_simple_test.go
import (
    "github.com/fxamacker/cbor/v2"  // For encoding
    "github.com/surrealdb/surrealdb.go/surrealcbor"  // For decoding
)

func TestDecode_float64(t *testing.T) {
    floatVal := 123.456
    enc, err := cbor.Marshal(floatVal)  // Standard library encoding
    require.NoError(t, err)
    
    var decoded float64
    err = surrealcbor.Unmarshal(enc, &decoded)  // Our decoder
    require.NoError(t, err)
    assert.Equal(t, floatVal, decoded)
}
```

The decode tests are organized by functionality:

- `decode_simple_test.go` - Simple value decoding (booleans, floats, nil)
- `decode_tag_*_test.go` - CBOR tag decoding for various SurrealDB types
- `decode_map_*_test.go` - Map and struct decoding
- `decode_invalid_test.go` - Error handling and invalid input testing

### `encode*_test.go` Files

These files test **both marshaling (encoding) and unmarshaling (decoding)** functionality of surrealcbor.

**Testing approach:**
- **Encoding**: Uses our `surrealcbor` implementation
- **Decoding**: Uses our `surrealcbor` implementation

This approach performs full round-trip testing, ensuring that data encoded by surrealcbor can be correctly decoded by surrealcbor. It validates the internal consistency of surrealcbor.

**Example:**

```go
// In encode_tag_durationbinary_test.go
import (
    "github.com/surrealdb/surrealdb.go/surrealcbor"  // For both encoding and decoding
)

func TestEncode_durationBinary(t *testing.T) {
    dur := models.CustomDuration{Duration: 90 * time.Minute}
    
    enc, err := surrealcbor.Marshal(dur)  // Our encoder
    require.NoError(t, err)
    
    var decoded models.CustomDuration
    err = surrealcbor.Unmarshal(enc, &decoded)  // Our decoder
    require.NoError(t, err)
    assert.Equal(t, dur.Duration, decoded.Duration)
}
```

Some `encode*_test.go` files may occasionally import `github.com/fxamacker/cbor/v2` for verification purposes (e.g., to inspect the raw CBOR structure of encoded data). This is used only for test assertions and debugging, not for the actual encoding/decoding being tested.
