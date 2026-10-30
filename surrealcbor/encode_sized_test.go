package surrealcbor

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Struct shapes where fxamacker/cbor's field rules decide which field a name
// is encoded for.

type emptyCBORTag struct {
	A int64 `cbor:"" json:"B"` // an empty cbor tag falls back to json, so A is "B"
	B int8
}

type fieldsInner struct{ X int8 }
type fieldsMid1 struct{ fieldsInner }
type fieldsMid2 struct{ fieldsInner }
type fieldsDeep struct{ X int64 }
type fieldsDeepWrap struct{ fieldsDeep }
type fieldsDeepWrap2 struct{ fieldsDeepWrap }

// sameTypeTwice embeds fieldsInner twice at depth 2, so fxamacker drops both
// and encodes fieldsDeep.X as "X".
type sameTypeTwice struct {
	fieldsMid1
	fieldsMid2
	fieldsDeepWrap2
}

type optionsOnly struct {
	X int8 `json:",omitempty"` // tagged, though named after the field
}
type untaggedX struct{ X int64 }

// tagWithoutName embeds two structs with an X at the same depth: the one
// with a tag wins, though the tag gives no name.
type tagWithoutName struct {
	optionsOnly
	untaggedX
}

type keyAsInt struct {
	A int8  `cbor:"1,keyasint"` // encoded under the integer 1
	B int64 `cbor:"1"`          // encoded under the text "1"
}

type unexportedInner struct{ Y int8 }

type promotedFromUnexported struct {
	unexportedInner
	Z int16
}

type shallowerWins struct {
	X int64
	fieldsInner
}

type embeddedWithOptions struct {
	fieldsInner `json:",omitempty"` // no name, so still promoted
	Y           int16
}

type pointerEmbedded struct {
	*fieldsInner
	Y int32
}

func TestEncodedFields_matchFxamacker(t *testing.T) {
	cases := []any{
		emptyCBORTag{A: 1, B: 2},
		sameTypeTwice{
			fieldsMid1:      fieldsMid1{fieldsInner{X: 1}},
			fieldsMid2:      fieldsMid2{fieldsInner{X: 2}},
			fieldsDeepWrap2: fieldsDeepWrap2{fieldsDeepWrap{fieldsDeep{X: 3}}},
		},
		tagWithoutName{optionsOnly{X: 1}, untaggedX{X: 2}},
		keyAsInt{A: 1, B: 2},
		promotedFromUnexported{unexportedInner{Y: 1}, 2},
		shallowerWins{X: 1, fieldsInner: fieldsInner{X: 2}},
		embeddedWithOptions{fieldsInner{X: 1}, 2},
		pointerEmbedded{&fieldsInner{X: 1}, 2},
	}
	for _, v := range cases {
		t.Run(reflect.TypeOf(v).Name(), func(t *testing.T) {
			// Every field holds a different number, so the number fxamacker
			// encodes under a name tells which field it used.
			data, err := cbor.Marshal(v)
			require.NoError(t, err)
			var encoded map[any]any
			require.NoError(t, cbor.Unmarshal(data, &encoded))

			fields := encodedFields(reflect.TypeOf(v))
			for key, want := range encoded {
				name, ok := key.(string)
				if !ok {
					continue // keyasint: the walk only looks up text keys
				}
				index, ok := fields[name]
				require.True(t, ok, "fxamacker encodes %q, the walk does not know it", name)
				field, err := reflect.ValueOf(v).FieldByIndexErr(index)
				require.NoError(t, err)
				assert.EqualValues(t, want, field.Int(), "field for %q", name)
			}
			for name := range fields {
				assert.Contains(t, encoded, name, "the walk knows %q, fxamacker does not encode it", name)
			}
		})
	}
}

func TestMarshalV2_fieldRules(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want map[string]any
	}{
		// A is an int64, so stays untagged, though B is an int8 with the same value.
		{"empty cbor tag", emptyCBORTag{A: 5, B: 5}, map[string]any{"B": uint64(5)}},
		{"same type embedded twice", sameTypeTwice{
			fieldsMid1:      fieldsMid1{fieldsInner{X: 5}},
			fieldsDeepWrap2: fieldsDeepWrap2{fieldsDeepWrap{fieldsDeep{X: 5}}},
		}, map[string]any{"X": uint64(5)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewV2().Marshal(tc.v)
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, NewV2().Unmarshal(b, &got))
			assert.Equal(t, tc.want, got)
		})
	}
}

// prefixedKey encodes itself as its text with a "p_" prefix.
type prefixedKey string

func (k prefixedKey) MarshalCBOR() ([]byte, error) { return cbor.Marshal("p_" + string(k)) }

func TestMarshalV2_selfEncodingMapKeys(t *testing.T) {
	// The key "a" is encoded as "p_a", which is another entry's key: the walk
	// must not tag the int64 under "a" as that entry's int8.
	v := map[prefixedKey]any{"a": int64(5), "p_a": int8(5)}
	b, err := NewV2().Marshal(v)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, NewV2().Unmarshal(b, &got))
	assert.Equal(t, map[string]any{"p_a": uint64(5), "p_p_a": uint64(5)}, got)
}

type sizedNode struct {
	V    int8       `json:"v"`
	Next *sizedNode `json:"next,omitempty"`
}

type plainNode struct {
	S    string     `json:"s"`
	Next *plainNode `json:"next,omitempty"`
}

type metaHolder struct {
	M map[string]any `json:"m"`
}

// hiddenMeta reaches its map through an unexported embedded struct, so the
// map cannot be read without reflection.
type hiddenMeta struct {
	metaHolder
}

func TestMarshalV2_walkPlans(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want any
	}{
		{"recursive type", &sizedNode{V: 1, Next: &sizedNode{V: 2}},
			map[string]any{"v": int8(1), "next": map[string]any{"v": int8(2)}}},
		{"recursive type with nothing to tag", plainNode{S: "a", Next: &plainNode{S: "b"}},
			map[string]any{"s": "a", "next": map[string]any{"s": "b"}}},
		{"map reached through an unexported field", hiddenMeta{metaHolder{M: map[string]any{"a": int16(3), "b": "x"}}},
			map[string]any{"m": map[string]any{"a": int16(3), "b": "x"}}},
		{"map of a type that cannot hold a sized kind", map[string][]string{"a": {"b"}},
			map[string]any{"a": []any{"b"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewV2().Marshal(tc.v)
			require.NoError(t, err)
			var got any
			require.NoError(t, NewV2().Unmarshal(b, &got))
			assert.Equal(t, tc.want, got)
		})
	}
	assert.Equal(t, planCopy, planFor(reflect.TypeOf(plainNode{})))
	assert.Equal(t, planCopy, planFor(reflect.TypeOf(map[string][]string{})))
	assert.Equal(t, planWalk, planFor(reflect.TypeOf(sizedNode{})))
	assert.Equal(t, planWalk, planFor(reflect.TypeOf(hiddenMeta{})))
}

type benchRow struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Email string            `json:"email"`
	Age   int64             `json:"age"`
	Score float64           `json:"score"`
	Tags  map[string]string `json:"tags"`
	Meta  map[string]any    `json:"meta"`
}

type benchSizedRow struct {
	benchRow
	Small int8    `json:"small"`
	Ratio float32 `json:"ratio"`
}

type benchRequest struct {
	ID     any    `json:"id"`
	Method string `json:"method,omitempty"`
	Params []any  `json:"params,omitempty"`
}

func benchRows() []benchRow {
	rows := make([]benchRow, 200)
	for i := range rows {
		rows[i] = benchRow{
			ID:    fmt.Sprintf("person:%d", i),
			Name:  fmt.Sprintf("Person %d", i),
			Email: fmt.Sprintf("person%d@example.com", i),
			Age:   int64(20 + i%50),
			Score: float64(i) * 1.25,
			Tags:  map[string]string{"team": "a", "role": "dev"},
			Meta:  map[string]any{"active": true, "visits": int64(i), "rank": 0.5, "note": "x"},
		}
	}
	return rows
}

// BenchmarkMarshalQuery encodes a query request with 200 rows in plain CBOR
// and in cbor.2. With no sized numbers in it, cbor.2 tags nothing.
func BenchmarkMarshalQuery(b *testing.B) {
	rows := benchRows()
	sized := make([]benchSizedRow, len(rows))
	for i, r := range rows {
		sized[i] = benchSizedRow{benchRow: r, Small: int8(i % 100), Ratio: 0.5}
	}
	payloads := []struct {
		name string
		v    any
	}{
		{"no sized numbers", benchRequest{ID: "1", Method: "query", Params: []any{"INSERT INTO person $rows", map[string]any{"rows": rows}}}},
		{"sized numbers", benchRequest{ID: "1", Method: "query", Params: []any{"INSERT INTO person $rows", map[string]any{"rows": sized}}}},
	}
	for _, p := range payloads {
		for _, c := range []struct {
			name  string
			codec *Codec
		}{{"v1", New()}, {"v2", NewV2()}} {
			b.Run(p.name+"/"+c.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := c.codec.Marshal(p.v); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// mutualA holds a sized kind; mutualB holds one only through mutualA.
type mutualA struct {
	B *mutualB
	X int8
}

type mutualB struct {
	A *mutualA
}

func TestPlanFor_mutuallyRecursiveTypes(t *testing.T) {
	// Working out mutualA first works out mutualB while mutualA is unknown.
	assert.Equal(t, planWalk, planFor(reflect.TypeOf(mutualA{})))
	assert.Equal(t, planWalk, planFor(reflect.TypeOf(mutualB{})))

	b, err := NewV2().Marshal(mutualB{A: &mutualA{X: 1}})
	require.NoError(t, err)
	var got any
	require.NoError(t, NewV2().Unmarshal(b, &got))
	assert.Equal(t, map[string]any{"A": map[string]any{"B": nil, "X": int8(1)}}, got)
}
