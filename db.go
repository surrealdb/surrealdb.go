package surrealdb

import (
	"context"
	"fmt"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"net/url"
)

type Result[T any] struct {
	ID     string `json:"id" msgpack:"id"`
	Result T      `json:"result,omitempty" msgpack:"result,omitempty"`
}

type QueryResult[T any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result []T    `json:"result"`
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

func (db *DB) Info() (interface{}, error) {
	var info interface{}
	err := db.conn.Send(&info, "info")
	return info, err
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

// Query is a convenient method for sending a query to the database.
func (db *DB) Query(dest interface{}, sql string, vars interface{}) error {
	return db.conn.Send(&dest, "query", sql, vars)
}

// Select a table or record from the database.
func (db *DB) Select(dest interface{}, what interface{}) error {
	return db.conn.Send(dest, "select", what)
}

// Creates a table or record in the database like a POST request.
func (db *DB) Create(dest interface{}, what interface{}, data interface{}) error {
	return db.conn.Send(dest, "create", what, data)
}

// Creates a table or record in the database like a POST request.
func (db *DB) Upsert(what interface{}, data interface{}) error {
	return db.conn.Send(nil, "upsert", what, data)
}

// Update a table or record in the database like a PUT request.
func (db *DB) Update(what interface{}, data interface{}) error {
	return db.conn.Send(nil, "update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func (db *DB) Merge(what interface{}, data interface{}) error {
	return db.conn.Send(nil, "merge", what, data)
}

// Patch applies a series of JSONPatches to a table or record.
func (db *DB) Patch(what interface{}, data []Patch) error {
	return db.conn.Send(nil, "patch", what, data)
}

// Delete a table or a row from the database like a DELETE request.
func (db *DB) Delete(what interface{}) error {
	return db.conn.Send(nil, "delete", what)
}

// Insert a table or a row from the database like a POST request.
func (db *DB) Insert(what interface{}, data interface{}) error {
	return db.conn.Send(nil, "insert", what, data)
}

// --------------------------------------------------

func (db *DB) Live(table models.Table, diff bool) (string, error) {
	var id string
	if err := db.conn.Send(&id, "live", table, diff); err != nil {
		return "", err
	}

	return id, nil
}

func (db *DB) Kill(liveQueryID string) error {
	return db.liveHandler.Kill(liveQueryID)
}

// LiveNotifications returns a channel for live query.
func (db *DB) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	return db.liveHandler.LiveNotifications(liveQueryID)
}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

// send is a helper method for sending a query to the database.
//func (db *DB) send(res interface{}, method string, params ...interface{}) error {
//	// here we send the args through our websocket connection
//	resp, err := db.conn.Send(method, params)
//	if err != nil {
//		return nil, fmt.Errorf("sending request failed for method '%s': %w", method, err)
//	}
//
//	switch method {
//	case "select", "create", "upsert", "update", "merge", "patch", "insert":
//		return db.resp(method, params, resp)
//	case "delete":
//		return nil, nil
//	default:
//		return resp, nil
//	}
//}

// resp is a helper method for parsing the response from a query.
func (db *DB) resp(_ string, _ []interface{}, res interface{}) (interface{}, error) {
	if res == nil {
		return nil, constants.ErrNoRow
	}
	return res, nil
}
