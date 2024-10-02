package surrealdb

// Patch represents a patch object set to MODIFY a record
type PatchData struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type QueryResult[T any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result []T    `json:"result"`
}

type QueryStatement[TResult any] struct {
	SQL  string
	Vars map[string]interface{}
}
