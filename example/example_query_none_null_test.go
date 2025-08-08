package main

import (
	"context"
	"fmt"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const (
	NullString = "null"
	NoneString = "none"
)

func ExampleQuery_none_and_null_handling_allExistingFields() {
	db := testenv.MustNewDeprecated("query", "t")

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
			nullable = NullString
		} else {
			nullable = fmt.Sprintf("%t", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NoneString
		} else {
			option = fmt.Sprintf("%t", *t.Option)
		}

		fmt.Printf("ID: %s, Nullable: %s, Option: %s\n", id, nullable, option)
	}

	// Output:
	// ID: t:a, Nullable: null, Option: none
	// ID: t:b, Nullable: true, Option: none
	// ID: t:c, Nullable: true, Option: false
	// ID: t:d, Nullable: false, Option: true
	// ID: t:e, Nullable: false, Option: none
	// ID: t:f, Nullable: false, Option: none
}

func ExampleQuery_none_and_null_handling_explicitFields() {
	db := testenv.MustNewDeprecated("query", "t")

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
		`SELECT id, nullable, option FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}
	for _, t := range (*selected)[0].Result {
		id := t.ID

		var nullable string
		if t.Nulabble == nil {
			nullable = NullString
		} else {
			nullable = fmt.Sprintf("%t", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NoneString
		} else {
			option = fmt.Sprintf("%t", *t.Option)
		}

		fmt.Printf("ID: %s, Nullable: %s, Option: %s\n", id, nullable, option)
	}

	// Output:
	// ID: t:a, Nullable: null, Option: false
	// ID: t:b, Nullable: true, Option: false
	// ID: t:c, Nullable: true, Option: false
	// ID: t:d, Nullable: false, Option: true
	// ID: t:e, Nullable: false, Option: false
	// ID: t:f, Nullable: false, Option: false
}

func ExampleQuery_none_and_null_handling_explicitFields_ints() {
	db := testenv.MustNewDeprecated("query", "t")

	_, err := surrealdb.Query[[]any](
		context.Background(),
		db,
		`DEFINE TABLE t SCHEMAFULL;
		 DEFINE FIELD nullable ON t TYPE int | null;
		 DEFINE FIELD option ON t TYPE option<int>;
		 CREATE t:a SET nullable = null;
		 CREATE t:b SET nullable = 2;
		 CREATE t:c SET nullable = 2, option = 1;
		 CREATE t:d SET nullable = 1, option = 2;
		 CREATE t:e SET nullable = 1;
		 CREATE t:f SET nullable = 1, option = NONE;
		`,
		map[string]any{
			"id": models.NewRecordID("t", 1),
		})
	if err != nil {
		panic(err)
	}

	type T struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Nulabble *int             `json:"nullable"`
		Option   *int             `json:"option"`
	}

	selected, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT id, nullable, option FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}
	for _, t := range (*selected)[0].Result {
		id := t.ID

		var nullable string
		if t.Nulabble == nil {
			nullable = NullString
		} else {
			nullable = fmt.Sprintf("%v", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NoneString
		} else {
			option = fmt.Sprintf("%v", *t.Option)
		}

		fmt.Printf("ID: %v, Nullable: %v, Option: %s\n", id, nullable, option)
	}

	// Output:
	// ID: t:a, Nullable: null, Option: 0
	// ID: t:b, Nullable: 2, Option: 0
	// ID: t:c, Nullable: 2, Option: 1
	// ID: t:d, Nullable: 1, Option: 2
	// ID: t:e, Nullable: 1, Option: 0
	// ID: t:f, Nullable: 1, Option: 0
}

func ExampleQuery_create_none_null_fields() {
	db := testenv.MustNewDeprecated("query", "t")

	_, err := surrealdb.Query[[]any](
		context.Background(),
		db,
		`DEFINE TABLE t SCHEMAFULL;
		 DEFINE FIELD nullable ON t TYPE bool | null;
		 DEFINE FIELD option ON t TYPE option<bool>;
		 CREATE t:a SET nullable = $nil;
		 CREATE t:b SET nullable = true;
		 CREATE t:c SET nullable = true, option = false;
		 CREATE t:d SET nullable = false, option = true;
		 CREATE t:e SET nullable = false;
		 CREATE t:f SET nullable = false, option = $none;
		`,
		map[string]any{
			"id":   models.NewRecordID("t", 1),
			"nil":  nil,
			"none": models.None,
		})
	if err != nil {
		panic(err)
	}

	fmt.Println("Created records with none and null fields successfully")

	type T struct {
		ID       *models.RecordID `json:"id,omitempty"`
		Nulabble *bool            `json:"nullable"`
		Option   *bool            `json:"option"`
	}

	selected, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT id, nullable, option FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}
	for _, t := range (*selected)[0].Result {
		id := t.ID

		var nullable string
		if t.Nulabble == nil {
			nullable = NullString
		} else {
			nullable = fmt.Sprintf("%t", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NoneString
		} else {
			option = fmt.Sprintf("%t", *t.Option)
		}

		fmt.Printf("ID: %s, Nullable: %s, Option: %s\n", id, nullable, option)
	}
	// Output:
	// Created records with none and null fields successfully
	// ID: t:a, Nullable: null, Option: false
	// ID: t:b, Nullable: true, Option: false
	// ID: t:c, Nullable: true, Option: false
	// ID: t:d, Nullable: false, Option: true
	// ID: t:e, Nullable: false, Option: false
	// ID: t:f, Nullable: false, Option: false
}
