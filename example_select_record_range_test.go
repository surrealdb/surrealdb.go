package surrealdb_test

import (
	"context"
	"fmt"
	"sort"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// ExampleSelect_recordRange_threePartArrayID shows selecting a prefix of
// three-part array-style record ids against a live SurrealDB.
//
// Array ids keep a fixed number of parts, so a city-wide range writes NONE
// and open `..` for the unbound dimensions. Proof is the Select result set —
// not RangeValue.String().
func ExampleSelect_recordRange_threePartArrayID() {
	db := testenv.MustNew("surrealdbexamples", "record_range_3part", "temp")
	ctx := context.Background()

	type Temp struct {
		ID   models.RecordID `json:"id"`
		City string          `json:"city"`
		Area string          `json:"area"`
		N    int             `json:"n"`
	}

	seeds := []struct {
		id   models.RecordID
		city string
		area string
		n    int
	}{
		{models.NewRecordID("temp", models.ID("London", "West", 1)), "London", "West", 1},
		{models.NewRecordID("temp", models.ID("London", "West", 2)), "London", "West", 2},
		{models.NewRecordID("temp", models.ID("London", "East", 1)), "London", "East", 1},
		{models.NewRecordID("temp", models.ID("Paris", "North", 1)), "Paris", "North", 1},
	}
	for _, s := range seeds {
		if _, err := surrealdb.Create[Temp](ctx, db, s.id, map[string]any{
			"city": s.city,
			"area": s.area,
			"n":    s.n,
		}); err != nil {
			panic(err)
		}
	}

	// All London records, any area / number.
	rr := models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive(models.ID("London", models.None, models.None)).
			EndInclusive(models.ID("London", models.OpenRange(), models.OpenRange())),
	}

	got, err := surrealdb.Select[[]Temp](ctx, db, rr)
	if err != nil {
		panic(err)
	}
	sort.Slice(*got, func(i, j int) bool {
		return (*got)[i].ID.String() < (*got)[j].ID.String()
	})
	for _, row := range *got {
		fmt.Printf("%s %s %s %d\n", row.ID.String(), row.City, row.Area, row.N)
	}
	// Output:
	// temp:[London East 1] London East 1
	// temp:[London West 1] London West 1
	// temp:[London West 2] London West 2
}

// ExampleSelect_recordRange_threePartArrayID_fixedMiddle shows fixing the
// middle dimension of a three-part array id range. Proof is the Select result
// set — not RangeValue.String().
func ExampleSelect_recordRange_threePartArrayID_fixedMiddle() {
	db := testenv.MustNew("surrealdbexamples", "record_range_3part_west", "temp")
	ctx := context.Background()

	type Temp struct {
		ID   models.RecordID `json:"id"`
		City string          `json:"city"`
		Area string          `json:"area"`
		N    int             `json:"n"`
	}

	seeds := []struct {
		id   models.RecordID
		city string
		area string
		n    int
	}{
		{models.NewRecordID("temp", models.ID("London", "West", 1)), "London", "West", 1},
		{models.NewRecordID("temp", models.ID("London", "West", 2)), "London", "West", 2},
		{models.NewRecordID("temp", models.ID("London", "East", 1)), "London", "East", 1},
		{models.NewRecordID("temp", models.ID("Paris", "North", 1)), "Paris", "North", 1},
	}
	for _, s := range seeds {
		if _, err := surrealdb.Create[Temp](ctx, db, s.id, map[string]any{
			"city": s.city,
			"area": s.area,
			"n":    s.n,
		}); err != nil {
			panic(err)
		}
	}

	rr := models.RecordID{
		Table: "temp",
		ID: models.OpenRange().
			BeginInclusive(models.ID("London", "West", models.None)).
			EndInclusive(models.ID("London", "West", models.OpenRange())),
	}

	got, err := surrealdb.Select[[]Temp](ctx, db, rr)
	if err != nil {
		panic(err)
	}
	sort.Slice(*got, func(i, j int) bool {
		return (*got)[i].ID.String() < (*got)[j].ID.String()
	})
	for _, row := range *got {
		fmt.Printf("%s %s %s %d\n", row.ID.String(), row.City, row.Area, row.N)
	}
	// Output:
	// temp:[London West 1] London West 1
	// temp:[London West 2] London West 2
}

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

func printOmitVsNoneIDs(label string, rows []omitVsNonePerson) {
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

// queryRangeBound runs SELECT through type::thing (v2) / type::record (v3).
// SurrealDB rejects NONE/NULL as scalar record-id bounds; this path returns
// that error cleanly (passing such a range to Select can stall the WS).
func queryRangeBound(ctx context.Context, db *surrealdb.DB, table string, r models.RangeValue) ([]omitVsNonePerson, error) {
	ver, err := testenv.GetVersion(ctx, db)
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("SELECT * FROM %s($tb, $r)", ver.ThingOrRecordFn())
	res, err := surrealdb.Query[[]omitVsNonePerson](ctx, db, sql, map[string]any{
		"tb": models.Table(table),
		"r":  r,
	})
	if err != nil {
		return nil, err
	}
	if (*res)[0].Error != nil {
		return nil, (*res)[0].Error
	}
	return (*res)[0].Result, nil
}

// ExampleSelect_recordRange_omitVsNone_end proves an open end ≠ EndInclusive(None).
//
// With person:1, person:2, person:10:
//
//	OpenRange().BeginInclusive(1) → open end           → rows 1, 2, 10
//	EndInclusive(None)            → end value is NONE  → SurrealDB rejects it
//
// Leave End unset for "no limit on this side". models.None is for values that
// must appear (especially inside array-style ids — see the three-part Examples
// above), not a substitute for an open bound. Matching surrealql builder
// proof: ExampleSelect_recordRange_omitVsNone_end in contrib/surrealql.
func ExampleSelect_recordRange_omitVsNone_end() {
	db := testenv.MustNew("surrealdbexamples", "rr_omit_vs_none_end", "person")
	ctx := context.Background()
	seedOmitVsNonePersons(ctx, db)

	openEnd := models.RecordID{Table: "person", ID: models.OpenRange().BeginInclusive(1)}
	got, err := surrealdb.Select[[]omitVsNonePerson](ctx, db, openEnd)
	if err != nil {
		panic(err)
	}
	printOmitVsNoneIDs("open end", *got)

	_, err = queryRangeBound(ctx, db, "person", models.OpenRange().BeginInclusive(1).EndInclusive(models.None))
	if err != nil {
		fmt.Printf("EndInclusive(None): rejected\n")
	} else {
		fmt.Printf("EndInclusive(None): unexpected success\n")
	}
	// Output:
	// open end: 1 10 2
	// EndInclusive(None): rejected
}

// ExampleSelect_recordRange_omitVsNone_begin proves an open begin ≠ BeginInclusive(None).
//
// With person:1, person:2, person:10:
//
//	OpenRange().EndInclusive(10) → open begin          → rows 1, 2, 10
//	BeginInclusive(None)         → begin value is NONE → SurrealDB rejects it
//
// Live observation on this numeric dataset: SurrealDB does not accept the NONE
// sentinel as a scalar record-id bound, so the outcomes differ (rows vs
// rejection) rather than two different non-empty row sets. Leave Begin unset
// for open starts. Matching surrealql builder proof:
// ExampleSelect_recordRange_omitVsNone_begin in contrib/surrealql.
func ExampleSelect_recordRange_omitVsNone_begin() {
	db := testenv.MustNew("surrealdbexamples", "rr_omit_vs_none_begin", "person")
	ctx := context.Background()
	seedOmitVsNonePersons(ctx, db)

	openBegin := models.RecordID{Table: "person", ID: models.OpenRange().EndInclusive(10)}
	got, err := surrealdb.Select[[]omitVsNonePerson](ctx, db, openBegin)
	if err != nil {
		panic(err)
	}
	printOmitVsNoneIDs("open begin", *got)

	_, err = queryRangeBound(ctx, db, "person", models.OpenRange().BeginInclusive(models.None).EndInclusive(10))
	if err != nil {
		fmt.Printf("BeginInclusive(None): rejected\n")
	} else {
		fmt.Printf("BeginInclusive(None): unexpected success\n")
	}
	// Output:
	// open begin: 1 10 2
	// BeginInclusive(None): rejected
}
