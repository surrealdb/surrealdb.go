package surrealql_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type omitVsNonePerson struct {
	ID   models.RecordID `json:"id"`
	Name string          `json:"name"`
}

func seedOmitVsNonePersons(ctx context.Context, db *surrealdb.DB) {
	_, err := surrealdb.Query[any](ctx, db, `
		DEFINE TABLE OVERWRITE person;
		DELETE person;
		CREATE person:1 SET name = 'one';
		CREATE person:2 SET name = 'two';
		CREATE person:10 SET name = 'ten';
	`, nil)
	if err != nil {
		panic(err)
	}
}

func printPersonIDs(label string, rows []omitVsNonePerson) {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, fmt.Sprintf("%v", r.ID.ID))
	}
	sort.Strings(ids)
	fmt.Printf("%s:", label)
	for _, id := range ids {
		fmt.Printf(" %s", id)
	}
	fmt.Printf("\n")
}

// queryRangeBound runs SELECT via type::thing (v2) / type::record (v3) so a
// rejected range bound returns a clear error instead of hanging the connection.
func queryRangeBound(ctx context.Context, db *surrealdb.DB, table string, r any) error {
	ver, err := testenv.GetVersion(ctx, db)
	if err != nil {
		return err
	}
	sql := fmt.Sprintf("SELECT * FROM %s($tb, $r)", ver.ThingOrRecordFn())
	res, err := surrealdb.Query[[]omitVsNonePerson](ctx, db, sql, map[string]any{
		"tb": models.Table(table),
		"r":  r,
	})
	if err != nil {
		return err
	}
	if (*res)[0].Error != nil {
		return (*res)[0].Error
	}
	return nil
}

// ExampleSelect_recordRange_omitVsNone_end proves that an open end bound is
// not the same as writing NONE as the end value.
//
// Dataset: person:1, person:2, person:10.
//
//   - RangeOpenBeginInclusive(1) → open end → returns 1, 2, and 10
//   - RangeClosed(1, None) → SurrealDB rejects NONE as a scalar record-id bound
//
// Leave the end open when that side has no limit. models.None belongs inside
// array-style ids (see the composite-ID Examples), not as a stand-in for
// "leave this side open".
func ExampleSelect_recordRange_omitVsNone_end() {
	db := testenv.MustNew("surrealqlexamples", "rr_omit_vs_none_end", "person")
	ctx := context.Background()
	seedOmitVsNonePersons(ctx, db)

	openEnd := models.RecordID{Table: "person", ID: surrealql.RangeOpenBeginInclusive(1)}
	sql, vars := surrealql.Select(openEnd).Build()
	res, err := surrealdb.Query[[]omitVsNonePerson](ctx, db, sql, vars)
	if err != nil {
		panic(err)
	}
	printPersonIDs("open end", (*res)[0].Result)

	err = queryRangeBound(ctx, db, "person", surrealql.RangeClosed(1, models.None))
	if err != nil {
		fmt.Printf("RangeClosed(1, None): rejected\n")
	} else {
		fmt.Printf("RangeClosed(1, None): unexpected success\n")
	}
	// Output:
	// open end: 1 10 2
	// RangeClosed(1, None): rejected
}

// ExampleSelect_recordRange_omitVsNone_begin proves that an open begin bound
// is not the same as writing NONE as the begin value.
//
// Dataset: person:1, person:2, person:10.
//
//   - RangeOpenEndInclusive(10) → open begin → returns 1, 2, and 10
//   - RangeClosed(None, 10) → SurrealDB rejects NONE as a scalar record-id bound
//
// On this numeric-only table, an accepted NONE-begin would still be a different
// *kind* of range from an open begin. In practice SurrealDB rejects the NONE
// value here, so the live outcomes differ (rows vs rejection). Leave Begin
// open for open starts.
func ExampleSelect_recordRange_omitVsNone_begin() {
	db := testenv.MustNew("surrealqlexamples", "rr_omit_vs_none_begin", "person")
	ctx := context.Background()
	seedOmitVsNonePersons(ctx, db)

	openBegin := models.RecordID{Table: "person", ID: surrealql.RangeOpenEndInclusive(10)}
	sql, vars := surrealql.Select(openBegin).Build()
	res, err := surrealdb.Query[[]omitVsNonePerson](ctx, db, sql, vars)
	if err != nil {
		panic(err)
	}
	printPersonIDs("open begin", (*res)[0].Result)

	err = queryRangeBound(ctx, db, "person", surrealql.RangeClosed(models.None, 10))
	if err != nil {
		fmt.Printf("RangeClosed(None, 10): rejected\n")
	} else {
		fmt.Printf("RangeClosed(None, 10): unexpected success\n")
	}
	// Output:
	// open begin: 1 10 2
	// RangeClosed(None, 10): rejected
}
