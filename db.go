package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type VersionData struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	Timestamp string `json:"timestamp"`
}

// DB is a client for the SurrealDB database that holds the connection.
type DB struct {
	con connection.Connection
}

// New creates a new SurrealDB client.
//
// Deprecated: New is deprecated. Use FromEndpointURLString instead.
func New(connectionURL string) (*DB, error) {
	return FromEndpointURLString(context.Background(), connectionURL)
}

// FromConnection creates a new SurrealDB client using the provided connection.
//
// Note that this function calls `conn.Connect(ctx)` for you,
// so you don't need to call it manually.
func FromConnection(ctx context.Context, conn connection.Connection) (*DB, error) {
	if err := conn.Connect(ctx); err != nil {
		return nil, err
	}

	return &DB{con: conn}, nil
}

// Deprecated: Use FromEndpointURLString instead.
func Connect(ctx context.Context, connectionURL string) (*DB, error) {
	return FromEndpointURLString(ctx, connectionURL)
}

// FromEndpointURLString creates a new SurrealDB client and connects to the database.
//
// This function incurs a network call (currently HTTP request) to the SurrealDB server to check the health of the connection in
// case of HTTP, or to establish a WebSocket connection in case of WebSocket.
//
// The provided `ctx` is used to cancel the connection attempt if needed,
// so that you control how long you want to block in case the network is not reliable
// or any other issues like OS network stack issues/settings/etc.
//
// # Connection Engines
//
// There are 2 different connection engines you can use to connect to SurrealDb backend. You can do so via Websocket or through HTTP
// connections
//
// # Via WebSocket
//
// WebSocket is required when using live queries.
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "ws://localhost:8000")
//
// or for a secure connection
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "wss://localhost:8000")
//
// # Via HTTP
//
// There are some functions that are not available on RPC when using HTTP but on WebSocket.
//
// All these except the "live" endpoint are effectively implemented in the HTTP library and
// provides the same result as though it is natively available on HTTP.
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "http://localhost:8000")
//
// or for a secure connection
//
//	db, err := surrealdb.FromEndpointURLString(ctx, "https://localhost:8000")
func FromEndpointURLString(ctx context.Context, connectionURL string) (*DB, error) {
	u, err := url.ParseRequestURI(connectionURL)
	if err != nil {
		return nil, err
	}

	conf := connection.NewConfig(u)

	if confErr := conf.Validate(); confErr != nil {
		return nil, fmt.Errorf("invalid connection config: %w", confErr)
	}

	var con connection.Connection

	switch conf.URL.Scheme {
	case "http", "https":
		con = http.New(conf)
	case "ws", "wss":
		con = gorillaws.New(conf)
	case "memory", "mem", "surrealkv":
		return nil, fmt.Errorf("embedded database not enabled")
	default:
		return nil, fmt.Errorf("invalid connection url")
	}

	return FromConnection(ctx, con)
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// WithContext
// Deprecated: WithContext is deprecated and does nothing. Use context parameters in individual method calls instead.
func (db *DB) WithContext(ctx context.Context) *DB {
	return db
}

// Close closes the underlying WebSocket connection.
func (db *DB) Close(ctx context.Context) error {
	return db.con.Close(ctx)
}

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ctx context.Context, ns, database string) error {
	return db.con.Use(ctx, ns, database)
}

func (db *DB) Info(ctx context.Context) (map[string]any, error) {
	var info connection.RPCResponse[map[string]any]
	err := connection.Send(db.con, ctx, &info, "info")
	return *info.Result, err
}

// SignUp signs up a new user.
//
// The authData parameter can be either:
//   - An Auth struct
//   - A map[string]any with keys like: "namespace", "database", "scope", "user", "pass"
//
// Example with struct:
//
//	db.SignUp(Auth{
//	  Namespace: "app",
//	  Database: "app",
//	  Access: "user",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
// Example with map:
//
//	db.SignUp(map[string]any{
//	  "NS": "app",
//	  "DB": "app",
//	  "AC": "user",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
func (db *DB) SignUp(ctx context.Context, authData any) (string, error) {
	return db.con.SignUp(ctx, authData)
}

// SignIn signs in an existing user.
//
// The authData parameter can be either:
//   - An Auth struct
//   - A map[string]any with keys like: "namespace", "database", "scope", "user", "pass"
//
// In either case, the username and the password are mandatory.
// Depending on whether namespace and database are provided or not,
// the user is signed in as a database-level user, a namespace-level user, or a root-level user.
//
// If namespace and database are provided, the user is signed in
// as a database-level user.
//
//	db.SignIn(Auth{
//	  Namespace: "app",
//	  Database: "app",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
//	db.SignIn(map[string]any{
//	  "NS": "app",
//	  "DB": "app",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// If namespace is provided but database is omitted, the user is signed in
// as a namespace-level user.
//
//	db.SignIn(Auth{
//	  Namespace: "app",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
//	db.SignIn(map[string]any{
//	  "NS": "app",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
//
// If both namespace and database are omitted, the user is signed in
// as a root-level user.
//
//	db.SignIn(Auth{
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
//	db.SignIn(map[string]any{
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
func (db *DB) SignIn(ctx context.Context, authData any) (string, error) {
	return db.con.SignIn(ctx, authData)
}

func (db *DB) Invalidate(ctx context.Context) error {
	return db.con.Invalidate(ctx)
}

func (db *DB) Authenticate(ctx context.Context, token string) error {
	return db.con.Authenticate(ctx, token)
}

func (db *DB) Let(ctx context.Context, key string, val any) error {
	return db.con.Let(ctx, key, val)
}

func (db *DB) Unset(ctx context.Context, key string) error {
	return db.con.Unset(ctx, key)
}

func (db *DB) Version(ctx context.Context) (*VersionData, error) {
	ver, err := send[any](ctx, db, "version")
	if err != nil {
		return nil, err
	}

	switch v := (*ver).(type) {
	case map[string]any:
		ver, ok := v["version"].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected version data: %v", v)
		}

		build, ok := v["build"].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected build data: %v", v)
		}

		timestamp, ok := v["timestamp"].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected timestamp data: %v", v)
		}
		return &VersionData{
			Version:   ver,
			Build:     build,
			Timestamp: timestamp,
		}, nil
	case string:
		ver := strings.TrimPrefix(v, "surrealdb-")
		return &VersionData{Version: ver}, nil
	default:
		return nil, fmt.Errorf("unexpected version data: %s (%T)", v, v)
	}
}

// Send sends a request to the SurrealDB server.
//
// It is a wrapper around [connection.Send], which is used by various RPC methods like
// [Query], [Insert] and so on.
//
// Compared to the original [connection.Send], [Send] is smarter about methods that are allowed to be sent.
// You usually want to use this function than using [connection.Send] directly.
//
// This function is limited to a selected set of RPC methods listed below:
//
// - select
// - create
// - insert
// - insert_relation
// - kill
// - live
// - merge
// - relate
// - update
// - upsert
// - patch
// - delete
// - query
//
// The `res` needs to be of type `*connection.RPCResponse[T]`.
//
// It returns an error in the following cases:
// - Error if the method is not allowed to be sent, which means that the request was not even sent.
// - Transport error like WebSocket message write timeout, connection closed, etc.
// - Unmarshal error if the response cannot be unmarshaled into the provided res parameter.
// - RPCError if the request was processed by SurrealDB but it failed there.
func Send[Result any](ctx context.Context, db *DB, res *connection.RPCResponse[Result], method string, params ...any) error {
	allowedSendMethods := []string{
		"select", "create", "insert", "insert_relation",
		"kill", "live", "merge", "relate", "update", "upsert",
		"patch", "delete", "query",
	}

	allowed := false
	for i := 0; i < len(allowedSendMethods); i++ {
		if strings.EqualFold(allowedSendMethods[i], strings.ToLower(method)) {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("provided method is not allowed")
	}

	return connection.Send(db.con, ctx, res, method, params...)
}

func (db *DB) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return db.con.LiveNotifications(liveQueryID)
}

func (db *DB) CloseLiveNotifications(liveQueryID string) error {
	return db.con.CloseLiveNotifications(liveQueryID)
}

//-------------------------------------------------------------------------------------------------------------------//

func Kill(ctx context.Context, db *DB, id string) error {
	// First kill the live query on the server
	_, err := send[any](ctx, db, "kill", id)
	if err != nil {
		return err
	}

	// Then close the notification channel to prevent leaks
	return db.CloseLiveNotifications(id)
}

func Live(ctx context.Context, db *DB, table models.Table, diff bool) (*models.UUID, error) {
	return send[models.UUID](ctx, db, "live", table, diff)
}

// Query executes a query against the SurrealDB database.
//
// [Query] supports:
//
//   - Full SurrealQL syntax including transactions
//   - Parameterized queries for security
//   - Typed results with generics
//   - Multiple statements in a single call
//
// It takes a SurrealQL query to be executed, and the variables to parameterize the query,
// and returns a slice of [QueryResult] whose type parameter is the result type.
//
// # Examples
//
// Execute a SurrealQL query with typed results:
//
//	results, err := surrealdb.Query[[]Person](
//	  context.Background(),
//	  db,
//	  "SELECT * FROM persons WHERE age > $minAge",
//	  map[string]any{
//	      "minAge": 18,
//	  },
//	)
//
// You can also use Query for transactions with variables:
//
//	transactionResults, err := surrealdb.Query[[]any](
//	  context.Background(),
//	  db,
//	  `
//	  BEGIN TRANSACTION;
//	  CREATE person:$johnId SET name = $johnName, age = $johnAge;
//	  CREATE person:$janeId SET name = $janeName, age = $janeAge;
//	  COMMIT TRANSACTION;
//	  `,
//	  map[string]any{
//	      "johnId": "john",
//	      "johnName": "John",
//	      "johnAge": 30,
//	      "janeId": "jane",
//	      "janeName": "Jane",
//	      "janeAge": 25,
//	  },
//	)
//
// Or use a single CREATE with content variable:
//
//	createResult, err := surrealdb.Query[[]Person](
//	    context.Background(),
//	    db,
//	    "CREATE person:$id CONTENT $content",
//	    map[string]any{
//			"id": "alice",
//			"content": map[string]any{
//				"name": "Alice",
//				"age": 28,
//				"city": "New York",
//			},
//		},
//	)
//
// # Handling errors
//
// If the query fails, the returned error will be a `joinError` created by the [errors.Join] function,
// which contains all the errors that occurred during the query execution.
// The caller can check the Error field of each [QueryResult] to see if the query failed,
// or check the returned error from the [Query] function to see if the query failed.
//
// If the caller wants to handle the query errors, if any, it can check the Error field of each [QueryResult],
// or call:
//
//	errors.Is(err, &surrealdb.QueryError{})
//
// on the returned error to see if it is (or contains) a [QueryError].
//
// # Query errors are non-retriable
//
// If the error is a [QueryError], the caller should NOT retry the query,
// because the query is already executed and the error is not recoverable,
// and often times the error is caused by a bug in the query itself.
//
// # When can you safely retry the query when this function returns an error?
//
// Generally speaking, automatic retries make sense only when the error is transient,
// such as a network error, a timeout, or a server error that is not related to the query itself.
// In such cases, the caller can retry the query by calling the [Query] function again.
//
// For this function, the caller may retry when the error is:
//   - [RPCError]: because we should get a RPC error only when the RPC failed due to anything other than the query error
//   - [constants.ErrTimeout]: This means we send the HTTP request or a WebSocket message to SurrealDB in timely manner,
//     which is often due to temporary network issues or server overload.
//
// # What non-retriable errors will Query return?
//
// However, if the error is any of the following, the caller should NOT retry the query:
//   - [QueryError]: This means the query failed due to a syntax error, a type error, or a logical error in the query itself.
//   - Unmarshal error: This means the response from the server could not be unmarshaled into the expected type,
//     which is often due to a bug in the code or a mismatch between the expected type and the actual response type.
//   - Marshal error: This means the request could not be marshaled using CBOR,
//     which is often due to a bug in the code that tries to send something that cannot be marshaled or understood by
//     SurrealDB, such as a struct with unsupported types.
//   - Anything else: It's just safer to not retry when we aren't sure if the error is whether transient or permanent.
//
// # RPCError is retriable only for Query
//
// Note that [RPCError] is retriable only for the [Query] RPC method,
// because in other cases, the [RPCError] may also indicate a query error.
// For example, if you tried to insert a duplicate record using the [Insert] RPC,
// you may get an [RPCError] saying so, which is not retriable.
//
// If you tried to insert the same duplicate record using the [Query] RPC method with `INSERT` statement,
// you may get no [RPCError], but a [QueryError] saying so, enabling you to easily diferentiate
// between retriable and non-retriable errors.
func Query[TResult any](ctx context.Context, db *DB, sql string, vars map[string]any) (*[]QueryResult[TResult], error) {
	res, err := send[[]QueryResult[cbor.RawMessage]](ctx, db, "query", sql, vars)
	if err != nil {
		return nil, err
	}

	// The query errors, if any
	var (
		errs error
	)

	// We unmarshal []QueryResult[cbor.RawMessage] first,
	// and then unmarshal each cbor.RawMessage to TResult.
	// This is necessary because the Result field can be a string in case Status is "ERR".
	// In that case if we directly unmarshaled to TResult using []QueryResult[TResult],
	// it would fail with "cannot unmarshal UTF-8 text string into Go struct field"
	// because CBOR string cannot be unmarshaled into TResult except when TResult is a string.
	qr := make([]QueryResult[TResult], len(*res))

	for i, result := range *res {
		var (
			r TResult
			e *QueryError
		)
		if result.Status == "ERR" {
			var errMsg string
			if result.Result != nil {
				if err := db.con.GetUnmarshaler().Unmarshal(result.Result, &errMsg); err != nil {
					return nil, fmt.Errorf("failed to unmarshal error message: %w", err)
				}
			}
			e = &QueryError{
				Message: errMsg,
			}
			errs = errors.Join(errs, e)
		} else if result.Result != nil {
			if err := db.con.GetUnmarshaler().Unmarshal(result.Result, &r); err != nil {
				return nil, fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}
		qr[i] = QueryResult[TResult]{
			Status: result.Status,
			Time:   result.Time,
			Result: r,
			Error:  e,
		}
	}

	return &qr, errs
}

func Create[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data any) (*TResult, error) {
	return send[TResult](ctx, db, "create", what, data)
}

func Select[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat) (*TResult, error) {
	return send[TResult](ctx, db, "select", what)
}

// Patches either all records in a table or a single record with specified patches.
func Patch[TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, patches []PatchData) (*[]PatchData, error) {
	return send[[]PatchData](ctx, db, "patch", what, patches, true)
}

func Delete[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat) (*TResult, error) {
	return send[TResult](ctx, db, "delete", what)
}

func Upsert[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data any) (*TResult, error) {
	return send[TResult](ctx, db, "upsert", what, data)
}

// Update a table or record in the database like a PUT request.
func Update[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data any) (*TResult, error) {
	return send[TResult](ctx, db, "update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func Merge[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data any) (*TResult, error) {
	return send[TResult](ctx, db, "merge", what, data)
}

// Insert creates records with either specified IDs or generated IDs.
//
// Insert cannot create a relationship. If you want to create a relationship,
// use InsertRelation if you need to specify the ID of the relationship,
// or use Relate if you want to create a relationship with a generated ID.
func Insert[TResult any](ctx context.Context, db *DB, what models.Table, data any) (*[]TResult, error) {
	return send[[]TResult](ctx, db, "insert", what, data)
}

// Relate creates a relationship between two records in the table
// with a generated relationship ID.
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
func Relate[TResult any](ctx context.Context, db *DB, rel *Relationship) (*TResult, error) {
	return send[TResult](ctx, db, "relate", rel.In, rel.Relation, rel.Out, rel.Data)
}

// InsertRelation inserts a relation between two records in the database.
//
// It creates a relationship from relationship.In to relationship.Out.
//
// The resulting relationship will have an autogenerated ID in case the Relationship.ID is nil,
// or the ID specified in the Relationship.ID field.
//
// In case you only care about the returned relationship's ID,
// use `connection.ResponseID[models.RecordID]` for the TResult type parameter.
func InsertRelation[TResult any](ctx context.Context, db *DB, relationship *Relationship) (*TResult, error) {
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

	return send[TResult](ctx, db, "insert_relation", relationship.Relation, rel)
}

// QueryRaw composes a query from the provided QueryStmt objects,
// and execute it using the query RPC method.
//
// You may want to use [Query] with [github.com/surrealdb/surrealdb.go/contrib/surrealql] instead.
func QueryRaw(ctx context.Context, db *DB, queries *[]QueryStmt) error {
	preparedQuery := ""
	parameters := map[string]any{}
	for i := 0; i < len(*queries); i++ {
		// append query
		preparedQuery += fmt.Sprintf("%s;", (*queries)[i].SQL)
		for k, v := range (*queries)[i].Vars {
			parameters[k] = v
		}
	}

	if preparedQuery == "" {
		return fmt.Errorf("no query to run")
	}

	res, err := send[[]QueryResult[cbor.RawMessage]](ctx, db, "query", preparedQuery, parameters)
	if err != nil {
		return err
	}

	for i := 0; i < len(*queries); i++ {
		// assign results
		(*queries)[i].Result = (*res)[i]
		(*queries)[i].unmarshaler = db.con.GetUnmarshaler()
	}

	return nil
}

// send is a helper function to send a request to the SurrealDB server
// in case the expected response is a connection.RPCResponse[TResult].
// If one expects other types of responses, use db.con.Send directly.
func send[TResult any](ctx context.Context, db *DB, method string, params ...any) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := connection.Send(db.con, ctx, &res, method, params...); err != nil {
		return nil, err
	}

	return res.Result, nil
}
