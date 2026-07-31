package surrealql

import (
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Thing creates a target for a SurrealQL query.
func Thing(tb string, id any) *models.RecordID {
	r := models.NewRecordID(tb, id)
	return &r
}

// RecordRange selects or updates a range of records in a table.
//
// begin and end are usually models.Included(...), models.Excluded(...),
// or nil when that side of the range has no limit.
//
// Example — all temp records whose id starts with "London":
//
//	surrealql.Select(surrealql.RecordRange("temp",
//	    models.IncludedMany[any]("London", models.None),
//	    models.IncludedMany[any]("London", models.OpenRange()),
//	))
func RecordRange(tb string, begin, end any) models.RecordRange {
	return models.NewRecordRange(tb, begin, end)
}

// Table creates a target for a SurrealQL query with a specified table name.
func Table(tb string) models.Table {
	if tb == "" {
		panic("table name cannot be empty")
	}
	return models.Table(tb)
}
