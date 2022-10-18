package sql

import "database/sql/driver"

type Stmt struct {
}

func (s *Stmt) Close() error {
	//TODO implement me
	panic("implement me")
}

func (s *Stmt) NumInput() int {
	//TODO implement me
	panic("implement me")
}

func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	//TODO implement me
	panic("implement me")
}
