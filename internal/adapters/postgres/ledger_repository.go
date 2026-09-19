package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

const (
	defaultLedgerLimit = 50
	maxLedgerLimit     = 100
)

type LedgerRepository struct {
	db dbtx
}

func NewLedgerRepository(db *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{
		db: db,
	}
}

func newLedgerRepositoryWithDBTX(db dbtx) *LedgerRepository {
	return &LedgerRepository{
		db: db,
	}
}

func (r *LedgerRepository) resolveDB(ctx context.Context) dbtx {
	if tx, ok := dbtxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

func (r *LedgerRepository) Create(
	ctx context.Context,
	entry domain.WalletLedgerEntry,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("ledger repository: database is nil")
	}

	db := r.resolveDB(ctx)

	_, err := db.Exec(
		ctx,
		`
		INSERT INTO wallet_ledger (
			id,
			wallet_id,
			transaction_id,
			direction,
			amount_minor,
			currency,
			balance_before_minor,
			balance_after_minor,
			created_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9
		)
		`,
		entry.ID(),
		entry.WalletID(),
		entry.TransactionID(),
		string(entry.Direction()),
		entry.Money().Amount(),
		string(entry.Money().Currency()),
		entry.BalanceBefore().Amount(),
		entry.BalanceAfter().Amount(),
		entry.CreatedAt(),
	)
	if err != nil {
		return mapLedgerRepositoryError(err)
	}

	return nil
}

func (r *LedgerRepository) GetByWalletID(
	ctx context.Context,
	walletID uuid.UUID,
	cursor string,
	limit int,
) ([]domain.WalletLedgerEntry, string, error) {
	if r == nil || r.db == nil {
		return nil, "", fmt.Errorf("ledger repository: database is nil")
	}

	if walletID == uuid.Nil {
		return nil, "", domain.ErrInvalidLedgerEntry
	}

	if limit <= 0 {
		limit = defaultLedgerLimit
	}

	if limit > maxLedgerLimit {
		limit = maxLedgerLimit
	}

	cursorCreatedAt, cursorID, err := decodeLedgerCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	db := r.resolveDB(ctx)

	var rows pgx.Rows

	if cursor == "" {
		rows, err = db.Query(
			ctx,
			`
			SELECT
				id,
				wallet_id,
				transaction_id,
				direction,
				amount_minor,
				currency,
				balance_before_minor,
				balance_after_minor,
				created_at
			FROM wallet_ledger
			WHERE wallet_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
			`,
			walletID,
			limit+1,
		)
	} else {
		rows, err = db.Query(
			ctx,
			`
			SELECT
				id,
				wallet_id,
				transaction_id,
				direction,
				amount_minor,
				currency,
				balance_before_minor,
				balance_after_minor,
				created_at
			FROM wallet_ledger
			WHERE wallet_id = $1
			  AND (
				created_at < $2
				OR (
					created_at = $2
					AND id < $3
				)
			  )
			ORDER BY created_at DESC, id DESC
			LIMIT $4
			`,
			walletID,
			cursorCreatedAt,
			cursorID,
			limit+1,
		)
	}

	if err != nil {
		return nil, "", err
	}

	defer rows.Close()

	entries := make([]domain.WalletLedgerEntry, 0, limit+1)

	for rows.Next() {
		entry, err := scanLedgerEntry(rows)
		if err != nil {
			return nil, "", err
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	nextCursor := ""

	if len(entries) > limit {
		last := entries[limit-1]

		nextCursor, err = encodeLedgerCursor(
			last.CreatedAt(),
			last.ID(),
		)
		if err != nil {
			return nil, "", err
		}

		entries = entries[:limit]
	}

	return entries, nextCursor, nil
}

func (r *LedgerRepository) GetByTransactionID(
	ctx context.Context,
	transactionID uuid.UUID,
) (domain.WalletLedgerEntry, error) {
	if r == nil || r.db == nil {
		return domain.WalletLedgerEntry{}, fmt.Errorf(
			"ledger repository: database is nil",
		)
	}

	if transactionID == uuid.Nil {
		return domain.WalletLedgerEntry{}, domain.ErrInvalidLedgerEntry
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			wallet_id,
			transaction_id,
			direction,
			amount_minor,
			currency,
			balance_before_minor,
			balance_after_minor,
			created_at
		FROM wallet_ledger
		WHERE transaction_id = $1
		`,
		transactionID,
	)

	return scanLedgerEntry(row)
}

type ledgerScanner interface {
	Scan(dest ...any) error
}

func scanLedgerEntry(row ledgerScanner) (domain.WalletLedgerEntry, error) {
	var (
		id            uuid.UUID
		walletID      uuid.UUID
		transactionID uuid.UUID
		direction     string
		amountMinor   int64
		currency      string
		balanceBefore int64
		balanceAfter  int64
		createdAt     time.Time
	)

	if err := row.Scan(
		&id,
		&walletID,
		&transactionID,
		&direction,
		&amountMinor,
		&currency,
		&balanceBefore,
		&balanceAfter,
		&createdAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WalletLedgerEntry{}, pgx.ErrNoRows
		}

		return domain.WalletLedgerEntry{}, err
	}

	money, err := domain.MoneyFromMinorUnits(
		amountMinor,
		domain.Currency(currency),
	)
	if err != nil {
		return domain.WalletLedgerEntry{}, err
	}

	before, err := domain.MoneyFromMinorUnits(
		balanceBefore,
		domain.Currency(currency),
	)
	if err != nil {
		return domain.WalletLedgerEntry{}, err
	}

	after, err := domain.MoneyFromMinorUnits(
		balanceAfter,
		domain.Currency(currency),
	)
	if err != nil {
		return domain.WalletLedgerEntry{}, err
	}

	return domain.NewLedgerEntry(
		id,
		walletID,
		transactionID,
		domain.LedgerDirection(direction),
		money,
		before,
		after,
		createdAt,
	)
}

func encodeLedgerCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	if createdAt.IsZero() || id == uuid.Nil {
		return "", domain.ErrInvalidLedgerEntry
	}

	raw := strconv.FormatInt(
		createdAt.UTC().UnixNano(),
		10,
	) + "|" + id.String()

	return base64.RawURLEncoding.EncodeToString(
		[]byte(raw),
	), nil
}

func decodeLedgerCursor(cursor string) (time.Time, uuid.UUID, error) {
	if strings.TrimSpace(cursor) == "" {
		return time.Time{}, uuid.Nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, domain.ErrInvalidLedgerEntry
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, domain.ErrInvalidLedgerEntry
	}

	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, domain.ErrInvalidLedgerEntry
	}

	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return time.Time{}, uuid.Nil, domain.ErrInvalidLedgerEntry
	}

	return time.Unix(0, nanos).UTC(), id, nil
}

func mapLedgerRepositoryError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrDuplicateTransaction

		case "23503":
			return domain.ErrInvalidLedgerEntry

		case "23514":
			return domain.ErrInvalidLedgerEntry
		}
	}

	return err
}
