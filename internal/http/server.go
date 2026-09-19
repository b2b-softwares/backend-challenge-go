package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	nethttp "net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/config"
)

type Server struct {
	server *nethttp.Server
	pool   *pgxpool.Pool
}

func NewServer(
	lifecycle fx.Lifecycle,
	cfg config.Config,
	pool *pgxpool.Pool,
	wagerService *application.WagerService,
) *Server {
	handler := NewHandler(
		pool,
		wagerService,
	)

	server := &Server{
		server: &nethttp.Server{
			Addr:              ":" + cfg.HTTPPort,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		pool: pool,
	}

	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					log.Printf(
						"HTTP server listening on %s",
						server.server.Addr,
					)

					err := server.server.ListenAndServe()
					if err != nil &&
						!errors.Is(err, nethttp.ErrServerClosed) {
						log.Printf(
							"HTTP server stopped with error: %v",
							err,
						)
					}
				}()

				return nil
			},

			OnStop: func(ctx context.Context) error {
				shutdownCtx, cancel := context.WithTimeout(
					ctx,
					5*time.Second,
				)
				defer cancel()

				return server.server.Shutdown(shutdownCtx)
			},
		},
	)

	return server
}

func writeJSON(
	w nethttp.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf(
			"write JSON response: %v",
			err,
		)
	}
}
