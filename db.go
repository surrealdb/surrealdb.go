package surrealdb

import (
	"context"
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/model"

	"github.com/surrealdb/surrealdb.go/pkg/conn"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

// DB is a client for the SurrealDB database that holds the connection.
type DB struct {
	conn conn.Connection
}

// Auth is a struct that holds surrealdb auth data for login.
type Auth struct {
	Namespace string `json:"NS,omitempty"`
	Database  string `json:"DB,omitempty"`
	Scope     string `json:"SC,omitempty"`
	Username  string `json:"user,omitempty"`
	Password  string `json:"pass,omitempty"`
}

// New creates a new SurrealDB client.
func New(ctx context.Context, url string, connection conn.Connection) (*DB, error) {
	connection, err := connection.Connect(ctx, url)
	if err != nil {
		return nil, err
	}
	return &DB{connection}, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// Close closes the underlying WebSocket connection.
func (db *DB) Close() {
	_ = db.conn.Close()
}

// --------------------------------------------------

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ctx context.Context, ns, database string) (interface{}, error) {
	return db.send(ctx, "use", ns, database)
}

func (db *DB) Info(ctx context.Context) (interface{}, error) {
	return db.send(ctx, "info")
}

// Signup is a helper method for signing up a new user.
func (db *DB) Signup(ctx context.Context, authData *Auth) (interface{}, error) {
	return db.send(ctx, "signup", authData)
}

// Signin is a helper method for signing in a user.
func (db *DB) Signin(ctx context.Context, authData *Auth) (interface{}, error) {
	return db.send(ctx, "signin", authData)
}

func (db *DB) Invalidate(ctx context.Context) (interface{}, error) {
	return db.send(ctx, "invalidate")
}

func (db *DB) Authenticate(ctx context.Context, token string) (interface{}, error) {
	return db.send(ctx, "authenticate", token)
}

// --------------------------------------------------

func (db *DB) Live(ctx context.Context, table string, diff bool) (string, error) {
	id, err := db.send(ctx, "live", table, diff)
	return id.(string), err
}

func (db *DB) Kill(ctx context.Context, liveQueryID string) (interface{}, error) {
	return db.send(ctx, "kill", liveQueryID)
}

func (db *DB) Let(ctx context.Context, key string, val interface{}) (interface{}, error) {
	return db.send(ctx, "let", key, val)
}

// Query is a convenient method for sending a query to the database.
func (db *DB) Query(ctx context.Context, sql string, vars interface{}) (interface{}, error) {
	return db.send(ctx, "query", sql, vars)
}

// Select a table or record from the database.
func (db *DB) Select(ctx context.Context, what string) (interface{}, error) {
	return db.send(ctx, "select", what)
}

// Creates a table or record in the database like a POST request.
func (db *DB) Create(ctx context.Context, thing string, data interface{}) (interface{}, error) {
	return db.send(ctx, "create", thing, data)
}

// Update a table or record in the database like a PUT request.
func (db *DB) Update(ctx context.Context, what string, data interface{}) (interface{}, error) {
	return db.send(ctx, "update", what, data)
}

// Merge a table or record in the database like a PATCH request.
func (db *DB) Merge(ctx context.Context, what string, data interface{}) (interface{}, error) {
	return db.send(ctx, "merge", what, data)
}

// Patch applies a series of JSONPatches to a table or record.
func (db *DB) Patch(ctx context.Context, what string, data []Patch) (interface{}, error) {
	return db.send(ctx, "patch", what, data)
}

// Delete a table or a row from the database like a DELETE request.
func (db *DB) Delete(ctx context.Context, what string) (interface{}, error) {
	return db.send(ctx, "delete", what)
}

// Insert a table or a row from the database like a POST request.
func (db *DB) Insert(ctx context.Context, what string, data interface{}) (interface{}, error) {
	return db.send(ctx, "insert", what, data)
}

// LiveNotifications returns a channel for live query.
func (db *DB) LiveNotifications(ctx context.Context, liveQueryID string) (chan model.Notification, error) {
	return db.conn.LiveNotifications(ctx, liveQueryID)
}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

// send is a helper method for sending a query to the database.
func (db *DB) send(ctx context.Context, method string, params ...interface{}) (interface{}, error) {
	// here we send the args through our websocket connection
	resp, err := db.conn.Send(ctx, method, params)
	if err != nil {
		return nil, fmt.Errorf("sending request failed for method '%s': %w", method, err)
	}

	switch method {
	case "select", "create", "update", "merge", "patch", "insert":
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
