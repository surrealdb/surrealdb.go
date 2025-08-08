package surrealql_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
)

func ExampleUpsert_noContent() {
	// UPSERT without data modification - creates record if it doesn't exist
	sql, vars := surrealql.Upsert("foo:1").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT foo:1
	// Variables: map[]
}

func ExampleUpsert_noContent_multiple() {
	// UPSERT multiple records without data modification
	sql, vars := surrealql.Upsert("foo:2", "foo:3").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT foo:2, foo:3
	// Variables: map[]
}

func ExampleUpsert_set() {
	// Basic UPSERT with SET
	sql, vars := surrealql.Upsert("product:laptop").
		Set("name", "Laptop Pro").
		Set("price", 1299).
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:laptop SET name = $upsert_name_1, price = $upsert_price_1
	// Variables: map[upsert_name_1:Laptop Pro upsert_price_1:1299]
}

func ExampleUpsert_content_returnAfter() {
	// UPSERT with CONTENT and RETURN AFTER
	sql, vars := surrealql.Upsert("product:tablet").
		Content(map[string]any{
			"name":  "Tablet Pro",
			"price": 899,
		}).
		ReturnAfter().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:tablet CONTENT $upsert_content_1 RETURN AFTER
	// Variables: map[upsert_content_1:map[name:Tablet Pro price:899]]
}

func ExampleUpsert_content() {
	// UPSERT with CONTENT
	sql, vars := surrealql.Upsert("product:phone").
		Content(map[string]any{
			"name":     "Smartphone X",
			"price":    799,
			"features": []string{"5G", "OLED", "Wireless Charging"},
		}).
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:phone CONTENT $upsert_content_1
	// Variables: map[upsert_content_1:map[features:[5G OLED Wireless Charging] name:Smartphone X price:799]]
}

func ExampleUpsert_merge() {
	// UPSERT with MERGE - updates specific fields
	sql, vars := surrealql.Upsert("product:headphones").
		Merge(map[string]any{
			"colors": []string{"Black", "White"},
		}).
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:headphones MERGE $upsert_merge_1
	// Variables: map[upsert_merge_1:map[colors:[Black White]]]
}

func ExampleUpsert_merge_returnDiff() {
	// UPSERT with MERGE and RETURN DIFF - shows changes
	sql, vars := surrealql.Upsert("product:watch").
		Merge(map[string]any{
			"available":  true,
			"updated_at": "2023-01-01",
		}).
		ReturnDiff().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:watch MERGE $upsert_merge_1 RETURN DIFF
	// Variables: map[upsert_merge_1:map[available:true updated_at:2023-01-01]]
}

func ExampleUpsert_patch() {
	// UPSERT with JSON Patch operations
	sql, vars := surrealql.Upsert("product:keyboard").
		Patch([]surrealql.PatchOp{
			// Note that `/features/-` appends to the array because
			// the `-` at the end of the path refers to the end of the array.
			// See https://jsonpatch.com/#json-pointer
			{Op: "add", Path: "/features/-", Value: "RGB Lighting"},
			{Op: "replace", Path: "/price", Value: 149},
		}).
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:keyboard PATCH $upsert_patch_1
	// Variables: map[upsert_patch_1:[{add /features/- RGB Lighting} {replace /price 149}]]
}

func ExampleUpsert_patch_returnBefore() {
	// UPSERT with PATCH and RETURN BEFORE - returns record before changes
	sql, vars := surrealql.Upsert("product:mouse").
		Patch([]surrealql.PatchOp{
			{Op: "remove", Path: "/deprecated"},
			{Op: "add", Path: "/warranty", Value: true},
		}).
		ReturnBefore().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:mouse PATCH $upsert_patch_1 RETURN BEFORE
	// Variables: map[upsert_patch_1:[{remove /deprecated <nil>} {add /warranty true}]]
}

func ExampleUpsert_replace_returnFields() {
	// UPSERT with REPLACE and RETURN specific fields
	sql, vars := surrealql.Upsert("product:monitor").
		Replace(map[string]any{
			"name":     "Ultra Monitor",
			"price":    599,
			"category": "displays",
			"specs":    "4K HDR",
		}).
		Return("id, name, category").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:monitor REPLACE $upsert_replace_1 RETURN id, name, category
	// Variables: map[upsert_replace_1:map[category:displays name:Ultra Monitor price:599 specs:4K HDR]]
}

func ExampleUpsert_withConditions() {
	// UPSERT with WHERE condition and RETURN clause
	sql, vars := surrealql.Upsert("product:speaker").
		Set("last_updated", "2024-01-01T00:00:00Z").
		Set("status", "in_stock").
		Where("price >= ?", 100).
		ReturnDiff().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:speaker SET last_updated = $upsert_last_updated_1, status = $upsert_status_1 WHERE price >= $param_1 RETURN DIFF
	// Variables: map[param_1:100 upsert_last_updated_1:2024-01-01T00:00:00Z upsert_status_1:in_stock]
}

func ExampleUpsertOnly() {
	// UPSERT ONLY returns a single record instead of an array
	sql, vars := surrealql.UpsertOnly("product:charger").
		Set("name", "Fast Charger").
		Set("available", true).
		ReturnAfter().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT ONLY product:charger SET available = $upsert_available_1, name = $upsert_name_1 RETURN AFTER
	// Variables: map[upsert_available_1:true upsert_name_1:Fast Charger]
}

func ExampleUpsert_unset() {
	// UPSERT with UNSET to remove fields
	sql, vars := surrealql.Upsert("product:cable").
		Set("name", "USB Cable").
		Unset("deprecated_field", "legacy_data").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:cable SET name = $upsert_name_1, UNSET deprecated_field, legacy_data
	// Variables: map[upsert_name_1:USB Cable]
}

func ExampleUpsert_returnNone() {
	// UPSERT with RETURN NONE - no data returned, improves performance
	sql, vars := surrealql.Upsert("product:adapter").
		Set("processed", true).
		ReturnNone().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:adapter SET processed = $upsert_processed_1 RETURN NONE
	// Variables: map[upsert_processed_1:true]
}

func ExampleUpsert_returnAfter() {
	// UPSERT with RETURN AFTER (default) - returns the record after changes
	sql, vars := surrealql.Upsert("product:desk").
		Set("name", "Standing Desk").
		Set("price", 450).
		ReturnAfter().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:desk SET name = $upsert_name_1, price = $upsert_price_1 RETURN AFTER
	// Variables: map[upsert_name_1:Standing Desk upsert_price_1:450]
}

func ExampleUpsert_returnBefore() {
	// UPSERT with RETURN BEFORE - returns the record before changes
	sql, vars := surrealql.Upsert("product:chair").
		Set("name", "Office Chair").
		Set("price", 250).
		ReturnBefore().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:chair SET name = $upsert_name_1, price = $upsert_price_1 RETURN BEFORE
	// Variables: map[upsert_name_1:Office Chair upsert_price_1:250]
}

func ExampleUpsert_returnDiff() {
	// UPSERT with RETURN DIFF - returns the differences before and after
	sql, vars := surrealql.Upsert("product:lamp").
		Set("name", "LED Lamp").
		Set("price", 75).
		ReturnDiff().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:lamp SET name = $upsert_name_1, price = $upsert_price_1 RETURN DIFF
	// Variables: map[upsert_name_1:LED Lamp upsert_price_1:75]
}

func ExampleUpsert_returnFields() {
	// UPSERT with RETURN specific fields - returns only specified fields
	sql, vars := surrealql.Upsert("product:webcam").
		Set("name", "HD Webcam").
		Set("price", 120).
		Set("resolution", "1080p").
		Return("name, price").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:webcam SET name = $upsert_name_1, price = $upsert_price_1, resolution = $upsert_resolution_1 RETURN name, price
	// Variables: map[upsert_name_1:HD Webcam upsert_price_1:120 upsert_resolution_1:1080p]
}

func ExampleUpsert_performance() {
	// UPSERT with performance optimizations using TIMEOUT and PARALLEL
	sql, vars := surrealql.Upsert("product:microphone").
		Set("processed", true).
		Timeout("5s").
		Parallel().
		ReturnNone().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:microphone SET processed = $upsert_processed_1 RETURN NONE TIMEOUT 5s PARALLEL
	// Variables: map[upsert_processed_1:true]
}

func ExampleUpsert_setRaw() {
	// UPSERT with raw SET expressions for compound operations (deprecated - use Set instead)
	sql, vars := surrealql.Upsert("product:book").
		SetRaw("tags += 'bestseller'").
		SetRaw("view_count += 1").
		Set("last_viewed", "2024-01-01T00:00:00Z").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:book SET last_viewed = $upsert_last_viewed_1, tags += 'bestseller', view_count += 1
	// Variables: map[upsert_last_viewed_1:2024-01-01T00:00:00Z]
}

func ExampleUpsert_setCompound() {
	// UPSERT with compound operations using the Set function
	sql, vars := surrealql.Upsert("product:book").
		Set("tags += ?", "bestseller").
		Set("view_count += ?", 1).
		Set("last_viewed", "2024-01-01T00:00:00Z").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:book SET last_viewed = $upsert_last_viewed_1, tags += $upsert_param_1, view_count += $upsert_param_2
	// Variables: map[upsert_last_viewed_1:2024-01-01T00:00:00Z upsert_param_1:bestseller upsert_param_2:1]
}

func ExampleUpsert_setRaw_arrayOperations() {
	// UPSERT with various raw SET expressions for array and numeric operations
	sql, vars := surrealql.Upsert("product:laptop").
		SetRaw("categories += ['electronics', 'computers']").
		SetRaw("stock -= 1").
		SetRaw("sales_count += 1").
		Set("available", true).
		ReturnAfter().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:laptop SET available = $upsert_available_1, categories += ['electronics', 'computers'], stock -= 1, sales_count += 1 RETURN AFTER
	// Variables: map[upsert_available_1:true]
}

func ExampleUpsert_setWithTime() {
	// UPSERT with time.Time values using the Set function
	lastViewed := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	sql, vars := surrealql.Upsert("product:watch").
		Set("name", "Smart Watch Pro").
		Set("price", 299.99).
		Set("last_viewed", lastViewed).
		Set("view_count += ?", 1).
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:watch SET last_viewed = $upsert_last_viewed_1, name = $upsert_name_1, price = $upsert_price_1, view_count += $upsert_param_1
	// Variables: map[upsert_last_viewed_1:2024-01-15 10:30:00 +0000 UTC upsert_name_1:Smart Watch Pro upsert_param_1:1 upsert_price_1:299.99]
}

func ExampleUpsert_set_arrayOperations() {
	// UPSERT with array and numeric operations using the Set function
	sql, vars := surrealql.Upsert("product:laptop").
		Set("categories += ?", []string{"electronics", "computers"}).
		Set("stock -= ?", 1).
		Set("sales_count += ?", 1).
		Set("available", true).
		ReturnAfter().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:laptop SET available = $upsert_available_1, categories += $upsert_param_1, stock -= $upsert_param_2, sales_count += $upsert_param_3 RETURN AFTER
	// Variables: map[upsert_available_1:true upsert_param_1:[electronics computers] upsert_param_2:1 upsert_param_3:1]
}

func ExampleUpsert_set_mixed() {
	// UPSERT with mixed operations showing the flexibility of the Set function
	createdAt := time.Date(2024, 1, 20, 15, 30, 0, 0, time.UTC)

	sql, vars := surrealql.Upsert("product:smartphone").
		Set("name", "Latest Phone").                // Simple string assignment
		Set("price", 899.99).                       // Simple numeric assignment
		Set("in_stock", true).                      // Simple boolean assignment
		Set("created_at", createdAt).               // Simple time.Time assignment
		Set("tags += ?", "flagship").               // Append single value to array
		Set("features += ?", []string{"5G", "AI"}). // Append multiple values to array
		Set("view_count += ?", 1).                  // Increment counter
		Set("discount_percentage", 10).             // Simple assignment
		Where("stock > ?", 0).                      // Only update if in stock
		ReturnAfter().
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:smartphone SET created_at = $upsert_created_at_1, discount_percentage = $upsert_discount_percentage_1, in_stock = $upsert_in_stock_1, name = $upsert_name_1, price = $upsert_price_1, tags += $upsert_param_1, features += $upsert_param_2, view_count += $upsert_param_3 WHERE stock > $param_1 RETURN AFTER
	// Variables: map[param_1:0 upsert_created_at_1:2024-01-20 15:30:00 +0000 UTC upsert_discount_percentage_1:10 upsert_in_stock_1:true upsert_name_1:Latest Phone upsert_param_1:flagship upsert_param_2:[5G AI] upsert_param_3:1 upsert_price_1:899.99]
}

func ExampleUpsert_returnValue() {
	// UPSERT with RETURN VALUE - returns just the field value, not the whole record
	sql, vars := surrealql.Upsert("product:counter").
		Set("view_count += ?", 1).
		ReturnValue("view_count").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:counter SET view_count += $upsert_param_1 RETURN VALUE view_count
	// Variables: map[upsert_param_1:1]
}

func ExampleUpsert_returnValue_withContent() {
	// UPSERT CONTENT with RETURN VALUE - returns just the specified field value
	sql, vars := surrealql.Upsert("product:item123").
		Content(map[string]any{
			"name":  "New Product",
			"price": 99.99,
			"stock": 100,
		}).
		ReturnValue("price").
		Build()

	fmt.Println(sql)
	fmt.Printf("Variables: %v\n", vars)
	// Output:
	// UPSERT product:item123 CONTENT $upsert_content_1 RETURN VALUE price
	// Variables: map[upsert_content_1:map[name:New Product price:99.99 stock:100]]
}
