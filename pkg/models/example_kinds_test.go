package models_test

import (
	"fmt"
	"time"

	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
)

func ExamplePlainDate() {
	due := models.MustParsePlainDate("2024-02-29")
	year, month, day := due.Date()
	fmt.Println(year, month, day)
	fmt.Println(due.SurrealString())
	fmt.Println(models.PlainDateInfinity, models.PlainDateInfinity.Compare(due))
	// Output:
	// 2024 February 29
	// pd'2024-02-29'
	// infinity 1
}

func ExampleTimestampOf() {
	// A timestamp is the reading of a wall clock, so it keeps 09:30 whatever
	// the time zone the time.Time is in.
	opens := time.Date(2024, 1, 15, 9, 30, 0, 0, time.FixedZone("UTC-5", -5*3600))
	ts, err := models.TimestampOf(opens)
	if err != nil {
		panic(err)
	}
	fmt.Println(ts)
	fmt.Println(ts.In(time.UTC))
	// Output:
	// 2024-01-15T09:30:00
	// 2024-01-15 09:30:00 +0000 UTC
}

func ExampleInterval_Equal() {
	month := models.Interval{Months: 1}
	days := models.Interval{Days: 30}
	fmt.Println(month, days)
	fmt.Println(month.Equal(days), month == days)
	// Output:
	// P1M P30D
	// true false
}

func ExampleBigDecimal() {
	price := models.MustParseBigDecimal("79228162514264337593543950336.50")
	fmt.Println(price, price.FitsDecimal())
	fmt.Println(models.MustParseBigDecimal("1.5e3"))
	fmt.Println(models.MustParseBigDecimal("1.50").Equal(models.MustParseBigDecimal("1.5")))
	// Output:
	// 79228162514264337593543950336.50 false
	// 1500
	// true
}

func ExampleRegex() {
	path := models.Regex{Pattern: "^/api/v[0-9]+/"}
	fmt.Println(path.SurrealString())
	// Output:
	// /^\/api\/v[0-9]+\//
}
