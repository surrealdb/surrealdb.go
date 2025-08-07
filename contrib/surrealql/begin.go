package surrealql

// Begin creates a new transaction query
func Begin() *TransactionQuery {
	return &TransactionQuery{
		baseQuery:  newBaseQuery(),
		statements: []TransactionStatement{},
	}
}
