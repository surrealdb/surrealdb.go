package surrealql_test

import (
	"context"
	"testing"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/surrealql"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Sale struct for aggregate tests
type Sale struct {
	ID       models.RecordID `json:"id,omitempty"`
	Product  string          `json:"product"`
	Quantity int             `json:"quantity"`
	Price    float64         `json:"price"`
	Total    float64         `json:"total"`
}

// SaleCreate struct for aggregate tests
type SaleCreate struct {
	Product  string  `json:"product"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Total    float64 `json:"total"`
}

// setupSalesData creates test sales data for aggregate tests
func setupSalesData(t *testing.T, ctx context.Context, db *surrealdb.DB, table string) {
	testSales := []SaleCreate{
		{Product: "Widget A", Quantity: 5, Price: 10.00, Total: 50.00},
		{Product: "Widget B", Quantity: 3, Price: 15.00, Total: 45.00},
		{Product: "Widget A", Quantity: 2, Price: 10.00, Total: 20.00},
		{Product: "Widget C", Quantity: 7, Price: 8.00, Total: 56.00},
		{Product: "Widget B", Quantity: 4, Price: 15.00, Total: 60.00},
	}

	for _, sale := range testSales {
		_, err := surrealdb.Create[Sale](ctx, db, table, sale)
		if err != nil {
			t.Fatalf("Failed to create sale: %v", err)
		}
	}
}

func TestIntegrationAggregate_Sum(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "sales_sum")
	ctx := context.Background()

	// Setup test data
	setupSalesData(t, ctx, db, "sales_sum")

	// First check if sales exist with raw query
	rawResults, _ := surrealdb.Query[[]map[string]any](ctx, db, "SELECT * FROM sales_sum", nil)
	if len(*rawResults) > 0 && len((*rawResults)[0].Result) > 0 {
		t.Logf("Raw sale record: %+v", (*rawResults)[0].Result[0])
	}

	// Try raw sum query
	rawSumResults, _ := surrealdb.Query[[]map[string]any](ctx, db, "SELECT math::sum(total) FROM sales_sum GROUP ALL", nil)
	if len(*rawSumResults) > 0 {
		t.Logf("Raw sum results: %+v", (*rawSumResults)[0].Result)
	}

	query := surrealql.Select("sales_sum").Fields(surrealql.Expr("math::sum(total)")).
		GroupAll()
	sql, vars := query.Build()
	t.Logf("SUM SurrealQL: %s", sql)
	t.Logf("SUM Params: %v", vars)

	type SumResult struct {
		Sum float64 `json:"math::sum"`
	}

	results, err := surrealdb.Query[[]SumResult](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	t.Logf("SUM results: %+v", results)
	sumResults := (*results)[0].Result
	if len(sumResults) > 0 {
		expected := 50.0 + 45.0 + 20.0 + 56.0 + 60.0
		if sumResults[0].Sum != expected {
			t.Errorf("Expected sum %.2f, got %.2f", expected, sumResults[0].Sum)
		}
	} else {
		t.Error("No sum results returned")
	}
}

func TestIntegrationAggregate_Average(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "sales_avg")
	ctx := context.Background()

	// Setup test data
	setupSalesData(t, ctx, db, "sales_avg")

	query := surrealql.Select("sales_avg").Fields(surrealql.Expr("math::mean(price)")).
		GroupAll()
	sql, vars := query.Build()

	type AvgResult struct {
		Avg float64 `json:"math::mean"`
	}

	results, err := surrealdb.Query[[]AvgResult](ctx, db, sql, vars)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	avgResults := (*results)[0].Result
	if len(avgResults) > 0 {
		// Expected average: (10 + 15 + 10 + 8 + 15) / 5 = 11.6
		expected := 11.6
		if avgResults[0].Avg != expected {
			t.Errorf("Expected avg %.2f, got %.2f", expected, avgResults[0].Avg)
		}
	} else {
		t.Error("No average results returned")
	}
}

func TestIntegrationAggregate_MinMax(t *testing.T) {
	db := testenv.MustNewDeprecated("surrealql_test", "sales_minmax")
	ctx := context.Background()

	// Setup test data
	setupSalesData(t, ctx, db, "sales_minmax")

	t.Run("Min", func(t *testing.T) {
		minQuery := surrealql.Select("sales_minmax").Fields(surrealql.Expr("math::min(total)")).
			GroupAll()
		sql, vars := minQuery.Build()

		type MinResult struct {
			Min float64 `json:"math::min"`
		}

		minResults, err := surrealdb.Query[[]MinResult](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Min query failed: %v", err)
		}

		if len((*minResults)[0].Result) > 0 {
			minValue := (*minResults)[0].Result[0].Min
			if minValue != 20.0 {
				t.Errorf("Expected min 20.0, got %.2f", minValue)
			}
		} else {
			t.Error("No min results returned")
		}
	})

	t.Run("Max", func(t *testing.T) {
		maxQuery := surrealql.Select("sales_minmax").Fields(surrealql.Expr("math::max(total)")).
			GroupAll()
		sql, vars := maxQuery.Build()

		type MaxResult struct {
			Max float64 `json:"math::max"`
		}

		maxResults, err := surrealdb.Query[[]MaxResult](ctx, db, sql, vars)
		if err != nil {
			t.Fatalf("Max query failed: %v", err)
		}

		if len((*maxResults)[0].Result) > 0 {
			maxValue := (*maxResults)[0].Result[0].Max
			if maxValue != 60.0 {
				t.Errorf("Expected max 60.0, got %.2f", maxValue)
			}
		} else {
			t.Error("No max results returned")
		}
	})
}
