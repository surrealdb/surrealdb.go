package surrealdb_test

import (
	"bytes"
	"context"
	"math"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// serverSpeaksCBORv2 asks the server the tests run against, without the SDK,
// whether it speaks cbor.2: SurrealDB 4.0 answers a plain CBOR request sent
// with "Accept: application/cbor; v=2" in cbor.2, and older servers in plain CBOR.
func serverSpeaksCBORv2(t *testing.T) bool {
	t.Helper()
	u, err := url.Parse(testenv.GetSurrealDBURL())
	require.NoError(t, err)
	u.Scheme = strings.Replace(u.Scheme, "ws", "http", 1)
	u.Path = "/rpc"

	// {"id": 1, "method": "version"}
	body := []byte{0xa2, 0x62, 'i', 'd', 0x01, 0x66, 'm', 'e', 't', 'h', 'o', 'd', 0x67, 'v', 'e', 'r', 's', 'i', 'o', 'n'}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", connection.MIMECBOR)
	req.Header.Set("Accept", connection.MIMECBOR2)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	v, ok := connection.CBORVersionFromContentType(res.Header.Get("Content-Type"))
	return ok && v == connection.CBORv2
}

// usesDeprecatedCodec reports whether the tests run with the deprecated
// fxamacker/cbor codec, which only speaks plain CBOR.
func usesDeprecatedCodec() bool {
	return os.Getenv(testenv.EnvSurrealCBORImpl) == "fxamackercbor"
}

// TestCBORVersion_negotiated checks the SDK negotiates cbor.2 with a server
// that speaks it (SurrealDB 4.0), and falls back to plain CBOR with one that
// does not (3.x and before), or when its codec cannot speak cbor.2.
func TestCBORVersion_negotiated(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("cbor2", "negotiation", "t")
	defer db.Close(ctx)

	// An HTTP connection negotiates as it sends requests; make sure it has.
	_, err := surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.NoError(t, err)

	want := connection.CBORv1
	if serverSpeaksCBORv2(t) && !usesDeprecatedCodec() {
		want = connection.CBORv2
	}
	t.Logf("negotiated %s", db.CBORVersion())
	assert.Equal(t, want, db.CBORVersion())
}

// sizedValues has one value of each Go kind that maps to a SurrealDB 4.0
// sized number.
func sizedValues() []any {
	return []any{
		int8(-128), uint8(255), int16(-32768), uint16(65535), int32(-2147483648), uint32(4294967295),
		float32(0.1), models.MustFloat16(1.5),
	}
}

// numberValue returns a decoded number as a float64, to compare values
// whatever kind they came back as.
func numberValue(t *testing.T, v any) float64 {
	t.Helper()
	if h, ok := v.(models.Float16); ok {
		return h.Float64()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	}
	t.Fatalf("not a number: %#v", v)
	return 0
}

// TestCBORSizedNumbers_plainCBOR sends Go's sized kinds over plain CBOR, as
// the SDK does with servers before SurrealDB 4.0: they go widened, as plain
// integers and floats, and come back as such.
func TestCBORSizedNumbers_plainCBOR(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("cbor2", "plain", "t")
	defer db.Close(ctx)
	if db.CBORVersion() == connection.CBORv2 {
		t.Skip("the connection speaks cbor.2")
	}

	in := sizedValues()
	res, err := surrealdb.Query[[]any](ctx, db, "RETURN $v", map[string]any{"v": in})
	require.NoError(t, err)
	got := (*res)[0].Result
	require.Len(t, got, len(in))
	for i := range in {
		assert.Equal(t, numberValue(t, in[i]), numberValue(t, got[i]), "value %d (%T)", i, in[i])
		switch got[i].(type) {
		case uint64, int64, float32, float64:
		default:
			t.Errorf("value %d came back as %T, want a plain number", i, got[i])
		}
	}
}

// TestCBORSizedNumbers_cbor2 round-trips SurrealDB 4.0's sized numbers over
// cbor.2. It skips unless the server negotiated cbor.2.
func TestCBORSizedNumbers_cbor2(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("cbor2", "sized", "t")
	defer db.Close(ctx)
	if _, err := surrealdb.Query[any](ctx, db, "RETURN 1", nil); err != nil {
		t.Fatal(err)
	}
	if db.CBORVersion() != connection.CBORv2 {
		t.Skip("the server did not negotiate cbor.2 (SurrealDB 4.0 does)")
	}

	t.Run("from SurrealQL literals", func(t *testing.T) {
		res, err := surrealdb.Query[[]any](ctx, db,
			"RETURN [-128i8, 255u8, -32768i16, 65535u16, -2147483648i32, 4294967295u32, 18446744073709551615u64, 1.5f16, 0.1f32, 5, 1.5f]", nil)
		require.NoError(t, err)
		assert.Equal(t, []any{
			int8(-128), uint8(255), int16(-32768), uint16(65535), int32(-2147483648), uint32(4294967295),
			uint64(math.MaxUint64), models.MustFloat16(1.5), float32(0.1),
			uint64(5), 1.5,
		}, (*res)[0].Result)
	})

	t.Run("through a parameter", func(t *testing.T) {
		in := append(sizedValues(), uint64(math.MaxUint64))
		res, err := surrealdb.Query[[]any](ctx, db, "RETURN $v", map[string]any{"v": in})
		require.NoError(t, err)
		assert.Equal(t, in, (*res)[0].Result)
	})

	t.Run("stored in a record", func(t *testing.T) {
		type sized struct {
			ID  *models.RecordID `json:"id,omitempty"`
			I8  int8             `json:"i8"`
			U32 uint32           `json:"u32"`
			F32 float32          `json:"f32"`
			F16 models.Float16   `json:"f16"`
			U64 uint64           `json:"u64"`
		}
		in := sized{I8: -1, U32: 7, F32: 0.25, F16: models.MustFloat16(-2), U64: math.MaxUint64}
		created, err := surrealdb.Create[sized](ctx, db, models.Table("t"), in)
		require.NoError(t, err)
		require.NotNil(t, created.ID)
		got, err := surrealdb.Select[sized](ctx, db, *created.ID)
		require.NoError(t, err)
		in.ID = created.ID
		assert.Equal(t, in, *got)

		raw, err := surrealdb.Query[[]map[string]any](ctx, db, "SELECT i8, u32, f32, f16, u64 FROM t", nil)
		require.NoError(t, err)
		require.Len(t, (*raw)[0].Result, 1)
		assert.Equal(t, map[string]any{
			"i8": int8(-1), "u32": uint32(7), "f32": float32(0.25), "f16": models.MustFloat16(-2), "u64": uint64(math.MaxUint64),
		}, (*raw)[0].Result[0])
	})
}
