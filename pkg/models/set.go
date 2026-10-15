package models

// Set is a SurrealDB set: a collection of distinct values, encoded as CBOR
// tag 56 holding an array.
//
// The server keeps a set ordered and deduplicated, and sorts and deduplicates
// whatever it is sent, so a Set need not be sent sorted. Sets decoded into an
// interface{} become a Set rather than a []any, so that a set read and written
// back stays a set. A set can also be decoded into a slice of a concrete type.
//
// See https://surrealdb.com/docs/surrealql/datamodel/sets.
type Set []any

// NewSet returns a Set of the given values.
func NewSet(values ...any) Set {
	return Set(values)
}
