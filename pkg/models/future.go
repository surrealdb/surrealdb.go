package models

import "fmt"

type Future struct {
	inner string
}

func (f *Future) String() string {
	return f.inner
}

func (f *Future) SurrealString() string {
	return fmt.Sprintf("<future> { %s }", f.String())
}
