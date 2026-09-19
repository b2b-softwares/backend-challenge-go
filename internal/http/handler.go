package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/junglegaming/backend-challenge-go/internal/application"
	"github.com/junglegaming/backend-challenge-go/internal/domain"
)

type Handler struct {
	pool         *pgxpool.Pool
	wagerService *application.WagerService
}

func NewHandler(
	pool *pgxpool.Pool,
	wagerService *application.WagerService,
) http.Handler {
	h := &Handler{
		pool:         pool,
		wagerService: wagerService,
	}

	mux := http.NewServeMux()

	mux.HandleFunc(
		"/health/live",
		h.handleLive,
	)

	mux.HandleFunc(
		"/health/ready",
		h.handleReady,
	)

	mux.HandleFunc(
		"/wagering/transactions",
		h.handleWagerTransaction,
	)

	mux.HandleFunc(
		"/docs/",
		h.handleDocs,
	)

	mux.HandleFunc(
		"/docs/openapi.json",
		h.handleOpenAPI,
	)

	return mux
}

type healthResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type wagerTransactionRequest struct {
	ExternalTransactionID string `json:"externalTransactionId"`
	ProviderID            string `json:"providerId"`
	WalletID              string `json:"walletId"`
	PlayerID              string `json:"playerId"`
	RoundID               string `json:"roundId"`
	GameID                string `json:"gameId"`
	Amount                string `json:"amount"`
	Currency              string `json:"currency"`
}

type wagerTransactionResponse struct {
	TransactionID    string `json:"transactionId"`
	Status           string `json:"status"`
	Balance          string `json:"balance"`
	Currency         string `json:"currency"`
	IdempotentReplay bool   `json:"idempotentReplay"`
}

func (h *Handler) handleLive(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			errorResponse{
				Error: "method not allowed",
			},
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		healthResponse{
			Status: "UP",
		},
	)
}

func (h *Handler) handleReady(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			errorResponse{
				Error: "method not allowed",
			},
		)
		return
	}

	if h.pool == nil {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			healthResponse{
				Status: "DOWN",
			},
		)
		return
	}

	if err := h.pool.Ping(r.Context()); err != nil {
		writeJSON(
			w,
			http.StatusServiceUnavailable,
			healthResponse{
				Status: "DOWN",
			},
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		healthResponse{
			Status: "UP",
		},
	)
}

func (h *Handler) handleWagerTransaction(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			errorResponse{
				Error: "method not allowed",
			},
		)
		return
	}

	idempotencyKey := strings.TrimSpace(
		r.Header.Get("Idempotency-Key"),
	)

	if idempotencyKey == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "Idempotency-Key header is required",
			},
		)
		return
	}

	body, err := io.ReadAll(
		io.LimitReader(r.Body, 1<<20),
	)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid request body",
			},
		)
		return
	}

	if len(bytes.TrimSpace(body)) == 0 {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "request body is required",
			},
		)
		return
	}

	var request wagerTransactionRequest

	decoder := json.NewDecoder(
		bytes.NewReader(body),
	)

	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid JSON request body",
			},
		)
		return
	}

	providerID := strings.TrimSpace(request.ProviderID)

	if providerID == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "providerId is required",
			},
		)
		return
	}

	externalTransactionID := strings.TrimSpace(
		request.ExternalTransactionID,
	)

	if externalTransactionID == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "externalTransactionId is required",
			},
		)
		return
	}

	walletID, err := uuid.Parse(
		strings.TrimSpace(request.WalletID),
	)
	if err != nil || walletID == uuid.Nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "walletId must be a valid UUID",
			},
		)
		return
	}

	playerID, err := uuid.Parse(
		strings.TrimSpace(request.PlayerID),
	)
	if err != nil || playerID == uuid.Nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "playerId must be a valid UUID",
			},
		)
		return
	}

	currency := domain.Currency(
		strings.ToUpper(
			strings.TrimSpace(request.Currency),
		),
	)

	if currency == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "currency is required",
			},
		)
		return
	}

	amount, err := domain.NewMoney(
		strings.TrimSpace(request.Amount),
		currency,
	)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid amount",
			},
		)
		return
	}

	payloadHash := hashPayload(body)

	result, err := h.wagerService.PlaceBet(
		r.Context(),
		application.PlaceBetInput{
			ID:                    uuid.New(),
			ExternalTransactionID: externalTransactionID,
			ProviderID:            providerID,
			IdempotencyKey:        idempotencyKey,
			PayloadHash:           payloadHash,
			WalletID:              walletID,
			PlayerID:              playerID,
			RoundID:               strings.TrimSpace(request.RoundID),
			GameID:                strings.TrimSpace(request.GameID),
			Amount:                amount,
		},
	)
	if err != nil {
		writeWagerError(
			w,
			err,
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		wagerTransactionResponse{
			TransactionID:    result.TransactionID.String(),
			Status:           string(result.Status),
			Balance:          result.Balance.String(),
			Currency:         string(result.Balance.Currency()),
			IdempotentReplay: result.IdempotentReplay,
		},
	)
}

func (h *Handler) handleDocs(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			errorResponse{
				Error: "method not allowed",
			},
		)
		return
	}

	const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Backend Challenge API</title>
	<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
	<div id="swagger-ui"></div>

	<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	<script>
		window.onload = function () {
			window.ui = SwaggerUIBundle({
				url: "/docs/openapi.json",
				dom_id: "#swagger-ui",
				deepLinking: true
			});
		};
	</script>
</body>
</html>`

	w.Header().Set(
		"Content-Type",
		"text/html; charset=utf-8",
	)

	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(swaggerHTML))
}

func (h *Handler) handleOpenAPI(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		writeJSON(
			w,
			http.StatusMethodNotAllowed,
			errorResponse{
				Error: "method not allowed",
			},
		)
		return
	}

	const openAPI = `{
	"openapi": "3.0.3",
	"info": {
		"title": "Backend Challenge - Wager Processing API",
		"description": "Distributed wagering backend API.",
		"version": "1.0.0"
	},
	"servers": [
		{
			"url": "http://localhost:8080"
		}
	],
	"paths": {
		"/health/live": {
			"get": {
				"summary": "Liveness check",
				"responses": {
					"200": {
						"description": "Application is alive",
						"content": {
							"application/json": {
								"schema": {
									"$ref": "#/components/schemas/HealthResponse"
								}
							}
						}
					}
				}
			}
		},
		"/health/ready": {
			"get": {
				"summary": "Readiness check",
				"responses": {
					"200": {
						"description": "Application and database are ready",
						"content": {
							"application/json": {
								"schema": {
									"$ref": "#/components/schemas/HealthResponse"
								}
							}
						}
					},
					"503": {
						"description": "Database is unavailable"
					}
				}
			}
		},
		"/wagering/transactions": {
			"post": {
				"summary": "Process a wager transaction",
				"description": "Creates and processes a BET transaction using the wallet balance.",
				"parameters": [
					{
						"name": "Idempotency-Key",
						"in": "header",
						"required": true,
						"description": "Unique key used to guarantee idempotent processing.",
						"schema": {
							"type": "string"
						}
					}
				],
				"requestBody": {
					"required": true,
					"content": {
						"application/json": {
							"schema": {
								"$ref": "#/components/schemas/WagerTransactionRequest"
							},
							"example": {
								"externalTransactionId": "http-test-ext-001",
								"providerId": "provider-test",
								"walletId": "5e37df95-1f00-4017-b8a8-b36ee182b284",
								"playerId": "df979626-e33d-4ad0-9f66-0dc629d16277",
								"roundId": "round-http-001",
								"gameId": "game-http-001",
								"amount": "10.00",
								"currency": "BRL"
							}
						}
					}
				},
				"responses": {
					"200": {
						"description": "Wager processed or idempotent replay",
						"content": {
							"application/json": {
								"schema": {
									"$ref": "#/components/schemas/WagerTransactionResponse"
								}
							}
						}
					},
					"400": {
						"description": "Invalid request"
					},
					"404": {
						"description": "Resource not found"
					},
					"409": {
						"description": "Idempotency conflict or insufficient balance"
					},
					"500": {
						"description": "Internal server error"
					}
				}
			}
		}
	},
	"components": {
		"schemas": {
			"HealthResponse": {
				"type": "object",
				"required": [
					"status"
				],
				"properties": {
					"status": {
						"type": "string",
						"example": "UP"
					}
				}
			},
			"WagerTransactionRequest": {
				"type": "object",
				"required": [
					"externalTransactionId",
					"providerId",
					"walletId",
					"playerId",
					"amount",
					"currency"
				],
				"properties": {
					"externalTransactionId": {
						"type": "string",
						"example": "http-test-ext-001"
					},
					"providerId": {
						"type": "string",
						"example": "provider-test"
					},
					"walletId": {
						"type": "string",
						"format": "uuid",
						"example": "5e37df95-1f00-4017-b8a8-b36ee182b284"
					},
					"playerId": {
						"type": "string",
						"format": "uuid",
						"example": "df979626-e33d-4ad0-9f66-0dc629d16277"
					},
					"roundId": {
						"type": "string",
						"example": "round-http-001"
					},
					"gameId": {
						"type": "string",
						"example": "game-http-001"
					},
					"amount": {
						"type": "string",
						"description": "Decimal monetary value. Do not use floating point.",
						"example": "10.00"
					},
					"currency": {
						"type": "string",
						"example": "BRL",
						"minLength": 3,
						"maxLength": 3
					}
				}
			},
			"WagerTransactionResponse": {
				"type": "object",
				"required": [
					"transactionId",
					"status",
					"balance",
					"currency",
					"idempotentReplay"
				],
				"properties": {
					"transactionId": {
						"type": "string",
						"format": "uuid"
					},
					"status": {
						"type": "string",
						"example": "PROCESSED"
					},
					"balance": {
						"type": "string",
						"example": "90.00"
					},
					"currency": {
						"type": "string",
						"example": "BRL"
					},
					"idempotentReplay": {
						"type": "boolean",
						"example": false
					}
				}
			}
		}
	}
}`

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(openAPI))
}

func hashPayload(payload []byte) string {
	hash := sha256.Sum256(payload)

	return hex.EncodeToString(
		hash[:],
	)
}

func writeWagerError(
	w http.ResponseWriter,
	err error,
) {
	status := http.StatusInternalServerError
	message := "internal server error"

	switch {
	case errors.Is(err, domain.ErrIdempotencyConflict):
		status = http.StatusConflict
		message = "idempotency key was already used with a different payload"

	case errors.Is(err, domain.ErrInvalidTransaction),
		errors.Is(err, domain.ErrInvalidMoney),
		errors.Is(err, domain.ErrInvalidWallet),
		errors.Is(err, domain.ErrCurrencyMismatch):
		status = http.StatusBadRequest
		message = err.Error()

	case errors.Is(err, domain.ErrInsufficientBalance):
		status = http.StatusConflict
		message = err.Error()

	case errors.Is(err, pgx.ErrNoRows):
		status = http.StatusNotFound
		message = "resource not found"
	}

	writeJSON(
		w,
		status,
		errorResponse{
			Error: message,
		},
	)
}
