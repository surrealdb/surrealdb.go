package surrealdb

import (
	"context"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// The CRUD functions compile to SurrealQL sent with the query RPC method,
// rather than calling the select, create, update, upsert, merge, patch,
// delete, insert, insert_relation, relate, live and kill RPC methods, which
// SurrealDB deprecates. Each statement is the one the server builds for the
// RPC method, so results keep their shape: a single record for a record ID,
// and an array for a table.

// ReturnMode chooses what a statement returns for the records it changes.
type ReturnMode string

const (
	// ReturnAfter returns each record as it is after the change.
	ReturnAfter ReturnMode = "AFTER"
	// ReturnBefore returns each record as it was before the change.
	ReturnBefore ReturnMode = "BEFORE"
	// ReturnDiff returns the JSON Patch operations that made the change.
	ReturnDiff ReturnMode = "DIFF"
	// ReturnNone returns nothing.
	ReturnNone ReturnMode = "NONE"
)

func (m ReturnMode) valid() bool {
	switch m {
	case ReturnAfter, ReturnBefore, ReturnDiff, ReturnNone:
		return true
	}
	return false
}

// statementResult is one result of the query RPC method, with the structured
// error fields SurrealDB v3 adds to a failed statement. Details is decoded by
// connection.DecodeErrorDetails rather than the connection's codec.
type statementResult struct {
	Status  string          `json:"status"`
	Result  cbor.RawMessage `json:"result"`
	Kind    string          `json:"kind,omitempty"`
	Details cbor.RawMessage `json:"details,omitempty"`
}

// queryOne runs a single statement and decodes its result into a TResult,
// which is nil when the statement returns nothing.
//
// A failed statement is returned as the error its RPC method would have
// returned: an *RPCError that unwraps to a *ServerError carrying the kind and
// details SurrealDB v3 reports.
func queryOne[TResult any, S sendable](ctx context.Context, s S, sql string, vars map[string]any) (*TResult, error) {
	raw, err := execOne(ctx, s, sql, vars)
	if err != nil || len(raw) == 0 {
		return nil, err
	}

	var out *TResult
	if err := unmarshalerFor(s).Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("Send: error unmarshaling result: %w", err)
	}
	return out, nil
}

// execOne runs a single statement and returns its result undecoded, or the
// error of a failed statement as queryOne does.
func execOne[S sendable](ctx context.Context, s S, sql string, vars map[string]any) (cbor.RawMessage, error) {
	results, err := send[[]statementResult](ctx, s, "query", sql, vars)
	if err != nil {
		return nil, err
	}
	if results == nil || len(*results) != 1 {
		n := 0
		if results != nil {
			n = len(*results)
		}
		return nil, fmt.Errorf("expected the result of 1 statement, got %d", n)
	}

	res := (*results)[0]
	if res.Status != "ERR" {
		return res.Result, nil
	}

	var message string
	if len(res.Result) > 0 {
		if err = unmarshalerFor(s).Unmarshal(res.Result, &message); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error message: %w", err)
		}
	}
	details, err := connection.DecodeErrorDetails(res.Details)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal error details: %w", err)
	}
	return nil, connection.NewStatementError(message, res.Kind, details)
}

func unmarshalerFor[S sendable](s S) interface{ Unmarshal([]byte, any) error } {
	switch v := any(s).(type) {
	case *Session:
		return v.db.con.GetUnmarshaler()
	case *Transaction:
		return v.db.con.GetUnmarshaler()
	case *DB:
		return v.con.GetUnmarshaler()
	}
	panic("unreachable")
}

// target returns the SurrealQL for what a statement works on, binding it to
// $what. A string names a table: it is never read as a record ID.
func target(what any, vars map[string]any) string {
	vars["what"] = what
	if _, ok := what.(string); ok {
		return "type::table($what)"
	}
	return "$what"
}

// isRecord reports whether what is a single record, for which a statement
// returns that record rather than an array.
//
// A record ID whose key is a range names every record in that range, not one
// record, so it is not a single record: ONLY on such a target makes SurrealDB
// refuse the statement with "Expected a single result output when using the
// ONLY keyword".
func isRecord(what any) bool {
	switch v := what.(type) {
	case models.RecordID:
		return !isKeyRange(v.ID)
	case *models.RecordID:
		return v != nil && !isKeyRange(v.ID)
	}
	return false
}

// isKeyRange reports whether a record ID key is a range of keys. models.Range
// is generic, so it is recognised by its GetJoinString method rather than by
// its type.
func isKeyRange(id any) bool {
	_, ok := id.(interface{ GetJoinString() string })
	return ok
}

// isNil reports whether v is nil, or holds a nil map, slice, pointer,
// interface, channel or function, which the RPC methods took as no data.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Map, reflect.Slice, reflect.Pointer, reflect.Interface, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

func only(single bool) string {
	if single {
		return "ONLY "
	}
	return ""
}

// withData appends a data clause, such as CONTENT $data, unless data is nil,
// or a nil map, slice or pointer.
func withData(sql, clause string, data any, vars map[string]any) string {
	if isNil(data) {
		return sql
	}
	vars["data"] = data
	return sql + " " + clause + " $data"
}

// Create creates a new record in the database.
// S can be *DB, *Session, or *Transaction.
//
// It returns the created record for a record ID or a table name given as a
// string, and an array of records for a [models.Table] or a slice.
func Create[TResult any, TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat, data any) (*TResult, error) {
	vars := map[string]any{}
	_, isString := any(what).(string)
	sql := "CREATE " + only(isString || isRecord(what)) + target(what, vars)
	sql = withData(sql, "CONTENT", data, vars)
	return queryOne[TResult](ctx, s, sql+" RETURN AFTER", vars)
}

// Select retrieves records from the database.
// S can be *DB, *Session, or *Transaction.
//
// It returns the record, or nil when it does not exist, for a record ID, and
// an array of records for a table.
func Select[TResult any, TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat) (*TResult, error) {
	vars := map[string]any{}
	sql := "SELECT * FROM " + only(isRecord(what)) + target(what, vars)
	return queryOne[TResult](ctx, s, sql, vars)
}

// Patch applies JSON Patch operations to records in the database, and returns
// the operations that changed them, as [PatchWithResult] does with [ReturnDiff].
// S can be *DB, *Session, or *Transaction.
//
// For a table, which can return the changes of many records, use
// [PatchWithResult] with a TResult of [][]PatchData.
func Patch[TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat, patches []PatchData) (*[]PatchData, error) {
	return PatchWithResult[[]PatchData](ctx, s, what, patches, ReturnDiff)
}

// PatchWithResult applies JSON Patch operations to records in the database,
// and returns what output asks for: the records after or before the change,
// the operations that changed them, or nothing.
// S can be *DB, *Session, or *Transaction.
func PatchWithResult[TResult any, TWhat TableOrRecord, S sendable](
	ctx context.Context, s S, what TWhat, patches []PatchData, output ReturnMode,
) (*TResult, error) {
	if !output.valid() {
		return nil, fmt.Errorf("invalid return mode %q", output)
	}
	vars := map[string]any{}
	if output == ReturnNone {
		// Nothing is returned, so there is nothing to decode: the server's
		// NONE would decode to a pointer to a zero TResult with some codecs.
		// ONLY is left out as SurrealDB 2.x and 3.0 reject it with RETURN
		// NONE, and it only shapes the result.
		sql := withData("UPDATE "+target(what, vars), "PATCH", patches, vars)
		_, err := execOne(ctx, s, sql+" RETURN NONE", vars)
		return nil, err
	}
	sql := "UPDATE " + only(isRecord(what)) + target(what, vars)
	sql = withData(sql, "PATCH", patches, vars)
	return queryOne[TResult](ctx, s, sql+" RETURN "+string(output), vars)
}

// Delete removes records from the database, and returns them as they were.
// S can be *DB, *Session, or *Transaction.
func Delete[TResult any, TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat) (*TResult, error) {
	vars := map[string]any{}
	sql := "DELETE " + only(isRecord(what)) + target(what, vars)
	return queryOne[TResult](ctx, s, sql+" RETURN BEFORE", vars)
}

// Upsert creates or replaces records in the database.
// S can be *DB, *Session, or *Transaction.
func Upsert[TResult any, TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat, data any) (*TResult, error) {
	vars := map[string]any{}
	sql := "UPSERT " + only(isRecord(what)) + target(what, vars)
	sql = withData(sql, "CONTENT", data, vars)
	return queryOne[TResult](ctx, s, sql+" RETURN AFTER", vars)
}

// Update replaces records in the database like a PUT request.
// S can be *DB, *Session, or *Transaction.
func Update[TResult any, TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat, data any) (*TResult, error) {
	vars := map[string]any{}
	sql := "UPDATE " + only(isRecord(what)) + target(what, vars)
	sql = withData(sql, "CONTENT", data, vars)
	return queryOne[TResult](ctx, s, sql+" RETURN AFTER", vars)
}

// Merge merges data into records in the database like a PATCH request.
// S can be *DB, *Session, or *Transaction.
func Merge[TResult any, TWhat TableOrRecord, S sendable](ctx context.Context, s S, what TWhat, data any) (*TResult, error) {
	vars := map[string]any{}
	sql := "UPDATE " + only(isRecord(what)) + target(what, vars)
	sql = withData(sql, "MERGE", data, vars)
	return queryOne[TResult](ctx, s, sql+" RETURN AFTER", vars)
}

// Insert creates records with either specified IDs or generated IDs.
// S can be *DB, *Session, or *Transaction.
//
// Insert cannot create a relationship. If you want to create a relationship,
// use InsertRelation if you need to specify the ID of the relationship,
// or use Relate if you want to create a relationship with a generated ID.
func Insert[TResult any, S sendable](ctx context.Context, s S, what models.Table, data any) (*[]TResult, error) {
	vars := map[string]any{"table": what, "data": data}
	return queryOne[[]TResult](ctx, s, "INSERT INTO $table $data RETURN AFTER", vars)
}

// Relate creates a relationship between two records in the table
// with a generated relationship ID.
// S can be *DB, *Session, or *Transaction.
//
// The relation needs to be specified via the `Relation` field of the Relationship struct.
//
// A relation is basically a table, so you can query it directly using SELECT
// if needed.
//
// Although the Relationship struct allows you to specify the ID,
// it is ignored when you use Relate, and the ID is generated by SurrealDB.
//
// In other words, Relationship.ID is meant for unmarshaling the relation from the database to the Relationship struct,
// in which case the ID is set to the ID of the relation record generated by SurrealDB.
//
// In case you only care about the returned relationship's ID,
// use `connection.ResponseID[models.RecordID]` for the TResult type parameter.
func Relate[TResult any, S sendable](ctx context.Context, s S, rel *Relationship) (*TResult, error) {
	vars := map[string]any{"in": rel.In, "relation": rel.Relation, "out": rel.Out}
	sql := "RELATE ONLY $in->$relation->$out"
	if rel.Data != nil {
		sql = withData(sql, "CONTENT", rel.Data, vars)
	}
	return queryOne[TResult](ctx, s, sql+" RETURN AFTER", vars)
}

// InsertRelation inserts a relation between two records in the database.
// S can be *DB, *Session, or *Transaction.
//
// It creates a relationship from relationship.In to relationship.Out.
//
// The resulting relationship will have an autogenerated ID in case the Relationship.ID is nil,
// or the ID specified in the Relationship.ID field.
//
// In case you only care about the returned relationship's ID,
// use `connection.ResponseID[models.RecordID]` for the TResult type parameter.
func InsertRelation[TResult any, S sendable](ctx context.Context, s S, relationship *Relationship) (*TResult, error) {
	rel := map[string]any{
		"in":  relationship.In,
		"out": relationship.Out,
	}
	if relationship.ID != nil {
		rel["id"] = relationship.ID
	}
	for k, v := range relationship.Data {
		rel[k] = v
	}

	vars := map[string]any{"table": relationship.Relation, "data": rel}
	return queryOne[TResult](ctx, s, "INSERT RELATION INTO $table $data RETURN AFTER", vars)
}
