package surrealdb

import (
	"context"
	"time"

	"github.com/buger/jsonparser"
	"github.com/goccy/go-json"
)

type QueryResult[T any] struct {
	Result []T    `json:"result"`
	Status string `json:"status"`
	Time   string `json:"time"`
}

type QueryParams = map[string]any

type QueryResolver[T any] struct {
	err          error
	response     *RPCRawResponse
	queryResults []QueryResult[T]
	query        string
	params       QueryParams
	ctx          context.Context
}

// Query creates a new query resolver
// It would be ideal if we create a global for *DB
func Query[T any](db *DB, ctx context.Context, query string, params ...any) *QueryResolver[T] {
	var paramsData = QueryParams{}
	if len(params) > 0 {
		paramsData = params[0].(QueryParams)
	}

	resolver := &QueryResolver[T]{
		query:  query,
		params: paramsData,
		ctx:    ctx,
	}

	return resolver.resolve(db)
}

func (r *QueryResolver[T]) resolve(db *DB) *QueryResolver[T] {
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()

	result := db.QueryRaw(ctx, r.query, r.params)
	r.response = result

	if result.HasError() {
		return r
	}

	value, dataType, _, err := jsonparser.Get(r.response.Data, "result")
	if err != nil {
		r.err = err
		return r
	}

	var results []QueryResult[T]
	if dataType == jsonparser.Array {
		err = json.Unmarshal(value, &results)
		if err != nil {
			r.err = err
			return r
		}

		r.queryResults = results

		return r
	}

	return r
}

func (r *QueryResolver[T]) All() []T {
	if len(r.queryResults) == 0 {
		return []T{}
	}

	return r.queryResults[len(r.queryResults)-1].Result
}

func (r *QueryResolver[T]) First() *T {
	if len(r.queryResults) == 0 {
		return nil
	}

	results := r.queryResults[len(r.queryResults)-1].Result
	if len(results) == 0 {
		return nil
	}

	first := results[0]

	return &first
}

func (r *QueryResolver[T]) Error() error {
	return r.err
}
