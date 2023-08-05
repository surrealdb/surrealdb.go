package surrealdb

import (
	"encoding/json"
	"errors"
	"fmt"

	"reflect"

	"github.com/surrealdb/surrealdb.go/pkg/websocket"
)

const statusOK = "OK"

var (
	InvalidResponse = errors.New("invalid SurrealDB response") //nolint:stylecheck
	ErrQuery        = errors.New("error occurred processing the SurrealDB query")
	ErrNoRow        = errors.New("error no row")
)

// DB is a client for the SurrealDB database that holds are websocket connection.
type DB struct {
	ws websocket.WebSocket
}

// New creates a new SurrealDB client.
func New(url string, ws websocket.WebSocket) (*DB, error) {
	return &DB{ws}, nil
}

// Unmarshal loads a SurrealDB response into a struct.
func Unmarshal(data, v interface{}) error {
	var jsonBytes []byte
	var err error
	if isSlice(v) {
		assertedData, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("failed to deserialise response to slice: %w", InvalidResponse)
		}
		jsonBytes, err = json.Marshal(assertedData)
		if err != nil {
			return fmt.Errorf("failed to deserialise response '%+v' to slice: %w", assertedData, InvalidResponse)
		}
	} else {
		jsonBytes, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to deserialise response '%+v' to object: %w", data, err)
		}
	}
	if err != nil {
		return err
	}

	err = json.Unmarshal(jsonBytes, v)
	if err != nil {
		return fmt.Errorf("failed unmarshaling jsonBytes '%+v': %w", jsonBytes, err)
	}
	return nil
}

// UnmarshalRaw loads a raw SurrealQL response returned by Query into a struct. Queries that return with results will
// return ok = true, and queries that return with no results will return ok = false.
func UnmarshalRaw(rawData, v interface{}) (ok bool, err error) {
	var data []interface{}
	if data, ok = rawData.([]interface{}); !ok {
		return false, fmt.Errorf("failed raw unmarshaling to interface slice: %w", InvalidResponse)
	}

	var responseObj map[string]interface{}
	if responseObj, ok = data[0].(map[string]interface{}); !ok {
		return false, fmt.Errorf("failed mapping to response object: %w", InvalidResponse)
	}

	var status string
	if status, ok = responseObj["status"].(string); !ok {
		return false, fmt.Errorf("failed retrieving status: %w", InvalidResponse)
	}
	if status != statusOK {
		return false, fmt.Errorf("status was not ok: %w", ErrQuery)
	}

	result := responseObj["result"]
	if len(result.([]interface{})) == 0 {
		return false, nil
	}
	err = Unmarshal(result, v)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return true, nil
}

// Used for RawQuery Unmarshaling
type RawQuery[I any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result I      `json:"result"`
	Detail string `json:"detail"`
}

// SmartUnmarshal using generics for return desired type.
// Supports both raw and normal queries.
func SmartUnmarshal[I any](respond interface{}, wrapperError error) (data I, err error) {
	if wrapperError != nil {
		return data, wrapperError
	}
	var bytes []byte
	if arrResp, isArr := respond.([]interface{}); len(arrResp) > 0 {
		if dataMap, ok := arrResp[0].(map[string]interface{}); ok && isArr {
			if _, ok := dataMap["status"]; ok {
				if bytes, err = json.Marshal(respond); err == nil {
					var raw []RawQuery[I]
					if err = json.Unmarshal(bytes, &raw); err == nil {
						if raw[0].Status != statusOK {
							err = fmt.Errorf("%s: %s", raw[0].Status, raw[0].Detail)
						}
						data = raw[0].Result
					}
				}
				return data, err
			}
		}
	}
	if bytes, err = json.Marshal(respond); err == nil {
		err = json.Unmarshal(bytes, &data)
	}
	return data, err
}

// SmartUnmarshalAll takes an interface input and unmarshals it into the given
// type, traversing the nested slices and maps, regardless of the input being
// wrapped in RawQuery or not. When any error is seen during the unmarshaling,
// it would keep the return value along with the error message.
func SmartUnmarshalAll[I any](input interface{}) ([]I, error) {
	// NOTE: We could use a dedicated error type to provide unmarshal failure,
	// which may allow users to ignore some erroneous data being returned from
	// the database.
	return handleInterfaces[I](input)
}

func handleInterfaces[I any](input interface{}) ([]I, error) {
	var result []I
	var errs []error
	inputs, ok := input.([]interface{})
	if !ok {
		return handleInput[I](input)
	}

	for _, i := range inputs {
		data, err := handleInterfaces[I](i)
		errs = append(errs, err)
		result = append(result, data...)
	}
	if len(errs) > 0 {
		return result, errors.Join(errs...)
	}
	return result, nil
}

// errNotRawQuery is used only internally for checking error content, and thus
// not exposed and should not be used from client directly.
var errNotRawQuery = errors.New("not a RawQuery")

// handleInput takes in an interface input and unmarshals it into the given
// type, regardless of the input wrapped in RawQuery or not. The input is
// expected to be a single entry, rather than a slice.
//
// In case of the input being a RawQuery, though, it could have a slice inside,
// and thus would recurse to check for interface input.
func handleInput[I any](input interface{}) ([]I, error) {
	unwrapped, err := handleAsRawQuery(input)
	if err != nil {
		// If error has to do with the input status, return the error.
		if !errors.Is(err, errNotRawQuery) {
			return nil, err
		}

		// If error is unmarshaling failure to RawQuery format, try to get the
		// single data as is.
		data, err := handleAsData[I](input)
		if err != nil {
			return nil, err
		}
		return []I{data}, nil
	}
	return handleInterfaces[I](unwrapped)
}

// handleAsData takes in an interface input and unmarshals it into the given
// type. The input is expected to be a single entry, rather than a slice.
func handleAsData[I any](input interface{}) (I, error) {
	var i I
	x, err := json.Marshal(input)
	if err != nil {
		return i, fmt.Errorf("failed to marshal to bytes: %w", err)
	}
	err = json.Unmarshal(x, &i)
	if err != nil {
		return i, fmt.Errorf("failed to unmarshal to RawQuery: %w", err)
	}
	return i, nil
}

// handleAsRawQuery takes in an interface input and unmarshals it into the
// RawQuery format. The input is expected to be a single entry, rather than a
// slice. It would return `errNotRawQuery` if the input is not in the RawQuery,
// so that the rest of the process could try unmarshalling against a separate
// type.
func handleAsRawQuery(input interface{}) (interface{}, error) {
	x, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to bytes: %w", err)
	}

	var raw RawQuery[any]
	err = json.Unmarshal(x, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to RawQuery: %w", err)
	}

	if raw.Status == "" {
		return nil, fmt.Errorf("%w", errNotRawQuery)
	}

	if raw.Status != statusOK {
		return nil, fmt.Errorf("%s: %s", raw.Status, raw.Detail)
	}

	return raw.Result, nil
}

// Used for define table name, it has no value.
type Basemodel struct{}

// Smart Marshal Errors
var (
	ErrNotStruct    = errors.New("data is not struct")
	ErrNotValidFunc = errors.New("invalid function")
)

// SmartUnmarshal can be used with all DB methods with generics and type safety.
// This handles errors and can use any struct tag with `BaseModel` type.
// Warning: "ID" field is case sensitive and expect string.
// Upon failure, the following will happen
// 1. If there are some ID on struct it will fill the table with the ID
// 2. If there are struct tags of the type `Basemodel`, it will use those values instead
// 3. If everything above fails or the IDs do not exist, SmartUnmarshal will use the struct name as the table name.
func SmartMarshal[I any](inputfunc interface{}, data I) (output interface{}, err error) {
	var table string
	datatype := reflect.TypeOf(data)
	datavalue := reflect.ValueOf(data)
	if datatype.Kind() == reflect.Pointer {
		datatype = datatype.Elem()
		datavalue = datavalue.Elem()
	}
	if datatype.Kind() == reflect.Struct {
		if _, ok := datavalue.Field(0).Interface().(Basemodel); ok {
			if temptable, ok := datatype.Field(0).Tag.Lookup("table"); ok {
				table = temptable
			} else {
				table = reflect.TypeOf(data).Name()
			}
		}
		if id, ok := datatype.FieldByName("ID"); ok {
			if id.Type.Kind() == reflect.String {
				if str, ok := datavalue.FieldByName("ID").Interface().(string); ok {
					if str != "" {
						table = str
					}
				}
			}
		}
	} else {
		return nil, ErrNotStruct
	}
	if function, ok := inputfunc.(func(thing string, data interface{}) (interface{}, error)); ok {
		return function(table, data)
	}
	if function, ok := inputfunc.(func(thing string) (interface{}, error)); ok {
		return function(table)
	}
	return nil, ErrNotValidFunc
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
	default:
		return resp, nil
	}
}

// resp is a helper method for parsing the response from a query.
func (db *DB) resp(_ string, _ []interface{}, res interface{}) (interface{}, error) {
	if res == nil {
		return nil, ErrNoRow
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
