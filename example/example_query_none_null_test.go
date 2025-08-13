package main

import (
	"context"
	"fmt"
	"time"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

const (
	NullString = "null"
	NoneString = "none"
	NilString  = "<nil>"
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
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = false

	db := c.MustNew()

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

func ExampleQuery_none_and_null_handling_explicitFields_surrealcbor() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = true

	db := c.MustNew()

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
			nullable = NilString
		} else {
			nullable = fmt.Sprintf("%t", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NilString
		} else {
			option = fmt.Sprintf("%t", *t.Option)
		}

		fmt.Printf("ID: %s, Nullable: %v, Option: %v\n", id, nullable, option)
	}

	// Output:
	// ID: t:a, Nullable: <nil>, Option: <nil>
	// ID: t:b, Nullable: true, Option: <nil>
	// ID: t:c, Nullable: true, Option: false
	// ID: t:d, Nullable: false, Option: true
	// ID: t:e, Nullable: false, Option: <nil>
	// ID: t:f, Nullable: false, Option: <nil>
}

func ExampleQuery_none_and_null_handling_explicitFields_ints() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = false

	db := c.MustNew()

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

func ExampleQuery_none_and_null_handling_explicitFields_ints_surrealcbor() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = true

	db := c.MustNew()

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
			nullable = NilString
		} else {
			nullable = fmt.Sprintf("%v", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NilString
		} else {
			option = fmt.Sprintf("%v", *t.Option)
		}

		fmt.Printf("ID: %v, Nullable: %v, Option: %s\n", id, nullable, option)
	}

	// Output:
	// ID: t:a, Nullable: <nil>, Option: <nil>
	// ID: t:b, Nullable: 2, Option: <nil>
	// ID: t:c, Nullable: 2, Option: 1
	// ID: t:d, Nullable: 1, Option: 2
	// ID: t:e, Nullable: 1, Option: <nil>
	// ID: t:f, Nullable: 1, Option: <nil>
}

func ExampleQuery_create_none_null_fields() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = false

	db := c.MustNew()

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

func ExampleQuery_create_none_null_fields_surrealcbor() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = true

	db := c.MustNew()

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
			nullable = NilString
		} else {
			nullable = fmt.Sprintf("%t", *t.Nulabble)
		}

		var option string
		if t.Option == nil {
			option = NilString
		} else {
			option = fmt.Sprintf("%t", *t.Option)
		}

		fmt.Printf("ID: %s, Nullable: %s, Option: %s\n", id, nullable, option)
	}
	// Output:
	// Created records with none and null fields successfully
	// ID: t:a, Nullable: <nil>, Option: <nil>
	// ID: t:b, Nullable: true, Option: <nil>
	// ID: t:c, Nullable: true, Option: false
	// ID: t:d, Nullable: false, Option: true
	// ID: t:e, Nullable: false, Option: <nil>
	// ID: t:f, Nullable: false, Option: <nil>
}

//nolint:gocritic
func ExampleQuery_null_none_customdatetime_roundtrip() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = false

	db := c.MustNew()

	_, err := surrealdb.Query[[]any](
		context.Background(),
		db,
		`DEFINE TABLE t SCHEMAFULL;
		//  DEFINE FIELD nullable_zero ON t TYPE datetime | null;
		 DEFINE FIELD nullable_nil ON t TYPE datetime | null;
		 DEFINE FIELD option_zero ON t TYPE option<datetime>;
		 DEFINE FIELD option_zero_omitempty ON t TYPE option<datetime>;
		 DEFINE FIELD option_ptr_zero ON t TYPE option<datetime>;
		 DEFINE FIELD option_ptr_nil ON t TYPE option<datetime>;
		 DEFINE FIELD option_ptr_nil_omitempty ON t TYPE option<datetime>;
		`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	type T struct {
		ID *models.RecordID `json:"id,omitempty"`
		// NullableZero tests how a Zero value of CustomDateTime is marshaled into a nullable field
		//
		// This fails like this:
		//   Found NONE for field `nullable_zero`, with record `t:1`, but expected a datetime | null
		// NullableZero  models.CustomDateTime  `json:"nullable_zero"`

		// NullableNil tests how a Go pointer to nil is marshaled into a nullable field
		NullableNil *models.CustomDateTime `json:"nullable_nil"`

		OptionZero models.CustomDateTime `json:"option_zero"`

		// OptionZeroOmitEmpty tests how a Zero value of CustomDateTime is marshaled into an option field
		OptionZeroOmitEmpty models.CustomDateTime `json:"option_zero_omitempty,omitempty"`

		// OptionZeroOmitZero tests how a Zero value of CustomDateTime is marshaled into an option field
		// when the field is omitzero
		OptionZeroOmitZero models.CustomDateTime `json:"option_zero_omitzero,omitzero"`

		// OptionPtrZero tests how a Go pointer to a Zero value of CustomDateTime is marshaled
		OptionPtrZero *models.CustomDateTime `json:"option_ptr_zero"`

		// OptionNil tests how a Go pointer to nil is marshaled
		//
		// This fails like this:
		//   Found NULL for field `option_ptr_nil`, with record `t:1`, but expected a option<datetime>
		// OptionPtrNil *models.CustomDateTime `json:"option_ptr_nil"`

		// OptionPtrNilOmitEmpty tests how a Go pointer to nil is marshaled into an option field
		// when the field is omitted if empty.
		OptionPtrNilOmitEmpty *models.CustomDateTime `json:"option_ptr_nil_omitempty,omitempty"`
	}

	// Marshaling rule:
	//
	// m1. Go `nil` w/o omitempty is SurrealDB `null`
	// m2. Go `nil` w/  omitempty is SurrealDB `none`
	// m3. Zero value of CustomDateTime is SurrealDB `none`

	// Unmarshaling rule:
	//
	// u1. SurrealDB `null`                       is Go `nil`
	// u2. SurrealDB `none` + Go       `any` type is Go `models.CustomNil{}`
	// u3. SurrealDB `none` + Go non-pointer type is Go zero value (no primitive type is supported yet, but CustomDateTime is supported)
	// u4. SurrealDB `none` + Go     pointer type is Go `nil`                     when `SELECT *` is used
	// u5. SurrealDB `none` + Go     pointer type is a pointer to a Go zero value when explicit fields are selected
	//     (e.g., you cannot unmarshal `none` into `nil` when the field is explicitly selected)

	// Future plans:
	//
	// Our plan is to fix u4 and u5 in the future, by unifying the two into:
	//
	// u4. SurrealDB `none` will unmarshal into Go `nil` if the field IS a pointer type,
	//   REGARDLESS of whether the field is explicitly selected or not

	_, err = surrealdb.Query[[]T](
		context.Background(),
		db,
		`CREATE $id CONTENT $value`,
		map[string]any{
			"id": models.NewRecordID("t", 1),
			"value": T{
				// NullableZero:  models.CustomDateTime{Time: time.Time{}},
				NullableNil:         nil,
				OptionZero:          models.CustomDateTime{Time: time.Time{}},
				OptionZeroOmitEmpty: models.CustomDateTime{Time: time.Time{}},
				OptionZeroOmitZero:  models.CustomDateTime{Time: time.Time{}},
				OptionPtrZero:       &models.CustomDateTime{Time: time.Time{}},
				// OptionPtrNil:  nil,
				OptionPtrNilOmitEmpty: nil,
			},
		},
	)
	if err != nil {
		panic(err)
	}

	selectExplicit, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT id,
		        nullable_nil,
				option_zero,
				option_zero_omitempty,
				option_zero_omitzero,
				option_ptr_zero,
				option_ptr_nil_omitempty
		FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("# SELECT with explicit fields")
	fmt.Println()

	for _, t := range (*selectExplicit)[0].Result {
		fmt.Printf("ID: %v\n", t.ID)
		// fmt.Printf("NullableZero: %v\n", t.NullableZero)
		fmt.Printf("NullableNil: %v\n", t.NullableNil)
		fmt.Printf("OptionZero: %v\n", t.OptionZero)
		fmt.Printf("OptionZeroOmitEmpty: %v\n", t.OptionZeroOmitEmpty)
		fmt.Printf("OptionZeroOmitZero: %v\n", t.OptionZeroOmitZero)
		fmt.Printf("OptionPtrZero: %v\n", t.OptionPtrZero)
		// fmt.Printf("OptionPtrNil: %v\n", t.OptionPtrNil)
		fmt.Printf("OptionPtrNilOmitEmpty: %v\n", t.OptionPtrNilOmitEmpty)
	}

	selectAll, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT * FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("# SELECT with all fields")
	fmt.Println()

	for _, t := range (*selectAll)[0].Result {
		fmt.Printf("ID: %v\n", t.ID)
		// fmt.Printf("NullableZero: %v\n", t.NullableZero)
		fmt.Printf("NullableNil: %v\n", t.NullableNil)
		fmt.Printf("OptionZero: %v\n", t.OptionZero)
		fmt.Printf("OptionZeroOmitEmpty: %v\n", t.OptionZeroOmitEmpty)
		fmt.Printf("OptionZeroOmitZero: %v\n", t.OptionZeroOmitZero)
		fmt.Printf("OptionPtrZero: %v\n", t.OptionPtrZero)
		// fmt.Printf("OptionPtrNil: %v\n", t.OptionPtrNil)
		fmt.Printf("OptionPtrNilOmitEmpty: %v\n", t.OptionPtrNilOmitEmpty)
	}

	selectExplicitMap, err := surrealdb.Query[[]map[string]any](
		context.Background(),
		db,
		`SELECT id,
		        nullable_nil,
				option_zero,
				option_zero_omitempty,
				option_zero_omitzero,
				option_ptr_zero,
				option_ptr_nil_omitempty
		FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("# SELECT with explicit fields into map[string]any")
	fmt.Println()

	for _, t := range (*selectExplicitMap)[0].Result {
		fmt.Printf("ID: %v\n", t["id"])
		// fmt.Printf("NullableZero: %+v\n", t["nullable_zero"])
		fmt.Printf("NullableNil: %T%+v\n", t["nullable_nil"], t["nullable_nil"])
		fmt.Printf("OptionZero: %T%+v\n", t["option_zero"], t["option_zero"])
		fmt.Printf("OptionZeroOmitEmpty: %T%+v\n", t["option_zero_omitempty"], t["option_zero_omitempty"])
		fmt.Printf("OptionZeroOmitZero: %T%+v\n", t["option_zero_omitzero"], t["option_zero_omitzero"])
		fmt.Printf("OptionPtrZero: %T%+v\n", t["option_ptr_zero"], t["option_ptr_zero"])
		// fmt.Printf("OptionPtrNil: %T%+v\n", t["option_ptr_nil"], t["option_ptr_nil"])
		fmt.Printf("OptionPtrNilOmitEmpty: %T%+v\n", t["option_ptr_nil_omitempty"], t["option_ptr_nil_omitempty"])
	}

	// Output:
	//
	// # SELECT with explicit fields
	//
	// ID: t:1
	// NullableNil: <nil>
	// OptionZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitEmpty: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionPtrZero: 0001-01-01T00:00:00Z
	// OptionPtrNilOmitEmpty: 0001-01-01T00:00:00Z
	//
	// # SELECT with all fields
	//
	// ID: t:1
	// NullableNil: <nil>
	// OptionZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitEmpty: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionPtrZero: <nil>
	// OptionPtrNilOmitEmpty: <nil>
	//
	// # SELECT with explicit fields into map[string]any
	//
	// ID: {t 1}
	// NullableNil: <nil><nil>
	// OptionZero: models.CustomNil{}
	// OptionZeroOmitEmpty: models.CustomNil{}
	// OptionZeroOmitZero: models.CustomNil{}
	// OptionPtrZero: models.CustomNil{}
	// OptionPtrNilOmitEmpty: models.CustomNil{}
}

//nolint:gocritic
func ExampleQuery_null_none_customdatetime_roundtrip_surrealcbor() {
	c := testenv.MustNewConfig("example", "query", "t")
	c.UseSurrealCBOR = true

	db := c.MustNew()

	_, err := surrealdb.Query[[]any](
		context.Background(),
		db,
		`DEFINE TABLE t SCHEMAFULL;
		//  DEFINE FIELD nullable_zero ON t TYPE datetime | null;
		 DEFINE FIELD nullable_nil ON t TYPE datetime | null;
		 DEFINE FIELD option_zero ON t TYPE option<datetime>;
		 DEFINE FIELD option_zero_omitempty ON t TYPE option<datetime>;
		 DEFINE FIELD option_ptr_zero ON t TYPE option<datetime>;
		 DEFINE FIELD option_ptr_nil ON t TYPE option<datetime>;
		 DEFINE FIELD option_ptr_nil_omitempty ON t TYPE option<datetime>;
		`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	type T struct {
		ID *models.RecordID `json:"id,omitempty"`
		// NullableZero tests how a Zero value of CustomDateTime is marshaled into a nullable field
		//
		// This fails like this:
		//   Found NONE for field `nullable_zero`, with record `t:1`, but expected a datetime | null
		// NullableZero  models.CustomDateTime  `json:"nullable_zero"`

		// NullableNil tests how a Go pointer to nil is marshaled into a nullable field
		NullableNil *models.CustomDateTime `json:"nullable_nil"`

		OptionZero models.CustomDateTime `json:"option_zero"`

		// OptionZeroOmitEmpty tests how a Zero value of CustomDateTime is marshaled into an option field
		OptionZeroOmitEmpty models.CustomDateTime `json:"option_zero_omitempty,omitempty"`

		// OptionZeroOmitZero tests how a Zero value of CustomDateTime is marshaled into an option field
		// when the field is omitzero
		OptionZeroOmitZero models.CustomDateTime `json:"option_zero_omitzero,omitzero"`

		// OptionPtrZero tests how a Go pointer to a Zero value of CustomDateTime is marshaled
		OptionPtrZero *models.CustomDateTime `json:"option_ptr_zero"`

		// OptionNil tests how a Go pointer to nil is marshaled
		//
		// This fails like this:
		//   Found NULL for field `option_ptr_nil`, with record `t:1`, but expected a option<datetime>
		// OptionPtrNil *models.CustomDateTime `json:"option_ptr_nil"`

		// OptionPtrNilOmitEmpty tests how a Go pointer to nil is marshaled into an option field
		// when the field is omitted if empty.
		OptionPtrNilOmitEmpty *models.CustomDateTime `json:"option_ptr_nil_omitempty,omitempty"`
	}

	_, err = surrealdb.Query[[]T](
		context.Background(),
		db,
		`CREATE $id CONTENT $value`,
		map[string]any{
			"id": models.NewRecordID("t", 1),
			"value": T{
				// NullableZero:  models.CustomDateTime{Time: time.Time{}},
				NullableNil:         nil,
				OptionZero:          models.CustomDateTime{Time: time.Time{}},
				OptionZeroOmitEmpty: models.CustomDateTime{Time: time.Time{}},
				OptionZeroOmitZero:  models.CustomDateTime{Time: time.Time{}},
				OptionPtrZero:       &models.CustomDateTime{Time: time.Time{}},
				// OptionPtrNil:  nil,
				OptionPtrNilOmitEmpty: nil,
			},
		},
	)
	if err != nil {
		panic(err)
	}

	selectExplicit, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT id,
		        nullable_nil,
				option_zero,
				option_zero_omitempty,
				option_zero_omitzero,
				option_ptr_zero,
				option_ptr_nil_omitempty
		FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("# SELECT with explicit fields")
	fmt.Println()

	for _, t := range (*selectExplicit)[0].Result {
		fmt.Printf("ID: %v\n", t.ID)
		// fmt.Printf("NullableZero: %v\n", t.NullableZero)
		fmt.Printf("NullableNil: %v\n", t.NullableNil)
		fmt.Printf("OptionZero: %v\n", t.OptionZero)
		fmt.Printf("OptionZeroOmitEmpty: %v\n", t.OptionZeroOmitEmpty)
		fmt.Printf("OptionZeroOmitZero: %v\n", t.OptionZeroOmitZero)
		fmt.Printf("OptionPtrZero: %v\n", t.OptionPtrZero)
		// fmt.Printf("OptionPtrNil: %v\n", t.OptionPtrNil)
		fmt.Printf("OptionPtrNilOmitEmpty: %v\n", t.OptionPtrNilOmitEmpty)
	}

	selectAll, err := surrealdb.Query[[]T](
		context.Background(),
		db,
		`SELECT * FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("# SELECT with all fields")
	fmt.Println()

	for _, t := range (*selectAll)[0].Result {
		fmt.Printf("ID: %v\n", t.ID)
		// fmt.Printf("NullableZero: %v\n", t.NullableZero)
		fmt.Printf("NullableNil: %v\n", t.NullableNil)
		fmt.Printf("OptionZero: %v\n", t.OptionZero)
		fmt.Printf("OptionZeroOmitEmpty: %v\n", t.OptionZeroOmitEmpty)
		fmt.Printf("OptionZeroOmitZero: %v\n", t.OptionZeroOmitZero)
		fmt.Printf("OptionPtrZero: %v\n", t.OptionPtrZero)
		// fmt.Printf("OptionPtrNil: %v\n", t.OptionPtrNil)
		fmt.Printf("OptionPtrNilOmitEmpty: %v\n", t.OptionPtrNilOmitEmpty)
	}

	selectExplicitMap, err := surrealdb.Query[[]map[string]any](
		context.Background(),
		db,
		`SELECT id,
		        nullable_nil,
				option_zero,
				option_zero_omitempty,
				option_zero_omitzero,
				option_ptr_zero,
				option_ptr_nil_omitempty
		FROM t ORDER BY id`,
		nil,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("# SELECT with explicit fields into map[string]any")
	fmt.Println()

	for _, t := range (*selectExplicitMap)[0].Result {
		fmt.Printf("ID: %v\n", t["id"])
		// fmt.Printf("NullableZero: %+v\n", t["nullable_zero"])
		fmt.Printf("NullableNil: %T%+v\n", t["nullable_nil"], t["nullable_nil"])
		fmt.Printf("OptionZero: %T%+v\n", t["option_zero"], t["option_zero"])
		fmt.Printf("OptionZeroOmitEmpty: %T%+v\n", t["option_zero_omitempty"], t["option_zero_omitempty"])
		fmt.Printf("OptionZeroOmitZero: %T%+v\n", t["option_zero_omitzero"], t["option_zero_omitzero"])
		fmt.Printf("OptionPtrZero: %T%+v\n", t["option_ptr_zero"], t["option_ptr_zero"])
		// fmt.Printf("OptionPtrNil: %T%+v\n", t["option_ptr_nil"], t["option_ptr_nil"])
		fmt.Printf("OptionPtrNilOmitEmpty: %T%+v\n", t["option_ptr_nil_omitempty"], t["option_ptr_nil_omitempty"])
	}

	// Output:
	//
	// # SELECT with explicit fields
	//
	// ID: t:1
	// NullableNil: <nil>
	// OptionZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitEmpty: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionPtrZero: <nil>
	// OptionPtrNilOmitEmpty: <nil>
	//
	// # SELECT with all fields
	//
	// ID: t:1
	// NullableNil: <nil>
	// OptionZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitEmpty: {0001-01-01 00:00:00 +0000 UTC}
	// OptionZeroOmitZero: {0001-01-01 00:00:00 +0000 UTC}
	// OptionPtrZero: <nil>
	// OptionPtrNilOmitEmpty: <nil>
	//
	// # SELECT with explicit fields into map[string]any
	//
	// ID: {t 1}
	// NullableNil: <nil><nil>
	// OptionZero: <nil><nil>
	// OptionZeroOmitEmpty: <nil><nil>
	// OptionZeroOmitZero: <nil><nil>
	// OptionPtrZero: <nil><nil>
	// OptionPtrNilOmitEmpty: <nil><nil>
}
