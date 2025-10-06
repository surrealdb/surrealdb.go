package surrealql

// Begin creates a new transaction query
func Begin() *TransactionQuery {
	q := &TransactionQuery{
		StatementsBuilder: &StatementsBuilder[TransactionQuery]{},
	}

	q.self = q

	return q
}
