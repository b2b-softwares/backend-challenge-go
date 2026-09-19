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

const failureCodeInsufficientBalance = "INSUFFICIENT_BALANCE"

const maxConcurrentModificationRetries = 3

type WagerService struct {
	wallets       ports.WalletRepository
	transactions  ports.WagerTransactionRepository
	ledger        ports.LedgerRepository
	idempotency   ports.IdempotencyRepository
	outbox        ports.OutboxRepository
	transactionDB ports.TransactionManager
}

func NewWagerService(
	wallets ports.WalletRepository,
	transactions ports.WagerTransactionRepository,
	ledger ports.LedgerRepository,
	idempotency ports.IdempotencyRepository,
	outbox ports.OutboxRepository,
	transactionDB ports.TransactionManager,
) *WagerService {
	return &WagerService{
		wallets:       wallets,
		transactions:  transactions,
		ledger:        ledger,
		idempotency:   idempotency,
		outbox:        outbox,
		transactionDB: transactionDB,
	}
}

type PlaceBetInput struct {
	ID                    uuid.UUID
	ExternalTransactionID string
	ProviderID            string
	IdempotencyKey        string
	PayloadHash           string
	WalletID              uuid.UUID
	PlayerID              uuid.UUID
	RoundID               string
	GameID                string
	Amount                domain.Money
}

type WagerResult struct {
	TransactionID    uuid.UUID
	Status           domain.TransactionStatus
	Balance          domain.Money
	IdempotentReplay bool
}

type wagerResultResponse struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
	Balance       string `json:"balance"`
	Currency      string `json:"currency"`
}

type betEventPayload struct {
	TransactionID         string `json:"transactionId"`
	ExternalTransactionID string `json:"externalTransactionId"`
	ProviderID            string `json:"providerId"`
	PlayerID              string `json:"playerId"`
	WalletID              string `json:"walletId"`
	RoundID               string `json:"roundId"`
	GameID                string `json:"gameId"`
	Kind                  string `json:"kind"`
	Amount                string `json:"amount"`
	Currency              string `json:"currency"`
	Balance               string `json:"balance"`
	Status                string `json:"status"`
	FailureCode           string `json:"failureCode,omitempty"`
}

func (s *WagerService) PlaceBet(
	ctx context.Context,
	input PlaceBetInput,
) (WagerResult, error) {
	if s == nil ||
		s.wallets == nil ||
		s.transactions == nil ||
		s.ledger == nil ||
		s.idempotency == nil ||
		s.outbox == nil ||
		s.transactionDB == nil {
		return WagerResult{}, errors.New(
			"wager service: dependencies are required",
		)
	}

	if err := validatePlaceBetInput(input); err != nil {
		return WagerResult{}, err
	}

	existing, err := s.idempotency.Find(
		ctx,
		input.ProviderID,
		input.IdempotencyKey,
	)
	if err == nil {
		return replayIdempotencyRecord(
			existing,
			input.PayloadHash,
		)
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return WagerResult{}, fmt.Errorf(
			"find idempotency record: %w",
			err,
		)
	}

	now := time.Now().UTC()

	transaction, err := domain.NewExternalTransaction(
		input.ID,
		input.ExternalTransactionID,
		input.ProviderID,
		input.IdempotencyKey,
		input.PayloadHash,
		input.WalletID,
		input.PlayerID,
		input.RoundID,
		input.GameID,
		domain.TransactionKindBet,
		input.Amount,
		"",
		now,
	)
	if err != nil {
		return WagerResult{}, err
	}

	var result WagerResult
	var rejectedByInsufficientBalance bool

	processTransaction := func() error {
		rejectedByInsufficientBalance = false

		return s.transactionDB.WithinTransaction(
			ctx,
			func(txCtx context.Context) error {
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

				if wallet.Currency() != input.Amount.Currency() {
					return domain.ErrCurrencyMismatch
				}

				balanceBefore := wallet.Balance()

				if err := wallet.Debit(
					input.Amount,
					now,
				); err != nil {
					if !errors.Is(err, domain.ErrInsufficientBalance) {
						return err
					}

					if err := transaction.MarkRejected(
						failureCodeInsufficientBalance,
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
						Balance:       balanceBefore,
					}

					responseBody, err := marshalWagerResult(result)
					if err != nil {
						return err
					}

					if err := s.idempotency.Create(
						txCtx,
						ports.IdempotencyRecord{
							ProviderID:              input.ProviderID,
							IdempotencyKey:          input.IdempotencyKey,
							PayloadHash:             input.PayloadHash,
							TransactionID:           transaction.ID(),
							Status:                  string(transaction.Status()),
							ResponseBody:            responseBody,
							ObservedBalanceAmount:   balanceBefore.Amount(),
							ObservedBalanceCurrency: string(balanceBefore.Currency()),
						},
					); err != nil {
						return err
					}

					eventPayload, err := json.Marshal(
						betEventPayload{
							TransactionID:         transaction.ID().String(),
							ExternalTransactionID: transaction.ExternalTransactionID(),
							ProviderID:            transaction.ProviderID(),
							PlayerID:              transaction.PlayerID().String(),
							WalletID:              transaction.WalletID().String(),
							RoundID:               transaction.RoundID(),
							GameID:                transaction.GameID(),
							Kind:                  string(transaction.Kind()),
							Amount:                transaction.Money().String(),
							Currency:              string(transaction.Money().Currency()),
							Balance:               balanceBefore.String(),
							Status:                string(transaction.Status()),
							FailureCode:           transaction.FailureCode(),
						},
					)
					if err != nil {
						return fmt.Errorf(
							"marshal rejected bet event: %w",
							err,
						)
					}

					if err := s.outbox.Create(
						txCtx,
						ports.OutgoingEvent{
							EventID:       uuid.New().String(),
							EventType:     "WAGER_BET_REJECTED",
							AggregateID:   wallet.ID().String(),
							CorrelationID: transaction.ID().String(),
							CausationID:   transaction.ID().String(),
							OccurredAt:    now.Format(time.RFC3339Nano),
							Version:       int(wallet.Version()),
							Payload:       eventPayload,
						},
					); err != nil {
						return err
					}

					rejectedByInsufficientBalance = true

					return nil
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
					domain.LedgerDirectionDebit,
					input.Amount,
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

				responseBody, err := marshalWagerResult(result)
				if err != nil {
					return err
				}

				if err := s.idempotency.Create(
					txCtx,
					ports.IdempotencyRecord{
						ProviderID:              input.ProviderID,
						IdempotencyKey:          input.IdempotencyKey,
						PayloadHash:             input.PayloadHash,
						TransactionID:           transaction.ID(),
						Status:                  string(transaction.Status()),
						ResponseBody:            responseBody,
						ObservedBalanceAmount:   wallet.Balance().Amount(),
						ObservedBalanceCurrency: string(wallet.Balance().Currency()),
					},
				); err != nil {
					return err
				}

				eventPayload, err := json.Marshal(
					betEventPayload{
						TransactionID:         transaction.ID().String(),
						ExternalTransactionID: transaction.ExternalTransactionID(),
						ProviderID:            transaction.ProviderID(),
						PlayerID:              transaction.PlayerID().String(),
						WalletID:              transaction.WalletID().String(),
						RoundID:               transaction.RoundID(),
						GameID:                transaction.GameID(),
						Kind:                  string(transaction.Kind()),
						Amount:                transaction.Money().String(),
						Currency:              string(transaction.Money().Currency()),
						Balance:               wallet.Balance().String(),
						Status:                string(transaction.Status()),
					},
				)
				if err != nil {
					return fmt.Errorf(
						"marshal bet event: %w",
						err,
					)
				}

				if err := s.outbox.Create(
					txCtx,
					ports.OutgoingEvent{
						EventID:       uuid.New().String(),
						EventType:     "WAGER_BET_PROCESSED",
						AggregateID:   wallet.ID().String(),
						CorrelationID: transaction.ID().String(),
						CausationID:   transaction.ID().String(),
						OccurredAt:    now.Format(time.RFC3339Nano),
						Version:       int(wallet.Version()),
						Payload:       eventPayload,
					},
				); err != nil {
					return err
				}

				return nil
			},
		)
	}

	for attempt := 1; attempt <= maxConcurrentModificationRetries; attempt++ {
		err = processTransaction()

		if !errors.Is(err, domain.ErrConcurrentModification) {
			break
		}

		if attempt == maxConcurrentModificationRetries {
			break
		}

		select {
		case <-ctx.Done():
			return WagerResult{}, ctx.Err()
		case <-time.After(time.Duration(attempt) * 10 * time.Millisecond):
		}
	}

	if err == nil && rejectedByInsufficientBalance {
		return result, domain.ErrInsufficientBalance
	}

	if err == nil {
		return result, nil
	}

	if errors.Is(err, domain.ErrDuplicateWagerIdempotency) {
		existing, lookupErr := s.idempotency.Find(
			ctx,
			input.ProviderID,
			input.IdempotencyKey,
		)

		if lookupErr != nil {
			return WagerResult{}, fmt.Errorf(
				"duplicate wager idempotency: lookup persisted result: %w",
				lookupErr,
			)
		}

		result, replayErr := replayIdempotencyRecord(
			existing,
			input.PayloadHash,
		)
		if replayErr != nil {
			return WagerResult{}, replayErr
		}

		result.IdempotentReplay = true

		return result, nil
	}

	if errors.Is(err, domain.ErrDuplicateIdempotencyRecord) {
		existing, lookupErr := s.idempotency.Find(
			ctx,
			input.ProviderID,
			input.IdempotencyKey,
		)

		if lookupErr != nil {
			return WagerResult{}, fmt.Errorf(
				"duplicate idempotency record: lookup persisted result: %w",
				lookupErr,
			)
		}

		result, replayErr := replayIdempotencyRecord(
			existing,
			input.PayloadHash,
		)
		if replayErr != nil {
			return WagerResult{}, replayErr
		}

		result.IdempotentReplay = true

		return result, nil
	}

	return WagerResult{}, err
}

func validatePlaceBetInput(input PlaceBetInput) error {
	if input.ID == uuid.Nil {
		return domain.ErrInvalidTransaction
	}

	if input.ExternalTransactionID == "" ||
		input.ProviderID == "" ||
		input.IdempotencyKey == "" ||
		input.PayloadHash == "" {
		return domain.ErrInvalidTransaction
	}

	if input.WalletID == uuid.Nil ||
		input.PlayerID == uuid.Nil {
		return domain.ErrInvalidTransaction
	}

	if !input.Amount.IsPositive() {
		return domain.ErrInvalidMoney
	}

	return nil
}

func replayIdempotencyRecord(
	record ports.IdempotencyRecord,
	payloadHash string,
) (WagerResult, error) {
	if record.PayloadHash != payloadHash {
		return WagerResult{}, domain.ErrIdempotencyConflict
	}

	if len(record.ResponseBody) == 0 {
		return WagerResult{}, errors.New(
			"idempotency record has empty response",
		)
	}

	var response wagerResultResponse

	if err := json.Unmarshal(
		record.ResponseBody,
		&response,
	); err != nil {
		return WagerResult{}, fmt.Errorf(
			"decode idempotency response: %w",
			err,
		)
	}

	transactionID, err := uuid.Parse(
		response.TransactionID,
	)
	if err != nil || transactionID == uuid.Nil {
		return WagerResult{}, errors.New(
			"idempotency record has invalid transaction id",
		)
	}

	status := domain.TransactionStatus(response.Status)

	if status != domain.TransactionStatusProcessed &&
		status != domain.TransactionStatusRejected &&
		status != domain.TransactionStatusFailed {
		return WagerResult{}, errors.New(
			"idempotency record has invalid terminal status",
		)
	}

	if response.Currency == "" {
		return WagerResult{}, errors.New(
			"idempotency record has empty currency",
		)
	}

	balance, err := domain.NewMoney(
		response.Balance,
		domain.Currency(response.Currency),
	)
	if err != nil {
		return WagerResult{}, fmt.Errorf(
			"decode idempotency balance: %w",
			err,
		)
	}

	return WagerResult{
		TransactionID:    transactionID,
		Status:           status,
		Balance:          balance,
		IdempotentReplay: true,
	}, nil
}

func marshalWagerResult(
	result WagerResult,
) ([]byte, error) {
	return json.Marshal(
		wagerResultResponse{
			TransactionID: result.TransactionID.String(),
			Status:        string(result.Status),
			Balance:       result.Balance.String(),
			Currency:      string(result.Balance.Currency()),
		},
	)
}
