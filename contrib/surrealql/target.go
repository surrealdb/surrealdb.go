package surrealql

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Thing creates a target for a SurrealQL query.
func Thing(tb string, id any) *target {
	return &target{table: tb, id: id}
}

// Table creates a target for a SurrealQL query with a specified table name.
func Table(tb string) *target {
	if tb == "" {
		panic("table name cannot be empty")
	}
	return &target{table: tb}
}

// mutationTarget is an interface for targets in mutation queries like CREATE, UPDATE, DELETE, etc.
type mutationTarget interface {
	string | *target | *models.RecordID
}

// The `f` parameter needs to be targetType.
// If not, it will panic.
func buildTargetExpr(f any) (sql string, vars map[string]any) {
	if f == nil {
		return "*", nil // Default to selecting all fields
	}

	switch v := f.(type) {
	case string:
		return v, nil
	case *target:
		return v.Build()
	case *models.RecordID:
		return buildTarget(v.Table, v.ID)
	default:
		panic(fmt.Sprintf("unsupported select field type: %T", f))
	}
}

// target represents a target in a SurrealQL query.
// It appears as an item in @targets of `UPDATE @targets`,
// `CREATE @targets`, `DELETE @targets`, `SELECT * FROM @target`, and so on.
type target struct {
	table string
	id    any
}

func (t *target) Build() (sql string, vars map[string]any) {
	return buildTarget(t.table, t.id)
}

func buildTarget(table string, id any) (sql string, vars map[string]any) {
	if table == "" {
		panic("target table cannot be empty")
	}
	var bq baseQuery
	if id == nil {
		tb := bq.generateParamName("tb")
		return fmt.Sprintf("type::table($%s)", tb), map[string]any{tb: table}
	}

	idParam := bq.generateParamName("id")
	return fmt.Sprintf("$%s", idParam), map[string]any{idParam: models.NewRecordID(table, id)}
}
