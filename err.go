package surrealdb

import (
	"fmt"
)

type PermissionError struct {
	what string
}

func (self PermissionError) Error() string {
	return fmt.Sprint("Unable to access record:", self.what)
}
