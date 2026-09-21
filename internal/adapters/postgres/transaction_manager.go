package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transactionContextKey struct{}

type TransactionManager struct {
	db *pgxpool.Pool
}

func NewTransactionManager(db *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{
		db: db,
	}
}

func (m *TransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	if m == nil || m.db == nil {
		return errors.New("transaction manager: database is nil")
	}

	if fn == nil {
		return errors.New("transaction manager: callback is nil")
	}

	if existingTx, ok := dbtxFromContext(ctx); ok {
		return fn(
			context.WithValue(
				ctx,
				transactionContextKey{},
				existingTx,
			),
		)
	}

	tx, err := m.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txContext := context.WithValue(
		ctx,
		transactionContextKey{},
		dbtx(tx),
	)

	if err := fn(txContext); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil &&
			!errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return fmt.Errorf(
				"transaction callback failed: %w; rollback failed: %v",
				err,
				rollbackErr,
			)
		}

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func dbtxFromContext(ctx context.Context) (dbtx, bool) {
	value := ctx.Value(transactionContextKey{})
	if value == nil {
		return nil, false
	}

	transaction, ok := value.(dbtx)
	return transaction, ok
}
