package surrealql

import (
	"reflect"
	"testing"
)

func TestUpsert_Basic(t *testing.T) {
	tests := []struct {
		name     string
		build    func() (string, map[string]any)
		wantSQL  string
		wantVars map[string]any
	}{
		{
			name: "basic upsert with SET",
			build: func() (string, map[string]any) {
				return Upsert("product:laptop").
					Set("name", "Laptop Pro").
					Set("price", 1299).
					Build()
			},
			wantSQL: "UPSERT product:laptop SET name = $upsert_name_1, price = $upsert_price_1",
			wantVars: map[string]any{
				"upsert_name_1":  "Laptop Pro",
				"upsert_price_1": 1299,
			},
		},
		{
			name: "upsert with CONTENT",
			build: func() (string, map[string]any) {
				return Upsert("product:phone").
					Content(map[string]any{
						"name":  "Smartphone X",
						"price": 799,
					}).
					Build()
			},
			wantSQL: "UPSERT product:phone CONTENT $upsert_content_1",
			wantVars: map[string]any{
				"upsert_content_1": map[string]any{
					"name":  "Smartphone X",
					"price": 799,
				},
			},
		},
		{
			name: "upsert with MERGE",
			build: func() (string, map[string]any) {
				return Upsert("product:headphones").
					Merge(map[string]any{
						"colors": []string{"Black", "White"},
					}).
					Build()
			},
			wantSQL: "UPSERT product:headphones MERGE $upsert_merge_1",
			wantVars: map[string]any{
				"upsert_merge_1": map[string]any{
					"colors": []string{"Black", "White"},
				},
			},
		},
		{
			name: "upsert with REPLACE",
			build: func() (string, map[string]any) {
				return Upsert("product:tablet").
					Replace(map[string]any{
						"name":  "Tablet Pro",
						"price": 899,
					}).
					Build()
			},
			wantSQL: "UPSERT product:tablet REPLACE $upsert_replace_1",
			wantVars: map[string]any{
				"upsert_replace_1": map[string]any{
					"name":  "Tablet Pro",
					"price": 899,
				},
			},
		},
		{
			name: "upsert with PATCH",
			build: func() (string, map[string]any) {
				return Upsert("product:keyboard").
					Patch([]PatchOp{
						{Op: "add", Path: "/features", Value: []string{"RGB Lighting"}},
					}).
					Build()
			},
			wantSQL: "UPSERT product:keyboard PATCH $upsert_patch_1",
			wantVars: map[string]any{
				"upsert_patch_1": []PatchOp{
					{Op: "add", Path: "/features", Value: []string{"RGB Lighting"}},
				},
			},
		},
		{
			name: "upsert ONLY with SET",
			build: func() (string, map[string]any) {
				return UpsertOnly("product:charger").
					Set("name", "Fast Charger").
					Build()
			},
			wantSQL: "UPSERT ONLY product:charger SET name = $upsert_name_1",
			wantVars: map[string]any{
				"upsert_name_1": "Fast Charger",
			},
		},
		{
			name: "upsert with WHERE clause",
			build: func() (string, map[string]any) {
				return Upsert("product:monitor").
					Set("updated", true).
					Where("price > ?", 100).
					Build()
			},
			wantSQL: "UPSERT product:monitor SET updated = $upsert_updated_1 WHERE price > $param_1",
			wantVars: map[string]any{
				"upsert_updated_1": true,
				"param_1":          100,
			},
		},
		{
			name: "upsert with RETURN NONE",
			build: func() (string, map[string]any) {
				return Upsert("product:adapter").
					Set("name", "Power Adapter").
					ReturnNone().
					Build()
			},
			wantSQL: "UPSERT product:adapter SET name = $upsert_name_1 RETURN NONE",
			wantVars: map[string]any{
				"upsert_name_1": "Power Adapter",
			},
		},
		{
			name: "upsert with RETURN DIFF",
			build: func() (string, map[string]any) {
				return Upsert("product:lamp").
					Set("name", "LED Lamp").
					ReturnDiff().
					Build()
			},
			wantSQL: "UPSERT product:lamp SET name = $upsert_name_1 RETURN DIFF",
			wantVars: map[string]any{
				"upsert_name_1": "LED Lamp",
			},
		},
		{
			name: "upsert with RETURN BEFORE",
			build: func() (string, map[string]any) {
				return Upsert("product:chair").
					Set("name", "Office Chair").
					ReturnBefore().
					Build()
			},
			wantSQL: "UPSERT product:chair SET name = $upsert_name_1 RETURN BEFORE",
			wantVars: map[string]any{
				"upsert_name_1": "Office Chair",
			},
		},
		{
			name: "upsert with RETURN AFTER",
			build: func() (string, map[string]any) {
				return Upsert("product:desk").
					Set("name", "Standing Desk").
					ReturnAfter().
					Build()
			},
			wantSQL: "UPSERT product:desk SET name = $upsert_name_1 RETURN AFTER",
			wantVars: map[string]any{
				"upsert_name_1": "Standing Desk",
			},
		},
		{
			name: "upsert with TIMEOUT",
			build: func() (string, map[string]any) {
				return Upsert("product:webcam").
					Set("name", "HD Webcam").
					Timeout("5s").
					Build()
			},
			wantSQL: "UPSERT product:webcam SET name = $upsert_name_1 TIMEOUT 5s",
			wantVars: map[string]any{
				"upsert_name_1": "HD Webcam",
			},
		},
		{
			name: "upsert with PARALLEL",
			build: func() (string, map[string]any) {
				return Upsert("product:mouse").
					Set("name", "Wireless Mouse").
					Parallel().
					Build()
			},
			wantSQL: "UPSERT product:mouse SET name = $upsert_name_1 PARALLEL",
			wantVars: map[string]any{
				"upsert_name_1": "Wireless Mouse",
			},
		},
		{
			name: "upsert with UNSET",
			build: func() (string, map[string]any) {
				return Upsert("product:cable").
					Set("name", "USB Cable").
					Unset("deprecated_field").
					Build()
			},
			wantSQL: "UPSERT product:cable SET name = $upsert_name_1, UNSET deprecated_field",
			wantVars: map[string]any{
				"upsert_name_1": "USB Cable",
			},
		},
		{
			name: "upsert with multiple UNSET",
			build: func() (string, map[string]any) {
				return Upsert("product:storage").
					Set("name", "SSD Storage").
					Unset("deprecated_field", "legacy_data", "old_column").
					Build()
			},
			wantSQL: "UPSERT product:storage SET name = $upsert_name_1, UNSET deprecated_field, legacy_data, old_column",
			wantVars: map[string]any{
				"upsert_name_1": "SSD Storage",
			},
		},
		{
			name: "upsert with SetMap",
			build: func() (string, map[string]any) {
				return Upsert("product:speaker").
					SetMap(map[string]any{
						"name":  "Bluetooth Speaker",
						"price": 89,
					}).
					Build()
			},
			wantSQL: "UPSERT product:speaker SET name = $upsert_name_1, price = $upsert_price_1",
			wantVars: map[string]any{
				"upsert_name_1":  "Bluetooth Speaker",
				"upsert_price_1": 89,
			},
		},
		{
			name: "upsert multiple targets",
			build: func() (string, map[string]any) {
				return Upsert("product:item1", "product:item2").
					Set("active", true).
					Build()
			},
			wantSQL: "UPSERT product:item1, product:item2 SET active = $upsert_active_1",
			wantVars: map[string]any{
				"upsert_active_1": true,
			},
		},
		{
			name: "upsert table without ID",
			build: func() (string, map[string]any) {
				return Upsert("product").
					Set("name", "Generic Product").
					Build()
			},
			wantSQL: "UPSERT product SET name = $upsert_name_1",
			wantVars: map[string]any{
				"upsert_name_1": "Generic Product",
			},
		},
		{
			name: "upsert with all features",
			build: func() (string, map[string]any) {
				return Upsert("product:premium").
					Set("name", "Premium Product").
					Set("updated_at", "2024-01-01T00:00:00Z").
					Where("price >= ?", 1000).
					ReturnDiff().
					Timeout("10s").
					Parallel().
					Build()
			},
			wantSQL: "UPSERT product:premium SET name = $upsert_name_1, updated_at = $upsert_updated_at_1 WHERE price >= $param_1 RETURN DIFF TIMEOUT 10s PARALLEL",
			wantVars: map[string]any{
				"upsert_name_1":       "Premium Product",
				"upsert_updated_at_1": "2024-01-01T00:00:00Z",
				"param_1":             1000,
			},
		},
		{
			name: "upsert with EXPLAIN",
			build: func() (string, map[string]any) {
				return Upsert("product:example").
					Set("name", "Example Product").
					Explain().
					Build()
			},
			wantSQL: "EXPLAIN UPSERT product:example SET name = $upsert_name_1",
			wantVars: map[string]any{
				"upsert_name_1": "Example Product",
			},
		},
		{
			name: "upsert with EXPLAIN FULL",
			build: func() (string, map[string]any) {
				return Upsert("product:demo").
					Set("name", "Demo Product").
					ExplainFull().
					Build()
			},
			wantSQL: "EXPLAIN FULL UPSERT product:demo SET name = $upsert_name_1",
			wantVars: map[string]any{
				"upsert_name_1": "Demo Product",
			},
		},
		{
			name: "upsert without content single record",
			build: func() (string, map[string]any) {
				return Upsert("foo:1").Build()
			},
			wantSQL:  "UPSERT foo:1",
			wantVars: map[string]any{},
		},
		{
			name: "upsert without content multiple records",
			build: func() (string, map[string]any) {
				return Upsert("foo:2", "foo:3").Build()
			},
			wantSQL:  "UPSERT foo:2, foo:3",
			wantVars: map[string]any{},
		},
		{
			name: "upsert with SetRaw",
			build: func() (string, map[string]any) {
				return Upsert("product:widget").
					SetRaw("stock += 10").
					SetRaw("views += 1").
					Build()
			},
			wantSQL:  "UPSERT product:widget SET stock += 10, views += 1",
			wantVars: map[string]any{},
		},
		{
			name: "upsert with SetRaw and regular Set",
			build: func() (string, map[string]any) {
				return Upsert("product:gadget").
					Set("name", "Smart Gadget").
					SetRaw("popularity += 1").
					Set("updated", true).
					Build()
			},
			wantSQL: "UPSERT product:gadget SET name = $upsert_name_1, updated = $upsert_updated_1, popularity += 1",
			wantVars: map[string]any{
				"upsert_name_1":    "Smart Gadget",
				"upsert_updated_1": true,
			},
		},
		{
			name: "upsert with unified Set for compound operations",
			build: func() (string, map[string]any) {
				return Upsert("product:item").
					Set("name", "Test Item").
					Set("stock += ?", 10).
					Set("price -= ?", 5).
					Build()
			},
			wantSQL: "UPSERT product:item SET name = $upsert_name_1, stock += $upsert_param_1, price -= $upsert_param_2",
			wantVars: map[string]any{
				"upsert_name_1":  "Test Item",
				"upsert_param_1": 10,
				"upsert_param_2": 5,
			},
		},
		{
			name: "upsert with unified Set mixing simple and compound",
			build: func() (string, map[string]any) {
				return Upsert("product:mix").
					Set("count += ?", 1).
					Set("updated", true).
					Set("tags -= ?", "old").
					Build()
			},
			wantSQL: "UPSERT product:mix SET updated = $upsert_updated_1, count += $upsert_param_1, tags -= $upsert_param_2",
			wantVars: map[string]any{
				"upsert_updated_1": true,
				"upsert_param_1":   1,
				"upsert_param_2":   "old",
			},
		},
		{
			name: "upsert with RETURN VALUE",
			build: func() (string, map[string]any) {
				return Upsert("product:counter").
					Set("count += ?", 1).
					ReturnValue("count").
					Build()
			},
			wantSQL: "UPSERT product:counter SET count += $upsert_param_1 RETURN VALUE count",
			wantVars: map[string]any{
				"upsert_param_1": 1,
			},
		},
		{
			name: "upsert content with RETURN VALUE",
			build: func() (string, map[string]any) {
				return Upsert("product:item").
					Content(map[string]any{
						"name":  "Test Product",
						"price": 99.99,
					}).
					ReturnValue("price").
					Build()
			},
			wantSQL: "UPSERT product:item CONTENT $upsert_content_1 RETURN VALUE price",
			wantVars: map[string]any{
				"upsert_content_1": map[string]any{
					"name":  "Test Product",
					"price": 99.99,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSQL, gotVars := tt.build()

			if gotSQL != tt.wantSQL {
				t.Errorf("SQL mismatch\ngot:  %q\nwant: %q", gotSQL, tt.wantSQL)
			}

			if len(gotVars) != len(tt.wantVars) {
				t.Errorf("Variables count mismatch\ngot:  %d\nwant: %d", len(gotVars), len(tt.wantVars))
			}

			for k, wantVal := range tt.wantVars {
				gotVal, exists := gotVars[k]
				if !exists {
					t.Errorf("Missing variable %q", k)
					continue
				}

				if !reflect.DeepEqual(gotVal, wantVal) {
					t.Errorf("Variable %q mismatch\ngot:  %v (%T)\nwant: %v (%T)",
						k, gotVal, gotVal, wantVal, wantVal)
				}
			}
		})
	}
}

func TestUpsert_TypeSafety(t *testing.T) {
	// Test that the API prevents mixing incompatible data modes at compile time

	t.Run("SET mode allows multiple Set and Unset", func(t *testing.T) {
		q := Upsert("product:gadget").
			Set("name", "Smart Gadget").
			Set("price", 199).
			Unset("deprecated")

		sql, _ := q.Build()
		expected := "UPSERT product:gadget SET name = $upsert_name_1, price = $upsert_price_1, UNSET deprecated"
		if sql != expected {
			t.Errorf("Expected %q, got %q", expected, sql)
		}
	})

	t.Run("CONTENT mode is immutable after setting", func(t *testing.T) {
		q := Upsert("product:device").
			Content(map[string]any{
				"name":  "Smart Device",
				"price": 349,
			})

		// The type system prevents calling Set() on UpsertContentQuery
		// This is enforced at compile time, not runtime
		sql, _ := q.Build()
		expected := "UPSERT product:device CONTENT $upsert_content_1"
		if sql != expected {
			t.Errorf("Expected %q, got %q", expected, sql)
		}
	})

	t.Run("Each mode returns appropriate type", func(t *testing.T) {
		// These are all different types, enforced at compile time
		setQuery := Upsert("product:item").Set("name", "Test Item")
		contentQuery := Upsert("product:item").Content(map[string]any{"name": "Test Item"})
		mergeQuery := Upsert("product:item").Merge(map[string]any{"name": "Test Item"})
		patchQuery := Upsert("product:item").Patch([]PatchOp{{Op: "add", Path: "/name", Value: "Test Item"}})
		replaceQuery := Upsert("product:item").Replace(map[string]any{"name": "Test Item"})

		// Each returns the expected SQL
		if sql, _ := setQuery.Build(); !contains(sql, "SET") {
			t.Errorf("SET query should contain SET: %q", sql)
		}
		if sql, _ := contentQuery.Build(); !contains(sql, "CONTENT") {
			t.Errorf("CONTENT query should contain CONTENT: %q", sql)
		}
		if sql, _ := mergeQuery.Build(); !contains(sql, "MERGE") {
			t.Errorf("MERGE query should contain MERGE: %q", sql)
		}
		if sql, _ := patchQuery.Build(); !contains(sql, "PATCH") {
			t.Errorf("PATCH query should contain PATCH: %q", sql)
		}
		if sql, _ := replaceQuery.Build(); !contains(sql, "REPLACE") {
			t.Errorf("REPLACE query should contain REPLACE: %q", sql)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
