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
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type WagerTransactionRepository struct {
	db dbtx
}

func NewWagerTransactionRepository(
	db *pgxpool.Pool,
) *WagerTransactionRepository {
	return &WagerTransactionRepository{
		db: db,
	}
}

func newWagerTransactionRepositoryWithDBTX(
	db dbtx,
) *WagerTransactionRepository {
	return &WagerTransactionRepository{
		db: db,
	}
}

func (r *WagerTransactionRepository) resolveDB(
	ctx context.Context,
) dbtx {
	if tx, ok := dbtxFromContext(ctx); ok {
		return tx
	}

	return r.db
}

func (r *WagerTransactionRepository) Create(
	ctx context.Context,
	transaction domain.WagerTransaction,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	db := r.resolveDB(ctx)

	var (
		referenceTransactionID any
		resultBalanceMinor     any
		resultBalanceCurrency  any
	)

	if transaction.ReferenceTransactionID() != nil {
		referenceTransactionID = *transaction.ReferenceTransactionID()
	}

	if transaction.ResultBalance() != nil {
		resultBalanceMinor = transaction.ResultBalance().Amount()
		resultBalanceCurrency = string(
			transaction.ResultBalance().Currency(),
		)
	}

	_, err := db.Exec(
		ctx,
		`
		INSERT INTO wager_transactions (
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
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
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15,
			$16,
			$17,
			$18,
			$19,
			$20
		)
		`,
		transaction.ID(),
		transaction.ExternalTransactionID(),
		transaction.ProviderID(),
		transaction.IdempotencyKey(),
		transaction.PayloadHash(),
		transaction.WalletID(),
		transaction.PlayerID(),
		transaction.RoundID(),
		transaction.GameID(),
		string(transaction.Kind()),
		transaction.Money().Amount(),
		string(transaction.Money().Currency()),
		transaction.ReferenceExternalID(),
		referenceTransactionID,
		string(transaction.Status()),
		transaction.FailureCode(),
		resultBalanceMinor,
		resultBalanceCurrency,
		transaction.CreatedAt(),
		transaction.UpdatedAt(),
	)
	if err != nil {
		return mapWagerTransactionRepositoryError(err)
	}

	return nil
}

func (r *WagerTransactionRepository) GetByID(
	ctx context.Context,
	transactionID uuid.UUID,
) (domain.WagerTransaction, error) {
	if r == nil || r.db == nil {
		return domain.WagerTransaction{}, fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if transactionID == uuid.Nil {
		return domain.WagerTransaction{}, domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
		FROM wager_transactions
		WHERE id = $1
		`,
		transactionID,
	)

	return scanWagerTransaction(row)
}

func (r *WagerTransactionRepository) GetByProviderAndExternalTransaction(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	if r == nil || r.db == nil {
		return domain.WagerTransaction{}, fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if providerID == "" || externalTransactionID == "" {
		return domain.WagerTransaction{}, domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
		FROM wager_transactions
		WHERE provider_id = $1
		  AND external_transaction_id = $2
		`,
		providerID,
		externalTransactionID,
	)

	return scanWagerTransaction(row)
}

func (r *WagerTransactionRepository) GetByProviderAndExternalTransactionForUpdate(
	ctx context.Context,
	providerID string,
	externalTransactionID string,
) (domain.WagerTransaction, error) {
	if r == nil || r.db == nil {
		return domain.WagerTransaction{}, fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if providerID == "" || externalTransactionID == "" {
		return domain.WagerTransaction{}, domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
		FROM wager_transactions
		WHERE provider_id = $1
		  AND external_transaction_id = $2
		FOR UPDATE
		`,
		providerID,
		externalTransactionID,
	)

	return scanWagerTransaction(row)
}

func (r *WagerTransactionRepository) GetByProviderAndReference(
	ctx context.Context,
	providerID string,
	referenceExternalID string,
) (domain.WagerTransaction, error) {
	if r == nil || r.db == nil {
		return domain.WagerTransaction{}, fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if providerID == "" || referenceExternalID == "" {
		return domain.WagerTransaction{}, domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
		FROM wager_transactions
		WHERE provider_id = $1
		  AND reference_external_id = $2
		  AND status = 'PROCESSED'
		ORDER BY created_at ASC, id ASC
		LIMIT 1
		`,
		providerID,
		referenceExternalID,
	)

	return scanWagerTransaction(row)
}

func (r *WagerTransactionRepository) GetByProviderReferenceAndKind(
	ctx context.Context,
	providerID string,
	referenceExternalID string,
	kind domain.TransactionKind,
) (domain.WagerTransaction, error) {
	if r == nil || r.db == nil {
		return domain.WagerTransaction{}, fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if providerID == "" || referenceExternalID == "" {
		return domain.WagerTransaction{}, domain.ErrInvalidTransaction
	}

	switch kind {
	case domain.TransactionKindRefund,
		domain.TransactionKindRollback:
	default:
		return domain.WagerTransaction{}, domain.ErrInvalidTransactionKind
	}

	db := r.resolveDB(ctx)

	row := db.QueryRow(
		ctx,
		`
		SELECT
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at
		FROM wager_transactions
		WHERE provider_id = $1
		  AND reference_external_id = $2
		  AND kind = $3
		ORDER BY created_at ASC, id ASC
		LIMIT 1
		`,
		providerID,
		referenceExternalID,
		string(kind),
	)

	return scanWagerTransaction(row)
}

func (r *WagerTransactionRepository) GetPendingReferenceBatch(
	ctx context.Context,
	limit int,
	now time.Time,
) ([]ports.PendingReferenceTransaction, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if limit <= 0 {
		return nil, domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	rows, err := db.Query(
		ctx,
		`
		SELECT
			id,
			external_transaction_id,
			provider_id,
			idempotency_key,
			payload_hash,
			wallet_id,
			player_id,
			round_id,
			game_id,
			kind,
			amount_minor,
			currency,
			reference_external_id,
			reference_transaction_id,
			status,
			failure_code,
			result_balance_minor,
			result_balance_currency,
			created_at,
			updated_at,
			reference_attempts,
			reference_available_at,
			reference_expires_at
		FROM wager_transactions
		WHERE status = 'PENDING_REFERENCE'
		  AND (
				reference_available_at IS NULL
				OR reference_available_at <= $1
		  )
		ORDER BY
			COALESCE(reference_available_at, created_at),
			created_at,
			id
		FOR UPDATE SKIP LOCKED
		LIMIT $2
		`,
		now,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(
		[]ports.PendingReferenceTransaction,
		0,
		limit,
	)

	for rows.Next() {
		transaction, attempts, availableAt, expiresAt, err :=
			scanPendingReferenceTransaction(rows)

		if err != nil {
			return nil, err
		}

		result = append(
			result,
			ports.PendingReferenceTransaction{
				Transaction: transaction,
				Attempts:    attempts,
				AvailableAt: availableAt,
				ExpiresAt:   expiresAt,
			},
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *WagerTransactionRepository) UpdateReferenceRetry(
	ctx context.Context,
	transactionID uuid.UUID,
	attempts int,
	availableAt *time.Time,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if transactionID == uuid.Nil || attempts < 0 {
		return domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	result, err := db.Exec(
		ctx,
		`
		UPDATE wager_transactions
		SET
			reference_attempts = $1,
			reference_available_at = $2,
			updated_at = $3
		WHERE id = $4
		  AND status = 'PENDING_REFERENCE'
		`,
		attempts,
		availableAt,
		time.Now().UTC(),
		transactionID,
	)
	if err != nil {
		return mapWagerTransactionRepositoryError(err)
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *WagerTransactionRepository) Update(
	ctx context.Context,
	transaction domain.WagerTransaction,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf(
			"wager transaction repository: database is nil",
		)
	}

	if transaction.ID() == uuid.Nil {
		return domain.ErrInvalidTransaction
	}

	db := r.resolveDB(ctx)

	var (
		referenceTransactionID any
		resultBalanceMinor     any
		resultBalanceCurrency  any
	)

	if transaction.ReferenceTransactionID() != nil {
		referenceTransactionID = *transaction.ReferenceTransactionID()
	}

	if transaction.ResultBalance() != nil {
		resultBalanceMinor = transaction.ResultBalance().Amount()
		resultBalanceCurrency = string(
			transaction.ResultBalance().Currency(),
		)
	}

	result, err := db.Exec(
		ctx,
		`
		UPDATE wager_transactions
		SET
			reference_transaction_id = $1,
			status = $2,
			failure_code = $3,
			result_balance_minor = $4,
			result_balance_currency = $5,
			updated_at = $6
		WHERE id = $7
		`,
		referenceTransactionID,
		string(transaction.Status()),
		transaction.FailureCode(),
		resultBalanceMinor,
		resultBalanceCurrency,
		transaction.UpdatedAt(),
		transaction.ID(),
	)
	if err != nil {
		return mapWagerTransactionRepositoryError(err)
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

type wagerTransactionScanner interface {
	Scan(dest ...any) error
}

func scanWagerTransaction(
	row wagerTransactionScanner,
) (domain.WagerTransaction, error) {
	var (
		id                     uuid.UUID
		externalTransactionID  string
		providerID             string
		idempotencyKey         string
		payloadHash            string
		walletID               uuid.UUID
		playerID               uuid.UUID
		roundID                string
		gameID                 string
		kind                   string
		amountMinor            int64
		currency               string
		referenceExternalID    string
		referenceTransactionID *uuid.UUID
		status                 string
		failureCode            string
		resultBalanceMinor     *int64
		resultBalanceCurrency  *string
		createdAt              time.Time
		updatedAt              time.Time
	)

	if err := row.Scan(
		&id,
		&externalTransactionID,
		&providerID,
		&idempotencyKey,
		&payloadHash,
		&walletID,
		&playerID,
		&roundID,
		&gameID,
		&kind,
		&amountMinor,
		&currency,
		&referenceExternalID,
		&referenceTransactionID,
		&status,
		&failureCode,
		&resultBalanceMinor,
		&resultBalanceCurrency,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WagerTransaction{}, pgx.ErrNoRows
		}

		return domain.WagerTransaction{}, err
	}

	return rehydrateWagerTransaction(
		id,
		externalTransactionID,
		providerID,
		idempotencyKey,
		payloadHash,
		walletID,
		playerID,
		roundID,
		gameID,
		kind,
		amountMinor,
		currency,
		referenceExternalID,
		referenceTransactionID,
		status,
		failureCode,
		resultBalanceMinor,
		resultBalanceCurrency,
		createdAt,
		updatedAt,
	)
}

func scanPendingReferenceTransaction(
	row wagerTransactionScanner,
) (
	domain.WagerTransaction,
	int,
	*time.Time,
	*time.Time,
	error,
) {
	var (
		id                     uuid.UUID
		externalTransactionID  string
		providerID             string
		idempotencyKey         string
		payloadHash            string
		walletID               uuid.UUID
		playerID               uuid.UUID
		roundID                string
		gameID                 string
		kind                   string
		amountMinor            int64
		currency               string
		referenceExternalID    string
		referenceTransactionID *uuid.UUID
		status                 string
		failureCode            string
		resultBalanceMinor     *int64
		resultBalanceCurrency  *string
		createdAt              time.Time
		updatedAt              time.Time
		attempts               int
		availableAt            *time.Time
		expiresAt              *time.Time
	)

	if err := row.Scan(
		&id,
		&externalTransactionID,
		&providerID,
		&idempotencyKey,
		&payloadHash,
		&walletID,
		&playerID,
		&roundID,
		&gameID,
		&kind,
		&amountMinor,
		&currency,
		&referenceExternalID,
		&referenceTransactionID,
		&status,
		&failureCode,
		&resultBalanceMinor,
		&resultBalanceCurrency,
		&createdAt,
		&updatedAt,
		&attempts,
		&availableAt,
		&expiresAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.WagerTransaction{}, 0, nil, nil, pgx.ErrNoRows
		}

		return domain.WagerTransaction{}, 0, nil, nil, err
	}

	transaction, err := rehydrateWagerTransaction(
		id,
		externalTransactionID,
		providerID,
		idempotencyKey,
		payloadHash,
		walletID,
		playerID,
		roundID,
		gameID,
		kind,
		amountMinor,
		currency,
		referenceExternalID,
		referenceTransactionID,
		status,
		failureCode,
		resultBalanceMinor,
		resultBalanceCurrency,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return domain.WagerTransaction{}, 0, nil, nil, err
	}

	return transaction, attempts, availableAt, expiresAt, nil
}

func rehydrateWagerTransaction(
	id uuid.UUID,
	externalTransactionID string,
	providerID string,
	idempotencyKey string,
	payloadHash string,
	walletID uuid.UUID,
	playerID uuid.UUID,
	roundID string,
	gameID string,
	kind string,
	amountMinor int64,
	currency string,
	referenceExternalID string,
	referenceTransactionID *uuid.UUID,
	status string,
	failureCode string,
	resultBalanceMinor *int64,
	resultBalanceCurrency *string,
	createdAt time.Time,
	updatedAt time.Time,
) (domain.WagerTransaction, error) {
	money, err := domain.MoneyFromMinorUnits(
		amountMinor,
		domain.Currency(currency),
	)
	if err != nil {
		return domain.WagerTransaction{}, err
	}

	var resultBalance *domain.Money

	if resultBalanceMinor != nil {
		if resultBalanceCurrency == nil {
			return domain.WagerTransaction{}, domain.ErrInvalidTransaction
		}

		balance, err := domain.MoneyFromMinorUnits(
			*resultBalanceMinor,
			domain.Currency(*resultBalanceCurrency),
		)
		if err != nil {
			return domain.WagerTransaction{}, err
		}

		resultBalance = &balance
	}

	return domain.RehydrateTransaction(
		id,
		externalTransactionID,
		providerID,
		idempotencyKey,
		payloadHash,
		walletID,
		playerID,
		roundID,
		gameID,
		domain.TransactionKind(kind),
		money,
		referenceExternalID,
		referenceTransactionID,
		domain.TransactionStatus(status),
		failureCode,
		resultBalance,
		createdAt,
		updatedAt,
	)
}

func mapWagerTransactionRepositoryError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			switch pgErr.ConstraintName {
			case "wager_transactions_provider_idempotency_uk":
				return domain.ErrDuplicateWagerIdempotency

			case "wager_transactions_provider_external_uk":
				return domain.ErrDuplicateTransaction

			default:
				return domain.ErrDuplicateTransaction
			}

		case "23503":
			return domain.ErrInvalidTransaction

		case "23514":
			return domain.ErrInvalidTransaction
		}
	}

	return err
}
