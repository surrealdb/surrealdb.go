package main

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleRelate() {
	db := testenv.MustNewDeprecated("query", "person", "follow")

	type Person struct {
		ID models.RecordID `json:"id,omitempty"`
	}

	type Follow struct {
		In    *models.RecordID      `json:"in,omitempty"`
		Out   *models.RecordID      `json:"out,omitempty"`
		Since models.CustomDateTime `json:"since"`
	}

	first, err := surrealdb.Create[Person](
		context.Background(),
		db,
		"person",
		map[string]any{
			"id": models.NewRecordID("person", "first"),
		})
	if err != nil {
		panic(err)
	}

	second, err := surrealdb.Create[Person](
		context.Background(),
		db,
		"person",
		map[string]any{
			"id": models.NewRecordID("person", "second"),
		})
	if err != nil {
		panic(err)
	}

	since, err := time.Parse(time.RFC3339, "2023-10-01T12:00:00Z")
	if err != nil {
		panic(err)
	}

	persons, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		"SELECT * FROM person ORDER BY id.id",
		nil,
	)
	if err != nil {
		panic(err)
	}
	for _, person := range (*persons)[0].Result {
		fmt.Printf("Person: %+v\n", person)
	}

	res, relateErr := surrealdb.Relate[connection.ResponseID[models.RecordID]](
		context.Background(),
		db,
		&surrealdb.Relationship{
			// ID is currently ignored, and the relation will have a generated ID.
			// If you want to set the ID, use InsertRelation, or use
			// Query with `RELATE` statement.
			ID:       &models.RecordID{Table: "follow", ID: "first_second"},
			In:       first.ID,
			Out:      second.ID,
			Relation: "follow",
			Data: map[string]any{
				"since": models.CustomDateTime{
					Time: since,
				},
			},
		},
	)
	if relateErr != nil {
		panic(relateErr)
	}
	if res == nil {
		panic("relation response is nil")
	}
	if res.ID.ID == "first_second" {
		panic("relation ID should not be set to 'first_second'")
	}

	//nolint:lll
	/// Here's an alternative way to create a relation using a query.
	//
	// if res, err := surrealdb.Query[any](
	// 	db,
	// 	"RELATE $in->follow:first_second->$out SET since = $since",
	// 	map[string]any{
	// 		// `RELATE $in->follow->$out` with "id" below is ignored,
	// 		// and the id becomes a generated one.
	// 		// If you want to set the id, use `RELATE $in->follow:the_id->$out` like above.
	// 		// "id":    models.NewRecordID("follow", "first_second"),
	// 		"in":    first.ID,
	// 		"out":   second.ID,
	// 		"since": models.CustomDateTime{Time: since},
	// 	},
	// ); err != nil {
	// 	panic(err)
	// } else {
	// 	fmt.Printf("Relation: %+v\n", (*res)[0].Result)
	// }
	// The output will be:
	// Relation: [map[id:{Table:follow ID:first_second} in:{Table:person ID:first} out:{Table:person ID:second} since:{Time:2023-10-01 12:00:00 +0000 UTC}]]

	type PersonWithFollows struct {
		Person
		Follows []models.RecordID `json:"follows,omitempty"`
	}
	selected, err := surrealdb.Query[[]PersonWithFollows](
		context.Background(),
		db,
		"SELECT id, name, ->follow->person AS follows FROM $id",
		map[string]any{
			"id": first.ID,
		},
	)
	if err != nil {
		panic(err)
	}

	for _, person := range (*selected)[0].Result {
		fmt.Printf("PersonWithFollows: %+v\n", person)
	}

	// Note we can select the relationships themselves because
	// RELATE creates a record in the relation table.
	follows, err := surrealdb.Query[[]Follow](
		context.Background(),
		db,
		"SELECT * from follow",
		nil,
	)
	if err != nil {
		panic(err)
	}

	for _, follow := range (*follows)[0].Result {
		fmt.Printf("Follow: %+v\n", follow)
	}

	//nolint:lll
	// Output:
	// Person: {ID:{Table:person ID:first}}
	// Person: {ID:{Table:person ID:second}}
	// PersonWithFollows: {Person:{ID:{Table:person ID:first}} Follows:[{Table:person ID:second}]}
	// Follow: {In:person:first Out:person:second Since:{Time:2023-10-01 12:00:00 +0000 UTC}}
}
