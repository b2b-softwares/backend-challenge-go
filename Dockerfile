# ============================================================
# Build stage
# ============================================================
FROM golang:1.24-bookworm AS builder

WORKDIR /app

# Dependências primeiro para aproveitar cache do Docker
COPY go.mod go.sum ./

RUN go mod download

# Código da aplicação
COPY . .

# Compilação estática
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /app/bin/backend \
    ./cmd/app


# ============================================================
# Runtime stage
# ============================================================
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /app/bin/backend /app/backend

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/backend"]