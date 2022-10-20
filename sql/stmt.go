package sql

import (
	"database/sql/driver"
	"fmt"
	"regexp"
)

var inputRegex = regexp.MustCompile("$[a-zA-Z]+")

type Stmt struct {
	conn *Conn

	rawQuery string
	method   string
	thing    string
}

func (s *Stmt) Close() error {
	return nil
}

func (s *Stmt) NumInput() int {
	inputs := inputRegex.FindAllString(s.rawQuery, -1)
	return len(inputs)
}

func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	rows, err := s.execute(args)
	if err != nil {
		return nil, err
	}

	cols := rows.Columns()
	data := make([]driver.Value, len(cols))

	driverResult := Result{}
	for err := rows.Next(data); err != nil; err = rows.Next(data) {
		driverResult.AffectedRows++
	}
	return driverResult, nil
}

func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := s.execute(args)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (s *Stmt) execute(args []driver.Value) (*Rows, error) {
	argInterfaces := make([]interface{}, len(args)+1)
	argInterfaces[0] = s.rawQuery

	for idx, arg := range args {
		argInterfaces[idx+1] = s.convertArgument(arg)
	}

	res, err := s.conn.Send("query", argInterfaces...)
	if err != nil {
		return nil, fmt.Errorf("error during Exec: %w", err)
	}

	arr, ok := res.([]interface{})
	if !ok || len(arr) != 1 {
		// No idea what the result is
		return nil, fmt.Errorf("unknown result")
	}

	lookup, ok := arr[0].(map[string]interface{})
	if !ok {
		// No idea what the result is
		return nil, fmt.Errorf("unknown result, expected map")
	}

	status, _ := lookup["status"]
	//duration, _ := lookup["time"]

	switch status {
	case "ERR":
		detail, _ := lookup["detail"]
		return nil, fmt.Errorf("query error: %s", detail)
	case "OK":
		result, _ := lookup["result"]
		rows, ok := result.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unknown result value")
		}

		return &Rows{RawData: rows}, nil
	default:
		return nil, fmt.Errorf("unknown response status: %s", status)
	}
}

func (s *Stmt) convertArgument(val driver.Value) interface{} {
	return val
}

type Result struct {
	AffectedRows int64
}

func (r Result) LastInsertId() (int64, error) {
	return 0, fmt.Errorf("surrealDB does not support numeric/int64 auto-increment ids")
}

func (r Result) RowsAffected() (int64, error) {
	return r.AffectedRows, nil
}
