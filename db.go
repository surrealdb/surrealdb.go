package surrealdb

import (
	"strings"
)

type DB struct {
	ws *WS
}

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

func (self *DB) Close() {
	self.ws.Close()
}

// --------------------------------------------------

func (self *DB) Use(ns string, db string) (any, error) {
	return self.send("use", ns, db)
}

func (self *DB) Info() (any, error) {
	return self.send("info")
}

func (self *DB) Signup(vars map[string]any) (any, error) {
	return self.send("signup")
}

func (self *DB) Signin(vars map[string]any) (any, error) {
	return self.send("signin")
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

func (self *DB) Query(sql string, vars map[string]any) (any, error) {
	return self.send("query", sql, vars)
}

func (self *DB) Select(what string) (any, error) {
	return self.send("select", what)
}

func (self *DB) Create(thing string, data map[string]any) (any, error) {
	return self.send("create", thing, data)
}

func (self *DB) Update(what string, data map[string]any) (any, error) {
	return self.send("update", what, data)
}

func (self *DB) Change(what string, data map[string]any) (any, error) {
	return self.send("change", what, data)
}

func (self *DB) Modify(what string, data map[string]any) (any, error) {
	return self.send("modify", what, data)
}

func (self *DB) Delete(what string) (any, error) {
	return self.send("delete", what)
}

// --------------------------------------------------
// Private methods
// --------------------------------------------------

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

func (self *DB) resp(method string, params []any, res any) (any, error) {

	arg, ok := params[0].(string)

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
