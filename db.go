package surrealdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const statusOK = "OK"

var (
	ErrInvalidResponse      = errors.New("invalid SurrealDB response")
	ErrQuery                = errors.New("error occurred processing the SurrealDB query")
	ErrInvalidLoginResponse = errors.New("invalid login response")
)

// DB is a client for the SurrealDB database that holds are websocket connection.
type DB struct {
	ws *WS
}

// New Creates a new DB instance given a WebSocket URL.
func New(ctx context.Context, url string) (*DB, error) {
	ws, err := NewWebsocket(ctx, url)
	if err != nil {
		return nil, err
	}
	return &DB{ws}, nil
}

// Unmarshal loads a SurrealDB response into a struct.
func Unmarshal(data any, v any) error {
	// TODO: make this function obsolete
	// currently, we get JSON bytes from the connection, unmarshall them to a *any, marshall them back into
	// JSON and then unmarshall them into the target struct
	// This is cumbersome to use and expensive to run

	assertedData, ok := data.([]any)
	if !ok {
		return ErrInvalidResponse
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
func UnmarshalRaw(rawData any, v any) (ok bool, err error) {
	data, ok := rawData.([]any)
	if !ok {
		return false, ErrInvalidResponse
	}

	responseObj, ok := data[0].(map[string]any)
	if !ok {
		return false, ErrInvalidResponse
	}

	status, ok := responseObj["status"].(string)
	if !ok {
		return false, ErrInvalidResponse
	}
	if status != statusOK {
		return false, ErrQuery
	}

	result := responseObj["result"]
	if len(result.([]any)) == 0 {
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
func (db *DB) Close() error {
	return db.ws.Close()
}

// --------------------------------------------------

// Use is a method to select the namespace and table to use.
func (db *DB) Use(ctx context.Context, ns string, dbname string) (any, error) {
	return db.send(ctx, "use", ns, dbname)
}

func (db *DB) Info(ctx context.Context) (any, error) {
	return db.send(ctx, "info")
}

// Signup is a helper method for signing up a new user.
func (db *DB) Signup(ctx context.Context, vars any) (any, error) {
	return db.send(ctx, "signup", vars)
}

// SignupUser is a helper method for signing in a user and returning a typed response
func (db *DB) SignupUser(ctx context.Context, vars UserInfo) (*AuthenticationResult, error) {
	authResult := &AuthenticationResult{Success: false}
	result, err := db.send(ctx, "signup", vars)
	if err != nil {
		return authResult, err
	}

	err = authResult.fromQuery(result)

	return authResult, err
}

// Signin is a helper method for signing in a user.
func (db *DB) Signin(ctx context.Context, vars UserInfo) (any, error) {
	return db.send(ctx, "signin", vars)
}

// SigninUser is a helper method for signing in a user and returning a typed response
// Note: This will probably fail when signing in as a root user, but for
// a regular user(via a scope for example) we get a JWT response
func (db *DB) SigninUser(ctx context.Context, vars UserInfo) (*AuthenticationResult, error) {
	authResult := &AuthenticationResult{Success: false}
	result, err := db.send(ctx, "signin", vars)
	if err != nil {
		return authResult, err
	}
	if err != nil {
		return authResult, err
	}

	err = authResult.fromQuery(result)

	return authResult, err
}

func (db *DB) Invalidate(ctx context.Context) (any, error) {
	return db.send(ctx, "invalidate")
}

func (db *DB) Authenticate(ctx context.Context, token string) (any, error) {
	return db.send(ctx, "authenticate", token)
}

// --------------------------------------------------

func (db *DB) Live(ctx context.Context, table string) (any, error) {
	return db.send(ctx, "live", table)
}

func (db *DB) Kill(ctx context.Context, query string) (any, error) {
	return db.send(ctx, "kill", query)
}

func (db *DB) Let(ctx context.Context, key string, val any) (any, error) {
	return db.send(ctx, "let", key, val)
}

// Query is a convenient method for sending a query to the database.
func (db *DB) Query(ctx context.Context, sql string, vars any) (any, error) {
	return db.send(ctx, "query", sql, vars)
}

// Select a table or record from the database.
func (db *DB) Select(ctx context.Context, what string) (any, error) {
	return db.send(ctx, "select", what)
}

// Creates a table or record in the database like a POST request.
func (db *DB) Create(ctx context.Context, thing string, data any) (any, error) {
	return db.send(ctx, "create", thing, data)
}

// Update a table or record in the database like a PUT request.
func (db *DB) Update(ctx context.Context, what string, data any) (any, error) {
	return db.send(ctx, "update", what, data)
}

// Change a table or record in the database like a PATCH request.
func (db *DB) Change(ctx context.Context, what string, data any) (any, error) {
	return db.send(ctx, "change", what, data)
}

// Modify applies a series of JSONPatches to a table or record.
func (db *DB) Modify(ctx context.Context, what string, data []Patch) (any, error) {
	return db.send(ctx, "modify", what, data)
}

// Delete a table or a row from the database like a DELETE request.
func (db *DB) Delete(ctx context.Context, what string) (any, error) {
	return db.send(ctx, "delete", what)
}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

// send is a helper method for sending a query to the database.
func (db *DB) send(ctx context.Context, method string, params ...any) (any, error) {

	// generate an id for the action, this is used to distinguish its response
	id := xid()
	// chn: the channel where the server response will arrive, err: the channel where errors will come
	chn := db.ws.Once(id, method)
	// here we send the args through our websocket connection
	db.ws.Send(id, method, params)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case r := <-chn:
		if r.err != nil {
			return nil, r.err
		}
		switch method {
		case "delete":
			return nil, nil
		case "select":
			return db.resp(method, params, r.value)
		case "create":
			return db.resp(method, params, r.value)
		case "update":
			return db.resp(method, params, r.value)
		case "change":
			return db.resp(method, params, r.value)
		case "modify":
			return db.resp(method, params, r.value)
		default:
			return r.value, nil
		}
	}

}

// resp is a helper method for parsing the response from a query.
func (db *DB) resp(_ string, params []any, res any) (any, error) {

	arg, ok := params[0].(string)

	if !ok {
		return res, nil
	}

	// TODO: explian what that condition is for
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

func isSlice(possibleSlice interface{}) bool {
	res := fmt.Sprintf("%s", possibleSlice)
	if res == "[]" || res == "&[]" || res == "*[]" {
		return true
	}

	return false
}
