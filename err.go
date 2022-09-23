package surrealdb

import (
	"fmt"
)

type PermissionError struct {
	what string
}

func (pe PermissionError) Error() string {
	return fmt.Sprint("Unable to access record:", pe.what)
}
