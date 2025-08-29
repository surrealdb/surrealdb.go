package surrealdb_test

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
)

// ExampleQuery_changeFeedSchemaless demonstrates how to use Change Feeds in SurrealDB
// to track changes made to database records.
//
//nolint:gocyclo
func ExampleQuery_changeFeedSchemaless() {
	// Connect to database
	db := testenv.MustNew("surrealdbexamples", "changefeed", "product")

	ctx := context.Background()

	// Enable change feed on the database
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE product CHANGEFEED 1h", nil)
	if err != nil {
		panic(err)
	}

	// Make some changes to generate change feed entries
	_, err = surrealdb.Query[any](ctx, db, `
		CREATE product:1 SET name = "Laptop", price = 999.99;
		UPDATE product:1 SET price = 899.99;
		CREATE product:2 SET name = "Mouse", price = 29.99;
		DELETE product:2;
	`, nil)
	if err != nil {
		panic(err)
	}

	type ChangeDefineTable struct {
		Name string `json:"name"`
	}

	// Change represents a change to a table in the database.
	type Change struct {
		// DefineTable represents the definition of a new table in the database.
		// It has Name and nothing else.
		DefineTable *ChangeDefineTable `json:"define_table"`
		// Update represents an update to a table in the database.
		// Note that this may represent a new record being created.
		// In case of an update, the "id" field must be present,
		// and all other fields including unchanged fields must be included.
		Update map[string]any `json:"update"`
		// Delete represents a deletion from a table in the database.
		Delete map[string]any `json:"delete"`
	}

	// ChangeSet represents a set of changes in the database.
	type ChangeSet struct {
		// Versionstamp is a unique identifier for the change set.
		// It is unique per database.
		Versionstamp uint64 `json:"versionstamp"`
		// Changes is a list of changes made in the database.
		// It may contain one or more table changes,
		// each represented as a map of field names to their new values.
		Changes []Change `json:"changes"`
	}

	result, err := surrealdb.Query[[]ChangeSet](ctx, db, "SHOW CHANGES FOR TABLE product SINCE 0", nil)
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
	var defineCount, updateCount, deleteCount int
	for _, changeSet := range showChangesResult {
		for _, change := range changeSet.Changes {
			if change.DefineTable != nil {
				defineCount++
			}
			if change.Update != nil {
				updateCount++
			}
			if change.Delete != nil {
				deleteCount++
			}
		}
	}

	if defineCount > 0 && updateCount > 0 && deleteCount > 0 {
		fmt.Println("Found change entries: defines, updates, and deletes")
	}

	// Show the pattern of the last few changes with actual data
	if len(showChangesResult) >= 5 {
		lastFive := showChangesResult[len(showChangesResult)-5:]
		fmt.Println("Last 5 changes:")
		for _, changeSet := range lastFive {
			for _, change := range changeSet.Changes {
				if change.DefineTable != nil {
					fmt.Printf("  DefineTable: %s\n", change.DefineTable.Name)
				}
				if change.Update != nil {
					// Extract key fields from the update
					if id, ok := change.Update["id"]; ok {
						if name, hasName := change.Update["name"]; hasName {
							if price, hasPrice := change.Update["price"]; hasPrice {
								fmt.Printf("  Update: id=%v, name=%v, price=%v\n", id, name, price)
							} else {
								fmt.Printf("  Update: id=%v, name=%v\n", id, name)
							}
						} else if price, hasPrice := change.Update["price"]; hasPrice {
							fmt.Printf("  Update: id=%v, price=%v\n", id, price)
						} else {
							fmt.Printf("  Update: id=%v\n", id)
						}
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
	// Found change entries: defines, updates, and deletes
	// Last 5 changes:
	//   DefineTable: product
	//   Update: id={product 1}, name=Laptop, price=999.99
	//   Update: id={product 1}, name=Laptop, price=899.99
	//   Update: id={product 2}, name=Mouse, price=29.99
	//   Delete: id={product 2}
}
