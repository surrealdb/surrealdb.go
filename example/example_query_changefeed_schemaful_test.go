package main

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// ExampleQuery_changeFeedSchemaful demonstrates how to use Change Feeds with a schemaful table in SurrealDB.
// This example shows how to define a table with schema enforcement, define required fields,
// and track changes made to records that must conform to the schema.
//
//nolint:gocyclo
func ExampleQuery_changeFeedSchemaful() {
	// Connect to database
	db := testenv.MustNew("surrealdbexamples", "changefeed_schemaful", "inventory")

	ctx := context.Background()

	// Define a schemaful table with change feed enabled
	// SCHEMAFULL enforces that all records must have the defined fields
	_, err := surrealdb.Query[any](ctx, db, `
		DEFINE TABLE inventory SCHEMAFULL CHANGEFEED 1h;
	`, nil)
	if err != nil {
		panic(err)
	}

	// Note that DEFINE FIELD statements are not tracked by the change feed,
	// although DEFINE TABLE statements are tracked.
	//
	// In schemaful tables:
	// - TYPE string means the field is required (cannot be none or null)
	// - TYPE option<string> means the field can be none
	// - TYPE string | null means the field can be null
	//
	// In change feeds:
	// - `null` fields appear as `null`
	// - `none` fields do not appear at all
	_, err = surrealdb.Query[any](ctx, db, `
		DEFINE FIELD sku ON TABLE inventory TYPE string;
		DEFINE FIELD name ON TABLE inventory TYPE string;
		DEFINE FIELD price ON TABLE inventory TYPE number ASSERT $value >= 0;
		DEFINE FIELD quantity ON TABLE inventory TYPE int ASSERT $value >= 0;
		DEFINE FIELD active ON TABLE inventory TYPE bool DEFAULT true;
		DEFINE FIELD notes ON TABLE inventory TYPE option<string>;
	`, nil)
	if err != nil {
		panic(err)
	}

	// Make some changes to generate change feed entries
	// All operations must comply with the schema
	_, err = surrealdb.Query[any](ctx, db, `
		CREATE inventory:item1 SET
			sku = "SKU001",
			name = "Wireless Mouse",
			price = 29.99,
			quantity = 100,
			active = true,
			notes = "Best seller";

		UPDATE inventory:item1 SET quantity = 95;

		CREATE inventory:item2 SET
			sku = "SKU002",
			name = "USB Cable",
			price = 9.99,
			quantity = 250,
			active = true;
		-- notes is optional, so we can omit it when
		-- creating and updating records

		UPDATE inventory:item2 SET active = false;

		DELETE inventory:item2;
	`, nil)
	if err != nil {
		panic(err)
	}

	type ChangeDefineTable struct {
		Name string `json:"name"`
	}

	type ChangeDefineField struct {
		Name  string `json:"name"`
		What  string `json:"what"`
		Table string `json:"table"`
	}

	// Change represents a change to a table in the database.
	type Change struct {
		// DefineTable represents the definition of a new table in the database.
		DefineTable *ChangeDefineTable `json:"define_table"`
		// DefineField represents the definition of a new field in the table.
		DefineField *ChangeDefineField `json:"define_field"`
		// Update represents an update to a table in the database.
		// Note that in schemaful tables, all defined fields are present.
		Update map[string]any `json:"update"`
		// Delete represents a deletion from a table in the database.
		Delete map[string]any `json:"delete"`
	}

	// ChangeSet represents a set of changes in the database.
	type ChangeSet struct {
		// Versionstamp is a unique identifier for the change set.
		Versionstamp uint64 `json:"versionstamp"`
		// Changes is a list of changes made in the database.
		Changes []Change `json:"changes"`
	}

	result, err := surrealdb.Query[[]ChangeSet](ctx, db, "SHOW CHANGES FOR TABLE inventory SINCE 0", nil)
	if err != nil {
		panic(err)
	}

	showChangesResult := (*result)[0].Result

	// Verify versionstamps are monotonic
	monotonic := true
	for i := 1; i < len(showChangesResult); i++ {
		if showChangesResult[i].Versionstamp <= showChangesResult[i-1].Versionstamp {
			monotonic = false
			break
		}
	}
	fmt.Printf("Versionstamps are monotonic: %v\n", monotonic)

	// Count different types of changes
	var defineTableCount, defineFieldCount, updateCount, deleteCount int
	for _, changeSet := range showChangesResult {
		for _, change := range changeSet.Changes {
			if change.DefineTable != nil {
				defineTableCount++
			}
			if change.DefineField != nil {
				defineFieldCount++
			}
			if change.Update != nil {
				updateCount++
			}
			if change.Delete != nil {
				deleteCount++
			}
		}
	}

	if defineTableCount > 0 && updateCount > 0 && deleteCount > 0 {
		fmt.Println("Found change entries: table definitions, updates, and deletes")
	}
	if defineFieldCount > 0 {
		fmt.Printf("Field definitions tracked: %d\n", defineFieldCount)
	}

	// Show the last few changes with actual data
	// Find the last table definition to show only recent changes
	lastTableDefIndex := -1
	for i := len(showChangesResult) - 1; i >= 0; i-- {
		for _, change := range showChangesResult[i].Changes {
			if change.DefineTable != nil {
				lastTableDefIndex = i
				break
			}
		}
		if lastTableDefIndex >= 0 {
			break
		}
	}

	if lastTableDefIndex >= 0 {
		// Show changes from the last table definition onwards
		recentChanges := showChangesResult[lastTableDefIndex:]

		// Limit to last 10 changes for readability
		startIndex := 0
		if len(recentChanges) > 10 {
			startIndex = len(recentChanges) - 10
		}

		fmt.Printf("Last %d changes:\n", len(recentChanges[startIndex:]))
		for _, changeSet := range recentChanges[startIndex:] {
			for _, change := range changeSet.Changes {
				if change.DefineTable != nil {
					fmt.Printf("  DefineTable: %s\n", change.DefineTable.Name)
				}
				if change.DefineField != nil {
					fmt.Printf("  DefineField: %s on %s\n", change.DefineField.Name, change.DefineField.Table)
				}
				if change.Update != nil {
					// Extract fields from the update (all fields present in schemaful table)
					if id, ok := change.Update["id"]; ok {
						sku := change.Update["sku"]
						name := change.Update["name"]
						price := change.Update["price"]
						quantity := change.Update["quantity"]
						active := change.Update["active"]
						notes, ok := change.Update["notes"]
						var notesStr string
						if !ok {
							notesStr = "<none>"
						} else if notes == nil {
							notesStr = "<null>"
						} else {
							notesStr = fmt.Sprintf("%v", notes)
						}
						fmt.Printf("  Update: id=%v, sku=%v, name=%v, price=%v, quantity=%v, active=%v, notes=%v\n",
							id, sku, name, price, quantity, active, notesStr)
					} else {
						fmt.Printf("  Update: %v\n", change.Update)
					}
				}
				if change.Delete != nil {
					// Extract id from the delete
					if id, ok := change.Delete["id"]; ok {
						fmt.Printf("  Delete: id=%v\n", id)
					} else {
						fmt.Printf("  Delete: %v\n", change.Delete)
					}
				}
			}
		}
	}

	// Output:
	// Versionstamps are monotonic: true
	// Found change entries: table definitions, updates, and deletes
	// Last 6 changes:
	//   DefineTable: inventory
	//   Update: id={inventory item1}, sku=SKU001, name=Wireless Mouse, price=29.99, quantity=100, active=true, notes=Best seller
	//   Update: id={inventory item1}, sku=SKU001, name=Wireless Mouse, price=29.99, quantity=95, active=true, notes=Best seller
	//   Update: id={inventory item2}, sku=SKU002, name=USB Cable, price=9.99, quantity=250, active=true, notes=<none>
	//   Update: id={inventory item2}, sku=SKU002, name=USB Cable, price=9.99, quantity=250, active=false, notes=<none>
	//   Delete: id={inventory item2}
}
