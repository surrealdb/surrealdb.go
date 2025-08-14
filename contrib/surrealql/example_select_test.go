package surrealql_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect demonstrates basic usage of the SELECT query builder.
//
// This example shows how to create a simple SELECT query and execute it with surrealdb.Query.
func ExampleSelect() {
	// Simple SELECT query
	query1 := surrealql.Select("users")
	sql1, vars1 := query1.Build()
	fmt.Println("SurrealQL:", sql1)
	fmt.Println("Vars:", vars1)

	// SELECT all users with id and name
	query2 := surrealql.Select("users").Fields("id", "name")
	sql2, vars2 := query2.Build()

	fmt.Println("SurrealQL:", sql2)
	fmt.Println("Vars:", vars2)

	// To execute with surrealdb.Query:
	// results, err := surrealdb.Query[User](ctx, db, sql, vars)

	// Output:
	// SurrealQL: SELECT * FROM users
	// Vars: map[]
	// SurrealQL: SELECT id, name FROM users
	// Vars: map[]
}

// ExampleSelect_omit demonstrates how to use the OMIT clause in a SELECT query.
func ExampleSelect_omit() {
	// Select all fields except 'password' and 'created_at'
	query := surrealql.Select("users").
		Omit("password").
		Omit("created_at")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT * OMIT password, created_at FROM users
	// Vars: map[]
}

// ExampleSelect_omitRaw demonstrates how to use the OMIT clause with a raw field.
func ExampleSelect_omitRaw_destructuring() {
	// Select all fields except a raw field
	query := surrealql.Select("users").
		Omit("password").
		OmitRaw("opts.{ security, enabled }")

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT * OMIT password, opts.{ security, enabled } FROM users
	// Vars: map[]
}

// ExampleSelect_fieldName demonstrates how to add a field to the SELECT query.
func ExampleSelect_fieldName() {
	// Select specific fields from users
	query := surrealql.Select("users").
		Fields("id").
		FieldName("name").
		FieldName("email")
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT id, name, email FROM users
	// Vars: map[]
}

func ExampleSelect_fieldNameAsAlias() {
	// Select specific fields with an alias
	query := surrealql.Select("users").
		Fields("id").
		FieldNameAs("name", "username").
		FieldName("email")
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT id, name AS username, email FROM users
	// Vars: map[]
}

// ExampleSelect_fieldRaw demonstrates how to add a raw field to the SELECT query.
func ExampleSelect_fieldRaw() {
	// Select specific fields with a raw field
	query := surrealql.Select("users").
		Fields("id").
		FieldName("name").
		Field("name + ' <' + email + '>' AS contact")
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT id, name, name + ' <' + email + '>' AS contact FROM users
	// Vars: map[]
}

func ExampleSelect_field() {
	// Add fields to the SELECT query
	query := surrealql.Select("users").
		Fields("id", "name").
		Field(surrealql.Expr("email"))
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT id, name, email FROM users
	// Vars: map[]
}

func ExampleSelect_field_fieldWithAlias() {
	// Add fields with alias to the SELECT query
	query := surrealql.Select("users").
		Fields("id", "name").
		Field(surrealql.Expr("email").As("contact"))
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT id, name, email AS contact FROM users
	// Vars: map[]
}

func ExampleSelect_field_queryWithAlias() {
	// Add a subquery with alias to the SELECT query
	subQuery := surrealql.Select("orders").
		Fields("id", "total").
		Where("user_id = $parent.id")
	query := surrealql.Select("users").
		Fields("id", "name").
		Field(surrealql.Expr(subQuery).As("orders"))
	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	fmt.Printf("Vars: %v\n", vars)

	// Output:
	// SurrealQL: SELECT id, name, (SELECT id, total FROM orders WHERE user_id = $parent.id) AS orders FROM users
	// Vars: map[]
}

func ExampleSelect_withConditions() {
	// Select active users with email
	query := surrealql.Select("users").
		Fields("id", "name", "email").
		WhereEq("active", true).
		WhereNotNull("email").
		OrderByDesc("created_at").
		Limit(10)

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT id, name, email FROM users WHERE type::field($param_1) = $param_2 AND type::field($param_3) IS NOT NULL ORDER BY created_at DESC LIMIT 10
	// Vars:
	//   param_1: active
	//   param_2: true
	//   param_3: email
}

func ExampleSelect_withF() {
	// Select users with a specific condition using F
	query := surrealql.Select("users").
		Fields(surrealql.Expr("id"), surrealql.Expr("name")).
		WhereEq("active", true).
		OrderBy("created_at").
		Limit(5)

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT id, name FROM users WHERE type::field($param_1) = $param_2 ORDER BY created_at LIMIT 5
	// Vars:
	//   param_1: active
	//   param_2: true
}

func ExampleSelect_withF_query() {
	// Select users and their orders
	query := surrealql.Select("users").
		Fields(
			surrealql.Expr("id"),
			surrealql.Expr("name"),
			surrealql.Expr(
				// $parent is a predefined variable.
				// See https://surrealdb.com/docs/surrealql/statements/select#using-parameters
				surrealql.Select("orders").Fields("id", "total").Where("user_id = $parent.id"),
			).As("orders"),
		).
		WhereEq("active", true).
		OrderBy("created_at").
		Limit(5)

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)
	// Output:
	// SurrealQL: SELECT id, name, (SELECT id, total FROM orders WHERE user_id = $parent.id) AS orders FROM users WHERE type::field($param_1) = $param_2 ORDER BY created_at LIMIT 5
	// Vars:
	//   param_1: active
	//   param_2: true
}

func ExampleSelect_fromTarget_recordID() {
	// Select users from a specific target
	query := surrealql.Select(surrealql.Thing("users", "123"))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: users:⟨123⟩
}

func ExampleSelect_fromTarget_table() {
	// Select users from a specific target
	query := surrealql.Select(surrealql.Table("users"))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT * FROM $from_table_1
	// Vars:
	//   from_table_1: users
}

func ExampleSelect_fromRecordID_intID() {
	// Select a user by RecordID
	query := surrealql.Select(models.NewRecordID("users", 123))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {users 123}
}

func ExampleSelect_fromRecordID_intLikeStringID() {
	// Select a user by RecordID
	query := surrealql.Select(models.NewRecordID("users", "123"))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {users 123}
}

func ExampleSelect_fromRecordID_stringID() {
	// Select a user by RecordID
	query := surrealql.Select(models.NewRecordID("users", "abc"))

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT * FROM $from_id_1
	// Vars:
	//   from_id_1: {users abc}
}

func ExampleSelect_fromQuery() {
	// Select users from a subquery
	subQuery := surrealql.Select(models.Table("users")).Fields("id", "name").WhereEq("active", true)
	query := surrealql.Select(subQuery)

	sql, vars := query.Build()
	fmt.Println("SurrealQL:", sql)
	dumpVars(vars)

	// Output:
	// SurrealQL: SELECT * FROM (SELECT id, name FROM $from_from_table_1 WHERE type::field($from_param_1) = $from_param_2)
	// Vars:
	//   from_from_table_1: users
	//   from_param_1: active
	//   from_param_2: true
}

func ExampleSelect_integration() {
	// This example shows how to use the query builder with surrealdb.Query

	// Assume we have a *surrealdb.DB instance
	var db *surrealdb.DB

	db, err := testenv.New("surrealqlexamples", "test", "users")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	create := surrealql.Create("users").
		Set("id", "123").
		Set("name", "Test Item").
		Set("email", "test@example.com").
		Set("active", true).
		Set("created_at", time.Now()).
		ReturnNone()

	createQuery, createParams := create.Build()
	_, err = surrealdb.Query[any](ctx, db, createQuery, createParams)
	if err != nil {
		log.Fatal(err)
	}

	// Build a query to find active users
	query := surrealql.Select("users").
		Fields("id", "name", "email").
		WhereEq("active", true).
		OrderBy("name").
		Limit(100)

	// Get the SurrealQL and parameters
	ql, vars := query.Build()

	// Execute the query
	type User struct {
		ID    models.RecordID `json:"id"`
		Name  string          `json:"name"`
		Email string          `json:"email"`
	}

	results, err := surrealdb.Query[[]User](ctx, db, ql, vars)
	if err != nil {
		log.Fatal(err)
	}

	// Process results
	for _, result := range *results {
		if result.Status == surrealql.StatusOK {
			fmt.Printf("Users: %+v\n", result.Result)
		}
	}

	// Output:
	// Users: [{ID:{Table:users ID:123} Name:Test Item Email:test@example.com}]
}
