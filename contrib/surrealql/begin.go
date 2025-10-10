package surrealql

// Begin creates a new transaction query
//
// See [TransactionQuery] for more details.
func Begin() *TransactionQuery {
	q := &TransactionQuery{
		StatementsBuilder: &StatementsBuilder[TransactionQuery]{},
	}

	q.self = q

	return q
}
