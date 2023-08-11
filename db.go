package surrealdb

import (
	"fmt"

	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/websocket"
)

// DB is a client for the SurrealDB database that holds are websocket connection.
type DB struct {
	ws websocket.WebSocket
}

// New creates a new SurrealDB client.
func New(url string, ws websocket.WebSocket) (*DB, error) {
	return &DB{ws}, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// Close closes the underlying WebSocket connection.
func (db *DB) Close() {
	_ = db.ws.Close()
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
func (db *DB) Signup(vars interface{}) (interface{}, error) {
	return db.send("signup", vars)
}

// Signin is a helper method for signing in a user.
func (db *DB) Signin(vars interface{}) (interface{}, error) {
	return db.send("signin", vars)
}

func (db *DB) Invalidate() (interface{}, error) {
	return db.send("invalidate")
}

func (db *DB) Authenticate(token string) (interface{}, error) {
	return db.send("authenticate", token)
}

// --------------------------------------------------

func (db *DB) Live(table string) (interface{}, error) {
	return db.send("live", table)
}

func (db *DB) Kill(query string) (interface{}, error) {
	return db.send("kill", query)
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

// Change a table or record in the database like a PATCH request.
func (db *DB) Change(what string, data interface{}) (interface{}, error) {
	return db.send("change", what, data)
}

// Modify applies a series of JSONPatches to a table or record.
func (db *DB) Modify(what string, data []Patch) (interface{}, error) {
	return db.send("modify", what, data)
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
// Private methods
// --------------------------------------------------

// send is a helper method for sending a query to the database.
func (db *DB) send(method string, params ...interface{}) (interface{}, error) {
	// here we send the args through our websocket connection
	resp, err := db.ws.Send(method, params)
	if err != nil {
		return nil, fmt.Errorf("sending request failed for method '%s': %w", method, err)
	}

	switch method {
	case "delete":
		return nil, nil
	case "select":
		return db.resp(method, params, resp)
	case "create":
		return db.resp(method, params, resp)
	case "update":
		return db.resp(method, params, resp)
	case "change":
		return db.resp(method, params, resp)
	case "modify":
		return db.resp(method, params, resp)
	case "insert":
		return db.resp(method, params, resp)
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
