package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
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
// Deprecated: New is deprecated. Use Connect instead to make your
// application more robust against network issues.
func New(connectionURL string) (*DB, error) {
	return Connect(context.Background(), connectionURL)
}

// Connect creates a new SurrealDB client and connects to the database.
//
// This function incurs a network call (currently HTTP request) to the SurrealDB server to check the health of the connection in
// case of HTTP, or to establish a WebSocket connection in case of WebSocket.
//
// The provided `ctx` is used to cancel the connection attempt if needed,
// so that you control how long you want to block in case the network is not reliable
// or any other issues like OS network stack issues/settings/etc.
func Connect(ctx context.Context, connectionURL string) (*DB, error) {
	u, err := url.ParseRequestURI(connectionURL)
	if err != nil {
		return nil, err
	}

	scheme := u.Scheme

	newParams := connection.NewConnectionParams{
		Marshaler:   &models.CborMarshaler{},
		Unmarshaler: &models.CborUnmarshaler{},
		BaseURL:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	var con connection.Connection
	switch scheme {
	case "http", "https":
		con = connection.NewHTTPConnection(newParams)
	case "ws", "wss":
		con = connection.NewWebSocketConnection(newParams)
	case "memory", "mem", "surrealkv":
		return nil, fmt.Errorf("embedded database not enabled")
	default:
		return nil, fmt.Errorf("invalid connection url")
	}

	err = con.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return &DB{con: con}, nil
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

func (db *DB) Info(ctx context.Context) (map[string]interface{}, error) {
	var info connection.RPCResponse[map[string]interface{}]
	err := db.con.Send(ctx, &info, "info")
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
func (db *DB) SignUp(ctx context.Context, authData interface{}) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.con.Send(ctx, &token, "signup", authData); err != nil {
		return "", err
	}

	if err := db.con.Let(ctx, constants.AuthTokenKey, *token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// SignIn signs in an existing user.
//
// The authData parameter can be either:
//   - An Auth struct
//   - A map[string]any with keys like: "namespace", "database", "scope", "user", "pass"
//
// Example with struct:
//
//	db.SignIn(Auth{
//	  Namespace: "app",
//	  Database: "app",
//	  Access: "user",
//	  Username: "yusuke",
//	  Password: "VerySecurePassword123!",
//	})
//
// Example with map:
//
//	db.SignIn(map[string]any{
//	  "NS": "app",
//	  "DB": "app",
//	  "AC": "user",
//	  "user": "yusuke",
//	  "pass": "VerySecurePassword123!",
//	})
func (db *DB) SignIn(ctx context.Context, authData interface{}) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.con.Send(ctx, &token, "signin", authData); err != nil {
		return "", err
	}

	if err := db.con.Let(ctx, constants.AuthTokenKey, *token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

func (db *DB) Invalidate(ctx context.Context) error {
	if err := db.con.Send(ctx, nil, "invalidate"); err != nil {
		return err
	}

	if err := db.con.Unset(ctx, constants.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

func (db *DB) Authenticate(ctx context.Context, token string) error {
	if err := db.con.Send(ctx, nil, "authenticate", token); err != nil {
		return err
	}

	if err := db.con.Let(ctx, constants.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

func (db *DB) Let(ctx context.Context, key string, val interface{}) error {
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
// It is a wrapper around db.con.Send that is smarter about methods that are allowed to be sent.
// You usually want to use this method instead of db.con.Send directly.
//
// The `res` needs to be of type `*connection.RPCResponse[T]`.
//
// It returns an error in the following cases:
// - Error if the method is not allowed to be sent, which means that the request was not even sent.
// - Transport error like WebSocket message write timeout, connection closed, etc.
// - Unmarshal error if the response cannot be unmarshaled into the provided res parameter.
// - RPCError if the request was processed by SurrealDB but it failed there.
func (db *DB) Send(ctx context.Context, res interface{}, method string, params ...interface{}) error {
	allowedSendMethods := []string{"select", "create", "insert", "update", "upsert", "patch", "delete", "query"}

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

	return db.con.Send(ctx, &res, method, params...)
}

func (db *DB) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return db.con.LiveNotifications(liveQueryID)
}

//-------------------------------------------------------------------------------------------------------------------//

func Kill(ctx context.Context, db *DB, id string) error {
	_, err := send[any](ctx, db, "kill", id)
	return err
}

func Live(ctx context.Context, db *DB, table models.Table, diff bool) (*models.UUID, error) {
	return send[models.UUID](ctx, db, "live", table, diff)
}

// Query executes a query against the SurrealDB database.
//
// It returns a slice of QueryResult[TResult] where TResult is the type of the result.
//
// If the query fails, the returned error will be a `joinError` created by the `errors.Join` function,
// which contains all the errors that occurred during the query execution.
// The caller can check the Error field of each QueryResult to see if the query failed,
// or check the returned error from the Query function to see if the query failed.
//
// If the caller wants to handle the query errors, if any, it can check the Error field of each QueryResult,
// or call errors.Is(err, &QueryError{}) on the returned error to see if it is (or contains) a `QueryError`.
//
// If the error is a query error, the caller should NOT retry the query,
// because the query is already executed and the error is not recoverable,
// and often times the error is caused by a bug in the query itself.
//
// When can you safely retry the query when this function returns an error?
//
// Generally speaking, automatic retries make sense only when the error is transient,
// such as a network error, a timeout, or a server error that is not related to the query itself.
// In such cases, the caller can retry the query by calling the Query function again.
//
// For this function, the caller may retry when the error is:
//   - RPCError: because we should get a RPC error only when the RPC failed due to anything other than the query error
//   - constants.ErrTimeout: This means we send the HTTP request or a WebSocket message to SurrealDB in timely manner,
//     which is often due to temporary network issues or server overload.
//
// However, if the error is any of the following, the caller should NOT retry the query:
//   - QueryError: This means the query failed due to a syntax error, a type error, or a logical error in the query itself.
//   - Unmarshal error: This means the response from the server could not be unmarshaled into the expected type,
//     which is often due to a bug in the code or a mismatch between the expected type and the actual response type.
//   - Marshal error: This means the request could not be marshaled using CBOR,
//     which is often due to a bug in the code that tries to send something that cannot be marshaled or understood by
//     SurrealDB, such as a struct with unsupported types.
//   - Anything else: It's just safer to not retry when we aren't sure if the error is whether transient or permanent.
//
// Note that RPCError is retriable only for the `query` RPC method,
// because in other cases, the RPCError may also indicate a query error.
// For example, if you tried to insert a duplicate record using the `insert` RPC,
// you may get an RPCError saying so, which is not retriable.
//
// If you tried to insert using the `query` RPC method with `INSERT` statement,
// you may get no RPCError, but a QueryError saying so, enabling you to easily diferentiate
// between retriable and non-retriable errors.
func Query[TResult any](ctx context.Context, db *DB, sql string, vars map[string]interface{}) (*[]QueryResult[TResult], error) {
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

func Create[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data interface{}) (*TResult, error) {
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

func Upsert[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](ctx, db, "upsert", what, data)
}

// Update a table or record in the database like a PUT request.
func Update[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](ctx, db, "update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func Merge[TResult any, TWhat TableOrRecord](ctx context.Context, db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](ctx, db, "merge", what, data)
}

// Insert creates records with either specified IDs or generated IDs.
//
// Insert cannot create a relationship. If you want to create a relationship,
// use InsertRelation if you need to specify the ID of the relationship,
// or use Relate if you want to create a relationship with a generated ID.
func Insert[TResult any](ctx context.Context, db *DB, what models.Table, data interface{}) (*[]TResult, error) {
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

func QueryRaw(ctx context.Context, db *DB, queries *[]QueryStmt) error {
	preparedQuery := ""
	parameters := map[string]interface{}{}
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
	if err := db.con.Send(ctx, &res, method, params...); err != nil {
		return nil, err
	}

	return res.Result, nil
}
