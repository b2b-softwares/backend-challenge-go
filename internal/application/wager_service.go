package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	otmetric "go.opentelemetry.io/otel/metric"

	"github.com/junglegaming/backend-challenge-go/internal/domain"
	"github.com/junglegaming/backend-challenge-go/internal/observability"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const failureCodeInsufficientBalance = "INSUFFICIENT_BALANCE"

const maxConcurrentModificationRetries = 3

type wagerMetrics struct {
	transactionsTotal          otmetric.Int64Counter
	transactionsProcessedTotal otmetric.Int64Counter
	transactionsRejectedTotal  otmetric.Int64Counter
	processingDurationSeconds  otmetric.Float64Histogram
	idempotencyReplaysTotal    otmetric.Int64Counter
	idempotencyConflictsTotal  otmetric.Int64Counter
	insufficientBalanceTotal   otmetric.Int64Counter
}

func newWagerMetrics(
	providers *observability.Providers,
) *wagerMetrics {
	meter := observability.Meter(providers)

	transactionsTotal, _ := meter.Int64Counter(
		"wager_transactions_total",
		otmetric.WithDescription(
			"Total number of wager transactions completed with a terminal outcome.",
		),
	)

	transactionsProcessedTotal, _ := meter.Int64Counter(
		"wager_transactions_processed_total",
		otmetric.WithDescription(
			"Total number of wager transactions processed successfully.",
		),
	)

	transactionsRejectedTotal, _ := meter.Int64Counter(
		"wager_transactions_rejected_total",
		otmetric.WithDescription(
			"Total number of wager transactions rejected.",
		),
	)

	processingDurationSeconds, _ := meter.Float64Histogram(
		"wager_processing_duration_seconds",
		otmetric.WithDescription(
			"Time spent processing wager requests.",
		),
		otmetric.WithUnit("s"),
	)

	idempotencyReplaysTotal, _ := meter.Int64Counter(
		"wager_idempotency_replays_total",
		otmetric.WithDescription(
			"Total number of wager requests served by idempotency replay.",
		),
	)

	idempotencyConflictsTotal, _ := meter.Int64Counter(
		"wager_idempotency_conflicts_total",
		otmetric.WithDescription(
			"Total number of idempotency conflicts.",
		),
	)

	insufficientBalanceTotal, _ := meter.Int64Counter(
		"wallet_insufficient_balance_total",
		otmetric.WithDescription(
			"Total number of wager attempts rejected due to insufficient wallet balance.",
		),
	)

	return &wagerMetrics{
		transactionsTotal:          transactionsTotal,
		transactionsProcessedTotal: transactionsProcessedTotal,
		transactionsRejectedTotal:  transactionsRejectedTotal,
		processingDurationSeconds:  processingDurationSeconds,
		idempotencyReplaysTotal:    idempotencyReplaysTotal,
		idempotencyConflictsTotal:  idempotencyConflictsTotal,
		insufficientBalanceTotal:   insufficientBalanceTotal,
	}
}

type WagerService struct {
	wallets       ports.WalletRepository
	transactions  ports.WagerTransactionRepository
	ledger        ports.LedgerRepository
	idempotency   ports.IdempotencyRepository
	outbox        ports.OutboxRepository
	transactionDB ports.TransactionManager
	metrics       *wagerMetrics
}

func NewWagerService(
	wallets ports.WalletRepository,
	transactions ports.WagerTransactionRepository,
	ledger ports.LedgerRepository,
	idempotency ports.IdempotencyRepository,
	outbox ports.OutboxRepository,
	transactionDB ports.TransactionManager,
	providers *observability.Providers,
) *WagerService {
	return &WagerService{
		wallets:       wallets,
		transactions:  transactions,
		ledger:        ledger,
		idempotency:   idempotency,
		outbox:        outbox,
		transactionDB: transactionDB,
		metrics:       newWagerMetrics(providers),
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
	startedAt := time.Now()

	metricStatus := "error"

	defer func() {
		if s == nil || s.metrics == nil {
			return
		}

		s.metrics.processingDurationSeconds.Record(
			ctx,
			time.Since(startedAt).Seconds(),
			otmetric.WithAttributes(
				attribute.String("status", metricStatus),
			),
		)
	}()

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
		result, replayErr := replayIdempotencyRecord(
			existing,
			input.PayloadHash,
		)

		if replayErr != nil {
			if errors.Is(
				replayErr,
				domain.ErrIdempotencyConflict,
			) {
				s.recordIdempotencyConflict(ctx)
				metricStatus = "idempotency_conflict"
			}

			return WagerResult{}, replayErr
		}

		s.recordIdempotencyReplay(ctx)
		metricStatus = "replayed"

		return result, nil
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
					if !errors.Is(
						err,
						domain.ErrInsufficientBalance,
					) {
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
		case <-time.After(
			time.Duration(attempt) * 10 * time.Millisecond,
		):
		}
	}

	if err == nil && rejectedByInsufficientBalance {
		s.recordRejectedTransaction(ctx)
		s.recordInsufficientBalance(ctx)

		metricStatus = "rejected"

		return result, domain.ErrInsufficientBalance
	}

	if err == nil {
		s.recordProcessedTransaction(ctx)

		metricStatus = "processed"

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
			if errors.Is(
				replayErr,
				domain.ErrIdempotencyConflict,
			) {
				s.recordIdempotencyConflict(ctx)
				metricStatus = "idempotency_conflict"
			}

			return WagerResult{}, replayErr
		}

		s.recordIdempotencyReplay(ctx)

		metricStatus = "replayed"

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
			if errors.Is(
				replayErr,
				domain.ErrIdempotencyConflict,
			) {
				s.recordIdempotencyConflict(ctx)
				metricStatus = "idempotency_conflict"
			}

			return WagerResult{}, replayErr
		}

		s.recordIdempotencyReplay(ctx)

		metricStatus = "replayed"

		result.IdempotentReplay = true

		return result, nil
	}

	return WagerResult{}, err
}

func (s *WagerService) recordProcessedTransaction(
	ctx context.Context,
) {
	if s == nil || s.metrics == nil {
		return
	}

	s.metrics.transactionsTotal.Add(
		ctx,
		1,
		otmetric.WithAttributes(
			attribute.String("status", "processed"),
		),
	)

	s.metrics.transactionsProcessedTotal.Add(
		ctx,
		1,
	)
}

func (s *WagerService) recordRejectedTransaction(
	ctx context.Context,
) {
	if s == nil || s.metrics == nil {
		return
	}

	s.metrics.transactionsTotal.Add(
		ctx,
		1,
		otmetric.WithAttributes(
			attribute.String("status", "rejected"),
		),
	)

	s.metrics.transactionsRejectedTotal.Add(
		ctx,
		1,
	)
}

func (s *WagerService) recordInsufficientBalance(
	ctx context.Context,
) {
	if s == nil || s.metrics == nil {
		return
	}

	s.metrics.insufficientBalanceTotal.Add(ctx, 1)
}

func (s *WagerService) recordIdempotencyReplay(
	ctx context.Context,
) {
	if s == nil || s.metrics == nil {
		return
	}

	s.metrics.idempotencyReplaysTotal.Add(ctx, 1)
}

func (s *WagerService) recordIdempotencyConflict(
	ctx context.Context,
) {
	if s == nil || s.metrics == nil {
		return
	}

	s.metrics.idempotencyConflictsTotal.Add(ctx, 1)
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
