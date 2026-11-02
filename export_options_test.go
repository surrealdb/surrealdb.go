package surrealdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportOptions_sendsOnlyWhatDiffersFromTheDefaults(t *testing.T) {
	var none *ExportOptions
	assert.Empty(t, none.config())
	assert.Empty(t, (&ExportOptions{}).config())

	assert.Equal(t, map[string]any{
		"users":   false,
		"records": false,
		"tables":  []string{"a", "b"},
	}, (&ExportOptions{
		ExcludeUsers:   true,
		ExcludeRecords: true,
		Tables:         []string{"a", "b"},
	}).config())
}
