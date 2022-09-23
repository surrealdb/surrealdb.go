package surrealdb

import (
	"strconv"
	"sync/atomic"
)

var _currentid uint64

// generate an incrementing id for uniqueness purposes
func xid() string {
	id := atomic.AddUint64(&_currentid, 1)
	return strconv.FormatUint(id, 16)
}
