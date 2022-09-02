package surrealdb

import (
	"strings"
)

// DB is a client for the SurrealDB database that holds are websocket connection.
type DB struct {
	ws *WS
}

// New Creates a new DB instance given a WebSocket URL.
func New(url string) (*DB, error) {
	ws, err := NewWebsocket(url)
	if err != nil {
		return nil, err
	}
	return &DB{ws}, nil
}

// --------------------------------------------------
// Public methods
// --------------------------------------------------

// Close closes the underlying WebSocket connection.
func (self *DB) Close() {
	self.ws.Close()
}

// --------------------------------------------------

// Use is a method to select the namespace and table to use.
func (self *DB) Use(ns string, db string) (any, error) {
	return self.send("use", ns, db)
}

func (self *DB) Info() (any, error) {
	return self.send("info")
}

// SignUp is a helper method for signing up a new user.
func (self *DB) Signup(vars any) (any, error) {
	return self.send("signup", vars)
}

// Signin is a helper method for signing in a user.
func (self *DB) Signin(vars any) (any, error) {
	return self.send("signin", vars)
}

func (self *DB) Invalidate() (any, error) {
	return self.send("invalidate")
}

func (self *DB) Authenticate(token string) (any, error) {
	return self.send("authenticate", token)
}

// --------------------------------------------------

func (self *DB) Live(table string) (any, error) {
	return self.send("live", table)
}

func (self *DB) Kill(query string) (any, error) {
	return self.send("kill", query)
}

func (self *DB) Let(key string, val any) (any, error) {
	return self.send("let", key, val)
}

// Query is a convenient method for sending a query to the database.
func (self *DB) Query(sql string, vars any) (any, error) {
	return self.send("query", sql, vars)
}

// Select a table or record from the database.
func (self *DB) Select(what string) (any, error) {
	return self.send("select", what)
}

// Creates a table or record in the database like a POST request.
func (self *DB) Create(thing string, data any) (any, error) {
	return self.send("create", thing, data)
}

// Update a table or record in the database like a PUT request.
func (self *DB) Update(what string, data any) (any, error) {
	return self.send("update", what, data)
}

// Change a table or record in the database like a PATCH request.
func (self *DB) Change(what string, data any) (any, error) {
	return self.send("change", what, data)
}

// Modify applies a series of JSONPatches to a table or record.
func (self *DB) Modify(what string, data any) (any, error) {
	return self.send("modify", what, data)
}

// Delete a table or a row from the database like a DELETE request.
func (self *DB) Delete(what string) (any, error) {
	return self.send("delete", what)
}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

// send is a helper method for sending a query to the database.
func (self *DB) send(method string, params ...any) (any, error) {

	id := xid(16)

	chn, err := self.ws.Once(id, method)

	self.ws.Send(id, method, params)

	for {
		select {
		default:
		case e := <-err:
			return nil, e
		case r := <-chn:
			switch method {
			case "delete":
				return nil, nil
			case "select":
				return self.resp(method, params, r)
			case "create":
				return self.resp(method, params, r)
			case "update":
				return self.resp(method, params, r)
			case "change":
				return self.resp(method, params, r)
			case "modify":
				return self.resp(method, params, r)
			default:
				return r, nil
			}
		}
	}

}

// resp is a helper method for parsing the response from a query.
func (self *DB) resp(method string, params []any, res any) (any, error) {

	arg, ok := params[0].(string)

	peeledResponse := res.([]interface{})

	if len(peeledResponse) == 0 {
		return nil, nil
	} else if len(peeledResponse) == 1 {
		return peeledResponse[0], nil
	} else if len(peeledResponse) > 1 {
		return peeledResponse, nil
	}

	if !ok {
		return res, nil
	}

	if strings.Contains(arg, ":") {

		arr, ok := res.([]any)

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
