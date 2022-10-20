package sql

import (
	"database/sql/driver"
	"fmt"
	"io"
	"sort"
)

type Rows struct {
	RawData    []interface{}
	nextInLine int

	detectedColumns []string
}

func (r *Rows) Columns() []string {
	if r.nextInLine >= len(r.RawData) {
		return []string{}
	}

	lookup, ok := r.RawData[r.nextInLine].(map[string]interface{})
	if !ok {
		return []string{} // This should never happen, but we cannot return an err...
	}

	r.detectedColumns = make([]string, 0, len(lookup))
	for key := range lookup {
		r.detectedColumns = append(r.detectedColumns, key)
	}

	// Because "lookup" is a map, this order is NOT guaranteed to be the order as queried by the user...
	// So let's sort it to at least be consistent
	sort.Strings(r.detectedColumns)
	
	return r.detectedColumns
}

func (r *Rows) Close() error {
	return nil
}

func (r *Rows) Next(dest []driver.Value) error {
	if r.nextInLine >= len(r.RawData) {
		return io.EOF
	}

	lookup, ok := r.RawData[r.nextInLine].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unknown format of row")
	}

	for idx, colName := range r.detectedColumns {
		dest[idx] = lookup[colName]
	}

	r.nextInLine++
	return nil
}
