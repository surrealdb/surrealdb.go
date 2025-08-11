package surrealql

import (
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Thing creates a target for a SurrealQL query.
func Thing(tb string, id any) *models.RecordID {
	r := models.NewRecordID(tb, id)
	return &r
}

// Table creates a target for a SurrealQL query with a specified table name.
func Table(tb string) models.Table {
	if tb == "" {
		panic("table name cannot be empty")
	}
	return models.Table(tb)
}
