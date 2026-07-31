package models

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for selecting a range of records (for example person:1..=1000).

func TestRecordRange_String(t *testing.T) {
	rr := NewRecordRange("person", Included(1), Included(1000))
	assert.Equal(t, "person:1..=1000", rr.String())

	rr2 := NewRecordRange("users", Included("a"), Excluded("z"))
	assert.Equal(t, "users:a..z", rr2.String())

	rr3 := NewRecordRange("logs", nil, Excluded("z"))
	assert.Equal(t, "logs:..z", rr3.String())

	rr4 := NewRecordRange("logs", Included("a"), nil)
	assert.Equal(t, "logs:a..", rr4.String())
}

func TestRecordRange_MarshalCBOR_Shape(t *testing.T) {
	rr := NewRecordRange("person", Included(1), Included(1000))

	encoded, err := cbor.Marshal(rr)
	require.NoError(t, err)

	var tag cbor.Tag
	require.NoError(t, cbor.Unmarshal(encoded, &tag))
	assert.Equal(t, TagRecordID, tag.Number)

	content, ok := tag.Content.([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	assert.Equal(t, "person", content[0])

	rangeTag, ok := content[1].(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagRange, rangeTag.Number)

	bounds, ok := rangeTag.Content.([]any)
	require.True(t, ok)
	require.Len(t, bounds, 2)

	beginTag, ok := bounds[0].(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagBoundIncluded, beginTag.Number)
	assert.EqualValues(t, 1, beginTag.Content)

	endTag, ok := bounds[1].(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagBoundIncluded, endTag.Number)
	assert.EqualValues(t, 1000, endTag.Content)
}

func TestRecordRange_MarshalCBOR_MatchesRecordIDWithRangeID(t *testing.T) {
	// A RecordID whose id is a Range should behave like RecordRange.
	viaRecordRange := NewRecordRange("person", Included(1), Excluded(10))
	viaRecordID := RecordID{
		Table: "person",
		ID: Range[int, BoundIncluded[int], BoundExcluded[int]]{
			Begin: Included(1),
			End:   Excluded(10),
		},
	}

	a, err := cbor.Marshal(viaRecordRange)
	require.NoError(t, err)
	b, err := cbor.Marshal(viaRecordID)
	require.NoError(t, err)
	assert.Equal(t, a, b, "RecordRange and RecordID{ID: Range} must encode the same way")
}

func TestRecordRange_ArrayStyleID(t *testing.T) {
	// Equivalent to: SELECT * FROM temp:['London',NONE]..=['London',..]
	rr := NewRecordRange("temp",
		IncludedMany[any]("London", None),
		IncludedMany[any]("London", OpenRange()),
	)

	assert.Contains(t, rr.String(), "temp:")

	encoded, err := cbor.Marshal(rr)
	require.NoError(t, err)

	var tag cbor.Tag
	require.NoError(t, cbor.Unmarshal(encoded, &tag))
	assert.Equal(t, TagRecordID, tag.Number)

	content := tag.Content.([]any)
	rangeTag := content[1].(cbor.Tag)
	assert.Equal(t, TagRange, rangeTag.Number)

	bounds := rangeTag.Content.([]any)
	begin := bounds[0].(cbor.Tag)
	assert.Equal(t, TagBoundIncluded, begin.Number)
	beginVal := begin.Content.([]any)
	require.Len(t, beginVal, 2)
	assert.Equal(t, "London", beginVal[0])
	noneTag, ok := beginVal[1].(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagNone, noneTag.Number)

	end := bounds[1].(cbor.Tag)
	assert.Equal(t, TagBoundIncluded, end.Number)
	endVal := end.Content.([]any)
	require.Len(t, endVal, 2)
	assert.Equal(t, "London", endVal[0])
	openTag, ok := endVal[1].(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagRange, openTag.Number)
}

func TestOpenRange_String(t *testing.T) {
	assert.Equal(t, "..", OpenRange().String())
}

func TestIncludedExcluded_Helpers(t *testing.T) {
	var typed *BoundIncluded[int] = Included(42)
	assert.Equal(t, 42, typed.Value)

	var excl *BoundExcluded[string] = Excluded("x")
	assert.Equal(t, "x", excl.Value)

	var manyInts *BoundIncluded[[]int] = IncludedMany(1, 2)
	assert.Equal(t, []int{1, 2}, manyInts.Value)

	var manyOne *BoundExcluded[[]int] = ExcludedMany(9)
	assert.Equal(t, []int{9}, manyOne.Value)

	var composite *BoundIncluded[[]any] = IncludedMany[any]("London", None)
	assert.Equal(t, []any{"London", None}, composite.Value)

	// ID is another way to build an array-style id for Included(...).
	var viaID *BoundIncluded[[]any] = Included(ID("London", None))
	assert.Equal(t, []any{"London", None}, viaID.Value)
}

func TestID_Helper(t *testing.T) {
	assert.Equal(t, []any{"London", None}, ID("London", None))
	assert.Equal(t, []any{"a"}, ID("a"))
}
