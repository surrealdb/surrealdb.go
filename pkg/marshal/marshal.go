package marshal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/util"
)

const StatusOK = "OK"

// Used for RawQuery Unmarshaling
type RawQuery[I any] struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result I      `json:"result"`
	Detail string `json:"detail"`
}

// Unmarshal loads a SurrealDB response into a struct.
func Unmarshal(data, v interface{}) (err error) {
	var jsonBytes []byte
	if util.IsSlice(v) {
		assertedData, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("failed to deserialise response to slice: %w", constants.InvalidResponse)
		}
		jsonBytes, err = json.Marshal(assertedData)
		if err != nil {
			return fmt.Errorf("failed to deserialise response '%+v' to slice: %w", assertedData, constants.InvalidResponse)
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
func UnmarshalRaw[I any](rawData interface{}, v *[]RawQuery[I]) (err error) {
	data, err := json.Marshal(rawData)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &v)
	if err != nil {
		return
	}
	for _, v := range *v {
		if v.Status != StatusOK {
			err = errors.Join(err, fmt.Errorf("status: %s, detail: %s", v.Status, v.Detail))
		}
	}
	return
}

// SmartUnmarshal using generics for return desired type.
// Supports both raw and normal queries.
func SmartUnmarshal[I any](respond interface{}, wrapperError error) (outputs []I, err error) {
	// Handle delete
	if respond == nil || wrapperError != nil {
		return outputs, wrapperError
	}
	data, err := json.Marshal(respond)
	if err != nil {
		return outputs, err
	}
	// Needed for checking fields
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if _, isArr := respond.([]interface{}); !isArr {
		// Non Arr Normal
		var output I
		err = decoder.Decode(&output)
		if err == nil {
			outputs = append(outputs, output)
		}
	} else {
		// Arr Normal
		if err = decoder.Decode(&outputs); err != nil {
			// Arr Raw
			var rawArr []RawQuery[[]I]
			if err = json.Unmarshal(data, &rawArr); err == nil {
				outputs = make([]I, 0)
				for _, raw := range rawArr {
					if raw.Status != StatusOK {
						err = errors.Join(err, errors.New(raw.Status))
					} else {
						outputs = append(outputs, raw.Result...)
					}
				}
			}
		}
	}
	return outputs, err
}

// Used for define table name, it has no value.
type Basemodel struct{}

// Smart Marshal Errors
var (
	ErrNotStruct    = errors.New("data is not struct")
	ErrNotValidFunc = errors.New("invalid function")
)

// Smartmarshal can be used with all DB methods with generics and type safety.
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
