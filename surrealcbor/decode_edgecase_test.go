package surrealcbor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecode_edgeCases tests edge cases and boundary conditions
func TestDecode_edgeCases(t *testing.T) {
	t.Run("maximum values", func(t *testing.T) {
		type MaxValues struct {
			MaxInt64  int64  `json:"max_int64"`
			MinInt64  int64  `json:"min_int64"`
			MaxUint64 uint64 `json:"max_uint64"`
		}

		original := MaxValues{
			MaxInt64:  9223372036854775807,
			MinInt64:  -9223372036854775808,
			MaxUint64: 18446744073709551615,
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded MaxValues
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
	})

	t.Run("deeply nested structure", func(t *testing.T) {
		type Level3 struct {
			Value string `json:"value"`
		}
		type Level2 struct {
			L3 *Level3 `json:"l3"`
		}
		type Level1 struct {
			L2 *Level2 `json:"l2"`
		}

		original := Level1{
			L2: &Level2{
				L3: &Level3{
					Value: "deep",
				},
			},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded Level1
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
	})

	t.Run("complex nested maps and slices", func(t *testing.T) {
		type Complex struct {
			MapOfSlices map[string][]int          `json:"map_of_slices"`
			SliceOfMaps []map[string]string       `json:"slice_of_maps"`
			MapOfMaps   map[string]map[string]int `json:"map_of_maps"`
		}

		original := Complex{
			MapOfSlices: map[string][]int{
				"nums": {1, 2, 3},
				"more": {4, 5, 6},
			},
			SliceOfMaps: []map[string]string{
				{"a": "1", "b": "2"},
				{"c": "3", "d": "4"},
			},
			MapOfMaps: map[string]map[string]int{
				"outer1": {"inner1": 1, "inner2": 2},
				"outer2": {"inner3": 3, "inner4": 4},
			},
		}

		data, err := Marshal(original)
		require.NoError(t, err)

		var decoded Complex
		err = Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original, decoded)
	})
}
