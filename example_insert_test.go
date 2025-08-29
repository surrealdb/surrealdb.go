package surrealdb_test

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleInsert_table() {
	db := testenv.MustNew("surrealdbexamples", "query", "persons")

	type Person struct {
		Name string `json:"name"`
		// Note that you must use CustomDateTime instead of time.Time.
		CreatedAt models.CustomDateTime  `json:"created_at,omitempty"`
		UpdatedAt *models.CustomDateTime `json:"updated_at,omitempty"`
	}

	createdAt, err := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	if err != nil {
		panic(err)
	}

	// Unlike Create which returns a pointer to the record itself,
	// Insert returns a pointer to the array of inserted records.
	var inserted *[]Person
	inserted, err = surrealdb.Insert[Person](
		context.Background(),
		db,
		"persons",
		map[string]any{
			"name":       "First",
			"created_at": createdAt,
		})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Insert result: %v\n", *inserted)

	_, err = surrealdb.Insert[struct{}](
		context.Background(),
		db,
		"persons",
		map[string]any{
			"name":       "Second",
			"created_at": createdAt,
		},
	)
	if err != nil {
		panic(err)
	}

	_, err = surrealdb.Insert[struct{}](
		context.Background(),
		db,
		"persons",
		Person{
			Name: "Third",
			CreatedAt: models.CustomDateTime{
				Time: createdAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}

	fourthAsMap, err := surrealdb.Insert[map[string]any](
		context.Background(),
		db,
		"persons",
		Person{
			Name: "Fourth",
			CreatedAt: models.CustomDateTime{
				Time: createdAt,
			},
		},
	)
	if err != nil {
		panic(err)
	}
	if _, ok := (*fourthAsMap)[0]["id"].(models.RecordID); ok {
		delete((*fourthAsMap)[0], "id")
	}
	fmt.Printf("Insert result: %v\n", *fourthAsMap)

	selected, err := surrealdb.Select[[]Person](
		context.Background(),
		db,
		"persons",
	)
	if err != nil {
		panic(err)
	}
	for _, person := range *selected {
		fmt.Printf("Selected person: %v\n", person)
	}

	// Unordered output:
	// Insert result: [{First {2023-10-01 12:00:00 +0000 UTC} <nil>}]
	// Insert result: [map[created_at:{2023-10-01 12:00:00 +0000 UTC} name:Fourth]]
	// Selected person: {First {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Second {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Third {2023-10-01 12:00:00 +0000 UTC} <nil>}
	// Selected person: {Fourth {2023-10-01 12:00:00 +0000 UTC} <nil>}
}

func ExampleInsert_bulk_insert_record() {
	db := testenv.MustNew("surrealdbexamples", "query", "person")

	type Person struct {
		ID models.RecordID `json:"id"`
	}

	persons := []Person{
		{ID: models.NewRecordID("person", "a")},
		{ID: models.NewRecordID("person", "b")},
		{ID: models.NewRecordID("person", "c")},
	}

	var inserted *[]Person
	inserted, err := surrealdb.Insert[Person](
		context.Background(),
		db,
		"person",
		persons,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Inserted: %+s\n", *inserted)

	selected, err := surrealdb.Select[[]Person](
		context.Background(),
		db,
		"person",
	)
	if err != nil {
		panic(err)
	}
	for _, person := range *selected {
		fmt.Printf("Selected person: %+s\n", person)
	}

	// Output:
	// Inserted: [{{person a}} {{person b}} {{person c}}]
	// Selected person: {{person a}}
	// Selected person: {{person b}}
	// Selected person: {{person c}}
}

func ExampleInsert_bulk_insert_relation_workaround_for_rpcv1() {
	db := testenv.MustNew("surrealdbexamples", "query", "person", "follow")

	type Person struct {
		ID models.RecordID `json:"id"`
	}

	type Follow struct {
		ID  models.RecordID `json:"id"`
		In  models.RecordID `json:"in"`
		Out models.RecordID `json:"out"`
	}

	persons := []Person{
		{ID: models.NewRecordID("person", "a")},
		{ID: models.NewRecordID("person", "b")},
		{ID: models.NewRecordID("person", "c")},
	}

	follows := []Follow{
		{ID: models.NewRecordID("follow", "person:a:person:b"), In: persons[0].ID, Out: persons[1].ID},
		{ID: models.NewRecordID("follow", "person:b:person:c"), In: persons[1].ID, Out: persons[2].ID},
		{ID: models.NewRecordID("follow", "person:c:person:a"), In: persons[2].ID, Out: persons[0].ID},
	}

	var err error

	var insertedPersons *[]Person
	insertedPersons, err = surrealdb.Insert[Person](
		context.Background(),
		db,
		"person",
		persons,
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Inserted: %+s\n", *insertedPersons)

	var selectedPersons *[]Person
	selectedPersons, err = surrealdb.Select[[]Person](
		context.Background(),
		db,
		"person",
	)
	if err != nil {
		panic(err)
	}
	for _, person := range *selectedPersons {
		fmt.Printf("Selected person: %+s\n", person)
	}

	/// Once the RPC v2 becomes mature, we could update this SDK to speak
	/// the RPC v2 protocol and use the `relation` parameter to insert
	/// the follows as relations.
	///
	/// But as of now, it will fail like SurrealDB responding with:
	///
	///   There was a problem with the database: The database encountered unreachable logic: /surrealdb/crates/core/src/expr/statements/insert.rs:123: Unknown data clause type in INSERT statement: ContentExpression(Array(Array([Object(Object({"id": Thing(Thing { tb: "follow", id: String("person:a:person:b") }), "in": Thing(Thing { tb: "person", id: String("a") }), "out": Thing(Thing { tb: "person", id: String("b") })})), Object(Object({"id": Thing(Thing { tb: "follow", id: String("person:b:person:c") }), "in": Thing(Thing { tb: "person", id: String("b") }), "out": Thing(Thing { tb: "person", id: String("c") })})), Object(Object({"id": Thing(Thing { tb: "follow", id: String("person:c:person:a") }), "in": Thing(Thing { tb: "person", id: String("c") }), "out": Thing(Thing { tb: "person", id: String("a") })}))])))
	///
	// var insertedFollows *[]Follow
	// insertedFollows, err = surrealdb.Insert[Follow](
	// 	db,
	// 	"follow",
	// 	follows,
	// 	map[string]any{
	// 		// The optional `relation` parameter is a boolean indicating whether the inserted records are relations.
	// 		// See https://surrealdb.com/docs/surrealdb/integration/rpc#parameters-7
	// 		"relation": true,
	// 	},
	// )
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("Inserted: %+s\n", *insertedFollows)

	/// You can also use `InsertRelation`.
	/// But refer to ExampleInsertRelation for that.
	// for _, follow := range follows {
	// 	err = surrealdb.InsertRelation(
	// 		db,
	// 		&surrealdb.Relationship{
	// 			Relation: "follow",
	// 			ID:       &follow.ID,
	// 			In:       follow.In,
	// 			Out:      follow.Out,
	// 		},
	// 	)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }

	// Here, we focus on what you could do the equivalent of
	// batch insert relation in RPC v2, using the RPC v1 query RPC.
	_, err = surrealdb.Query[any](
		context.Background(),
		db,
		"INSERT RELATION INTO follow $content",
		map[string]any{
			"content": follows,
		},
	)
	if err != nil {
		panic(err)
	}

	var selectedFollows *[]Follow
	selectedFollows, err = surrealdb.Select[[]Follow](
		context.Background(),
		db,
		"follow",
	)
	if err != nil {
		panic(err)
	}
	for _, follow := range *selectedFollows {
		fmt.Printf("Selected follow: %+s\n", follow)
	}

	type PersonWithFollows struct {
		ID     models.RecordID   `json:"id"`
		Follow []models.RecordID `json:"follows"`
	}

	var followedByA *[]surrealdb.QueryResult[[]PersonWithFollows]
	followedByA, err = surrealdb.Query[[]PersonWithFollows](
		context.Background(),
		db,
		"SELECT id, <->follow<->person AS follows FROM person ORDER BY id",
		nil,
	)
	if err != nil {
		panic(err)
	}
	for _, person := range (*followedByA)[0].Result {
		fmt.Printf("PersonWithFollows: %+s\n", person)
	}

	// Unordered output:
	// Inserted: [{{person a}} {{person b}} {{person c}}]
	// Selected person: {{person a}}
	// Selected person: {{person b}}
	// Selected person: {{person c}}
	// Selected follow: {{follow person:a:person:b} {person a} {person b}}
	// Selected follow: {{follow person:b:person:c} {person b} {person c}}
	// Selected follow: {{follow person:c:person:a} {person c} {person a}}
	// PersonWithFollows: {{person a} [{person c} {person a} {person a} {person b}]}
	// PersonWithFollows: {{person b} [{person a} {person b} {person b} {person c}]}
	// PersonWithFollows: {{person c} [{person b} {person c} {person c} {person a}]}
}
