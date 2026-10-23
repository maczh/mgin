package db

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// MySQLTx runs fn atomically on one MySQL connection. DAOs used by fn must be
// bound with WithTx(tx); otherwise they intentionally use their normal pool.
func MySQLTx[T any](ctx context.Context, fn func(tx *gorm.DB) (T, error), dbName ...string) (T, error) {
	var zero T
	if fn == nil { return zero, errors.New("transaction callback is nil") }
	conn, err := Mysql.GetConnection(dbName...)
	if err != nil { return zero, fmt.Errorf("get database connection: %w", err) }
	return TransactionOn(ctx, conn, fn)
}

// TransactionOn executes a callback and rolls back on every callback error.
func TransactionOn[T any](ctx context.Context, conn *gorm.DB, fn func(tx *gorm.DB) (T, error)) (T, error) {
	var zero T
	if fn == nil { return zero, errors.New("transaction callback is nil") }
	if conn == nil { return zero, errors.New("database connection is nil") }
	if ctx == nil { ctx = context.Background() }
	var result T
	err := conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = fn(tx)
		return err
	})
	if err != nil { return zero, fmt.Errorf("transaction failed: %w", err) }
	return result, nil
}
