package surrealdb

import (
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
func New(url string, connection conn.Connection) (*DB, error) {
	connection, err := connection.Connect(url)
	if err != nil {
		return nil, err
	}
	return &DB{connection}, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// Close closes the underlying WebSocket connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// --------------------------------------------------

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ns, database string) (interface{}, error) {
	return db.send("use", ns, database)
}

func (db *DB) Info() (interface{}, error) {
	return db.send("info")
}

// Signup is a helper method for signing up a new user.
func (db *DB) Signup(authData *Auth) (interface{}, error) {
	return db.send("signup", authData)
}

// Signin is a helper method for signing in a user.
func (db *DB) Signin(authData *Auth) (interface{}, error) {
	return db.send("signin", authData)
}

func (db *DB) Invalidate() (interface{}, error) {
	return db.send("invalidate")
}

func (db *DB) Authenticate(token string) (interface{}, error) {
	return db.send("authenticate", token)
}

// --------------------------------------------------

func (db *DB) Live(table string, diff bool) (string, error) {
	id, err := db.send("live", table, diff)
	return id.(string), err
}

func (db *DB) Kill(liveQueryID string) (interface{}, error) {
	return db.send("kill", liveQueryID)
}

func (db *DB) Let(key string, val interface{}) (interface{}, error) {
	return db.send("let", key, val)
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

// LiveNotifications returns a channel for live query.
func (db *DB) LiveNotifications(liveQueryID string) (chan model.Notification, error) {
	return db.conn.LiveNotifications(liveQueryID)
}

// Create a relation between two records. The data parameter is optional.
func (db *DB) Relate(fromRecordId, table, toRecordId, data interface{}) (interface{}, error) {
	return db.send("relate", fromRecordId, table, toRecordId, data)
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
	case "select", "create", "update", "merge", "patch", "insert", "relate":
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
