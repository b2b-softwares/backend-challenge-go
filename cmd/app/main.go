package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/junglegaming/backend-challenge-go/internal/adapters/postgres"
	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/config"
	httpserver "github.com/junglegaming/backend-challenge-go/internal/http"
	"github.com/junglegaming/backend-challenge-go/internal/ports"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,

			func(cfg config.Config) string {
				return cfg.Database
			},

			postgres.NewPool,

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

			func(db *pgxpool.Pool) ports.TransactionManager {
				return postgres.NewTransactionManager(db)
			},

			application.NewWagerService,

			httpserver.NewServer,
		),

		fx.Invoke(
			registerLifecycle,
			registerHTTPServer,
		),

		fx.NopLogger,
	).Run()
}

func registerHTTPServer(
	server *httpserver.Server,
) {
	// The HTTP server lifecycle is registered by NewServer.
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
