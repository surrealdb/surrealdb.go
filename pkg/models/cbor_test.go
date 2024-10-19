package models

import (
	"fmt"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

func TestForGeometryPoint(t *testing.T) {
	em := getCborEncoder()
	dm := getCborDecoder()

	gp := NewGeometryPoint(12.23, 45.65)
	encoded, err := em.Marshal(gp)
	assert.Nil(t, err, "Should not encounter an error while encoding")

	decoded := GeometryPoint{}
	err = dm.Unmarshal(encoded, &decoded)

	assert.Nil(t, err, "Should not encounter an error while decoding")
	assert.Equal(t, gp, decoded)
}

func TestForGeometryLine(t *testing.T) {
	em := getCborEncoder()
	dm := getCborDecoder()

	gp1 := NewGeometryPoint(12.23, 45.65)
	gp2 := NewGeometryPoint(23.34, 56.75)
	gp3 := NewGeometryPoint(33.45, 86.99)

	gl := GeometryLine{gp1, gp2, gp3}

	encoded, err := em.Marshal(gl)
	assert.Nil(t, err, "Should not encounter an error while encoding")

	decoded := GeometryLine{}
	err = dm.Unmarshal(encoded, &decoded)
	assert.Nil(t, err, "Should not encounter an error while decoding")
	assert.Equal(t, gl, decoded)
}

func TestForGeometryPolygon(t *testing.T) {
	em := getCborEncoder()
	dm := getCborDecoder()

	gl1 := GeometryLine{NewGeometryPoint(12.23, 45.65), NewGeometryPoint(23.33, 44.44)}
	gl2 := GeometryLine{GeometryPoint{12.23, 45.65}, GeometryPoint{23.33, 44.44}}
	gl3 := GeometryLine{NewGeometryPoint(12.23, 45.65), NewGeometryPoint(23.33, 44.44)}
	gp := GeometryPolygon{gl1, gl2, gl3}

	encoded, err := em.Marshal(gp)
	assert.Nil(t, err, "Should not encounter an error while encoding")

	decoded := GeometryPolygon{}
	err = dm.Unmarshal(encoded, &decoded)

	assert.Nil(t, err, "Should not encounter an error while decoding")
	assert.Equal(t, gp, decoded)
}

func TestForRequestPayload(t *testing.T) {
	em := getCborEncoder()

	params := []interface{}{
		"SELECT marketing, count() FROM $tb GROUP BY marketing",
		map[string]interface{}{
			"tb":       Table("person"),
			"line":     GeometryLine{NewGeometryPoint(11.11, 22.22), NewGeometryPoint(33.33, 44.44)},
			"datetime": time.Now(),
			"testNone": None,
			"testNil":  nil,
			"duration": time.Duration(340),
			// "custom_duration": CustomDuration(340),
			"custom_datetime": CustomDateTime(time.Now()),
		},
	}

	requestPayload := map[string]interface{}{
		"id":     "2",
		"method": "query",
		"params": params,
	}

	encoded, err := em.Marshal(requestPayload)

	assert.Nil(t, err, "should not return an error while encoding payload")

	diagStr, err := cbor.Diagnose(encoded)
	assert.Nil(t, err, "should not return an error while diagnosing payload")

	fmt.Println(diagStr)
}

func TestRange_GetJoinString(t *testing.T) {
	t.Run("begin excluded, end excluded", func(s *testing.T) {
		r := &Range[int, BoundExcluded[int], BoundExcluded[int]]{
			Begin: &BoundExcluded[int]{0},
			End:   &BoundExcluded[int]{10},
		}
		assert.Equal(t, ">..", r.GetJoinString())
	})

	t.Run("begin excluded, end included", func(t *testing.T) {
		r := Range[int, BoundExcluded[int], BoundIncluded[int]]{
			Begin: &BoundExcluded[int]{0},
			End:   &BoundIncluded[int]{10},
		}
		assert.Equal(t, ">..=", r.GetJoinString())
	})

	t.Run("begin included, end excluded", func(t *testing.T) {
		r := Range[int, BoundIncluded[int], BoundExcluded[int]]{
			Begin: &BoundIncluded[int]{0},
			End:   &BoundExcluded[int]{10},
		}
		assert.Equal(t, "..", r.GetJoinString())
	})

	t.Run("begin included, end included", func(t *testing.T) {
		r := Range[int, BoundIncluded[int], BoundIncluded[int]]{
			Begin: &BoundIncluded[int]{0},
			End:   &BoundIncluded[int]{10},
		}
		assert.Equal(t, "..=", r.GetJoinString())
	})
}

func TestCustomDateTime_String(t *testing.T) {
	tm, _ := time.Parse("2006-01-02T15:04:05Z", "2022-01-21T05:53:19Z")
	ct := CustomDateTime(tm)

	assert.Equal(t, "<datetime> '2022-01-21T05:53:19Z'", ct.String())
}

func TestRange_Bounds(t *testing.T) {
	em := getCborEncoder()
	dm := getCborDecoder()

	t.Run("bound included should be marshaled and unmarshaled properly", func(t *testing.T) {
		bi := BoundIncluded[int]{10}
		encoded, err := em.Marshal(bi)
		assert.NoError(t, err)

		var decoded BoundIncluded[int]
		err = dm.Unmarshal(encoded, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, bi, decoded)
	})

	t.Run("bound excluded should be marshaled and unmarshaled properly", func(t *testing.T) {
		be := BoundExcluded[int]{10}
		encoded, err := em.Marshal(be)
		assert.NoError(t, err)

		var decoded BoundExcluded[int]
		err = dm.Unmarshal(encoded, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, be, decoded)
	})
}

func TestRange_CODEC(t *testing.T) {
	em := getCborEncoder()
	dm := getCborDecoder()

	r := Range[int, BoundIncluded[int], BoundExcluded[int]]{
		Begin: &BoundIncluded[int]{0},
		End:   &BoundExcluded[int]{10},
	}

	encoded, err := em.Marshal(r)
	assert.NoError(t, err)

	var decoded Range[int, BoundIncluded[int], BoundExcluded[int]]
	err = dm.Unmarshal(encoded, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, r, decoded)
}
