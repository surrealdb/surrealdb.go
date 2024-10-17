package models

import "fmt"

type Future struct {
	inner string
}

func (f *Future) String() string {
	return fmt.Sprintf("<future> { %s }", f.inner)
}
