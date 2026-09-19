package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

type PendingReferenceWorker struct {
	service       *ReversalService
	transactionDB ports.TransactionManager
	transactions  ports.WagerTransactionRepository
}

func NewPendingReferenceWorker(
	service *ReversalService,
	transactionDB ports.TransactionManager,
	transactions ports.WagerTransactionRepository,
) *PendingReferenceWorker {
	return &PendingReferenceWorker{
		service:       service,
		transactionDB: transactionDB,
		transactions:  transactions,
	}
}

// ProcessBatch processes at most limit pending-reference transactions.
//
// Each transaction is processed in its own database transaction. The
// repository uses FOR UPDATE SKIP LOCKED, allowing multiple workers to run
// concurrently without global application locks.
func (w *PendingReferenceWorker) ProcessBatch(
	ctx context.Context,
	limit int,
) (int, error) {
	if w == nil ||
		w.service == nil ||
		w.transactionDB == nil ||
		w.transactions == nil {
		return 0, errors.New(
			"pending reference worker: dependencies are required",
		)
	}

	if limit <= 0 {
		return 0, errors.New(
			"pending reference worker: limit must be greater than zero",
		)
	}

	processed := 0

	for processed < limit {
		handled, err := w.processOne(ctx)
		if err != nil {
			return processed, err
		}

		if !handled {
			break
		}

		processed++
	}

	return processed, nil
}

// ProcessOne processes a single available pending-reference transaction.
//
// The method is intentionally small: orchestration and retry decisions stay
// in the worker, while the actual reversal financial operation remains in
// ReversalService.
func (w *PendingReferenceWorker) ProcessOne(
	ctx context.Context,
) (bool, error) {
	if w == nil ||
		w.service == nil ||
		w.transactionDB == nil ||
		w.transactions == nil {
		return false, errors.New(
			"pending reference worker: dependencies are required",
		)
	}

	return w.processOne(ctx)
}

func (w *PendingReferenceWorker) processOne(
	ctx context.Context,
) (bool, error) {
	now := time.Now().UTC()

	handled := false

	err := w.transactionDB.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			transactions, err :=
				w.transactions.GetPendingReferenceBatch(
					txCtx,
					1,
					now,
				)
			if err != nil {
				return fmt.Errorf(
					"load pending reference batch: %w",
					err,
				)
			}

			if len(transactions) == 0 {
				return nil
			}

			handled = true

			pending := transactions[0]

			return w.service.processPendingReference(
				txCtx,
				pending,
				now,
			)
		},
	)

	if err != nil {
		return handled, err
	}

	return handled, nil
}

// processPendingReference is deliberately kept inside ReversalService so the
// worker does not duplicate financial business rules.
//
// The worker owns retry scheduling; ReversalService owns the actual reversal
// operation.
func (s *ReversalService) processPendingReference(
	ctx context.Context,
	pending ports.PendingReferenceTransaction,
	now time.Time,
) error {
	transaction := pending.Transaction

	if transaction.Status() != domain.TransactionStatusPendingReference {
		return nil
	}

	/*
		When the transaction has exceeded its retry budget or TTL, the
		reversal becomes terminally rejected.

		A missing expiry is allowed for legacy rows. In that case the maximum
		attempt count remains the safety boundary.
	*/
	if pending.ExpiresAt != nil &&
		!now.Before(pending.ExpiresAt.UTC()) {
		return s.rejectPendingReference(
			ctx,
			pending,
			reversalFailureCodeReferenceNotFound,
			now,
		)
	}

	if pending.Attempts >= maxReferenceAttempts {
		return s.rejectPendingReference(
			ctx,
			pending,
			reversalFailureCodeReferenceNotFound,
			now,
		)
	}

	reference, err :=
		s.transactions.GetByProviderAndExternalTransactionForUpdate(
			ctx,
			transaction.ProviderID(),
			transaction.ReferenceExternalID(),
		)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.schedulePendingReferenceRetry(
				ctx,
				pending,
				now,
			)
		}

		return fmt.Errorf(
			"find pending reversal reference: %w",
			err,
		)
	}

	if reference.Status() != domain.TransactionStatusProcessed {
		return s.schedulePendingReferenceRetry(
			ctx,
			pending,
			now,
		)
	}

	/*
		At this point the reference is available. The same validation used by
		the synchronous Reverse flow is applied before any wallet mutation.
	*/
	if reference.PlayerID() != transaction.PlayerID() ||
		reference.WalletID() != transaction.WalletID() {
		return s.rejectPendingReference(
			ctx,
			pending,
			reversalFailureCodeInvalidReference,
			now,
		)
	}

	existingReversal, err :=
		s.transactions.GetByProviderReferenceAndKind(
			ctx,
			transaction.ProviderID(),
			transaction.ReferenceExternalID(),
			transaction.Kind(),
		)

	if err == nil {
		if existingReversal.ID() == transaction.ID() {
			return s.completePendingReference(
				ctx,
				pending,
				reference,
				now,
			)
		}

		return s.rejectPendingReference(
			ctx,
			pending,
			reversalFailureCodeAlreadyReversed,
			now,
		)
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf(
			"check existing pending reversal: %w",
			err,
		)
	}

	if transaction.Kind() == domain.TransactionKindRollback &&
		reference.Kind() == domain.TransactionKindLoss {
		return s.rejectPendingReference(
			ctx,
			pending,
			reversalFailureCodeInvalidReference,
			now,
		)
	}

	return s.completePendingReference(
		ctx,
		pending,
		reference,
		now,
	)
}

func (s *ReversalService) schedulePendingReferenceRetry(
	ctx context.Context,
	pending ports.PendingReferenceTransaction,
	now time.Time,
) error {
	nextAttempt := pending.Attempts + 1

	if nextAttempt >= maxReferenceAttempts {
		return s.rejectPendingReference(
			ctx,
			pending,
			reversalFailureCodeReferenceNotFound,
			now,
		)
	}

	availableAt := now.Add(referenceRetryDelay)

	if err := s.transactions.UpdateReferenceRetry(
		ctx,
		pending.Transaction.ID(),
		nextAttempt,
		&availableAt,
	); err != nil {
		return fmt.Errorf(
			"schedule pending reference retry: %w",
			err,
		)
	}

	return nil
}

func (s *ReversalService) rejectPendingReference(
	ctx context.Context,
	pending ports.PendingReferenceTransaction,
	failureCode string,
	now time.Time,
) error {
	transaction := pending.Transaction

	if err := transaction.MarkRejected(
		failureCode,
		now,
	); err != nil {
		return fmt.Errorf(
			"reject pending reference transaction: %w",
			err,
		)
	}

	if err := s.transactions.Update(
		ctx,
		transaction,
	); err != nil {
		return fmt.Errorf(
			"update rejected pending reference transaction: %w",
			err,
		)
	}

	wallet, err := s.wallets.GetByID(
		ctx,
		transaction.WalletID(),
	)
	if err != nil {
		return fmt.Errorf(
			"load wallet for rejected pending reference: %w",
			err,
		)
	}

	result := WagerResult{
		TransactionID: transaction.ID(),
		Status:        transaction.Status(),
		Balance:       wallet.Balance(),
	}

	responseBody, err := marshalWagerResult(result)
	if err != nil {
		return fmt.Errorf(
			"marshal rejected pending reference result: %w",
			err,
		)
	}

	record := ports.IdempotencyRecord{
		ProviderID:              transaction.ProviderID(),
		IdempotencyKey:          transaction.IdempotencyKey(),
		PayloadHash:             transaction.PayloadHash(),
		TransactionID:           transaction.ID(),
		Status:                  string(result.Status),
		ResponseBody:            responseBody,
		ObservedBalanceAmount:   result.Balance.Amount(),
		ObservedBalanceCurrency: string(result.Balance.Currency()),
	}

	if err := s.idempotency.Update(
		ctx,
		record,
	); err != nil {
		return fmt.Errorf(
			"update pending reversal idempotency: %w",
			err,
		)
	}

	return nil
}

func (s *ReversalService) completePendingReference(
	ctx context.Context,
	pending ports.PendingReferenceTransaction,
	reference domain.WagerTransaction,
	now time.Time,
) error {
	transaction := pending.Transaction
	amount := reference.Money()

	if err := transaction.ResolveReferenceWithMoney(
		reference.ID(),
		amount,
		now,
	); err != nil {
		return fmt.Errorf(
			"resolve pending reversal reference: %w",
			err,
		)
	}

	wallet, err := s.wallets.GetByID(
		ctx,
		transaction.WalletID(),
	)
	if err != nil {
		return fmt.Errorf(
			"load wallet for pending reversal: %w",
			err,
		)
	}

	if wallet.PlayerID() != transaction.PlayerID() {
		return domain.ErrInvalidWallet
	}

	if wallet.Currency() != amount.Currency() {
		return domain.ErrCurrencyMismatch
	}

	balanceBefore := wallet.Balance()

	direction := reversalLedgerDirection(
		transaction.Kind(),
		reference.Kind(),
	)

	switch direction {
	case domain.LedgerDirectionCredit:
		err = wallet.Credit(
			amount,
			now,
		)

	case domain.LedgerDirectionDebit:
		err = wallet.Debit(
			amount,
			now,
		)

	default:
		return domain.ErrInvalidExternalOperation
	}

	if err != nil {
		if errors.Is(err, domain.ErrInsufficientBalance) {
			if markErr := transaction.MarkRejected(
				reversalFailureCodeInsufficient,
				now,
			); markErr != nil {
				return fmt.Errorf(
					"reject insufficient pending reversal: %w",
					markErr,
				)
			}

			if updateErr := s.transactions.Update(
				ctx,
				transaction,
			); updateErr != nil {
				return fmt.Errorf(
					"update insufficient pending reversal: %w",
					updateErr,
				)
			}

			result := WagerResult{
				TransactionID: transaction.ID(),
				Status:        transaction.Status(),
				Balance:       balanceBefore,
			}

			if err := s.updatePendingReversalIdempotency(
				ctx,
				transaction,
				result,
			); err != nil {
				return err
			}

			return nil
		}

		return fmt.Errorf(
			"apply pending reversal to wallet: %w",
			err,
		)
	}

	expectedVersion := wallet.Version() - 1

	if err := s.wallets.UpdateBalance(
		ctx,
		wallet,
		expectedVersion,
	); err != nil {
		return fmt.Errorf(
			"update pending reversal wallet: %w",
			err,
		)
	}

	ledgerEntry, err := domain.NewLedgerEntry(
		// A new ledger entry is created only after the wallet mutation
		// succeeds. The surrounding DB transaction guarantees atomicity.
		newUUID(),
		wallet.ID(),
		transaction.ID(),
		direction,
		amount,
		balanceBefore,
		wallet.Balance(),
		now,
	)
	if err != nil {
		return fmt.Errorf(
			"create pending reversal ledger entry: %w",
			err,
		)
	}

	if err := s.ledger.Create(
		ctx,
		ledgerEntry,
	); err != nil {
		return fmt.Errorf(
			"persist pending reversal ledger entry: %w",
			err,
		)
	}

	if err := transaction.MarkProcessed(
		wallet.Balance(),
		now,
	); err != nil {
		return fmt.Errorf(
			"mark pending reversal processed: %w",
			err,
		)
	}

	if err := s.transactions.Update(
		ctx,
		transaction,
	); err != nil {
		return fmt.Errorf(
			"update processed pending reversal: %w",
			err,
		)
	}

	result := WagerResult{
		TransactionID: transaction.ID(),
		Status:        transaction.Status(),
		Balance:       wallet.Balance(),
	}

	if err := s.updatePendingReversalIdempotency(
		ctx,
		transaction,
		result,
	); err != nil {
		return err
	}

	return s.createReversalEvent(
		ctx,
		transaction,
		reference,
		wallet.Balance(),
		now,
	)
}

func (s *ReversalService) updatePendingReversalIdempotency(
	ctx context.Context,
	transaction domain.WagerTransaction,
	result WagerResult,
) error {
	responseBody, err := marshalWagerResult(result)
	if err != nil {
		return fmt.Errorf(
			"marshal pending reversal result: %w",
			err,
		)
	}

	if err := s.idempotency.Update(
		ctx,
		ports.IdempotencyRecord{
			ProviderID:              transaction.ProviderID(),
			IdempotencyKey:          transaction.IdempotencyKey(),
			PayloadHash:             transaction.PayloadHash(),
			TransactionID:           transaction.ID(),
			Status:                  string(result.Status),
			ResponseBody:            responseBody,
			ObservedBalanceAmount:   result.Balance.Amount(),
			ObservedBalanceCurrency: string(result.Balance.Currency()),
		},
	); err != nil {
		return fmt.Errorf(
			"update pending reversal idempotency: %w",
			err,
		)
	}

	return nil
}

// newUUID is isolated here so the worker/service remains easy to test and
// keeps UUID generation out of the domain.
func newUUID() uuid.UUID {
	return uuid.New()
}
