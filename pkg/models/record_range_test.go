package models

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for selecting a range of records via RecordID + Range.

func personRange(id any) RecordID {
	return RecordID{Table: "person", ID: id}
}

func TestRecordID_Range_String(t *testing.T) {
	// String() on Range is for debugging; do not treat it as SurrealQL.
	assert.Equal(t, "1..=1000", (Range[any, BoundIncluded[any], BoundIncluded[any]]{
		Begin: &BoundIncluded[any]{Value: 1},
		End:   &BoundIncluded[any]{Value: 1000},
	}).String())
	assert.Equal(t, "a..z", (Range[any, BoundIncluded[any], BoundExcluded[any]]{
		Begin: &BoundIncluded[any]{Value: "a"},
		End:   &BoundExcluded[any]{Value: "z"},
	}).String())
	assert.Equal(t, "..z", (Range[any, BoundIncluded[any], BoundExcluded[any]]{
		End: &BoundExcluded[any]{Value: "z"},
	}).String())
	assert.Equal(t, "a..", (Range[any, BoundIncluded[any], BoundIncluded[any]]{
		Begin: &BoundIncluded[any]{Value: "a"},
	}).String())
}

func TestRecordID_Range_MarshalCBOR_Shape(t *testing.T) {
	rr := personRange(Range[any, BoundIncluded[any], BoundIncluded[any]]{
		Begin: &BoundIncluded[any]{Value: 1},
		End:   &BoundIncluded[any]{Value: 1000},
	})

	encoded, err := cbor.Marshal(&rr)
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

func TestRecordID_Range_MarshalCBOR_MatchesTypedRange(t *testing.T) {
	viaAny := personRange(Range[any, BoundIncluded[any], BoundExcluded[any]]{
		Begin: &BoundIncluded[any]{Value: 1},
		End:   &BoundExcluded[any]{Value: 10},
	})
	viaTypedRange := RecordID{
		Table: "person",
		ID: Range[int, BoundIncluded[int], BoundExcluded[int]]{
			Begin: &BoundIncluded[int]{Value: 1},
			End:   &BoundExcluded[int]{Value: 10},
		},
	}

	a, err := cbor.Marshal(&viaAny)
	require.NoError(t, err)
	b, err := cbor.Marshal(&viaTypedRange)
	require.NoError(t, err)
	assert.Equal(t, a, b, "any and typed Range must encode the same way")
}

func TestRecordID_Range_ArrayStyleID(t *testing.T) {
	rr := RecordID{
		Table: "temp",
		ID: Range[any, BoundIncluded[any], BoundIncluded[any]]{
			Begin: &BoundIncluded[any]{Value: []any{"London", None}},
			End:   &BoundIncluded[any]{Value: []any{"London", Range[any, BoundIncluded[any], BoundIncluded[any]]{}}},
		},
	}

	encoded, err := cbor.Marshal(&rr)
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

func TestRange_String_OpenSides(t *testing.T) {
	assert.Equal(t, "..", (Range[any, BoundIncluded[any], BoundIncluded[any]]{}).String())
	assert.Equal(t, "1..", (Range[int, BoundIncluded[int], BoundIncluded[int]]{
		Begin: &BoundIncluded[int]{Value: 1},
	}).String())
	assert.Equal(t, "1>..", (Range[int, BoundExcluded[int], BoundIncluded[int]]{
		Begin: &BoundExcluded[int]{Value: 1},
	}).String())
	assert.Equal(t, "..=10", (Range[int, BoundIncluded[int], BoundIncluded[int]]{
		End: &BoundIncluded[int]{Value: 10},
	}).String())
	assert.Equal(t, "..10", (Range[int, BoundIncluded[int], BoundExcluded[int]]{
		End: &BoundExcluded[int]{Value: 10},
	}).String())

	empty := Range[any, BoundIncluded[any], BoundIncluded[any]]{}
	assert.Nil(t, empty.Begin)
	assert.Nil(t, empty.End)

	viaEmpty, err := cbor.Marshal(empty)
	require.NoError(t, err)
	assert.NotEmpty(t, viaEmpty)
}

func TestRecordRangeID_MarshalCBOR_UsesRange(t *testing.T) {
	rr := RecordRangeID[int, BoundIncluded[int], BoundIncluded[int]]{
		Range: Range[int, BoundIncluded[int], BoundIncluded[int]]{
			Begin: &BoundIncluded[int]{Value: 1},
			End:   &BoundIncluded[int]{Value: 10},
		},
		Table: Table("person"),
	}
	viaRecordID := personRange(Range[int, BoundIncluded[int], BoundIncluded[int]]{
		Begin: &BoundIncluded[int]{Value: 1},
		End:   &BoundIncluded[int]{Value: 10},
	})

	a, err := cbor.Marshal(rr)
	require.NoError(t, err)
	b, err := cbor.Marshal(&viaRecordID)
	require.NoError(t, err)
	assert.Equal(t, a, b)

	var tag cbor.Tag
	require.NoError(t, cbor.Unmarshal(a, &tag))
	assert.Equal(t, TagRecordID, tag.Number)
	content := tag.Content.([]any)
	rangeTag := content[1].(cbor.Tag)
	assert.Equal(t, TagRange, rangeTag.Number)
}

func TestRecordID_Range_MarshalCBOR_OpenNoneNullNested(t *testing.T) {
	t.Run("open end is null slot", func(t *testing.T) {
		rr := personRange(Range[any, BoundIncluded[any], BoundIncluded[any]]{
			Begin: &BoundIncluded[any]{Value: 1},
		})
		bounds := encodeRecordIDRangeBounds(t, rr)
		assertBoundIncluded(t, bounds[0], int64(1))
		assert.Nil(t, bounds[1], "open end must be a null slot")
	})

	t.Run("open begin is null slot", func(t *testing.T) {
		rr := personRange(Range[any, BoundIncluded[any], BoundIncluded[any]]{
			End: &BoundIncluded[any]{Value: 10},
		})
		bounds := encodeRecordIDRangeBounds(t, rr)
		assert.Nil(t, bounds[0], "open begin must be a null slot")
		assertBoundIncluded(t, bounds[1], int64(10))
	})

	t.Run("Begin=None is TagBoundIncluded + TagNone", func(t *testing.T) {
		rr := personRange(Range[any, BoundIncluded[any], BoundIncluded[any]]{
			Begin: &BoundIncluded[any]{Value: None},
			End:   &BoundIncluded[any]{Value: 10},
		})
		bounds := encodeRecordIDRangeBounds(t, rr)
		begin := assertBoundIncludedTag(t, bounds[0])
		noneTag, ok := begin.(cbor.Tag)
		require.True(t, ok)
		assert.Equal(t, TagNone, noneTag.Number)
	})

	t.Run("End=nil is TagBoundIncluded + null value", func(t *testing.T) {
		rr := personRange(Range[any, BoundIncluded[any], BoundIncluded[any]]{
			Begin: &BoundIncluded[any]{Value: 1},
			End:   &BoundIncluded[any]{Value: nil},
		})
		bounds := encodeRecordIDRangeBounds(t, rr)
		end := assertBoundIncludedTag(t, bounds[1])
		assert.Nil(t, end, "NULL bound value must be CBOR null inside the included tag")
	})

	t.Run("End=open Range nests TagRange with both slots null", func(t *testing.T) {
		rr := personRange(Range[any, BoundIncluded[any], BoundIncluded[any]]{
			Begin: &BoundIncluded[any]{Value: 1},
			End:   &BoundIncluded[any]{Value: Range[any, BoundIncluded[any], BoundIncluded[any]]{}},
		})
		bounds := encodeRecordIDRangeBounds(t, rr)
		end := assertBoundIncludedTag(t, bounds[1])
		openTag, ok := end.(cbor.Tag)
		require.True(t, ok)
		assert.Equal(t, TagRange, openTag.Number)
		slots, ok := openTag.Content.([]any)
		require.True(t, ok)
		require.Len(t, slots, 2)
		assert.Nil(t, slots[0])
		assert.Nil(t, slots[1])
	})
}

func encodeRecordIDRangeBounds(t *testing.T, rr RecordID) []any {
	t.Helper()
	encoded, err := cbor.Marshal(&rr)
	require.NoError(t, err)

	var tag cbor.Tag
	require.NoError(t, cbor.Unmarshal(encoded, &tag))
	require.Equal(t, TagRecordID, tag.Number)

	content, ok := tag.Content.([]any)
	require.True(t, ok)
	require.Len(t, content, 2)

	rangeTag, ok := content[1].(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagRange, rangeTag.Number)

	bounds, ok := rangeTag.Content.([]any)
	require.True(t, ok)
	require.Len(t, bounds, 2)
	return bounds
}

func assertBoundIncluded(t *testing.T, v, want any) {
	t.Helper()
	content := assertBoundIncludedTag(t, v)
	assert.EqualValues(t, want, content)
}

func assertBoundIncludedTag(t *testing.T, v any) any {
	t.Helper()
	tag, ok := v.(cbor.Tag)
	require.True(t, ok)
	assert.Equal(t, TagBoundIncluded, tag.Number)
	return tag.Content
}
