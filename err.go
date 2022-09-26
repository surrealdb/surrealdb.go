package surrealdb

type PermissionError struct {
	what string
}

func (pe PermissionError) Error() string {
	return "Unable to access record: " + pe.what
}
