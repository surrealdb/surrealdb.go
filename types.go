package surrealdb

import "github.com/surrealdb/surrealdb.go/v2/pkg/models"

// Patch represents a patch object set to MODIFY a record
type PatchData struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type QueryResult[T any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result T      `json:"result"`
}

type QueryStatement[TResult any] struct {
	SQL  string
	Vars map[string]interface{}
}

type Relation[T any] struct {
	ID  string          `json:"id"`
	In  models.RecordID `json:"in"`
	Out models.RecordID `json:"out"`
}

// Auth is a struct that holds surrealdb auth data for login.
type Auth struct {
	Namespace string `json:"NS,omitempty"`
	Database  string `json:"DB,omitempty"`
	Scope     string `json:"SC,omitempty"`
	Username  string `json:"user,omitempty"`
	Password  string `json:"pass,omitempty"`
}

type H map[string]interface{}

type Result[T any] struct {
	T any
}
