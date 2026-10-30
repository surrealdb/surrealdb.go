package surrealdb

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

var (
	// A name is namespaced (string::len, fn::greet, ml::model) or one of the
	// built-ins without a namespace. A bare word would be parsed as whatever it
	// names, so "DELETE" would send RETURN DELETE($arg0).
	functionName    = regexp.MustCompile(`^(?:[a-zA-Z0-9_]+(?:::[a-zA-Z0-9_]+)+|count|not|rand|sleep)$`)
	functionVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// Run calls a function with the given arguments and returns its result. The
// function can be a built-in one, such as "string::len" or "count", or one
// defined with DEFINE FUNCTION, such as "fn::greet".
//
// The name must have segments joined by "::". The only names without "::"
// are the built-ins count, not, rand and sleep. Any other name is refused
// before anything is sent.
// S can be *DB, *Session, or *Transaction.
func Run[TResult any, S sendable](ctx context.Context, s S, name string, args ...any) (*TResult, error) {
	return RunVersion[TResult](ctx, s, name, "", args...)
}

// RunVersion calls a function, such as a machine learning model "ml::model",
// at the given version, such as "1.0.0". An empty version is the same as
// [Run].
// S can be *DB, *Session, or *Transaction.
func RunVersion[TResult any, S sendable](ctx context.Context, s S, name, version string, args ...any) (*TResult, error) {
	if !functionName.MatchString(name) {
		return nil, fmt.Errorf("invalid function name %q", name)
	}

	var sql strings.Builder
	sql.WriteString(name)
	if version != "" {
		if !functionVersion.MatchString(version) {
			return nil, fmt.Errorf("invalid function version %q", version)
		}
		sql.WriteString("<" + version + ">")
	}

	vars := make(map[string]any, len(args))
	sql.WriteString("(")
	for i, arg := range args {
		name := "arg" + strconv.Itoa(i)
		vars[name] = arg
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString("$" + name)
	}
	sql.WriteString(")")

	return queryOne[TResult](ctx, s, "RETURN "+sql.String(), vars)
}

// APIRequest is a request to an endpoint defined with DEFINE API.
type APIRequest struct {
	// Method is the HTTP method, "get" when empty.
	Method string
	// Body is the request body, which can be any value.
	Body any
	// Headers are the request headers.
	Headers map[string]string
	// Query holds the query parameters.
	Query map[string]string
}

// APIResponse is the response of an endpoint defined with DEFINE API.
type APIResponse[T any] struct {
	// Status is the HTTP status code.
	Status int `json:"status"`
	// Headers are the response headers.
	Headers map[string]string `json:"headers,omitempty"`
	// Body is the response body.
	Body T `json:"body"`
}

// API invokes an endpoint defined with DEFINE API, at a path such as
// "/users/123", in the namespace and database of the session, without going
// through the server's HTTP router. A nil request is a GET without a body.
//
// As with net/http, a response with an unsuccessful status is not an error:
// check Status. The body of such a response is often a string, such as
// "Not found". When it does not fit TBody, Body is the zero value.
// S can be *DB, *Session, or *Transaction.
func API[TBody any, S sendable](ctx context.Context, s S, path string, req *APIRequest) (*APIResponse[TBody], error) {
	if req == nil {
		req = &APIRequest{}
	}

	method := strings.ToLower(req.Method)
	if method == "" {
		method = "get"
	}

	options := map[string]any{"method": method}
	if req.Body != nil {
		options["body"] = req.Body
	}
	if req.Headers != nil {
		options["headers"] = req.Headers
	}
	if req.Query != nil {
		options["query"] = req.Query
	}

	vars := map[string]any{"path": path, "request": options}
	raw, err := queryOne[APIResponse[cbor.RawMessage]](ctx, s, "RETURN api::invoke($path, $request)", vars)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("api::invoke returned nothing for %s %s", method, path)
	}

	res := &APIResponse[TBody]{Status: raw.Status, Headers: raw.Headers}
	if len(raw.Body) > 0 {
		if err := unmarshalerFor(s).Unmarshal(raw.Body, &res.Body); err != nil {
			if res.Status >= 200 && res.Status < 300 {
				return nil, fmt.Errorf("api::invoke: error unmarshaling body: %w", err)
			}
			// The body of an error, such as "Not found", need not have the
			// type of a successful one.
			var zero TBody
			res.Body = zero
		}
	}
	return res, nil
}

// GQL runs an ISO GQL query, with the gql method of servers built with GQL
// support, and returns one result per statement, as [Query] does.
// S can be *DB, *Session, or *Transaction.
func GQL[TResult any, S sendable](ctx context.Context, s S, query string, vars map[string]any) (*[]QueryResult[TResult], error) {
	return queryWith[TResult](ctx, s, "gql", query, vars)
}
