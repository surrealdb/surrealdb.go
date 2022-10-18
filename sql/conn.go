package sql

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/surrealdb/surrealdb.go"
)

type Conn struct {
	*surrealdb.DB
}

func (s *Conn) Prepare(query string) (driver.Stmt, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Conn) Close() error {
	return s.Close()
}

func (s *Conn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("this method is deprecated")
}

func (s *Conn) Ping(ctx context.Context) error {
	// TODO: Is there something more reliable?
	// TODO: How do we utilize context.Context ?
	_, err := s.Info()
	return err
}

func (s *Conn) ResetSession(ctx context.Context) error {
	return nil // We can do some cleanup here, once needed
}

func (s *Conn) IsValid() bool {
	return true // Might change once we have something that will invalidate the connection
}

// TODO: Potentially implement: ExecerContext, QueryerContext, ConnPrepareContext, and ConnBeginTx.
