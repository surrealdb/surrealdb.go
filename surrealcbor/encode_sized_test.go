package surrealcbor

import (
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
