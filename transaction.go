package surrealdb

import (
	"context"
	"sync"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Transaction represents an interactive SurrealDB transaction on a WebSocket connection.
// Unlike text-based transactions (BEGIN TRANSACTION; ... COMMIT;), interactive transactions
// allow executing statements one at a time and conditionally committing or canceling.
//
// Transactions are only supported on WebSocket connections (SurrealDB v3+).
//
// Transaction satisfies the sendable constraint, so all surrealdb.Query,
// surrealdb.Create, etc. functions work with transactions directly.
//
// Note: Transactions do NOT support session state changes like SignIn, Use, Let, etc.
// The namespace/database and authentication are inherited from the session or connection
// that started the transaction.
type Transaction struct {
	db        *DB
	id        *models.UUID
	sessionID *models.UUID // optional, nil for default session
	closed    bool
	mu        sync.RWMutex
}

// Begin starts a new interactive transaction on the default session.
// Interactive transactions are only supported on WebSocket connections (SurrealDB v3+).
//
// Example:
//
//	tx, err := db.Begin(ctx)
//	if err != nil {
//	    return err
//	}
//	defer tx.Cancel(ctx) // Cancel if not committed
//
//	// Execute queries within the transaction
//	_, err = surrealdb.Query[[]any](ctx, tx, "CREATE user:1 SET name = 'Alice'", nil)
//	if err != nil {
//	    return err
//	}
//
//	// Commit the transaction
//	return tx.Commit(ctx)
func (db *DB) Begin(ctx context.Context) (*Transaction, error) {
	// Check if the connection is a WebSocket connection
	if _, ok := db.con.(connection.WebSocketConnection); !ok {
		return nil, constants.ErrTransactionsNotSupported
	}

	// Send the begin RPC request
	req := &connection.RPCRequest{
		Method: string(connection.Begin),
	}

	var res connection.RPCResponse[models.UUID]
	if err := connection.Call(db.con, ctx, &res, req); err != nil {
		return nil, err
	}

	return &Transaction{
		db: db,
		id: res.Result,
	}, nil
}

// ID returns the transaction's UUID.
func (tx *Transaction) ID() *models.UUID {
	return tx.id
}

// SessionID returns the session UUID if the transaction was started within a session.
// Returns nil if the transaction was started on the default session.
func (tx *Transaction) SessionID() *models.UUID {
	return tx.sessionID
}

// IsClosed returns whether the transaction has been committed or canceled.
func (tx *Transaction) IsClosed() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.closed
}

// Commit commits the transaction, making all changes permanent.
// After calling Commit, the transaction cannot be used anymore.
func (tx *Transaction) Commit(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.closed {
		return constants.ErrTransactionClosed
	}

	// Send the commit RPC request with the transaction UUID as a positional param
	var res connection.RPCResponse[any]
	if err := connection.Send(tx.db.con, ctx, &res, string(connection.Commit), tx.id); err != nil {
		return err
	}

	tx.closed = true
	return nil
}

// Cancel cancels the transaction, discarding all changes.
// After calling Cancel, the transaction cannot be used anymore.
//
// It's safe to call Cancel on an already committed or canceled transaction;
// it will return ErrTransactionClosed but won't cause any harm.
func (tx *Transaction) Cancel(ctx context.Context) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	if tx.closed {
		return constants.ErrTransactionClosed
	}

	// Send the cancel RPC request with the transaction UUID as a positional param
	var res connection.RPCResponse[any]
	if err := connection.Send(tx.db.con, ctx, &res, string(connection.Cancel), tx.id); err != nil {
		return err
	}

	tx.closed = true
	return nil
}
