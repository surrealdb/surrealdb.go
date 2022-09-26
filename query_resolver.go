package surrealdb

import (
	"context"
)

type QueryResolver[T any] struct {
	err    error
	query  string
	params any
	ctx    context.Context
}

func createResolver[T any](ctx context.Context, query string, params any) *QueryResolver[T] {
	resolver := &QueryResolver[T]{
		query:  query,
		params: params,
		ctx:    ctx,
	}

	return resolver
}

// Query creates a new query resolver
// It would be ideal if we create a global for *DB
func Query[T any](ctx context.Context, db *DB, query string, params ...map[string]any) *ResolvedQuery[T] {
	if len(params) == 0 {
		// Ensure there's always a default, surreal doesn't like it missing
		params = append(params, map[string]any{})
	}

	return createResolver[T](ctx, query, params[0]).runQuery(db)
}

func (resolver *QueryResolver[T]) runQuery(db *DB) *ResolvedQuery[T] {
	result, err := db.send(resolver.ctx, "query", resolver.query, resolver.params)
	if err != nil {
		panic(err)
	}
	if _, ok := result.(*RPCRawResponse); !ok {
		panic("Invalid response")
	}
	return NewResolvedQuery[T](result.(*RPCRawResponse))
}

func Create[T any](ctx context.Context, db *DB, what string, params ...map[string]any) ResolvedCreateResult[T] {
	if len(params) == 0 {
		// Ensure there's always a default, surreal doesn't like it missing
		params = append(params, map[string]any{})
	}

	return createResolver[T](ctx, what, params[0]).runCrud(db, "create")
}

func Update[T any](ctx context.Context, db *DB, what string, params ...map[string]any) ResolvedUpdateResult[T] {
	if len(params) == 0 {
		// Ensure there's always a default, surreal doesn't like it missing
		params = append(params, map[string]any{})
	}

	return createResolver[T](ctx, what, params[0]).runCrud(db, "update")
}

func Change[T any](ctx context.Context, db *DB, what string, params ...map[string]any) ResolvedUpdateResult[T] {
	if len(params) == 0 {
		// Ensure there's always a default, surreal doesn't like it missing
		params = append(params, map[string]any{})
	}

	return createResolver[T](ctx, what, params[0]).runCrud(db, "change")
}

func Modify(ctx context.Context, db *DB, what string, data []Patch) *ResolvedModifyResult {
	return createResolver[any](ctx, what, data).runModify(db)
}

func Delete[T any](ctx context.Context, db *DB, what string, params ...map[string]any) ResolvedUpdateResult[T] {
	if len(params) == 0 {
		// Ensure there's always a default, surreal doesn't like it missing
		params = append(params, map[string]any{})
	}

	return createResolver[T](ctx, what, params[0]).runCrud(db, "delete")
}

func (resolver *QueryResolver[T]) runCrud(db *DB, method string) *ResolvedCrudResult[T] {
	result, err := db.send(resolver.ctx, method, resolver.query, resolver.params)
	if err != nil {
		panic(err)
	}
	if _, ok := result.(*RPCRawResponse); !ok {
		panic("Invalid response")
	}
	return NewResolvedCrudResult[T](result.(*RPCRawResponse))
}

func (resolver *QueryResolver[T]) runModify(db *DB) *ResolvedModifyResult {
	result, err := db.send(resolver.ctx, "modify", resolver.query, resolver.params)
	if err != nil {
		panic(err)
	}
	if _, ok := result.(*RPCRawResponse); !ok {
		panic("Invalid response")
	}
	return NewResolvedModifyResult(result.(*RPCRawResponse))
}
