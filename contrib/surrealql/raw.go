package surrealql

// Helper function to create raw queries
func Raw(sql string, params map[string]any) Query {
	return &rawQuery{
		sql:    sql,
		params: params,
	}
}

type rawQuery struct {
	sql    string
	params map[string]any
}

func (q *rawQuery) Build() (sql string, params map[string]any) {
	return q.sql, q.params
}

func (q *rawQuery) String() string {
	return q.sql
}
