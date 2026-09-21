package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/oidc"
	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/adapters/sqs"
	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/config"
	httpserver "github.com/junglegaming/backend-challenge-go/internal/http"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

const (
	pendingReferenceWorkerInterval = 5 * time.Second
	pendingReferenceWorkerBatch    = 10
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,

			func(cfg config.Config) string {
				return cfg.Database
			},

			postgres.NewPool,

			postgres.NewMigrator,

			func(db *pgxpool.Pool) ports.WalletRepository {
				return postgres.NewWalletRepository(db)
			},

			func(db *pgxpool.Pool) ports.WagerTransactionRepository {
				return postgres.NewWagerTransactionRepository(db)
			},

			func(db *pgxpool.Pool) ports.LedgerRepository {
				return postgres.NewLedgerRepository(db)
			},

			func(db *pgxpool.Pool) ports.IdempotencyRepository {
				return postgres.NewIdempotencyRepository(db)
			},

			func(db *pgxpool.Pool) ports.OutboxRepository {
				return postgres.NewOutboxRepository(db)
			},

			func(db *pgxpool.Pool) ports.InboxRepository {
				return postgres.NewInboxRepository(db)
			},

			func(db *pgxpool.Pool) ports.TransactionManager {
				return postgres.NewTransactionManager(db)
			},

			func(cfg config.Config) (ports.MessageConsumer, error) {
				return sqs.NewMessageConsumer(
					context.Background(),
					cfg.AWSRegion,
					cfg.AWSAccessKeyID,
					cfg.AWSSecretAccessKey,
					cfg.AWSEndpointURL,
					cfg.SQSQueueURL,
				)
			},

			func(cfg config.Config) (ports.EventPublisher, error) {
				return sqs.NewEventPublisher(
					context.Background(),
					cfg.AWSRegion,
					cfg.AWSAccessKeyID,
					cfg.AWSSecretAccessKey,
					cfg.AWSEndpointURL,
					cfg.SQSEventsQueueURL,
				)
			},

			func(cfg config.Config) (ports.TokenValidator, error) {
				return oidc.NewVerifier(
					context.Background(),
					cfg.OIDCIssuerURL,
					cfg.OIDCClientID,
					cfg.OIDCAudience,
				)
			},

			application.NewReversalService,
			application.NewWagerService,
			application.NewWagerConsumer,
			application.NewPendingReferenceWorker,

			func(
				outbox ports.OutboxRepository,
				eventPublisher ports.EventPublisher,
				cfg config.Config,
			) *application.OutboxPublisher {
				return application.NewOutboxPublisher(
					outbox,
					eventPublisher,
					cfg.OutboxPollInterval,
					cfg.OutboxBatchSize,
					cfg.OutboxWorkerID,
				)
			},

			httpserver.NewServer,
		),

		fx.Invoke(
			registerMigrations,
			registerLifecycle,
			registerHTTPServer,
			registerOutboxPublisher,
			registerWagerConsumer,
			registerPendingReferenceWorker,
		),

		fx.NopLogger,
	).Run()
}

func registerMigrations(
	migrator *postgres.Migrator,
) error {
	if err := migrator.Run(
		context.Background(),
	); err != nil {
		return err
	}

	log.Println(
		"database migrations completed",
	)

	return nil
}

func registerHTTPServer(
	server *httpserver.Server,
) {
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	pool *pgxpool.Pool,
) {
	lifecycle.Append(
		fx.Hook{
			OnStop: func(ctx context.Context) error {
				log.Println(
					"closing postgres connection pool",
				)

				pool.Close()

				return nil
			},
		},
	)
}

func registerOutboxPublisher(
	lifecycle fx.Lifecycle,
	publisher *application.OutboxPublisher,
) {
	var cancel context.CancelFunc

	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				publisherCtx, publisherCancel :=
					context.WithCancel(context.Background())

				cancel = publisherCancel

				go func() {
					if err := publisher.Run(
						publisherCtx,
					); err != nil &&
						publisherCtx.Err() == nil {
						log.Printf(
							"outbox publisher stopped with error: %v",
							err,
						)
					}
				}()

				log.Println(
					"outbox publisher started",
				)

				return nil
			},

			OnStop: func(ctx context.Context) error {
				if cancel != nil {
					cancel()
				}

				log.Println(
					"outbox publisher stopped",
				)

				return nil
			},
		},
	)
}

func registerWagerConsumer(
	lifecycle fx.Lifecycle,
	consumer *application.WagerConsumer,
) {
	var cancel context.CancelFunc

	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				consumerCtx, consumerCancel :=
					context.WithCancel(context.Background())

				cancel = consumerCancel

				go func() {
					if err := consumer.Run(
						consumerCtx,
					); err != nil &&
						consumerCtx.Err() == nil {
						log.Printf(
							"wager consumer stopped with error: %v",
							err,
						)
					}
				}()

				log.Println(
					"wager consumer started",
				)

				return nil
			},

			OnStop: func(ctx context.Context) error {
				if cancel != nil {
					cancel()
				}

				log.Println(
					"wager consumer stopped",
				)

				return nil
			},
		},
	)
}

func registerPendingReferenceWorker(
	lifecycle fx.Lifecycle,
	worker *application.PendingReferenceWorker,
) {
	var cancel context.CancelFunc

	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				workerCtx, workerCancel :=
					context.WithCancel(context.Background())

				cancel = workerCancel

				go func() {
					ticker := time.NewTicker(
						pendingReferenceWorkerInterval,
					)
					defer ticker.Stop()

					for {
						processed, err := worker.ProcessBatch(
							workerCtx,
							pendingReferenceWorkerBatch,
						)

						if err != nil {
							if workerCtx.Err() != nil {
								return
							}

							log.Printf(
								"pending reference worker error: %v",
								err,
							)

							select {
							case <-workerCtx.Done():
								return
							case <-ticker.C:
							}

							continue
						}

						if processed > 0 {
							log.Printf(
								"pending reference worker processed %d transaction(s)",
								processed,
							)
						}

						select {
						case <-workerCtx.Done():
							return
						case <-ticker.C:
						}
					}
				}()

				log.Println(
					"pending reference worker started",
				)

				return nil
			},

			OnStop: func(ctx context.Context) error {
				if cancel != nil {
					cancel()
				}

				log.Println(
					"pending reference worker stopped",
				)

				return nil
			},
		},
	)
}
