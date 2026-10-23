package db

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// Transaction executes fn in a MySQL transaction and returns its result.
// The transaction is committed only when fn returns a nil error. Any error
// returned by fn, or by starting/committing/rolling back the transaction, is
// returned to the caller.
//
// All database work performed by fn must use tx (or a DAO explicitly bound to
// tx); opening another connection inside fn is outside this transaction.
func MySQLTx[T any](ctx context.Context, fn func(tx *gorm.DB) (T, error), dbName ...string) (T, error) {
	conn, err := Mysql.GetConnection(dbName...)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("get database connection: %w", err)
	}
	return TransactionOn(ctx, conn, fn)
}

// TransactionOn executes fn in a transaction on conn. It is useful when the
// caller already owns a *gorm.DB, including a connection for SQLite or a
// specific MySQL database.
func TransactionOn[T any](ctx context.Context, conn *gorm.DB, fn func(tx *gorm.DB) (T, error)) (T, error) {
	var zero T
	if fn == nil {
		return zero, errors.New("transaction callback is nil")
	}
	if conn == nil {
		return zero, errors.New("database connection is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var result T
	err := conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var callbackErr error
		result, callbackErr = fn(tx)
		if callbackErr != nil {
			return fmt.Errorf("transaction callback: %w", callbackErr)
		}
		return nil
	})
	if err != nil {
		return zero, fmt.Errorf("transaction failed: %w", err)
	}
	return result, nil
}
