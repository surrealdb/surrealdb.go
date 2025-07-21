package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func ExampleQuery_none_and_null_handling() {
	db := newSurrealDBWSConnection("query", "t")

	_, err := surrealdb.Query[[]any](
		context.Background(),
		db,
		`DEFINE TABLE t SCHEMAFULL;
		 DEFINE FIELD nullable ON t TYPE bool | null;
		 DEFINE FIELD option ON t TYPE option<bool>;
		 CREATE t:a SET nullable = null;
		 CREATE t:b SET nullable = true;
		 CREATE t:c SET nullable = true, option = false;
		 CREATE t:d SET nullable = false, option = true;
		 CREATE t:e SET nullable = false;
		 CREATE t:f SET nullable = false, option = NONE;
		`,
		map[string]any{
			"id": models.NewRecordID("t", 1),
		})
	if err != nil {
		panic(err)
	}

	type T struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Nulabble *bool            `json:"nullable"`
		Option   *bool            `json:"option"`
	}

	selected, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT * FROM t ORDER BY id.id`,
		nil,
	)
	if err != nil {
		panic(err)
	}
	for _, t := range (*selected)[0].Result {
		id := t.ID

		var nullable string
		if t.Nulabble == nil {
			nullable = "null"
		} else {
			nullable = fmt.Sprintf("%t", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = "none"
		} else {
			option = fmt.Sprintf("%t", *t.Option)
		}

		fmt.Printf("ID: %s, Nullable: %s, Option: %s\n", id, nullable, option)
	}

	// Output:
	//ID: t:a, Nullable: null, Option: none
	// ID: t:b, Nullable: true, Option: none
	// ID: t:c, Nullable: true, Option: false
	// ID: t:d, Nullable: false, Option: true
	// ID: t:e, Nullable: false, Option: none
	// ID: t:f, Nullable: false, Option: none
}
