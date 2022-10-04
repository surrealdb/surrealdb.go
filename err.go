package surrealdb

import (
	"fmt"
)

type PermissionError struct {
	what string
}

func (err PermissionError) Error() string {
	return fmt.Sprint("Unable to access record:", err.what)
}
