package models_test

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleIncluded shows an inclusive bound with a single value.
func ExampleIncluded() {
	fmt.Println(models.Included(1).Value)
	// Output:
	// 1
}

// ExampleIncludedMany shows inclusive bounds built from several parts.
func ExampleIncludedMany() {
	fmt.Println(models.IncludedMany(1, 2).Value)
	fmt.Println(models.IncludedMany[any]("London", models.None).Value)
	// Output:
	// [1 2]
	// [London {}]
}

// ExampleExcluded shows an exclusive bound with a single value.
func ExampleExcluded() {
	fmt.Println(models.Excluded("z").Value)
	// Output:
	// z
}

// ExampleExcludedMany shows an exclusive bound built from several parts.
func ExampleExcludedMany() {
	fmt.Println(models.ExcludedMany(1, 2).Value)
	// Output:
	// [1 2]
}
