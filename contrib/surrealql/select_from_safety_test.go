package surrealql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestSelectFrom_Safety verifies that placeholder replacement is safe
func TestSelectFrom_Safety(t *testing.T) {
	t.Run("record_id_preserved", func(t *testing.T) {
		// Record IDs become parameters when using placeholder
		sql, vars := surrealql.SelectFrom("?", "users:123").Build()
		assert.Equal(t, "SELECT * FROM $from_param_1", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.Equal(t, "users:123", vars["from_param_1"])
	})

	t.Run("first_position_parameter", func(t *testing.T) {
		// RecordID at first position becomes a parameter
		recordID := models.NewRecordID("users", "admin")
		sql, vars := surrealql.SelectFrom("?->manages->projects", recordID).Build()
		assert.Equal(t, "SELECT * FROM $from_param_1->manages->projects", sql)
		assert.Contains(t, vars, "from_param_1")
		assert.IsType(t, models.RecordID{}, vars["from_param_1"])
	})
}
