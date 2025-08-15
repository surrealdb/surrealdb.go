package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// formatRecordResult formats a record result (map[string]any) for testing.
// This is used for regular live query results (without diff) and DELETE operations.
// It handles the id field specially, formatting RecordID as table:⟨UUID⟩.
func formatRecordResult(record map[string]any) string {
	keys := make([]string, 0, len(record))
	for k := range record {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		val := record[k]
		if k == "id" {
			// The id field must be a models.RecordID
			recordID := val.(models.RecordID)
			parts = append(parts, fmt.Sprintf("id=%s:⟨UUID⟩", recordID.Table))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%v", k, val))
		}
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// formatDiffResult formats a diff result ([]any) for testing.
// Each item in the array is a diff operation (map[string]any).
func formatDiffResult(diffs []any) string {
	var items []string
	for _, item := range diffs {
		diffOp, ok := item.(map[string]any)
		if !ok {
			panic(fmt.Sprintf("Expected diff operation to be map[string]any, got %T", item))
		}
		items = append(items, formatDiffOperation(diffOp))
	}
	return "[" + strings.Join(items, " ") + "]"
}

// formatPatchDataMap formats a map representation of PatchData.
// This is the data contained in the "value" field of a diff operation.
func formatPatchDataMap(data map[string]any) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		val := data[k]
		if k == "id" {
			// The id field in patch data is also a models.RecordID
			recordID := val.(models.RecordID)
			parts = append(parts, fmt.Sprintf("id=%s:⟨UUID⟩", recordID.Table))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%v", k, val))
		}
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// formatDiffOperation formats a single diff operation.
// A diff operation contains fields like "op", "path", and optionally "value".
func formatDiffOperation(op map[string]any) string {
	keys := make([]string, 0, len(op))
	for k := range op {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		val := op[k]
		if k == "value" {
			// The value field contains patch data (not a regular record)
			if patchData, ok := val.(map[string]any); ok {
				parts = append(parts, fmt.Sprintf("value=%s", formatPatchDataMap(patchData)))
			} else {
				// For non-map values (like simple value replacements)
				parts = append(parts, fmt.Sprintf("value=%v", val))
			}
		} else {
			parts = append(parts, fmt.Sprintf("%s=%v", k, val))
		}
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// ExampleLive demonstrates using the Live RPC method to receive notifications.
// Live queries without diff return the full record as map[string]any in notification.Result.
// The notification channel is automatically closed when Kill is called.
//
//nolint:gocyclo
func ExampleLive() {
	config := testenv.MustNewConfig("surrealdbexamples", "livequery_rpc", "users")
	config.Endpoint = testenv.GetSurrealDBWSURL()

	db := config.MustNew()

	type User struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Username string           `json:"username"`
		Email    string           `json:"email"`
	}

	ctx := context.Background()

	live, err := surrealdb.Live(ctx, db, "users", false)
	if err != nil {
		panic(fmt.Sprintf("Failed to start live query: %v", err))
	}

	fmt.Println("Started live query")

	notifications, err := db.LiveNotifications(live.String())
	if err != nil {
		panic(fmt.Sprintf("Failed to get live notifications channel: %v", err))
	}

	received := make(chan struct{})
	done := make(chan bool)
	go func() {
		for notification := range notifications {
			// Live queries without diff return the record as map[string]any
			record, ok := notification.Result.(map[string]any)
			if !ok {
				panic(fmt.Sprintf("Expected map[string]any, got %T", notification.Result))
			}

			fmt.Printf("Received notification - Action: %s, Result: %s\n", notification.Action, formatRecordResult(record))

			switch notification.Action {
			case connection.CreateAction:
				fmt.Println("New user created")
			case connection.UpdateAction:
				fmt.Println("User updated")
			case connection.DeleteAction:
				fmt.Println("User deleted")
				close(received)
			}
		}
		// Channel was closed
		fmt.Println("Notification channel closed")
		done <- true
	}()

	createdUser, err := surrealdb.Create[User](ctx, db, "users", map[string]any{
		"username": "alice",
		"email":    "alice@example.com",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create user: %v", err))
	}

	_, err = surrealdb.Update[User](ctx, db, *createdUser.ID, map[string]any{
		"email": "alice.updated@example.com",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to update user: %v", err))
	}

	_, err = surrealdb.Delete[User](ctx, db, *createdUser.ID)
	if err != nil {
		panic(fmt.Sprintf("Failed to delete user: %v", err))
	}

	// Wait for all expected notifications to be received
	select {
	case <-received:
		// All notifications received
	case <-time.After(2 * time.Second):
		panic("Timeout waiting for all notifications")
	}

	err = surrealdb.Kill(ctx, db, live.String())
	if err != nil {
		panic(fmt.Sprintf("Failed to kill live query: %v", err))
	}

	fmt.Println("Live query terminated")

	select {
	case <-done:
		fmt.Println("Goroutine exited after channel closed")
	case <-time.After(2 * time.Second):
		panic("Timeout: notification channel was not closed after Kill")
	}

	// Output:
	// Started live query
	// Received notification - Action: CREATE, Result: {email=alice@example.com id=users:⟨UUID⟩ username=alice}
	// New user created
	// Received notification - Action: UPDATE, Result: {email=alice.updated@example.com id=users:⟨UUID⟩}
	// User updated
	// Received notification - Action: DELETE, Result: {email=alice.updated@example.com id=users:⟨UUID⟩}
	// User deleted
	// Live query terminated
	// Notification channel closed
	// Goroutine exited after channel closed
}

// ExampleQuery_live demonstrates using LIVE SELECT via the Query RPC.
// LIVE SELECT returns matching records as map[string]any in notification.Result.
// The notification channel is automatically closed when Kill is called.
func ExampleQuery_live() {
	config := testenv.MustNewConfig("surrealdbexamples", "livequery_query", "products")
	config.Endpoint = testenv.GetSurrealDBWSURL()

	db := config.MustNew()

	type Product struct {
		ID    *models.RecordID `json:"id,omitempty"`
		Name  string           `json:"name"`
		Price float64          `json:"price"`
		Stock int              `json:"stock"`
	}

	ctx := context.Background()

	result, err := surrealdb.Query[models.UUID](ctx, db, "LIVE SELECT * FROM products WHERE stock < 10", map[string]any{})
	if err != nil {
		panic(fmt.Sprintf("Failed to start live query: %v", err))
	}

	liveID := (*result)[0].Result.String()
	fmt.Println("Started live query")

	notifications, err := db.LiveNotifications(liveID)
	if err != nil {
		panic(fmt.Sprintf("Failed to get live notifications channel: %v", err))
	}

	received := make(chan struct{})
	done := make(chan bool)
	notificationCount := 0
	go func() {
		for notification := range notifications {
			notificationCount++

			// LIVE SELECT returns matching records as map[string]any
			record, ok := notification.Result.(map[string]any)
			if !ok {
				panic(fmt.Sprintf("Expected map[string]any for LIVE SELECT result, got %T", notification.Result))
			}

			fmt.Printf("Notification %d - Action: %s, Result: %s\n", notificationCount, notification.Action, formatRecordResult(record))

			if notificationCount >= 3 {
				close(received)
			}
		}
		// Channel was closed
		fmt.Println("Notification channel closed")
		done <- true
	}()

	_, err = surrealdb.Create[Product](ctx, db, "products", map[string]any{
		"name":  "Widget",
		"price": 9.99,
		"stock": 5,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create product: %v", err))
	}

	_, err = surrealdb.Create[Product](ctx, db, "products", map[string]any{
		"name":  "Gadget",
		"price": 19.99,
		"stock": 3,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create second product: %v", err))
	}

	_, err = surrealdb.Create[Product](ctx, db, "products", map[string]any{
		"name":  "Abundant Item",
		"price": 5.99,
		"stock": 100,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create third product: %v", err))
	}

	_, err = surrealdb.Create[Product](ctx, db, "products", map[string]any{
		"name":  "Rare Item",
		"price": 99.99,
		"stock": 1,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create fourth product: %v", err))
	}

	// Wait for all expected notifications to be received
	select {
	case <-received:
		// All notifications received
	case <-time.After(2 * time.Second):
		panic("Timeout waiting for all notifications")
	}

	err = surrealdb.Kill(ctx, db, liveID)
	if err != nil {
		panic(fmt.Sprintf("Failed to kill live query: %v", err))
	}

	fmt.Println("Live query terminated")

	select {
	case <-done:
		fmt.Println("Goroutine exited after channel closed")
	case <-time.After(2 * time.Second):
		panic("Timeout: notification channel was not closed after Kill")
	}

	// Output:
	// Started live query
	// Notification 1 - Action: CREATE, Result: {id=products:⟨UUID⟩ name=Widget price=9.99 stock=5}
	// Notification 2 - Action: CREATE, Result: {id=products:⟨UUID⟩ name=Gadget price=19.99 stock=3}
	// Notification 3 - Action: CREATE, Result: {id=products:⟨UUID⟩ name=Rare Item price=99.99 stock=1}
	// Live query terminated
	// Notification channel closed
	// Goroutine exited after channel closed
}

// ExampleLive_withDiff demonstrates using live queries with diff enabled.
// With diff=true, CREATE and UPDATE return diff operations as []any,
// while DELETE still returns the deleted record as map[string]any.
// The notification channel is automatically closed when Kill is called.
func ExampleLive_withDiff() {
	config := testenv.MustNewConfig("surrealdbexamples", "livequery_diff", "inventory")
	config.Endpoint = testenv.GetSurrealDBWSURL()

	db := config.MustNew()

	type Item struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Name     string           `json:"name"`
		Quantity int              `json:"quantity"`
	}

	ctx := context.Background()

	live, err := surrealdb.Live(ctx, db, "inventory", true)
	if err != nil {
		panic(fmt.Sprintf("Failed to start live query with diff: %v", err))
	}

	fmt.Println("Started live query with diff enabled")

	notifications, err := db.LiveNotifications(live.String())
	if err != nil {
		panic(fmt.Sprintf("Failed to get live notifications channel: %v", err))
	}

	received := make(chan struct{})
	done := make(chan bool)
	go func() {
		for notification := range notifications {
			var resultStr string

			// With diff=true:
			// - CREATE and UPDATE return diff operations as []any
			// - DELETE returns the full deleted record as map[string]any (same as without diff)
			if notification.Action == connection.DeleteAction {
				// DELETE always returns a regular record, even with diff=true
				record, ok := notification.Result.(map[string]any)
				if !ok {
					panic(fmt.Sprintf("Expected map[string]any for DELETE result, got %T", notification.Result))
				}
				resultStr = formatRecordResult(record)
				close(received)
			} else {
				// CREATE and UPDATE return an array of diff operations
				diffs, ok := notification.Result.([]any)
				if !ok {
					panic(fmt.Sprintf("Expected []any for diff result, got %T", notification.Result))
				}
				resultStr = formatDiffResult(diffs)
			}

			fmt.Printf("Action: %s, Result: %s\n", notification.Action, resultStr)
		}
		// Channel was closed
		fmt.Println("Notification channel closed")
		done <- true
	}()

	item, err := surrealdb.Create[Item](ctx, db, "inventory", map[string]any{
		"name":     "Screwdriver",
		"quantity": 50,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create item: %v", err))
	}

	_, err = surrealdb.Update[Item](ctx, db, *item.ID, map[string]any{
		"quantity": 45,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to update item: %v", err))
	}

	_, err = surrealdb.Delete[Item](ctx, db, *item.ID)
	if err != nil {
		panic(fmt.Sprintf("Failed to delete item: %v", err))
	}

	// Wait for all expected notifications to be received
	select {
	case <-received:
		// All notifications received
	case <-time.After(2 * time.Second):
		panic("Timeout waiting for all notifications")
	}

	err = surrealdb.Kill(ctx, db, live.String())
	if err != nil {
		panic(fmt.Sprintf("Failed to kill live query: %v", err))
	}

	fmt.Println("Live query with diff terminated")

	select {
	case <-done:
		fmt.Println("Goroutine exited after channel closed")
	case <-time.After(2 * time.Second):
		panic("Timeout: notification channel was not closed after Kill")
	}

	// Output:
	// Started live query with diff enabled
	// Action: CREATE, Result: [{op=replace path=/ value={id=inventory:⟨UUID⟩ name=Screwdriver quantity=50}}]
	// Action: UPDATE, Result: [{op=remove path=/name} {op=replace path=/quantity value=45}]
	// Action: DELETE, Result: {id=inventory:⟨UUID⟩ quantity=45}
	// Live query with diff terminated
	// Notification channel closed
	// Goroutine exited after channel closed
}
