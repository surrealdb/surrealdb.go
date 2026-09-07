package surrealql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRangeOpen_Helpers(t *testing.T) {
	assert.Equal(t, "..", RangeOpen().String())
	assert.Nil(t, RangeOpen().Begin)
	assert.Nil(t, RangeOpen().End)

	assert.Equal(t, "1..", RangeOpenBeginInclusive(1).String())
	assert.Equal(t, "1>..", RangeOpenBeginExclusive(1).String())
	assert.Equal(t, "..=10", RangeOpenEndInclusive(10).String())
	assert.Equal(t, "..10", RangeOpenEndExclusive(10).String())
}

func TestRangeBeginEnd_Helpers(t *testing.T) {
	assert.Equal(t, "1..=10", RangeBeginInclusiveEndInclusive(1, 10).String())
	assert.Equal(t, "a..z", RangeBeginInclusiveEndExclusive("a", "z").String())
	assert.Equal(t, "1>..=10", RangeBeginExclusiveEndInclusive(1, 10).String())
	assert.Equal(t, "a>..z", RangeBeginExclusiveEndExclusive("a", "z").String())
}

func TestRangeOpen_TypedBounds(t *testing.T) {
	beginInc := RangeOpenBeginInclusive(1)
	require.NotNil(t, beginInc.Begin)
	assert.Equal(t, 1, beginInc.Begin.Value)
	assert.Nil(t, beginInc.End)

	endExc := RangeOpenEndExclusive("z")
	require.NotNil(t, endExc.End)
	assert.Equal(t, "z", endExc.End.Value)
	assert.Nil(t, endExc.Begin)
}

func TestRangeBeginInclusiveEndInclusive_MixedEnds(t *testing.T) {
	r := RangeBeginInclusiveEndInclusive(1, "z")
	require.NotNil(t, r.Begin)
	assert.Equal(t, 1, r.Begin.Value)
	require.NotNil(t, r.End)
	assert.Equal(t, "z", r.End.Value)
}
