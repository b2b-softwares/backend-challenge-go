package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

type WalletRepository struct {
	db dbtx
}

func NewWalletRepository(db *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{
		db: db,
	}
}

func newWalletRepositoryWithDBTX(db dbtx) *WalletRepository {
	return &WalletRepository{
		db: db,
	}
}

func (r *WalletRepository) resolveDB(ctx context.Context) dbtx {
	if tx, ok := dbtxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

func (r *WalletRepository) Create(
	ctx context.Context,
	wallet domain.Wallet,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("wallet repository: database is nil")
	}

	db := r.resolveDB(ctx)

	_, err := db.Exec(
		ctx,
		`
		INSERT INTO wallets (
			id,
			player_id,
			currency,
			balance_minor,
			version,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
		wallet.ID(),
		wallet.PlayerID(),
		string(wallet.Currency()),
		wallet.Balance().Amount(),
		wallet.Version(),
		wallet.CreatedAt(),
		wallet.UpdatedAt(),
	)
	if err != nil {
		return mapWalletRepositoryError(err)
	}

	return nil
}

func (r *WalletRepository) GetByID(
	ctx context.Context,
	walletID uuid.UUID,
) (domain.Wallet, error) {
	if r == nil || r.db == nil {
		return domain.Wallet{}, fmt.Errorf("wallet repository: database is nil")
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			player_id,
			currency,
			balance_minor,
			version,
			created_at,
			updated_at
		FROM wallets
		WHERE id = $1
		`,
		walletID,
	)

	return scanWallet(row)
}

func (r *WalletRepository) GetByPlayerAndCurrency(
	ctx context.Context,
	playerID uuid.UUID,
	currency domain.Currency,
) (domain.Wallet, error) {
	if r == nil || r.db == nil {
		return domain.Wallet{}, fmt.Errorf("wallet repository: database is nil")
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			player_id,
			currency,
			balance_minor,
			version,
			created_at,
			updated_at
		FROM wallets
		WHERE player_id = $1
		  AND currency = $2
		`,
		playerID,
		string(currency),
	)

	return scanWallet(row)
}

func (r *WalletRepository) UpdateBalance(
	ctx context.Context,
	wallet domain.Wallet,
	expectedVersion int64,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("wallet repository: database is nil")
	}

	if expectedVersion < 1 {
		return domain.ErrInvalidWallet
	}

	db := r.resolveDB(ctx)

	result, err := db.Exec(
		ctx,
		`
		UPDATE wallets
		SET
			balance_minor = $1,
			version = version + 1,
			updated_at = $2
		WHERE id = $3
		  AND version = $4
		`,
		wallet.Balance().Amount(),
		wallet.UpdatedAt(),
		wallet.ID(),
		expectedVersion,
	)
	if err != nil {
		return mapWalletRepositoryError(err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrConcurrentModification
	}

	return nil
}

type walletScanner interface {
	Scan(dest ...any) error
}

func scanWallet(row walletScanner) (domain.Wallet, error) {
	var (
		id           uuid.UUID
		playerID     uuid.UUID
		currency     string
		balanceMinor int64
		version      int64
		createdAt    time.Time
		updatedAt    time.Time
	)

	if err := row.Scan(
		&id,
		&playerID,
		&currency,
		&balanceMinor,
		&version,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Wallet{}, pgx.ErrNoRows
		}

		return domain.Wallet{}, err
	}

	money, err := domain.MoneyFromMinorUnits(
		balanceMinor,
		domain.Currency(currency),
	)
	if err != nil {
		return domain.Wallet{}, err
	}

	return domain.RehydrateWallet(
		id,
		playerID,
		domain.Currency(currency),
		money,
		version,
		createdAt,
		updatedAt,
	)
}

func mapWalletRepositoryError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrInvalidWallet

		case "23514":
			return domain.ErrInvalidWallet
		}
	}

	return err
}
