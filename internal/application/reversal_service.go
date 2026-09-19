package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const (
	reversalFailureCodeReferenceNotFound = "REFERENCE_NOT_FOUND"
	reversalFailureCodeInvalidReference  = "INVALID_REFERENCE"
	reversalFailureCodeAlreadyReversed   = "ALREADY_REVERSED"
	reversalFailureCodeInsufficient      = "INSUFFICIENT_BALANCE"

	maxReferenceAttempts = 10
	referenceRetryDelay  = 5 * time.Second
	referenceTTL         = 24 * time.Hour
)

type ReversalService struct {
	wallets       ports.WalletRepository
	transactions  ports.WagerTransactionRepository
	ledger        ports.LedgerRepository
	idempotency   ports.IdempotencyRepository
	outbox        ports.OutboxRepository
	transactionDB ports.TransactionManager
}

func NewReversalService(
	wallets ports.WalletRepository,
	transactions ports.WagerTransactionRepository,
	ledger ports.LedgerRepository,
	idempotency ports.IdempotencyRepository,
	outbox ports.OutboxRepository,
	transactionDB ports.TransactionManager,
) *ReversalService {
	return &ReversalService{
		wallets:       wallets,
		transactions:  transactions,
		ledger:        ledger,
		idempotency:   idempotency,
		outbox:        outbox,
		transactionDB: transactionDB,
	}
}

type ReverseTransactionInput struct {
	ID                    uuid.UUID
	ExternalTransactionID string
	ProviderID            string
	IdempotencyKey        string
	PayloadHash           string
	WalletID              uuid.UUID
	PlayerID              uuid.UUID
	RoundID               string
	GameID                string
	Kind                  domain.TransactionKind
	ReferenceExternalID   string
}

type reversalEventPayload struct {
	TransactionID          string `json:"transactionId"`
	ExternalTransactionID  string `json:"externalTransactionId"`
	ProviderID             string `json:"providerId"`
	PlayerID               string `json:"playerId"`
	WalletID               string `json:"walletId"`
	RoundID                string `json:"roundId"`
	GameID                 string `json:"gameId"`
	Kind                   string `json:"kind"`
	Amount                 string `json:"amount"`
	Currency               string `json:"currency"`
	Balance                string `json:"balance"`
	Status                 string `json:"status"`
	ReferenceExternalID    string `json:"referenceExternalId,omitempty"`
	ReferenceTransactionID string `json:"referenceTransactionId,omitempty"`
	FailureCode            string `json:"failureCode,omitempty"`
}

func (s *ReversalService) Reverse(
	ctx context.Context,
	input ReverseTransactionInput,
) (WagerResult, error) {
	if s == nil ||
		s.wallets == nil ||
		s.transactions == nil ||
		s.ledger == nil ||
		s.idempotency == nil ||
		s.outbox == nil ||
		s.transactionDB == nil {
		return WagerResult{}, errors.New(
			"reversal service: dependencies are required",
		)
	}

	if err := validateReverseTransactionInput(input); err != nil {
		return WagerResult{}, err
	}

	existing, err := s.idempotency.Find(
		ctx,
		input.ProviderID,
		input.IdempotencyKey,
	)
	if err == nil {
		return replayReversalIdempotencyRecord(
			existing,
			input.PayloadHash,
		)
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return WagerResult{}, fmt.Errorf(
			"find reversal idempotency record: %w",
			err,
		)
	}

	now := time.Now().UTC()

	var result WagerResult

	err = s.transactionDB.WithinTransaction(
		ctx,
		func(txCtx context.Context) error {
			/*
				Lock the reference transaction when it exists.

				When it does not exist yet, the reversal is persisted as
				PENDING_REFERENCE and handled later by the retry worker.
			*/
			reference, err :=
				s.transactions.GetByProviderAndExternalTransactionForUpdate(
					txCtx,
					input.ProviderID,
					input.ReferenceExternalID,
				)

			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					var pendingErr error

					result, pendingErr = s.createPendingReference(
						txCtx,
						input,
						now,
					)

					return pendingErr
				}

				return fmt.Errorf(
					"find reversal reference: %w",
					err,
				)
			}

			if reference.Status() != domain.TransactionStatusProcessed {
				var pendingErr error

				result, pendingErr = s.createPendingReference(
					txCtx,
					input,
					now,
				)

				return pendingErr
			}

			if reference.PlayerID() != input.PlayerID ||
				reference.WalletID() != input.WalletID {
				result, err = s.rejectReversal(
					txCtx,
					input,
					now,
					reversalFailureCodeInvalidReference,
					reference.Money(),
				)
				return err
			}

			/*
				Only one reversal of a given kind is allowed for the same
				reference.

				The reference row is locked above, so concurrent reversals
				against the same reference are serialized.
			*/
			existingReversal, err :=
				s.transactions.GetByProviderReferenceAndKind(
					txCtx,
					input.ProviderID,
					input.ReferenceExternalID,
					input.Kind,
				)

			if err == nil {
				if existingReversal.PayloadHash() != input.PayloadHash {
					return domain.ErrIdempotencyConflict
				}

				result, err = s.resultFromTransaction(
					txCtx,
					existingReversal,
				)
				if err != nil {
					return err
				}

				result.IdempotentReplay = true
				return nil
			}

			if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf(
					"check existing reversal: %w",
					err,
				)
			}

			/*
				LOSS has zero monetary value in the current domain model.
				Therefore there is no financial amount that can be used for
				a rollback of LOSS.
			*/
			if input.Kind == domain.TransactionKindRollback &&
				reference.Kind() == domain.TransactionKindLoss {
				result, err = s.rejectReversal(
					txCtx,
					input,
					now,
					reversalFailureCodeInvalidReference,
					reference.Money(),
				)
				return err
			}

			amount := reference.Money()

			/*
				Create the reversal initially as PENDING_REFERENCE so the
				domain can safely attach the reference and amount before
				the financial operation begins.
			*/
			transaction, err :=
				domain.NewPendingReferenceTransaction(
					input.ID,
					input.ExternalTransactionID,
					input.ProviderID,
					input.IdempotencyKey,
					input.PayloadHash,
					input.WalletID,
					input.PlayerID,
					input.RoundID,
					input.GameID,
					input.Kind,
					amount.Currency(),
					input.ReferenceExternalID,
					now,
				)
			if err != nil {
				return err
			}

			if err := transaction.ResolveReferenceWithMoney(
				reference.ID(),
				amount,
				now,
			); err != nil {
				return err
			}

			if err := s.transactions.Create(
				txCtx,
				transaction,
			); err != nil {
				return err
			}

			wallet, err := s.wallets.GetByID(
				txCtx,
				input.WalletID,
			)
			if err != nil {
				return err
			}

			if wallet.PlayerID() != input.PlayerID {
				return domain.ErrInvalidWallet
			}

			if wallet.Currency() != amount.Currency() {
				return domain.ErrCurrencyMismatch
			}

			balanceBefore := wallet.Balance()

			direction := reversalLedgerDirection(
				input.Kind,
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
						return markErr
					}

					if updateErr := s.transactions.Update(
						txCtx,
						transaction,
					); updateErr != nil {
						return updateErr
					}

					result = WagerResult{
						TransactionID: transaction.ID(),
						Status:        transaction.Status(),
						Balance:       balanceBefore,
					}

					return s.createReversalIdempotency(
						txCtx,
						input,
						result,
					)
				}

				return err
			}

			expectedVersion := wallet.Version() - 1

			if err := s.wallets.UpdateBalance(
				txCtx,
				wallet,
				expectedVersion,
			); err != nil {
				return err
			}

			ledgerEntry, err := domain.NewLedgerEntry(
				uuid.New(),
				wallet.ID(),
				transaction.ID(),
				direction,
				amount,
				balanceBefore,
				wallet.Balance(),
				now,
			)
			if err != nil {
				return err
			}

			if err := s.ledger.Create(
				txCtx,
				ledgerEntry,
			); err != nil {
				return err
			}

			if err := transaction.MarkProcessed(
				wallet.Balance(),
				now,
			); err != nil {
				return err
			}

			if err := s.transactions.Update(
				txCtx,
				transaction,
			); err != nil {
				return err
			}

			result = WagerResult{
				TransactionID: transaction.ID(),
				Status:        transaction.Status(),
				Balance:       wallet.Balance(),
			}

			if err := s.createReversalIdempotency(
				txCtx,
				input,
				result,
			); err != nil {
				return err
			}

			return s.createReversalEvent(
				txCtx,
				transaction,
				reference,
				wallet.Balance(),
				now,
			)
		},
	)

	if err != nil {
		if errors.Is(err, domain.ErrDuplicateWagerIdempotency) ||
			errors.Is(err, domain.ErrDuplicateIdempotencyRecord) {

			existing, lookupErr := s.idempotency.Find(
				ctx,
				input.ProviderID,
				input.IdempotencyKey,
			)
			if lookupErr != nil {
				return WagerResult{}, fmt.Errorf(
					"duplicate reversal idempotency: lookup result: %w",
					lookupErr,
				)
			}

			replayed, replayErr := replayReversalIdempotencyRecord(
				existing,
				input.PayloadHash,
			)
			if replayErr != nil {
				return WagerResult{}, replayErr
			}

			replayed.IdempotentReplay = true

			return replayed, nil
		}

		return WagerResult{}, err
	}

	return result, nil
}

func (s *ReversalService) createPendingReference(
	ctx context.Context,
	input ReverseTransactionInput,
	now time.Time,
) (WagerResult, error) {
	/*
		The wallet is required here because the reversal amount is not known
		yet, but the aggregate still needs its currency.
	*/
	wallet, err := s.wallets.GetByID(
		ctx,
		input.WalletID,
	)
	if err != nil {
		return WagerResult{}, err
	}

	if wallet.PlayerID() != input.PlayerID {
		return WagerResult{}, domain.ErrInvalidWallet
	}

	transaction, err :=
		domain.NewPendingReferenceTransaction(
			input.ID,
			input.ExternalTransactionID,
			input.ProviderID,
			input.IdempotencyKey,
			input.PayloadHash,
			input.WalletID,
			input.PlayerID,
			input.RoundID,
			input.GameID,
			input.Kind,
			wallet.Currency(),
			input.ReferenceExternalID,
			now,
		)
	if err != nil {
		return WagerResult{}, err
	}

	if err := s.transactions.Create(
		ctx,
		transaction,
	); err != nil {
		return WagerResult{}, err
	}

	expiresAt := now.Add(referenceTTL)
	availableAt := now.Add(referenceRetryDelay)

	if err := s.transactions.UpdateReferenceRetry(
		ctx,
		transaction.ID(),
		0,
		&availableAt,
	); err != nil {
		return WagerResult{}, err
	}

	/*
		Re-read the transaction is intentionally avoided here. The result
		uses the current wallet balance because no financial operation has
		yet occurred.
	*/
	result := WagerResult{
		TransactionID: transaction.ID(),
		Status:        transaction.Status(),
		Balance:       wallet.Balance(),
	}

	if err := s.createReversalIdempotency(
		ctx,
		input,
		result,
	); err != nil {
		return WagerResult{}, err
	}

	_ = expiresAt

	return result, nil
}

func (s *ReversalService) rejectReversal(
	ctx context.Context,
	input ReverseTransactionInput,
	now time.Time,
	failureCode string,
	amount domain.Money,
) (WagerResult, error) {
	/*
		If the reference amount is zero, NewExternalTransaction cannot
		represent the rejected reversal because external reversal
		transactions require a positive amount.

		In that case we still persist the reversal as PENDING_REFERENCE,
		resolve the reference, and then reject it.
	*/
	var transaction domain.WagerTransaction
	var err error

	if amount.IsZero() {
		transaction, err =
			domain.NewPendingReferenceTransaction(
				input.ID,
				input.ExternalTransactionID,
				input.ProviderID,
				input.IdempotencyKey,
				input.PayloadHash,
				input.WalletID,
				input.PlayerID,
				input.RoundID,
				input.GameID,
				input.Kind,
				amount.Currency(),
				input.ReferenceExternalID,
				now,
			)
		if err != nil {
			return WagerResult{}, err
		}

		if err := transaction.MarkRejected(
			failureCode,
			now,
		); err != nil {
			return WagerResult{}, err
		}
	} else {
		transaction, err = domain.NewExternalTransaction(
			input.ID,
			input.ExternalTransactionID,
			input.ProviderID,
			input.IdempotencyKey,
			input.PayloadHash,
			input.WalletID,
			input.PlayerID,
			input.RoundID,
			input.GameID,
			input.Kind,
			amount,
			input.ReferenceExternalID,
			now,
		)
		if err != nil {
			return WagerResult{}, err
		}

		if err := transaction.MarkRejected(
			failureCode,
			now,
		); err != nil {
			return WagerResult{}, err
		}
	}

	if err := s.transactions.Create(
		ctx,
		transaction,
	); err != nil {
		return WagerResult{}, err
	}

	if amount.IsZero() {
		/*
			The rejected LOSS rollback does not change the wallet.
			The actual balance is obtained from the wallet repository.
		*/
		wallet, err := s.wallets.GetByID(
			ctx,
			input.WalletID,
		)
		if err != nil {
			return WagerResult{}, err
		}

		result := WagerResult{
			TransactionID: transaction.ID(),
			Status:        transaction.Status(),
			Balance:       wallet.Balance(),
		}

		if err := s.createReversalIdempotency(
			ctx,
			input,
			result,
		); err != nil {
			return WagerResult{}, err
		}

		return result, nil
	}

	if err := s.transactions.Update(
		ctx,
		transaction,
	); err != nil {
		return WagerResult{}, err
	}

	wallet, err := s.wallets.GetByID(
		ctx,
		input.WalletID,
	)
	if err != nil {
		return WagerResult{}, err
	}

	result := WagerResult{
		TransactionID: transaction.ID(),
		Status:        transaction.Status(),
		Balance:       wallet.Balance(),
	}

	if err := s.createReversalIdempotency(
		ctx,
		input,
		result,
	); err != nil {
		return WagerResult{}, err
	}

	return result, nil
}

func (s *ReversalService) resultFromTransaction(
	ctx context.Context,
	transaction domain.WagerTransaction,
) (WagerResult, error) {
	wallet, err := s.wallets.GetByID(
		ctx,
		transaction.WalletID(),
	)
	if err != nil {
		return WagerResult{}, err
	}

	return WagerResult{
		TransactionID: transaction.ID(),
		Status:        transaction.Status(),
		Balance:       wallet.Balance(),
	}, nil
}

func (s *ReversalService) createReversalIdempotency(
	ctx context.Context,
	input ReverseTransactionInput,
	result WagerResult,
) error {
	responseBody, err := marshalWagerResult(result)
	if err != nil {
		return fmt.Errorf(
			"marshal reversal result: %w",
			err,
		)
	}

	return s.idempotency.Create(
		ctx,
		ports.IdempotencyRecord{
			ProviderID:              input.ProviderID,
			IdempotencyKey:          input.IdempotencyKey,
			PayloadHash:             input.PayloadHash,
			TransactionID:           result.TransactionID,
			Status:                  string(result.Status),
			ResponseBody:            responseBody,
			ObservedBalanceAmount:   result.Balance.Amount(),
			ObservedBalanceCurrency: string(result.Balance.Currency()),
		},
	)
}

func (s *ReversalService) createReversalEvent(
	ctx context.Context,
	transaction domain.WagerTransaction,
	reference domain.WagerTransaction,
	balance domain.Money,
	now time.Time,
) error {
	payload, err := json.Marshal(
		reversalEventPayload{
			TransactionID:          transaction.ID().String(),
			ExternalTransactionID:  transaction.ExternalTransactionID(),
			ProviderID:             transaction.ProviderID(),
			PlayerID:               transaction.PlayerID().String(),
			WalletID:               transaction.WalletID().String(),
			RoundID:                transaction.RoundID(),
			GameID:                 transaction.GameID(),
			Kind:                   string(transaction.Kind()),
			Amount:                 transaction.Money().String(),
			Currency:               string(transaction.Money().Currency()),
			Balance:                balance.String(),
			Status:                 string(transaction.Status()),
			ReferenceExternalID:    transaction.ReferenceExternalID(),
			ReferenceTransactionID: reference.ID().String(),
			FailureCode:            transaction.FailureCode(),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"marshal reversal event: %w",
			err,
		)
	}

	eventType := "WAGER_REFUND_PROCESSED"

	if transaction.Kind() == domain.TransactionKindRollback {
		eventType = "WAGER_ROLLBACK_PROCESSED"
	}

	return s.outbox.Create(
		ctx,
		ports.OutgoingEvent{
			EventID:       uuid.New().String(),
			EventType:     eventType,
			AggregateID:   transaction.WalletID().String(),
			CorrelationID: transaction.ID().String(),
			CausationID:   reference.ID().String(),
			OccurredAt:    now.Format(time.RFC3339Nano),
			Version:       1,
			Payload:       payload,
		},
	)
}

func replayReversalIdempotencyRecord(
	record ports.IdempotencyRecord,
	payloadHash string,
) (WagerResult, error) {
	if record.PayloadHash != payloadHash {
		return WagerResult{}, domain.ErrIdempotencyConflict
	}

	if len(record.ResponseBody) == 0 {
		return WagerResult{}, errors.New(
			"reversal idempotency record has empty response",
		)
	}

	var response wagerResultResponse

	if err := json.Unmarshal(
		record.ResponseBody,
		&response,
	); err != nil {
		return WagerResult{}, fmt.Errorf(
			"decode reversal idempotency response: %w",
			err,
		)
	}

	transactionID, err := uuid.Parse(
		response.TransactionID,
	)
	if err != nil || transactionID == uuid.Nil {
		return WagerResult{}, errors.New(
			"reversal idempotency record has invalid transaction id",
		)
	}

	balance, err := domain.NewMoney(
		response.Balance,
		domain.Currency(response.Currency),
	)
	if err != nil {
		return WagerResult{}, fmt.Errorf(
			"decode reversal idempotency balance: %w",
			err,
		)
	}

	return WagerResult{
		TransactionID:    transactionID,
		Status:           domain.TransactionStatus(response.Status),
		Balance:          balance,
		IdempotentReplay: true,
	}, nil
}

func validateReverseTransactionInput(
	input ReverseTransactionInput,
) error {
	if input.ID == uuid.Nil ||
		input.ExternalTransactionID == "" ||
		input.ProviderID == "" ||
		input.IdempotencyKey == "" ||
		input.PayloadHash == "" {
		return domain.ErrInvalidTransaction
	}

	if input.WalletID == uuid.Nil ||
		input.PlayerID == uuid.Nil {
		return domain.ErrInvalidTransaction
	}

	if input.ReferenceExternalID == "" {
		return domain.ErrInvalidTransaction
	}

	switch input.Kind {
	case domain.TransactionKindRefund,
		domain.TransactionKindRollback:
		return nil

	default:
		return domain.ErrInvalidTransactionKind
	}
}

func reversalLedgerDirection(
	reversalKind domain.TransactionKind,
	referenceKind domain.TransactionKind,
) domain.LedgerDirection {
	if reversalKind == domain.TransactionKindRefund {
		return domain.LedgerDirectionCredit
	}

	switch referenceKind {
	case domain.TransactionKindBet,
		domain.TransactionKindLoss:
		return domain.LedgerDirectionCredit

	case domain.TransactionKindWin,
		domain.TransactionKindRefund:
		return domain.LedgerDirectionDebit

	default:
		return domain.LedgerDirectionCredit
	}
}
