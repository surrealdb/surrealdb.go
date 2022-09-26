package surrealdb

import (
	"errors"
	"fmt"
	"time"

	"github.com/buger/jsonparser"
	"github.com/goccy/go-json"
)

var (
	// ErrResolvedQueryResultIsInvalid Need a better name :|
	ErrResolvedQueryResultIsInvalid = errors.New("the result from the database response is not valid, expected an array")
)

// --------------------------------------------------

// ResolvedQuery Handles the results of database "query" responses
type ResolvedQuery[T any] struct {
	// The error from the DB or processing of the response
	err error

	response *RPCRawResponse

	results []ResultQuery[T]
}

func NewResolvedQuery[T any](response *RPCRawResponse) *ResolvedQuery[T] {
	resolved := &ResolvedQuery[T]{
		response: response,
		results:  []ResultQuery[T]{},
	}

	if response.HasError() {
		resolved.err = response.Error()
	} else {
		resolved.process()
	}

	return resolved
}

func (resolver *ResolvedQuery[T]) process() {
	rpcResult := resolver.response.Result()
	if rpcResult == nil {
		return
	}

	if rpcResult.Type != jsonparser.Array {
		resolver.err = ErrResolvedQueryResultIsInvalid
		return
	}

	err := json.Unmarshal(rpcResult.Result, &resolver.results)
	if err != nil {
		resolver.err = err
	}
}

func (resolver *ResolvedQuery[T]) HasError() bool {
	return resolver.err != nil
}

func (resolver *ResolvedQuery[T]) Error() error {
	return resolver.err
}

// Some utility/helper methods available to the user

// AllAreSuccessful Check if all are results from the query have a status of "OK"
func (resolver *ResolvedQuery[T]) AllAreSuccessful() bool {
	if resolver.HasError() {
		return false
	}

	for _, result := range resolver.results {
		if result.Status != "OK" {
			return false
		}
	}

	return true
}

// TotalTimeTaken Get the total time taken for all the queries to complete
func (resolver *ResolvedQuery[T]) TotalTimeTaken() time.Duration {
	timeTaken := time.Duration(0).Nanoseconds()

	if resolver.HasError() {
		return time.Duration(timeTaken)
	}

	for _, result := range resolver.results {
		t, err := time.ParseDuration(result.Time)
		if err != nil {
			fmt.Println("Error parsing time:", err)
			continue
		}
		timeTaken += t.Nanoseconds()
	}

	return time.Duration(timeTaken)
}

// FirstQueryResult Get the first query result from the response
//
// For clarity, our resolver.results holds something like:
// [{"result":[],"status":"OK","time":"29.375µs"}]
//
// This method will return this first object:
// {"result":[],"status":"OK","time":"29.375µs"}
func (resolver *ResolvedQuery[T]) FirstQueryResult() *ResultQuery[T] {
	if resolver.HasError() || len(resolver.results) == 0 {
		return nil
	}
	return &resolver.results[0]
}

// First Get the first item from the query
// Used when you expect only 1 item to be returned
func (resolver *ResolvedQuery[T]) First() *T {
	if resolver.HasError() || len(resolver.results) == 0 {
		return nil
	}

	firstResult := resolver.FirstQueryResult()
	if firstResult == nil || len(firstResult.Result) == 0 {
		return nil
	}

	return &firstResult.Result[0]
}

// All Get all the items from the query,
// Used when you expect > 1 result
func (resolver *ResolvedQuery[T]) All() []T {
	if resolver.HasError() || len(resolver.results) == 0 {
		return nil
	}
	firstResult := resolver.FirstQueryResult()
	if firstResult == nil || len(firstResult.Result) == 0 {
		return nil
	}

	return firstResult.Result
}

func (resolver *ResolvedQuery[T]) Results() []ResultQuery[T] {
	if resolver.HasError() || len(resolver.results) == 0 {
		return []ResultQuery[T]{}
	}

	return resolver.results
}

// --------------------------------------------------

// ResolvedCrudResult Handles the results of database, create, update, delete etc responses
type ResolvedCrudResult[T any] struct {
	// The error from the DB or processing of the response
	err error

	response *RPCRawResponse

	results []T
}

func NewResolvedCrudResult[T any](response *RPCRawResponse) *ResolvedCrudResult[T] {
	resolved := &ResolvedCrudResult[T]{
		response: response,
	}

	if response.HasError() {
		resolved.err = response.Error()
	} else {
		resolved.process()
	}

	return resolved
}

// ResolvedCreateResult Handles the results of database "create" responses
// Only returns one item in the response
type ResolvedCreateResult[T any] interface {
	HasError() bool
	Error() error
	Item() *T
}

// ResolvedUpdateResult Handles the results of database "create" responses
// Only returns one item in the response
type ResolvedUpdateResult[T any] interface {
	HasError() bool
	Error() error
	First() *T
	All() []T
}

func (resolver *ResolvedCrudResult[T]) process() {
	rpcResult := resolver.response.Result()
	if rpcResult == nil {
		return
	}

	if rpcResult.Type != jsonparser.Array {
		resolver.err = ErrResolvedQueryResultIsInvalid
		return
	}

	err := json.Unmarshal(rpcResult.Result, &resolver.results)
	if err != nil {
		resolver.err = err
	}
}

func (resolver *ResolvedCrudResult[T]) HasError() bool {
	return resolver.err != nil
}

func (resolver *ResolvedCrudResult[T]) Error() error {
	return resolver.err
}

// Some utility/helper methods available to the user

// First Get the first item from the query
// Used when you expect only 1 item to be returned
func (resolver *ResolvedCrudResult[T]) First() *T {
	if resolver.HasError() || len(resolver.results) == 0 {
		return nil
	}

	return &resolver.results[0]
}

// All Get all the items from the query,
// Used when you expect > 1 result
func (resolver *ResolvedCrudResult[T]) All() []T {
	if resolver.HasError() || len(resolver.results) == 0 {
		return nil
	}

	return resolver.results
}

func (resolver *ResolvedCrudResult[T]) Response() []T {
	return resolver.All()
}

// Item This is used for the ResolvedCreateResult interface
func (resolver *ResolvedCrudResult[T]) Item() *T {
	return resolver.First()
}

// --------------------------------------------------

// ResolvedModifyResult Handles the results of database, create, update, delete etc responses
type ResolvedModifyResult struct {
	// The error from the DB or processing of the response
	err error

	response *RPCRawResponse

	results [][]Patch
}

func NewResolvedModifyResult(response *RPCRawResponse) *ResolvedModifyResult {
	resolved := &ResolvedModifyResult{
		response: response,
	}

	if response.HasError() {
		resolved.err = response.Error()
	} else {
		resolved.process()
	}

	return resolved
}

func (resolver *ResolvedModifyResult) process() {
	rpcResult := resolver.response.Result()
	if rpcResult == nil {
		return
	}

	if rpcResult.Type != jsonparser.Array {
		resolver.err = ErrResolvedQueryResultIsInvalid
		return
	}

	err := json.Unmarshal(rpcResult.Result, &resolver.results)
	if err != nil {
		resolver.err = err
	}
}

func (resolver *ResolvedModifyResult) HasError() bool {
	return resolver.err != nil
}

func (resolver *ResolvedModifyResult) Error() error {
	return resolver.err
}

// Some utility/helper methods available to the user

// First Get the first item from the query
// Used when you expect only 1 item to be returned
func (resolver *ResolvedModifyResult) First() []Patch {
	if resolver.HasError() || len(resolver.results) == 0 {
		return []Patch{}
	}

	results := resolver.results[0]
	if len(results) == 0 {
		return []Patch{}
	}

	return results
}

// All Get all the items from the query,
// Used when you expect > 1 result
func (resolver *ResolvedModifyResult) All() [][]Patch {
	if resolver.HasError() || len(resolver.results) == 0 {
		return nil
	}

	return resolver.results
}

func (resolver *ResolvedModifyResult) Response() [][]Patch {
	return resolver.All()
}
