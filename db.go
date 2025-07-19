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
	ctx context.Context
	con connection.Connection
}

// New creates a new SurrealDB client.
func New(connectionURL string) (*DB, error) {
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

	err = con.Connect()
	if err != nil {
		return nil, err
	}

	return &DB{con: con}, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// WithContext
func (db *DB) WithContext(ctx context.Context) *DB {
	db.ctx = ctx
	return db
}

// Close closes the underlying WebSocket connection.
func (db *DB) Close() error {
	return db.con.Close()
}

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ns, database string) error {
	return db.con.Use(ns, database)
}

func (db *DB) Info() (map[string]interface{}, error) {
	var info connection.RPCResponse[map[string]interface{}]
	err := db.con.Send(&info, "info")
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
func (db *DB) SignUp(authData interface{}) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.con.Send(&token, "signup", authData); err != nil {
		return "", err
	}

	if err := db.con.Let(constants.AuthTokenKey, token.Result); err != nil {
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
func (db *DB) SignIn(authData interface{}) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.con.Send(&token, "signin", authData); err != nil {
		return "", err
	}

	if err := db.con.Let(constants.AuthTokenKey, *token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

func (db *DB) Invalidate() error {
	if err := db.con.Send(nil, "invalidate"); err != nil {
		return err
	}

	if err := db.con.Unset(constants.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

func (db *DB) Authenticate(token string) error {
	if err := db.con.Send(nil, "authenticate", token); err != nil {
		return err
	}

	if err := db.con.Let(constants.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

func (db *DB) Let(key string, val interface{}) error {
	return db.con.Let(key, val)
}

func (db *DB) Unset(key string) error {
	return db.con.Unset(key)
}

func (db *DB) Version() (*VersionData, error) {
	ver, err := send[any](db, "version")
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
func (db *DB) Send(res interface{}, method string, params ...interface{}) error {
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

	return db.con.Send(&res, method, params...)
}

func (db *DB) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return db.con.LiveNotifications(liveQueryID)
}

//-------------------------------------------------------------------------------------------------------------------//

func Kill(db *DB, id string) error {
	_, err := send[any](db, "kill", id)
	return err
}

func Live(db *DB, table models.Table, diff bool) (*models.UUID, error) {
	return send[models.UUID](db, "live", table, diff)
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
// If the error is not a query error, the caller can retry the query.
// Check for RPCError to retry the query instead of QueryError.
func Query[TResult any](db *DB, sql string, vars map[string]interface{}) (*[]QueryResult[TResult], error) {
	res, err := send[[]QueryResult[cbor.RawMessage]](db, "query", sql, vars)
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

func Create[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](db, "create", what, data)
}

func Select[TResult any, TWhat TableOrRecord](db *DB, what TWhat) (*TResult, error) {
	return send[TResult](db, "select", what)
}

func Patch(db *DB, what interface{}, patches []PatchData) (*[]PatchData, error) {
	return send[[]PatchData](db, "patch", what, patches, true)
}

func Delete[TResult any, TWhat TableOrRecord](db *DB, what TWhat) (*TResult, error) {
	return send[TResult](db, "delete", what)
}

func Upsert[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](db, "upsert", what, data)
}

// Update a table or record in the database like a PUT request.
func Update[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](db, "update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func Merge[TResult any, TWhat TableOrRecord](db *DB, what TWhat, data interface{}) (*TResult, error) {
	return send[TResult](db, "merge", what, data)
}

// Insert creates records with either specified IDs or generated IDs.
//
// Insert cannot create a relationship. If you want to create a relationship,
// use InsertRelation if you need to specify the ID of the relationship,
// or use Relate if you want to create a relationship with a generated ID.
func Insert[TResult any](db *DB, what models.Table, data interface{}) (*[]TResult, error) {
	return send[[]TResult](db, "insert", what, data)
}

// Relate creates a relationship between two records in the table,
// from `in` to `out` with the specified `relation`.
//
// The `rellation` is basically a table name, so you can query it directly using SELECT
// if needed.
//
// The relation always get a generated ID. Although you can specify it when creating
// a relation using by setting Relationship.ID, it is ignored.
//
// Relationship.ID is meant for unmarshaling the relation from the database to the Relationship struct,
// in which case the ID is set to the ID of the relation record.
func Relate(db *DB, rel *Relationship) error {
	res, err := send[connection.ResponseID[models.RecordID]](db, "relate", rel.In, rel.Relation, rel.Out, rel.Data)
	if err != nil {
		return err
	}

	rel.ID = res.ID

	return nil
}

func InsertRelation(db *DB, relationship *Relationship) error {
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

	res, err := send[[]connection.ResponseID[models.RecordID]](db, "insert_relation", relationship.Relation, rel)
	if err != nil {
		return err
	}

	relationship.ID = (*res)[0].ID
	return nil
}

func QueryRaw(db *DB, queries *[]QueryStmt) error {
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

	res, err := send[[]QueryResult[cbor.RawMessage]](db, "query", preparedQuery, parameters)
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
func send[TResult any](db *DB, method string, params ...any) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.con.Send(&res, method, params...); err != nil {
		return nil, err
	}

	return res.Result, nil
}
