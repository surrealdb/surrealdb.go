package surrealdb

import (
	"context"
	"fmt"
	"net/url"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

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
		BaseURL:     connectionURL,
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
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
	liveconn := connection.NewWebSocketConnection(newParams)
	err = liveconn.Connect()
	if err != nil {
		return nil, err
	}

	return &DB{conn: conn, liveHandler: liveconn}, nil
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
func (db *DB) Use(ns, database string) (interface{}, error) {
	return db.send("use", ns, database)
}

func (db *DB) Info() (interface{}, error) {
	return db.send("info")
}

// Signup is a helper method for signing up a new user.
func (db *DB) Signup(authData *models.Auth) (interface{}, error) {
	return db.send("signup", authData)
}

// Signin is a helper method for signing in a user.
func (db *DB) Signin(authData *models.Auth) (interface{}, error) {
	return db.send("signin", authData)
}

func (db *DB) Invalidate() (interface{}, error) {
	return db.send("invalidate")
}

func (db *DB) Authenticate(token string) (interface{}, error) {
	return db.send("authenticate", token)
}

func (db *DB) Let(key string, val interface{}) error {
	return db.conn.Let(key, val)
}

func (db *DB) Unset(key string) error {
	return db.conn.Unset(key)
}

// Query is a convenient method for sending a query to the database.
func (db *DB) Query(sql string, vars interface{}) (interface{}, error) {
	return db.send("query", sql, vars)
}

// Select a table or record from the database.
func (db *DB) Select(what string) (interface{}, error) {
	return db.send("select", what)
}

// Creates a table or record in the database like a POST request.
func (db *DB) Create(thing string, data interface{}) (interface{}, error) {
	return db.send("create", thing, data)
}

// Creates a table or record in the database like a POST request.
func (db *DB) Upsert(thing string, data interface{}) (interface{}, error) {
	return db.send("upsert", thing, data)
}

// Update a table or record in the database like a PUT request.
func (db *DB) Update(what string, data interface{}) (interface{}, error) {
	return db.send("update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func (db *DB) Merge(what string, data interface{}) (interface{}, error) {
	return db.send("merge", what, data)
}

// Patch applies a series of JSONPatches to a table or record.
func (db *DB) Patch(what string, data []Patch) (interface{}, error) {
	return db.send("patch", what, data)
}

// Delete a table or a row from the database like a DELETE request.
func (db *DB) Delete(what string) (interface{}, error) {
	return db.send("delete", what)
}

// Insert a table or a row from the database like a POST request.
func (db *DB) Insert(what string, data interface{}) (interface{}, error) {
	return db.send("insert", what, data)
}

// --------------------------------------------------

func (db *DB) Live(table string, diff bool) (string, error) {
	id, err := db.send("live", table, diff)
	return id.(string), err
}

func (db *DB) Kill(liveQueryID string) (interface{}, error) {
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
func (db *DB) send(method string, params ...interface{}) (interface{}, error) {
	// here we send the args through our websocket connection
	resp, err := db.conn.Send(method, params)
	if err != nil {
		return nil, fmt.Errorf("sending request failed for method '%s': %w", method, err)
	}

	switch method {
	case "select", "create", "upsert", "update", "merge", "patch", "insert":
		return db.resp(method, params, resp)
	case "delete":
		return nil, nil
	default:
		return resp, nil
	}
}

// resp is a helper method for parsing the response from a query.
func (db *DB) resp(_ string, _ []interface{}, res interface{}) (interface{}, error) {
	if res == nil {
		return nil, constants.ErrNoRow
	}
	return res, nil
}
