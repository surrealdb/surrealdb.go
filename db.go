package surrealdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/surrealdb/surrealdb.go/internal/websocket"
)

const statusOK = "OK"

var (
	InvalidResponse = errors.New("invalid SurrealDB response") //nolint:stylecheck
	ErrQuery        = errors.New("error occurred processing the SurrealDB query")
)

// DB is a client for the SurrealDB database that holds are websocket connection.
type DB struct {
	ws *websocket.WebSocket
}

// New Creates a new DB instance given a WebSocket URL.
func New(url string) (*DB, error) {
	ws, err := websocket.NewWebsocket(url)
	if err != nil {
		return nil, err
	}
	return &DB{ws}, nil
}

// Unmarshal loads a SurrealDB response into a struct.
func Unmarshal(data, v interface{}) error {
	var ok bool

	assertedData, ok := data.([]interface{})
	if !ok {
		return InvalidResponse
	}
	sliceFlag := isSlice(v)

	var jsonBytes []byte
	var err error
	if !sliceFlag && len(assertedData) > 0 {
		jsonBytes, err = json.Marshal(assertedData[0])
	} else {
		jsonBytes, err = json.Marshal(assertedData)
	}
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonBytes, v)
	if err != nil {
		return err
	}

	return err
}

// UnmarshalRaw loads a raw SurrealQL response returned by Query into a struct. Queries that return with results will
// return ok = true, and queries that return with no results will return ok = false.
func UnmarshalRaw(rawData, v interface{}) (ok bool, err error) {
	var data []interface{}
	if data, ok = rawData.([]interface{}); !ok {
		return false, InvalidResponse
	}

	var responseObj map[string]interface{}
	if responseObj, ok = data[0].(map[string]interface{}); !ok {
		return false, InvalidResponse
	}

	var status string
	if status, ok = responseObj["status"].(string); !ok {
		return false, InvalidResponse
	}
	if status != statusOK {
		return false, ErrQuery
	}

	result := responseObj["result"]
	if len(result.([]interface{})) == 0 {
		return false, nil
	}
	err = Unmarshal(result, v)
	if err != nil {
		return false, err
	}

	return true, nil
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

// --------------------------------------------------
// Private methods
// --------------------------------------------------

// send is a helper method for sending a query to the database.
func (db *DB) send(method string, params ...interface{}) (interface{}, error) {
	// generate an id for the action, this is used to distinguish its response
	id := xid(16) //nolint:gomnd
	// here we send the args through our websocket connection
	resp, err := db.ws.Send(id, method, params)
	if err != nil {
		return nil, err
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
	default:
		return resp, nil
	}
}

// resp is a helper method for parsing the response from a query.
func (db *DB) resp(_ string, params []interface{}, res interface{}) (interface{}, error) {
	arg, ok := params[0].(string)

	if !ok {
		return res, nil
	}

	if strings.Contains(arg, ":") {
		arr, ok := res.([]interface{})
		if !ok {
			return nil, PermissionError{what: arg}
		}

		if len(arr) < 1 {
			return nil, PermissionError{what: arg}
		}

		return arr[0], nil
	}

	return res, nil
}

func isSlice(possibleSlice interface{}) bool {
	slice := false

	switch v := possibleSlice.(type) { //nolint:gocritic
	default:
		res := fmt.Sprintf("%s", v)
		if res == "[]" || res == "&[]" || res == "*[]" {
			slice = true
		}
	}

	return slice
}
