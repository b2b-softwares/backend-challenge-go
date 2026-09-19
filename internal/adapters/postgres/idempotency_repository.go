package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type IdempotencyRepository struct {
	db dbtx
}

func NewIdempotencyRepository(db *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{
		db: db,
	}
}

func (r *IdempotencyRepository) resolveDB(ctx context.Context) dbtx {
	txDB, ok := dbtxFromContext(ctx)
	if ok {
		return txDB
	}

	return r.db
}

func (r *IdempotencyRepository) Find(
	ctx context.Context,
	providerID string,
	idempotencyKey string,
) (ports.IdempotencyRecord, error) {
	if providerID == "" || idempotencyKey == "" {
		return ports.IdempotencyRecord{}, errors.New(
			"provider id and idempotency key are required",
		)
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return ports.IdempotencyRecord{}, errors.New(
			"idempotency repository: database is nil",
		)
	}

	var record ports.IdempotencyRecord
	var observedBalanceCurrency *string

	err := db.QueryRow(
		ctx,
		`
		SELECT
			provider_id,
			idempotency_key,
			payload_hash,
			transaction_id,
			status,
			response_body,
			observed_balance_amount,
			observed_balance_currency
		FROM idempotency_records
		WHERE provider_id = $1
		  AND idempotency_key = $2
		`,
		providerID,
		idempotencyKey,
	).Scan(
		&record.ProviderID,
		&record.IdempotencyKey,
		&record.PayloadHash,
		&record.TransactionID,
		&record.Status,
		&record.ResponseBody,
		&record.ObservedBalanceAmount,
		&observedBalanceCurrency,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.IdempotencyRecord{}, pgx.ErrNoRows
		}

		return ports.IdempotencyRecord{}, fmt.Errorf(
			"find idempotency record: %w",
			err,
		)
	}

	if observedBalanceCurrency != nil {
		record.ObservedBalanceCurrency = *observedBalanceCurrency
	}

	return record, nil
}

func (r *IdempotencyRepository) Create(
	ctx context.Context,
	record ports.IdempotencyRecord,
) error {
	if record.ProviderID == "" ||
		record.IdempotencyKey == "" ||
		record.PayloadHash == "" ||
		record.TransactionID == uuid.Nil {
		return errors.New("invalid idempotency record")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return errors.New(
			"idempotency repository: database is nil",
		)
	}

	var observedBalanceCurrency any

	if record.ObservedBalanceCurrency != "" {
		observedBalanceCurrency = record.ObservedBalanceCurrency
	}

	_, err := db.Exec(
		ctx,
		`
		INSERT INTO idempotency_records (
			provider_id,
			idempotency_key,
			payload_hash,
			transaction_id,
			status,
			response_body,
			observed_balance_amount,
			observed_balance_currency,
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
			NOW()
		)
		`,
		record.ProviderID,
		record.IdempotencyKey,
		record.PayloadHash,
		record.TransactionID,
		record.Status,
		record.ResponseBody,
		record.ObservedBalanceAmount,
		observedBalanceCurrency,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" &&
				pgErr.ConstraintName == "idempotency_records_pkey" {
				return domain.ErrDuplicateIdempotencyRecord
			}
		}

		return fmt.Errorf(
			"create idempotency record: %w",
			err,
		)
	}

	return nil
}

func (r *IdempotencyRepository) Update(
	ctx context.Context,
	record ports.IdempotencyRecord,
) error {
	if record.ProviderID == "" ||
		record.IdempotencyKey == "" ||
		record.PayloadHash == "" ||
		record.TransactionID == uuid.Nil {
		return errors.New("invalid idempotency record")
	}

	db := r.resolveDB(ctx)
	if db == nil {
		return errors.New(
			"idempotency repository: database is nil",
		)
	}

	var observedBalanceCurrency any

	if record.ObservedBalanceCurrency != "" {
		observedBalanceCurrency = record.ObservedBalanceCurrency
	}

	commandTag, err := db.Exec(
		ctx,
		`
		UPDATE idempotency_records
		SET
			payload_hash = $3,
			transaction_id = $4,
			status = $5,
			response_body = $6,
			observed_balance_amount = $7,
			observed_balance_currency = $8
		WHERE provider_id = $1
		  AND idempotency_key = $2
		`,
		record.ProviderID,
		record.IdempotencyKey,
		record.PayloadHash,
		record.TransactionID,
		record.Status,
		record.ResponseBody,
		record.ObservedBalanceAmount,
		observedBalanceCurrency,
	)

	if err != nil {
		return fmt.Errorf(
			"update idempotency record: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf(
			"update idempotency record: record not found",
		)
	}

	return nil
}
