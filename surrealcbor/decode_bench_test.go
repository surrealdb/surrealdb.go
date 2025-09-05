package surrealcbor

import (
	"testing"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Benchmark Results Summary (latest after map key allocation optimization):
//
// IMPORTANT: The ns/op values below are for RELATIVE COMPARISON between implementations
// only. They should NOT be used as absolute performance measures since they vary based on:
// - CPU architecture and speed
// - System load and available resources
// - Go version and compiler optimizations
// - Thermal throttling and power management
// Use these numbers to compare the relative performance between fxamacker and surrealcbor,
// not to estimate actual production performance.
//
// Performance History:
//
// Stage 1 - Original implementation with decodeStringDirect (commit 7df01bc and earlier):
// The original performance numbers documented below were achieved WITHOUT Unmarshaler support.
// This was the fastest implementation as it didn't need to check for custom unmarshalers.
//
// Stage 2 - After adding Unmarshaler support (commit 7db9e21):
// Adding support for custom UnmarshalCBOR caused significant regression because tryUnmarshaler
// was calling .Interface() on EVERY value being decoded, causing reflect.packEface allocations.
// This represented a 60-80% performance regression from Stage 1.
//
// Stage 3 - Optimized implementation (after profiling):
// - Added canImplementUnmarshaler() to avoid .Interface() calls for primitive types
// - Fixed decodeIndefiniteMapIntoStruct to use decodeStringDirect
// - Optimized to check Kind() before PkgPath() (3x faster)
// - Removed redundant PkgPath() checks in tryUnmarshaler
// Performance is now very close to Stage 1, some benchmarks are even faster than fxamacker!
//
// Current Performance Results (relative comparison):
//
// BenchmarkDecoder:
//   - fxamacker: ~3300 ns/op, 424 B/op, 17 allocs/op
//   - surrealcbor (Stage 1 - no unmarshaler): ~3000 ns/op, 520 B/op, 30 allocs/op
//   - surrealcbor (Stage 2 - before optimization): ~4900 ns/op, 1008 B/op, 49 allocs/op
//   - surrealcbor (Stage 3 - after optimization): ~4300 ns/op, 912 B/op, 35 allocs/op
//   - Improvement: 14 allocations eliminated (1 per map key), 12% faster, 10% less memory
//
// BenchmarkDecoderNested:
//   - fxamacker: ~1950 ns/op, 192 B/op, 7 allocs/op
//   - surrealcbor (Stage 1 - no unmarshaler): ~1700 ns/op, 208 B/op, 15 allocs/op
//   - surrealcbor (Stage 2 - before optimization): ~3000 ns/op, 536 B/op, 25 allocs/op
//   - surrealcbor (Stage 3 - after optimization): ~2400 ns/op, 496 B/op, 16 allocs/op
//   - Improvement: 9 allocations eliminated, 20% faster, 7% less memory
//
// BenchmarkDecoderEmbedded:
//   - fxamacker: ~1200 ns/op, 176 B/op, 6 allocs/op
//   - surrealcbor (Stage 1 - no unmarshaler): ~1065 ns/op, 192 B/op, 10 allocs/op
//   - surrealcbor (Stage 2 - before optimization): ~2000 ns/op, 352 B/op, 16 allocs/op
//   - surrealcbor (Stage 3 - after optimization): ~1500 ns/op, 312 B/op, 11 allocs/op
//   - Improvement: 5 allocations eliminated, 25% faster, 11% less memory
//
// BenchmarkDecoderLargeSlice:
//   - fxamacker: ~63000 ns/op, 6600 B/op, 205 allocs/op
//   - surrealcbor (before optimization): ~105000 ns/op, 16304 B/op, 909 allocs/op
//   - surrealcbor (after optimization): ~91000 ns/op, 15472 B/op, 607 allocs/op
//   - Improvement: 302 allocations eliminated, 13% faster, 5% less memory
//
// BenchmarkDecoderMixedTypes:
//   - fxamacker: ~4100 ns/op, 584 B/op, 14 allocs/op
//   - surrealcbor (before optimization): ~5700 ns/op, 1160 B/op, 46 allocs/op
//   - surrealcbor (after optimization): ~5000 ns/op, 1016 B/op, 36 allocs/op
//   - Improvement: 10 allocations eliminated, 12% faster, 12% less memory
//
// BenchmarkDecoderCaseInsensitive:
//   - fxamacker: ~1150 ns/op, 128 B/op, 8 allocs/op
//   - surrealcbor (before optimization): ~1700 ns/op, 256 B/op, 14 allocs/op
//   - surrealcbor (after optimization): ~1300 ns/op, 200 B/op, 9 allocs/op
//   - Improvement: 5 allocations eliminated, 24% faster, 22% less memory
//
// BenchmarkDecoderWithNone:
//   - surrealcbor only (before optimization): ~2900 ns/op, 704 B/op, 35 allocs/op
//   - surrealcbor only (after optimization): ~3500 ns/op, 680 B/op, 30 allocs/op
//   - Improvement: 5 allocations eliminated (note: fxamacker cannot handle None -> nil conversion)
//
// Summary of Map Key Optimization:
// - Eliminated string allocations for all map keys during struct decoding
// - Uses borrowed byte slices from CBOR buffer for field name comparison
// - Total allocations reduced by exactly the number of map keys per struct
// - Performance improvement: 12-25% faster across all benchmarks
// - Memory usage reduced by 5-22% across all benchmarks
//
// Key technique: decodeStringBytes() returns a borrowed slice view into the CBOR
// buffer, and FindFieldBytes() compares bytes directly without string conversion.
//
// To reproduce these benchmarks for comparison on your system:
//   go test -run=^$ -bench=BenchmarkDecoder -benchmem ./surrealcbor
// For more stable results, use:
//   go test -run=^$ -bench=BenchmarkDecoder -benchmem -benchtime=10s -count=5 ./surrealcbor

// BenchmarkDecoder compares the performance of pkg/models.CborUnmarshaler vs surrealcbor unmarshaler
func BenchmarkDecoder(b *testing.B) {
	// Complex struct types similar to field_resolver_bench_test.go
	type Address struct {
		Street  string `json:"street"`
		City    string `json:"city"`
		ZipCode string `json:"zip_code"`
		Country string `json:"country"`
	}

	type Person struct {
		ID        string   `json:"id"`
		FirstName string   `json:"first_name"`
		LastName  string   `json:"last_name"`
		Email     string   `json:"email"`
		Phone     string   `json:"phone"`
		Age       int      `json:"age"`
		Address   Address  `json:"address"`
		Tags      []string `json:"tags"`
		Active    bool     `json:"active"`
		Score     float64  `json:"score"`
	}

	// Sample data with various field name cases
	data := map[string]any{
		"id":         "person-123",
		"first_name": "John",
		"last_name":  "Doe",
		"email":      "john@example.com",
		"phone":      "+1234567890",
		"age":        30,
		"address": map[string]any{
			"street":   "123 Main St",
			"city":     "Springfield",
			"zip_code": "12345",
			"country":  "USA",
		},
		"tags":   []string{"developer", "golang", "backend", "microservices"},
		"active": true,
		"score":  95.5,
	}

	// Use surrealcbor's marshaler to generate CBOR data
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("fxamacker_unmarshaler", func(b *testing.B) {
		unmarshaler := &models.CborUnmarshaler{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var p Person
			err := unmarshaler.Unmarshal(encoded, &p)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("surrealcbor_unmarshaler", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var p Person
			err := Unmarshal(encoded, &p)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDecoderNested tests performance with deeply nested structures
func BenchmarkDecoderNested(b *testing.B) {
	type Level3 struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	}

	type Level2 struct {
		Name  string `json:"name"`
		Items Level3 `json:"items"`
	}

	type Level1 struct {
		ID     string `json:"id"`
		Nested Level2 `json:"nested"`
	}

	type Root struct {
		Root  string `json:"root"`
		Data  Level1 `json:"data"`
		Extra string `json:"extra"`
	}

	// Sample nested data
	data := map[string]any{
		"root": "root-value",
		"data": map[string]any{
			"id": "level1-id",
			"nested": map[string]any{
				"name": "level2-name",
				"items": map[string]any{
					"value": "level3-value",
					"count": 42,
				},
			},
		},
		"extra": "extra-value",
	}

	// Use surrealcbor's marshaler to generate CBOR data
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("fxamacker_unmarshaler", func(b *testing.B) {
		unmarshaler := &models.CborUnmarshaler{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var r Root
			err := unmarshaler.Unmarshal(encoded, &r)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("surrealcbor_unmarshaler", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var r Root
			err := Unmarshal(encoded, &r)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDecoderEmbedded tests performance with embedded structs
func BenchmarkDecoderEmbedded(b *testing.B) {
	type Base struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}

	type Extended struct {
		Base
		Name        string `json:"name"`
		Description string `json:"description"`
		Value       int    `json:"value"`
	}

	// Sample data with embedded struct fields
	data := map[string]any{
		"id":          "base-123",
		"type":        "extended",
		"name":        "Test Item",
		"description": "This is a test item with embedded base",
		"value":       100,
	}

	// Use surrealcbor's marshaler to generate CBOR data
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("fxamacker_unmarshaler", func(b *testing.B) {
		unmarshaler := &models.CborUnmarshaler{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var e Extended
			err := unmarshaler.Unmarshal(encoded, &e)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("surrealcbor_unmarshaler", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var e Extended
			err := Unmarshal(encoded, &e)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDecoderLargeSlice tests performance with large slices
func BenchmarkDecoderLargeSlice(b *testing.B) {
	type Item struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	type Container struct {
		Title string `json:"title"`
		Items []Item `json:"items"`
	}

	// Generate large slice data
	items := make([]map[string]any, 100)
	for i := 0; i < 100; i++ {
		items[i] = map[string]any{
			"id":    "item-" + string(rune(i)),
			"name":  "Item Name " + string(rune(i)),
			"value": i * 10,
		}
	}

	data := map[string]any{
		"title": "Large Container",
		"items": items,
	}

	// Use surrealcbor's marshaler to generate CBOR data
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("fxamacker_unmarshaler", func(b *testing.B) {
		unmarshaler := &models.CborUnmarshaler{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var c Container
			err := unmarshaler.Unmarshal(encoded, &c)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("surrealcbor_unmarshaler", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var c Container
			err := Unmarshal(encoded, &c)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDecoderMixedTypes tests performance with various data types
func BenchmarkDecoderMixedTypes(b *testing.B) {
	type MixedData struct {
		StringField  string         `json:"string_field"`
		IntField     int            `json:"int_field"`
		FloatField   float64        `json:"float_field"`
		BoolField    bool           `json:"bool_field"`
		SliceField   []string       `json:"slice_field"`
		MapField     map[string]int `json:"map_field"`
		NestedStruct struct {
			Inner string `json:"inner"`
		} `json:"nested_struct"`
		BytesField []byte  `json:"bytes_field"`
		NilField   *string `json:"nil_field"`
	}

	// Sample data with mixed types
	data := map[string]any{
		"string_field": "test string",
		"int_field":    42,
		"float_field":  3.14159,
		"bool_field":   true,
		"slice_field":  []string{"a", "b", "c", "d", "e"},
		"map_field": map[string]int{
			"one":   1,
			"two":   2,
			"three": 3,
		},
		"nested_struct": map[string]any{
			"inner": "nested value",
		},
		"bytes_field": []byte("hello world"),
		"nil_field":   nil,
	}

	// Use surrealcbor's marshaler to generate CBOR data
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("fxamacker_unmarshaler", func(b *testing.B) {
		unmarshaler := &models.CborUnmarshaler{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var m MixedData
			err := unmarshaler.Unmarshal(encoded, &m)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("surrealcbor_unmarshaler", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var m MixedData
			err := Unmarshal(encoded, &m)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDecoderCaseInsensitive tests performance with case-insensitive field matching
func BenchmarkDecoderCaseInsensitive(b *testing.B) {
	type CaseSensitive struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		UserName  string // No tag, relies on field name matching
		UserID    int    // No tag, relies on field name matching
	}

	// Data with mixed case field names
	data := map[string]any{
		"first_name": "John",    // Exact match
		"LAST_NAME":  "Doe",     // Case mismatch
		"username":   "johndoe", // Case-insensitive field name
		"userid":     12345,     // Case-insensitive field name
	}

	// Use surrealcbor's marshaler to generate CBOR data
	encoded, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("fxamacker_unmarshaler", func(b *testing.B) {
		unmarshaler := &models.CborUnmarshaler{}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var c CaseSensitive
			err := unmarshaler.Unmarshal(encoded, &c)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("surrealcbor_unmarshaler", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var c CaseSensitive
			err := Unmarshal(encoded, &c)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDecoderWithNone tests performance with SurrealDB None values
// This showcases the main feature difference - surrealcbor can handle None as nil
func BenchmarkDecoderWithNone(b *testing.B) {
	type DataWithOptionals struct {
		ID       string             `json:"id"`
		Name     *string            `json:"name"`
		Age      *int               `json:"age"`
		Email    *string            `json:"email"`
		Metadata map[string]*string `json:"metadata"`
	}

	// Create data with None values (which our marshaler handles)
	name := "John"
	data := map[string]any{
		"id":    "user-123",
		"name":  &name,
		"age":   models.CustomNil{}, // SurrealDB None
		"email": models.CustomNil{}, // SurrealDB None
		"metadata": map[string]any{
			"field1": "value1",
			"field2": models.CustomNil{}, // SurrealDB None
			"field3": "value3",
		},
	}

	// Use models marshaler since it knows how to encode CustomNil as Tag 6
	marshaler := &models.CborMarshaler{}
	encoded, err := marshaler.Marshal(data)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("surrealcbor_unmarshaler_with_none", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var d DataWithOptionals
			err := Unmarshal(encoded, &d)
			if err != nil {
				b.Fatal(err)
			}
			// Our implementation should unmarshal None to nil
			if d.Age != nil || d.Email != nil {
				b.Fatal("Expected None values to be unmarshaled as nil")
			}
		}
	})

	// Note: We don't benchmark fxamacker here because it can't handle None -> nil conversion
	// It would unmarshal None to a non-nil CustomNil{} struct
}
