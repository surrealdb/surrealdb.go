package models

import (
	"fmt"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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
