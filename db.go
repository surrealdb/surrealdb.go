package surrealdb

import (
	"context"
	"fmt"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"net/url"
)

type VersionData struct {
	Version   string `json:"version"`
	Build     string `json:"build"`
	Timestamp string `json:"timestamp"`
}

// DB is a client for the SurrealDB database that holds the connection.
type DB struct {
	ctx         context.Context
	conn        connection.Connection
	liveHandler connection.LiveHandler
}

// New creates a new SurrealDB client.
func New(connectionURL string) (*DB, error) {
	u, err := url.ParseRequestURI(connectionURL)
	if err != nil {
		return nil, err
	}

	scheme := u.Scheme

	newParams := connection.NewConnectionParams{
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
		BaseURL:     connectionURL,
	}
	var conn connection.Connection
	if scheme == "http" || scheme == "https" {
		conn = connection.NewHTTPConnection(newParams)
	} else if scheme == "ws" || scheme == "wss" {
		conn = connection.NewWebSocketConnection(newParams)
	} else {
		return nil, fmt.Errorf("invalid connection url")
	}

	err = conn.Connect()
	if err != nil {
		return nil, err
	}

	// Only Websocket exposes live fields, try to connect to ws
	liveScheme := "ws"
	if scheme == "wss" || scheme == "https" {
		liveScheme = "wss"
	}
	newLiveConnParams := newParams
	newLiveConnParams.BaseURL = fmt.Sprintf("%s://%s", liveScheme, u.Host)
	livecon := connection.NewWebSocketConnection(newLiveConnParams)
	err = livecon.Connect()
	if err != nil {
		return nil, err
	}

	return &DB{conn: conn, liveHandler: livecon}, nil
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
	return db.conn.Close()
}

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ns, database string) error {
	return db.conn.Use(ns, database)
}

func (db *DB) Info() (map[string]interface{}, error) {
	var info connection.RPCResponse[map[string]interface{}]
	err := db.conn.Send(&info, "info")
	return info.Result, err
}

// SignUp is a helper method for signing up a new user.
func (db *DB) SignUp(authData *models.Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.conn.Send(&token, "signup", authData); err != nil {
		return "", err
	}

	if err := db.conn.Let(connection.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return token.Result, nil
}

// SignIn is a helper method for signing in a user.
func (db *DB) SignIn(authData *models.Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := db.conn.Send(&token, "signin", authData); err != nil {
		return "", err
	}

	if err := db.conn.Let(connection.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return token.Result, nil
}

func (db *DB) Invalidate() error {
	if err := db.conn.Send(nil, "invalidate"); err != nil {
		return err
	}

	if err := db.conn.Unset(connection.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

func (db *DB) Authenticate(token string) error {
	if err := db.conn.Send(nil, "authenticate", token); err != nil {
		return err
	}

	if err := db.conn.Let(connection.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

func (db *DB) Let(key string, val interface{}) error {
	return db.conn.Let(key, val)
}

func (db *DB) Unset(key string) error {
	return db.conn.Unset(key)
}

func (db *DB) Version() (*VersionData, error) {
	var ver connection.RPCResponse[VersionData]
	if err := db.conn.Send(&ver, "version"); err != nil {
		return nil, err
	}
	return &ver.Result, nil
}

//-------------------------------------------------------------------------------------------------------------------//

func Query[T any](db *DB, sql string, vars map[string]interface{}) (*[]QueryResult[T], error) {
	var res connection.RPCResponse[[]QueryResult[T]]
	if err := db.conn.Send(&res, "query", sql, vars); err != nil {
		return nil, err
	}

	return &res.Result, nil
}

func Create[TWhat models.TableOrRecord](db *DB, what TWhat, data interface{}) error {
	return db.conn.Send(&data, "create", what, data)
}

func Select[TResult any, TWhat models.TableOrRecord](db *DB, what TWhat) (*TResult, error) {
	var res connection.RPCResponse[TResult]
	if err := db.conn.Send(&res, "select", what); err != nil {
		return nil, err
	}

	return &res.Result, nil
}

func Patch(db *DB, what interface{}, data []interface{}) (*[][]PatchData, error) {
	var patchRes connection.RPCResponse[[][]PatchData]
	err := db.conn.Send(&patchRes, "patch", what, data)
	return &patchRes.Result, err
}

func Delete[TWhat models.TableOrRecord](db *DB, what TWhat) error {
	return db.conn.Send(nil, "delete", what)
}

func Live(db *DB, table models.Table, diff bool) (string, error) {
	var id string
	if err := db.conn.Send(&id, "live", table, diff); err != nil {
		return "", err
	}

	return id, nil
}

func Kill(db *DB, liveQueryID string) error {
	return db.liveHandler.Kill(liveQueryID)
}

func LiveNotifications(db *DB, liveQueryID string) (chan connection.Notification, error) {
	return db.liveHandler.LiveNotifications(liveQueryID)
}

func Upsert(db *DB, what interface{}, data interface{}) error {
	return db.conn.Send(nil, "upsert", what, data)
}

// Update a table or record in the database like a PUT request.
func Update(db *DB, what interface{}, data interface{}) error {
	return db.conn.Send(nil, "update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func Merge(db *DB, what interface{}, data interface{}) error {
	return db.conn.Send(nil, "merge", what, data)
}

// Insert a table or a row from the database like a POST request.
func Insert(db *DB, what interface{}, data interface{}) error {
	return db.conn.Send(nil, "insert", what, data)
}
