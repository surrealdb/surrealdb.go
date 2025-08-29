package surrealdb_test

import (
	"context"
	"fmt"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// This example demonstrates how you can batch insert and upsert records,
// with specifying RETURN NONE to avoid unnecessary data transfer and decoding.
//
//nolint:funlen
func ExampleQuery_bluk_insert_upsert() {
	db := testenv.MustNew("surrealdbexamples", "query", "persons")

	/// You can make it a schemaful table by defining fields like this:
	//
	// _, err := surrealdb.Query[any](
	// 	db,
	// 	`DEFINE TABLE persons SCHEMAFULL;
	// 	DEFINE FIELD note ON persons TYPE string;
	// 	DEFINE FIELD num ON persons TYPE int;
	// 	DEFINE FIELD loc ON persons TYPE geometry<point>;
	// `,
	// 	nil,
	// )
	// if err != nil {
	// 	panic(err)
	// }
	//
	/// If you do that, ensure that fields do not have `omitempty` json tags!
	///
	/// Why?
	/// Our cbor library reuses `json` tags for CBOR encoding/decoding,
	/// and `omitempty` skips the encoding of the field if it is empty.
	///
	/// For example, if you define an `int` field with `omitempty` tag,
	/// a value of `0` will not be encoded, resulting in an query error due:
	///   Found NONE for field `num`, with record `persons:p0`, but expected a int

	type Person struct {
		ID   *models.RecordID `json:"id"`
		Note string           `json:"note"`
		// As writte nabove whether it is `json:"num,omitempty"` or `json:"num"` is important,.
		// depending on what you want to achieve.
		Num int                  `json:"num"`
		Loc models.GeometryPoint `json:"loc"`
	}

	nthPerson := func(i int) Person {
		return Person{
			ID:   &models.RecordID{Table: "persons", ID: fmt.Sprintf("p%d", i)},
			Note: fmt.Sprintf("inserted%d", i),
			Num:  i,
			Loc: models.GeometryPoint{
				Longitude: 12.34 + float64(i),
				Latitude:  45.65 + float64(i),
			},
		}
	}

	var persons []Person
	for i := 0; i < 2; i++ {
		persons = append(persons, nthPerson(i))
	}

	insert, err := surrealdb.Query[any](
		context.Background(),
		db,
		`INSERT INTO persons $persons RETURN NONE`,
		map[string]any{
			"persons": persons,
		})
	if err != nil {
		panic(err)
	}
	fmt.Println("# INSERT INTO")
	fmt.Printf("Count   : %d\n", len(*insert))
	fmt.Printf("Status  : %+s\n", (*insert)[0].Status)
	fmt.Printf("Result  : %+v\n", (*insert)[0].Result)

	select1, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`SELECT * FROM persons ORDER BY id.id`,
		nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected: %+v\n", (*select1)[0].Result)

	persons = append(persons, nthPerson(2))

	insertIgnore, err := surrealdb.Query[any](
		context.Background(),
		db,
		`INSERT IGNORE INTO persons $persons RETURN NONE`,
		map[string]any{
			"persons": persons,
		})
	if err != nil {
		panic(err)
	}
	fmt.Println("# INSERT IGNORE INTO")
	fmt.Printf("Count   : %d\n", len(*insertIgnore))
	fmt.Printf("Status  : %+s\n", (*insertIgnore)[0].Status)
	fmt.Printf("Result  : %+v\n", (*insertIgnore)[0].Result)

	select2, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`SELECT * FROM persons ORDER BY id.id`,
		nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected: %+v\n", (*select2)[0].Result)

	for i := 0; i < 3; i++ {
		persons[i].Note = fmt.Sprintf("updated%d", i)
	}
	persons = append(persons, nthPerson(3))
	var upsertQueries []string
	vars := make(map[string]any)
	for i, p := range persons {
		upsertQueries = append(upsertQueries,
			fmt.Sprintf(`UPSERT persons CONTENT $content%d RETURN NONE`, i),
		)
		vars[fmt.Sprintf("content%d", i)] = p
	}
	upsert, err := surrealdb.Query[any](
		context.Background(),
		db,
		strings.Join(upsertQueries, ";"),
		vars,
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("# UPSERT CONTENT")
	fmt.Printf("Count   : %d\n", len(*upsert))
	fmt.Printf("Status  : %+s\n", (*upsert)[0].Status)
	fmt.Printf("Result  : %+v\n", (*upsert)[0].Result)

	select3, err := surrealdb.Query[[]Person](
		context.Background(),
		db,
		`SELECT * FROM persons ORDER BY id.id`,
		nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected: %+v\n", (*select3)[0].Result)

	//nolint:lll
	// Output:
	// # INSERT INTO
	// Count   : 1
	// Status  : OK
	// Result  : []
	// Selected: [{ID:persons:p0 Note:inserted0 Num:0 Loc:{Latitude:45.65 Longitude:12.34}} {ID:persons:p1 Note:inserted1 Num:1 Loc:{Latitude:46.65 Longitude:13.34}}]
	// # INSERT IGNORE INTO
	// Count   : 1
	// Status  : OK
	// Result  : []
	// Selected: [{ID:persons:p0 Note:inserted0 Num:0 Loc:{Latitude:45.65 Longitude:12.34}} {ID:persons:p1 Note:inserted1 Num:1 Loc:{Latitude:46.65 Longitude:13.34}} {ID:persons:p2 Note:inserted2 Num:2 Loc:{Latitude:47.65 Longitude:14.34}}]
	// # UPSERT CONTENT
	// Count   : 4
	// Status  : OK
	// Result  : []
	// Selected: [{ID:persons:p0 Note:updated0 Num:0 Loc:{Latitude:45.65 Longitude:12.34}} {ID:persons:p1 Note:updated1 Num:1 Loc:{Latitude:46.65 Longitude:13.34}} {ID:persons:p2 Note:updated2 Num:2 Loc:{Latitude:47.65 Longitude:14.34}} {ID:persons:p3 Note:inserted3 Num:3 Loc:{Latitude:48.65 Longitude:15.34}}]
}
