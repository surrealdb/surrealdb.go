package surrealdb

// Patch represents a patch object set to MODIFY a record
type Patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}
