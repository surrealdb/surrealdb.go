package surrealql

import (
	"fmt"
	"strings"
)

// Fn creates a function call string for SurrealDB
// which can be used in SELECT queries.
func Fn(name string) *FunCall {
	return &FunCall{
		fn: name,
	}
}

// FunCall represents a function call.
// It can be used as a field directly or aliased in a SELECT query.
// It can also be used for `return` statement in queries.
type FunCall struct {
	fn     string
	args   []string
	params map[string]any
}

func (f *FunCall) Build() (sql string, params map[string]any) {
	sql = fmt.Sprintf("%s(%s)", f.fn, strings.Join(f.args, ", "))

	return sql, f.params
}

// WithArg adds a field as an argument to the function call.
// The field must exist in the select target.
func (f *FunCall) ArgFromField(name string) *FunCall {
	f.args = append(f.args, escapeIdent(name))
	return f
}

// ArgFromValue adds a value as an argument to the function call.
//
// The value can be anything that can be marshaled using CBOR.
func (f *FunCall) ArgFromValue(value any) *FunCall {
	paramName := f.generateParamName()
	f.args = append(f.args, "$"+escapeIdent(paramName))
	if f.params == nil {
		f.params = make(map[string]any)
	}
	f.addParam(paramName, value)
	return f
}

// ArgFromQuery adds a query as an argument to the function call.
func (f *FunCall) ArgFromQuery(query *SelectQuery) *FunCall {
	sql, vars := query.Build()
	f.args = append(f.args, sql)
	for k, v := range vars {
		f.addParam(k, v)
	}
	return f
}

func (f *FunCall) generateParamName() string {
	return fmt.Sprintf("fn_%s_%d", strings.ReplaceAll(f.fn, "::", "_"), len(f.params))
}

func (f *FunCall) addParam(name string, value any) {
	if f.params == nil {
		f.params = make(map[string]any)
	}
	f.params[name] = value
}
