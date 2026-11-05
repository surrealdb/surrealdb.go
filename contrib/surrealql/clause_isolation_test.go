package surrealql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/surrealdb/surrealdb.go/v2/contrib/surrealql"
)

// Caller text in one clause cannot reach the clauses after it.
func TestClauses_keepCallerTextInPlace(t *testing.T) {
	tests := []struct {
		name  string
		query surrealql.Query
		want  string
	}{
		{
			"a line comment in a condition ends before the closing parenthesis",
			surrealql.Select("item").Where("n >= ? -- at least", 1).Where("tenant = ?", "a").OrderBy("n").Limit(1),
			"SELECT * FROM item WHERE (n >= $param_1 -- at least\n) AND (tenant = $param_2) ORDER BY n LIMIT 1",
		},
		{
			"an OR in one condition does not take in the others",
			surrealql.Delete("item").Where("status = ? OR n = ?", "x", 99).Where("tenant = ?", "a"),
			"DELETE item WHERE (status = $param_1 OR n = $param_2) AND (tenant = $param_3)",
		},
		{
			"a ; in a condition stays inside its parentheses",
			surrealql.Delete("item").Where("n = 0; CREATE pwned"),
			"DELETE item WHERE (n = 0; CREATE pwned)",
		},
		{
			"a line comment in a SET expression ends before the WHERE",
			surrealql.Update("item").Set("status = 'z' -- mark").Where("tenant = ?", "a"),
			"UPDATE item SET status = 'z' -- mark\n WHERE (tenant = $param_1)",
		},
		{
			"field names are quoted unless plain",
			surrealql.Select("item").FieldName("none").FieldName("address.city").OrderBy("n;CREATE/**/pwned").Fetch("author.true"),
			"SELECT `none`, address.city FROM item ORDER BY `n;CREATE/**/pwned` FETCH author.`true`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _ := tt.query.Build()
			assert.Equal(t, tt.want, sql)
		})
	}
}
