package sql

import (
	"context"
	"database/sql/driver"
	"fmt"
	"github.com/surrealdb/surrealdb.go"
	"strings"
)

type Conn struct {
	*surrealdb.DB
}

func (s *Conn) Prepare(query string) (driver.Stmt, error) {
	//method, thing, err := s.parseMethod(query)
	//if err != nil {
	//	return nil, fmt.Errorf("invalid rawQuery: %w", err)
	//}

	return &Stmt{
		conn:     s,
		rawQuery: query,
		//method:   method,
		//thing:    thing,
	}, nil
}

func (s *Conn) Close() error {
	s.DB.Close()
	return nil
}

func (s *Conn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("this method is deprecated")
}

func (s *Conn) Ping(ctx context.Context) error {
	// TODO: Is there something more reliable?
	// TODO: How do we utilize context.Context ?
	_, err := s.Select("1")
	return err
}

func (s *Conn) ResetSession(ctx context.Context) error {
	return nil // We can do some cleanup here, once needed
}

func (s *Conn) IsValid() bool {
	return true // Might change once we have something that will invalidate the connection
}

func (s *Conn) parseMethod(query string) (string, string, error) {
	idx := strings.IndexRune(query, ' ')
	if idx <= 0 {
		return query, "", nil
	}

	// TODO: do we validate this?
	return strings.ToLower(query[:idx]), query[idx+1:], nil
}

// TODO: Potentially implement: ExecerContext, QueryerContext, ConnPrepareContext, and ConnBeginTx.
