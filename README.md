# Backend Challenge — Processamento Distribuído de Apostas em Go

Backend para processamento distribuído de transações de apostas, desenvolvido em Go, com foco em consistência financeira, idempotência, processamento assíncrono, concorrência segura, observabilidade e desacoplamento arquitetural.

---

# Documentação

A documentação do projeto foi organizada por responsabilidade, mantendo este README como ponto de entrada operacional.

| Documento | Descrição |
|---|---|
| [REQUIREMENTS.md](REQUIREMENTS.md) | Requisitos funcionais e não funcionais do desafio |
| [BUSINESS_RULES.md](BUSINESS_RULES.md) | Regras de negócio, invariantes e restrições financeiras |
| [TRACEABILITY.md](TRACEABILITY.md) | Rastreabilidade entre requisitos, regras, implementação e testes |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Arquitetura, componentes, padrões, decisões e infraestrutura |
| [docs/use-cases.md](docs/use-cases.md) | Casos de uso e fluxos funcionais |
| [docs/api-contract.md](docs/api-contract.md) | Contrato da API HTTP |
| [docs/events.md](docs/events.md) | Eventos, Outbox, Inbox, SQS e processamento assíncrono |

## Mapa da documentação

```text
README.md
   │
   ├── REQUIREMENTS.md
   │      └── O que o sistema precisa fazer
   │
   ├── BUSINESS_RULES.md
   │      └── Quais regras o negócio deve respeitar
   │
   ├── TRACEABILITY.md
   │      └── Como requisitos e regras são implementados e testados
   │
   ├── ARCHITECTURE.md
   │      └── Como o sistema foi arquitetado
   │
   └── docs/
          ├── use-cases.md
          │      └── Fluxos funcionais
          │
          ├── api-contract.md
          │      └── Contrato HTTP
          │
          └── events.md
                 └── Mensageria e eventos
```

Para uma leitura rápida:

1. Comece por este `README.md` para executar e validar o projeto.
2. Consulte `REQUIREMENTS.md` para entender os requisitos.
3. Consulte `BUSINESS_RULES.md` para entender as regras de negócio.
4. Consulte `ARCHITECTURE.md` para entender a arquitetura.
5. Consulte `TRACEABILITY.md` para relacionar requisitos, regras, código e testes.
6. Consulte `docs/` para os contratos e fluxos detalhados.

---

# 1. Status do Projeto

Projeto funcionalmente implementado e validado.

Principais componentes implementados:

- Go 1.25
- Uber Fx
- HTTP REST API
- Keycloak / OIDC
- PostgreSQL
- Money decimal exato
- Wallet
- Wager Transactions
- Append-only Ledger
- Idempotency
- Inbox Pattern
- Outbox Pattern
- AWS SQS
- MiniStack para ambiente local
- Consumer assíncrono
- DLQ
- Redelivery
- Pending Reference Worker
- Docker Compose
- OpenTelemetry
- Prometheus
- Grafana
- Jaeger
- Unit Tests
- Integration Tests
- E2E
- Race Detector

Validações realizadas:

```bash
go test ./...
```

e:

```bash
go test -race ./...
```

---

# 2. Objetivo

O projeto implementa uma API para processamento de apostas com requisitos típicos de sistemas financeiros e distribuídos.

O sistema precisa garantir que uma mesma operação não produza efeitos financeiros duplicados mesmo quando ocorrerem:

- retries HTTP;
- mensagens duplicadas;
- redelivery do SQS;
- concorrência;
- reinicialização da aplicação;
- falhas de infraestrutura;
- falha durante o processamento;
- falha após commit;
- indisponibilidade temporária de serviços.

A arquitetura utiliza persistência transacional, idempotência, Inbox, Outbox e processamento assíncrono para atingir esses objetivos.

---

# 3. Arquitetura

Visão simplificada:

```text
                         CLIENT
                           │
                           ▼
                    ┌─────────────┐
                    │  Keycloak   │
                    │    OIDC     │
                    └──────┬──────┘
                           │
                           │ Access Token
                           ▼
                    ┌─────────────┐
                    │  HTTP API   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ Idempotency │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ Application │
                    │   Service   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ PostgreSQL  │
                    │             │
                    │ Wallet      │
                    │ Transaction │
                    │ Ledger      │
                    │ Inbox       │
                    │ Outbox      │
                    └──────┬──────┘
                           │
                           │ Outbox
                           ▼
                    ┌─────────────┐
                    │    SQS      │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │  Consumer   │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    Inbox    │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
                    │ Application │
                    └─────────────┘
```

Observabilidade:

```text
Application
     │
     ▼
OpenTelemetry
     │
     ▼
OTel Collector
     │
     ├──────────────► Jaeger
     │
     └──────────────► Prometheus
                           │
                           ▼
                        Grafana
```

---

# 4. Princípios Arquiteturais

O projeto segue princípios de:

- Clean Architecture;
- Hexagonal Architecture;
- Ports and Adapters;
- SOLID;
- Dependency Inversion;
- DDD Lite;
- Repository Pattern;
- Transactional Integrity;
- Inbox Pattern;
- Outbox Pattern;
- Idempotent Consumer;
- Event-Driven Architecture.

O domínio não depende diretamente de:

- HTTP;
- PostgreSQL;
- SQS;
- AWS;
- Docker;
- Uber Fx;
- OpenTelemetry;
- Keycloak.

Detalhes completos estão em [ARCHITECTURE.md](ARCHITECTURE.md).

---

# 5. Estrutura do Projeto

```text
.
├── cmd/
│   └── app/
│       └── main.go
│
├── internal/
│   ├── application/
│   │   ├── wager_service.go
│   │   ├── wager_consumer.go
│   │   ├── reversal_service.go
│   │   └── pending_reference_worker.go
│   │
│   ├── domain/
│   │   ├── money.go
│   │   ├── wallet.go
│   │   ├── transaction.go
│   │   └── errors.go
│   │
│   ├── ports/
│   │   ├── repositories.go
│   │   ├── queue.go
│   │   └── transaction.go
│   │
│   ├── adapters/
│   │   ├── postgres/
│   │   └── sqs/
│   │
│   ├── http/
│   │   └── handler.go
│   │
│   ├── config/
│   │
│   └── observability/
│       └── observability.go
│
├── migrations/
│
├── infra/
│   ├── grafana/
│   ├── prometheus/
│   ├── otel-collector/
│   ├── keycloak/
│   └── ministack/
│
├── tests/
│   ├── unit/
│   ├── integration/
│   └── e2e/
│
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── README.md
├── REQUIREMENTS.md
├── BUSINESS_RULES.md
├── TRACEABILITY.md
├── ARCHITECTURE.md
└── docs/
    ├── use-cases.md
    ├── api-contract.md
    └── events.md
```

---

# 6. Tecnologias

| Tecnologia | Utilização |
|---|---|
| Go 1.25 | Linguagem |
| Uber Fx | Dependency Injection / Lifecycle |
| PostgreSQL | Persistência transacional |
| MiniStack | Simulação AWS local |
| SQS | Mensageria |
| Keycloak | OIDC / OAuth2 |
| OpenTelemetry | Observabilidade |
| Prometheus | Métricas |
| Grafana | Dashboards |
| Jaeger | Distributed Tracing |
| Docker | Containerização |
| Docker Compose | Ambiente local |

---

# 7. Pré-requisitos

Para executar o projeto localmente:

- Go 1.25+
- Docker
- Docker Compose
- Git
- curl
- PostgreSQL client opcional
- AWS CLI opcional para inspeção do SQS

Verificar Go:

```bash
go version
```

Verificar Docker:

```bash
docker --version
```

Verificar Compose:

```bash
docker compose version
```

---

# 8. Clonar o Projeto

```bash
git clone git@github.com:b2b-softwares/backend-challenge-go.git

cd backend-challenge-go
```

---

# 9. Subir o Ambiente

Inicializar toda a infraestrutura:

```bash
docker compose up -d
```

Verificar containers:

```bash
docker compose ps
```

Ver logs:

```bash
docker compose logs -f
```

Ver logs somente da API:

```bash
docker compose logs -f api
```

---

# 10. Serviços Locais

Após iniciar o ambiente:

| Serviço | URL |
|---|---|
| API | http://localhost:8080 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Jaeger | http://localhost:16686 |
| Keycloak | http://localhost:8081 |
| MiniStack | http://localhost:4566 |
| OTel Collector Metrics | http://localhost:8889/metrics |

---

# 11. Health Checks

Os endpoints de health e readiness são protegidos pelo middleware de autenticação da API.

Sem token:

```bash
curl -i http://localhost:8080/health
```

e:

```bash
curl -i http://localhost:8080/health/ready
```

O resultado esperado sem autenticação é:

```text
401 Unauthorized
```

Para consultar os endpoints autenticados, primeiro obtenha um access token conforme a seção 16.

Depois:

```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/health
```

Readiness:

```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/health/ready
```

Resposta esperada:

```json
{
  "status": "UP"
}
```

---

# 12. Fase 0 — Validar Infraestrutura

Primeiro confirme se todos os containers estão funcionando:

```bash
docker compose ps
```

Verificar PostgreSQL:

```bash
docker compose logs postgres
```

Verificar MiniStack:

```bash
docker compose logs ministack
```

Verificar Keycloak:

```bash
docker compose logs keycloak
```

Verificar OTel:

```bash
docker compose logs otel-collector
```

Verificar Prometheus:

```bash
docker compose logs prometheus
```

Verificar Grafana:

```bash
docker compose logs grafana
```

---

# 13. Fase 1 — Banco de Dados e Migrations

As migrations são executadas durante a inicialização da aplicação.

Verificar logs:

```bash
docker compose logs api
```

Procurar mensagens relacionadas às migrations:

```bash
docker compose logs api | grep -i migration
```

---

# 14. Validar PostgreSQL

Entrar no PostgreSQL:

```bash
docker compose exec postgres psql \
  -U postgres \
  -d backend_challenge
```

Listar tabelas:

```sql
\dt
```

Sair:

```sql
\q
```

---

# 15. Fase 2 — Testar OIDC / Keycloak

A API exige autenticação através de OAuth2/OIDC.

O fluxo local utiliza `client_credentials`.

Sem token:

```bash
curl -i http://localhost:8080/health/ready
```

Resultado esperado:

```text
401 Unauthorized
```

A chamada autenticada deve utilizar:

```text
Authorization: Bearer <access-token>
```

Importante: a API valida o **OAuth2 access token JWT**, verificando issuer, assinatura/JWKS, audience e demais claims de validade aplicáveis.

---

# 16. Obter Token do Keycloak

O ambiente local disponibiliza um client configurado para `client_credentials`.

Client utilizado:

```text
backend-api
```

Exemplo:

```bash
curl -s \
  -X POST \
  "http://localhost:8081/realms/backend-challenge/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=backend-api" \
  -d "client_secret=backend-api-secret"
```

Extrair somente o access token, caso `jq` esteja instalado:

```bash
TOKEN=$(curl -s \
  -X POST \
  "http://localhost:8081/realms/backend-challenge/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=backend-api" \
  -d "client_secret=backend-api-secret" \
  | jq -r '.access_token')
```

Validar:

```bash
echo "$TOKEN"
```

---

# 17. Fase 3 — Testar API Autenticada

Exemplo:

```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/health/ready
```

Resultado esperado:

```json
{
  "status": "UP"
}
```

---

# 18. Endpoint de Wager

Endpoint:

```text
POST /transactions/wager
```

Headers obrigatórios:

```text
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
Content-Type: application/json
```

Payload:

```json
{
  "externalTransactionId": "external-001",
  "providerId": "provider-001",
  "walletId": "WALLET_UUID",
  "playerId": "PLAYER_UUID",
  "roundId": "round-001",
  "gameId": "game-001",
  "amount": "10.00",
  "currency": "BRL"
}
```

O contrato completo está em [docs/api-contract.md](docs/api-contract.md).

---

# 19. Fase 4 — Primeira Aposta

Defina as variáveis:

```bash
WALLET_ID="<wallet-uuid>"
PLAYER_ID="<player-uuid>"
```

Execute:

```bash
curl -i \
  -X POST \
  http://localhost:8080/transactions/wager \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: wager-001" \
  -d "{
    \"externalTransactionId\": \"external-001\",
    \"providerId\": \"provider-001\",
    \"walletId\": \"$WALLET_ID\",
    \"playerId\": \"$PLAYER_ID\",
    \"roundId\": \"round-001\",
    \"gameId\": \"game-001\",
    \"amount\": \"10.00\",
    \"currency\": \"BRL\"
  }"
```

Resposta esperada:

```json
{
  "transactionId": "...",
  "status": "PROCESSED",
  "balance": "90.00",
  "currency": "BRL",
  "idempotentReplay": false
}
```

O saldo depende do saldo inicial da wallet utilizada no ambiente.

---

# 20. Fase 5 — Testar Idempotência

Reenvie exatamente a mesma requisição com a mesma chave:

```bash
curl -i \
  -X POST \
  http://localhost:8080/transactions/wager \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: wager-001" \
  -d "{
    \"externalTransactionId\": \"external-001\",
    \"providerId\": \"provider-001\",
    \"walletId\": \"$WALLET_ID\",
    \"playerId\": \"$PLAYER_ID\",
    \"roundId\": \"round-001\",
    \"gameId\": \"game-001\",
    \"amount\": \"10.00\",
    \"currency\": \"BRL\"
  }"
```

A resposta deve indicar replay:

```json
{
  "idempotentReplay": true
}
```

A wallet não deve ser debitada novamente.

---

# 21. Fase 6 — Testar Idempotency Conflict

Use a mesma chave:

```text
wager-001
```

mas altere o valor:

```json
"amount": "20.00"
```

Resultado esperado:

```http
409 Conflict
```

O saldo não deve sofrer novo débito.

Isso valida a associação entre:

```text
Idempotency-Key
        +
Payload Hash
```

---

# 22. Fase 7 — Testar Saldo Insuficiente

Execute uma aposta com valor superior ao saldo disponível:

```bash
curl -i \
  -X POST \
  http://localhost:8080/transactions/wager \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: insufficient-001" \
  -d "{
    \"externalTransactionId\": \"external-insufficient-001\",
    \"providerId\": \"provider-001\",
    \"walletId\": \"$WALLET_ID\",
    \"playerId\": \"$PLAYER_ID\",
    \"roundId\": \"round-insufficient\",
    \"gameId\": \"game-001\",
    \"amount\": \"999999.99\",
    \"currency\": \"BRL\"
  }"
```

Resultado esperado:

```http
409 Conflict
```

Nenhum débito financeiro deve ser realizado.

---

# 23. Fase 8 — Testar Concorrência

Um dos testes importantes é executar duas apostas simultâneas.

Exemplo:

```text
Saldo = 90
Request A = 50
Request B = 50
```

Executar duas chamadas em paralelo:

```bash
curl -s \
  -X POST \
  http://localhost:8080/transactions/wager \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: concurrent-a" \
  -d "{
    \"externalTransactionId\": \"concurrent-a\",
    \"providerId\": \"provider-001\",
    \"walletId\": \"$WALLET_ID\",
    \"playerId\": \"$PLAYER_ID\",
    \"roundId\": \"round-concurrent-a\",
    \"gameId\": \"game-001\",
    \"amount\": \"50.00\",
    \"currency\": \"BRL\"
  }" &

curl -s \
  -X POST \
  http://localhost:8080/transactions/wager \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: concurrent-b" \
  -d "{
    \"externalTransactionId\": \"concurrent-b\",
    \"providerId\": \"provider-001\",
    \"walletId\": \"$WALLET_ID\",
    \"playerId\": \"$PLAYER_ID\",
    \"roundId\": \"round-concurrent-b\",
    \"gameId\": \"game-001\",
    \"amount\": \"50.00\",
    \"currency\": \"BRL\"
  }" &

wait
```

Com saldo inicial de R$90, o resultado esperado é:

```text
Uma operação  → PROCESSED
Uma operação  → REJECTED

Saldo final → R$40
```

---

# 24. Fase 9 — Validar Ledger

Consultar as entradas:

```sql
SELECT
    *
FROM ledger
ORDER BY created_at;
```

O ledger mantém o histórico das movimentações financeiras.

O ledger é append-only e suas entradas não devem ser alteradas após criação.

---

# 25. Fase 10 — Validar Transactions

Consultar:

```sql
SELECT
    *
FROM wager_transactions
ORDER BY created_at DESC;
```

Verificar os estados:

```text
PROCESSED
REJECTED
```

---

# 26. Fase 11 — Validar Wallet

Consultar:

```sql
SELECT
    *
FROM wallets;
```

Confirmar que:

- saldo não ficou negativo;
- concorrência foi corretamente serializada;
- replay não produziu novo débito.

---

# 27. Fase 12 — Validar Idempotency

Consultar:

```sql
SELECT
    *
FROM idempotency_keys
ORDER BY created_at DESC;
```

Verificar:

- chave;
- payload hash;
- transaction;
- resultado;
- replay.

---

# 28. Fase 13 — Validar Inbox

Consultar:

```sql
SELECT
    *
FROM inbox
ORDER BY created_at DESC;
```

Verificar os estados registrados pelo consumidor:

```text
RECEIVED
PROCESSING
PROCESSED
```

A Inbox possui identidade por consumidor e mensagem, evitando que uma mesma mensagem produza novamente o efeito financeiro.

---

# 29. Fase 14 — Validar Outbox

Consultar:

```sql
SELECT
    *
FROM outbox
ORDER BY created_at DESC;
```

O fluxo esperado é:

```text
PENDING
   │
   ▼
Publicação
   │
   ▼
Published
```

O evento é persistido junto da operação transacional antes de ser publicado no broker.

---

# 30. Fase 15 — Validar SQS

Listar filas através do MiniStack/AWS CLI:

```bash
aws --endpoint-url=http://localhost:4566 sqs list-queues
```

Se o ambiente exigir região:

```bash
aws \
  --endpoint-url=http://localhost:4566 \
  --region us-east-1 \
  sqs list-queues
```

---

# 31. Inspecionar Mensagens SQS

Exemplo:

```bash
aws \
  --endpoint-url=http://localhost:4566 \
  --region us-east-1 \
  sqs receive-message \
  --queue-url "<QUEUE_URL>"
```

O nome exato da fila deve ser obtido através do ambiente configurado.

---

# 32. Fase 16 — Validar Consumer

Ver logs:

```bash
docker compose logs -f api
```

O consumer deve seguir aproximadamente o fluxo:

```text
Receive message
     │
     ▼
Validate
     │
     ▼
Inbox
     │
     ▼
Claim
     │
     ▼
Process
     │
     ▼
Mark Processed
     │
     ▼
Delete SQS
```

---

# 33. Fase 17 — Redelivery

O consumidor não deve depender de entrega exactly-once.

O comportamento esperado é:

```text
Message
   │
   ▼
Processing
   │
   X
Failure
   │
   ▼
Redelivery
```

Quando a mesma mensagem chegar novamente, a Inbox deve impedir a duplicação do efeito financeiro.

Detalhes do mecanismo estão em [docs/events.md](docs/events.md).

---

# 34. Fase 18 — DLQ

Mensagens que falharem repetidamente devem ser direcionadas à DLQ conforme a política configurada no ambiente.

Verificar filas:

```bash
aws \
  --endpoint-url=http://localhost:4566 \
  --region us-east-1 \
  sqs list-queues
```

A DLQ deve ser monitorada para identificar mensagens que precisam de análise ou recuperação.

---

# 35. Fase 19 — Pending Reference Worker

O worker pode ser observado através dos logs:

```bash
docker compose logs -f api
```

O fluxo esperado é:

```text
Pending Reference
       │
       ▼
Worker
       │
       ▼
Reference Available?
       │
       ├── Yes → Process
       │
       └── No  → Retry
```

---

# 36. Fase 20 — Unit Tests

Executar todos os testes unitários:

```bash
go test ./tests/unit/...
```

Executar com verbose:

```bash
go test -v ./tests/unit/...
```

---

# 37. Fase 21 — Integration Tests

Executar:

```bash
go test ./tests/integration/...
```

Verbose:

```bash
go test -v ./tests/integration/...
```

---

# 38. Fase 22 — Todos os Testes

Executar:

```bash
go test ./...
```

Esse é o primeiro regression gate.

---

# 39. Fase 23 — Race Detector

Executar:

```bash
go test -race ./...
```

Esse teste é obrigatório para validar o comportamento concorrente.

Especialmente importante para:

- wallet;
- consumer;
- workers;
- repositories;
- goroutines;
- processamento paralelo.

---

# 40. Fase 24 — Build

Executar:

```bash
go build ./...
```

Ou:

```bash
go build -o backend-challenge ./cmd/app
```

---

# 41. Fase 25 — Formatação

Formatar os arquivos Go do projeto:

```bash
find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w
```

Verificar arquivos que ainda precisariam ser formatados:

```bash
gofmt -l $(find . -name '*.go' -not -path './vendor/*')
```

O comando acima não deve retornar arquivos pendentes.

---

# 42. Fase 26 — Docker Build

Build da aplicação:

```bash
docker compose build
```

Subir:

```bash
docker compose up -d
```

---

# 43. Fase 27 — Logs

Todos os serviços:

```bash
docker compose logs -f
```

API:

```bash
docker compose logs -f api
```

PostgreSQL:

```bash
docker compose logs -f postgres
```

MiniStack:

```bash
docker compose logs -f ministack
```

Keycloak:

```bash
docker compose logs -f keycloak
```

Grafana:

```bash
docker compose logs -f grafana
```

Prometheus:

```bash
docker compose logs -f prometheus
```

OTel Collector:

```bash
docker compose logs -f otel-collector
```

---

# 44. Observabilidade

A observabilidade utiliza:

```text
OpenTelemetry
      │
      ▼
OTel Collector
      │
      ├────────────► Prometheus
      │                  │
      │                  ▼
      │               Grafana
      │
      └────────────► Jaeger
```

A instrumentação permite acompanhar tanto aspectos técnicos quanto indicadores relacionados ao processamento das apostas e mensageria.

---

# 45. Grafana

Abrir:

```text
http://localhost:3000
```

Dashboard:

```text
Backend Challenge - Distributed Wager Processing
```

O dashboard contém 10 painéis principais:

1. HTTP Request Rate
2. HTTP Error Rate
3. HTTP Latency P95
4. HTTP Latency P50
5. Wager Transactions
6. Wager Processing Latency
7. Wager Business Outcomes
8. SQS Consumer
9. SQS Processing Latency
10. Inbox / Idempotent Consumer

Os painéis permitem acompanhar disponibilidade, latência, volume de transações, resultados de negócio, processamento assíncrono e comportamento da Inbox.

---

# 46. Painéis do Grafana

## HTTP Request Rate

Mostra a taxa de requisições HTTP.

PromQL:

```promql
sum(rate(http_server_request_duration_seconds_count[5m]))
```

---

## HTTP Error Rate

PromQL:

```promql
sum(
  rate(
    http_server_request_duration_seconds_count{
      http_response_status_code=~"4..|5.."
    }[5m]
  )
)
/
clamp_min(
  sum(rate(http_server_request_duration_seconds_count[5m])),
  1
)
```

---

## HTTP Latency P95

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(http_server_request_duration_seconds_bucket[5m])
  )
)
```

---

## HTTP Latency P50

```promql
histogram_quantile(
  0.50,
  sum by (le) (
    rate(http_server_request_duration_seconds_bucket[5m])
  )
)
```

---

# 47. Wager Metrics

Total:

```promql
sum(rate(wager_transactions_total[5m]))
```

Processadas:

```promql
sum(
  rate(
    wager_transactions_total{
      status="processed"
    }[5m]
  )
)
```

Rejeitadas:

```promql
sum(
  rate(
    wager_transactions_total{
      status="rejected"
    }[5m]
  )
)
```

---

# 48. Wager Processing Latency

P95:

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(wager_processing_duration_seconds_bucket[5m])
  )
)
```

P50:

```promql
histogram_quantile(
  0.50,
  sum by (le) (
    rate(wager_processing_duration_seconds_bucket[5m])
  )
)
```

---

# 49. SQS Metrics

Mensagens recebidas:

```promql
sum(rate(sqs_messages_received_total[5m]))
```

Processadas:

```promql
sum(rate(sqs_messages_processed_total[5m]))
```

Deletadas:

```promql
sum(rate(sqs_messages_deleted_total[5m]))
```

---

# 50. SQS Processing Latency

P95:

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(sqs_message_processing_duration_seconds_bucket[5m])
  )
)
```

---

# 51. Inbox Metrics

Recebidas:

```promql
sum(rate(inbox_messages_received_total[5m]))
```

Processadas:

```promql
sum(rate(inbox_messages_processed_total[5m]))
```

---

# 52. Prometheus

Abrir:

```text
http://localhost:9090
```

Consultar:

```promql
up
```

Principais métricas atualmente expostas:

```text
http_server_request_body_size_bytes
http_server_request_duration_seconds
http_server_response_body_size_bytes

inbox_messages_processed_total
inbox_messages_received_total

sqs_message_processing_duration_seconds
sqs_messages_deleted_total
sqs_messages_processed_total
sqs_messages_received_total

wager_processing_duration_seconds
wager_transactions_processed_total
wager_transactions_total
```

As métricas podem variar conforme a instrumentação automática e os componentes ativos.

---

# 53. API Metrics

Endpoint:

```text
http://localhost:8080/metrics
```

Ou:

```bash
curl -s http://localhost:8080/metrics
```

Listar somente nomes de métricas:

```bash
curl -s http://localhost:8080/metrics \
  | grep -v "^#" \
  | cut -d'{' -f1 \
  | cut -d' ' -f1 \
  | sort -u
```

---

# 54. OTel Collector

Métricas do Collector:

```bash
curl -s http://localhost:8889/metrics
```

Ver logs:

```bash
docker compose logs -f otel-collector
```

---

# 55. Jaeger

Abrir:

```text
http://localhost:16686
```

Selecionar o serviço da aplicação.

O tracing permite investigar:

- requests;
- latência;
- chamadas internas;
- dependências;
- falhas.

---

# 56. Diagnóstico de uma Requisição

Fluxo recomendado para investigação:

```text
Grafana
   │
   ▼
Detectar aumento de latency/error
   │
   ▼
Jaeger
   │
   ▼
Encontrar trace
   │
   ▼
Identificar span lento
   │
   ▼
Investigar PostgreSQL / SQS
```

---

# 57. Banco — Consultas Úteis

Entrar:

```bash
docker compose exec postgres psql \
  -U postgres \
  -d backend_challenge
```

Wallets:

```sql
SELECT * FROM wallets;
```

Transactions:

```sql
SELECT *
FROM wager_transactions
ORDER BY created_at DESC;
```

Ledger:

```sql
SELECT *
FROM ledger
ORDER BY created_at DESC;
```

Idempotency:

```sql
SELECT *
FROM idempotency_keys
ORDER BY created_at DESC;
```

Inbox:

```sql
SELECT *
FROM inbox
ORDER BY created_at DESC;
```

Outbox:

```sql
SELECT *
FROM outbox
ORDER BY created_at DESC;
```

---

# 58. Verificar Integridade Financeira

Alguns pontos importantes:

```text
Wallet balance >= 0
```

Uma requisição idempotente não deve gerar:

```text
novo debit
```

Uma operação rejeitada por saldo insuficiente não deve gerar:

```text
ledger debit
```

Uma mensagem redeliverada não deve gerar:

```text
efeito financeiro duplicado
```

As regras formais estão documentadas em [BUSINESS_RULES.md](BUSINESS_RULES.md).

---

# 59. Verificar Idempotência Manualmente

Executar uma requisição:

```text
Idempotency-Key = test-123
Amount = 10.00
```

Repetir:

```text
Idempotency-Key = test-123
Amount = 10.00
```

Esperado:

```text
Primeira → Processed
Segunda  → Replay
```

Depois:

```text
Idempotency-Key = test-123
Amount = 20.00
```

Esperado:

```text
409 Conflict
```

---

# 60. Verificar Concorrência Manualmente

Para uma wallet com:

```text
Balance = 90
```

disparar:

```text
50
50
```

simultaneamente.

Esperado:

```text
Processed = 1
Rejected  = 1
Balance   = 40
```

---

# 61. Regression Checklist

Antes de considerar a versão pronta:

```text
[ ] gofmt -l $(find . -name '*.go' -not -path './vendor/*')

[ ] go test ./...

[ ] go test -race ./...

[ ] go build ./...

[ ] docker compose build

[ ] docker compose up -d

[ ] docker compose ps

[ ] /health sem token → 401

[ ] /health/ready sem token → 401

[ ] Keycloak token

[ ] API authenticated

[ ] Wager processed

[ ] Idempotency replay

[ ] Idempotency conflict

[ ] Insufficient balance

[ ] Concurrent wagers

[ ] Ledger

[ ] Inbox

[ ] Outbox

[ ] SQS consumer

[ ] Redelivery

[ ] DLQ

[ ] Pending Reference Worker

[ ] Prometheus

[ ] Grafana

[ ] Jaeger

[ ] OpenTelemetry
```

---

# 62. Troubleshooting

## API não inicia

Ver:

```bash
docker compose logs api
```

Verificar:

- PostgreSQL;
- migrations;
- variáveis de ambiente;
- conexão;
- porta 8080.

---

## PostgreSQL não conecta

Ver:

```bash
docker compose ps postgres
```

Logs:

```bash
docker compose logs postgres
```

Teste:

```bash
docker compose exec postgres pg_isready \
  -U postgres
```

---

## Keycloak não responde

Ver:

```bash
docker compose ps keycloak
```

Logs:

```bash
docker compose logs keycloak
```

Testar:

```bash
curl -I http://localhost:8081
```

---

## Token não é gerado

Verifique:

- realm;
- `client_id`;
- `client_secret`;
- URL;
- client configurado para `client_credentials`.

Client local:

```text
backend-api
```

Logs:

```bash
docker compose logs keycloak
```

---

## API retorna 401

Verifique:

```text
Authorization: Bearer <TOKEN>
```

e confirme se o access token:

- foi emitido pelo realm correto;
- está válido;
- não está expirado;
- possui a audiência esperada;
- possui assinatura válida.

---

## API retorna 409

Pode representar uma rejeição de negócio, como:

```text
insufficient balance
```

ou:

```text
idempotency conflict
```

Verifique o corpo da resposta.

---

## SQS não processa

Verifique:

```bash
docker compose logs -f api
```

e:

```bash
docker compose logs ministack
```

Liste filas:

```bash
aws \
  --endpoint-url=http://localhost:4566 \
  --region us-east-1 \
  sqs list-queues
```

---

## Outbox permanece PENDING

Verifique:

```bash
docker compose logs -f api
```

e a disponibilidade do MiniStack/SQS.

Consultar:

```sql
SELECT *
FROM outbox
ORDER BY created_at DESC;
```

---

## Inbox permanece PROCESSING

Verifique:

```bash
docker compose logs -f api
```

e consulte:

```sql
SELECT *
FROM inbox
ORDER BY created_at DESC;
```

---

## Grafana não mostra dashboard

Verifique:

```bash
docker compose ps grafana
```

Logs:

```bash
docker compose logs grafana
```

O diretório de dashboards deve estar disponível para o container.

---

## Grafana sem métricas

Verifique:

```bash
curl -s http://localhost:9090/api/v1/targets
```

Verifique:

```bash
docker compose logs prometheus
```

E:

```bash
docker compose logs otel-collector
```

---

## Jaeger sem traces

Verifique:

```bash
docker compose logs otel-collector
```

e:

```bash
docker compose logs jaeger
```

---

# 63. Desenvolvimento Local sem Docker

Para executar somente os testes Go:

```bash
go test ./...
```

Race detector:

```bash
go test -race ./...
```

Build:

```bash
go build ./...
```

Para executar a aplicação diretamente:

```bash
go run ./cmd/app
```

Nesse caso, as dependências externas precisam estar disponíveis e as variáveis de ambiente devem estar corretamente configuradas.

---

# 64. Variáveis de Ambiente

A configuração é externalizada.

As variáveis relacionadas à infraestrutura e execução devem seguir a implementação existente em `internal/config` e o arquivo `.env.example`, quando disponível.

Principais categorias:

```text
DATABASE_URL

DATABASE_HOST
DATABASE_PORT
DATABASE_USER
DATABASE_PASSWORD
DATABASE_NAME

SQS_ENDPOINT
SQS_QUEUE_NAME
SQS_DLQ_NAME
AWS_REGION

OIDC_ISSUER_URL
OIDC_CLIENT_ID
OIDC_CLIENT_SECRET
OIDC_AUDIENCE

OTEL_EXPORTER_OTLP_ENDPOINT
OTEL_SERVICE_NAME
OTEL_ENVIRONMENT
```

Configuração OIDC local utilizada pelo ambiente:

```text
OIDC_ISSUER_URL=http://keycloak:8080/realms/backend-challenge
OIDC_CLIENT_ID=backend-api
OIDC_AUDIENCE=backend-api
```

Os nomes exatos devem seguir a configuração existente em `internal/config`.

---

# 65. Segurança

Não utilizar secrets reais no código-fonte.

Para ambientes reais:

- utilizar Secret Manager;
- utilizar IAM;
- utilizar TLS;
- restringir permissões;
- não expor PostgreSQL publicamente;
- não expor SQS diretamente;
- utilizar credenciais específicas por ambiente;
- utilizar menor privilégio;
- realizar rotação de secrets.

O `client_secret` utilizado no ambiente local é exclusivamente para desenvolvimento.

---

# 66. Local vs AWS

O projeto foi estruturado para que o ambiente local possa simular a arquitetura de produção.

Local:

```text
PostgreSQL
MiniStack
Keycloak
Docker Compose
OpenTelemetry
Prometheus
Grafana
Jaeger
```

Possível produção:

```text
RDS PostgreSQL
Amazon SQS
OIDC Provider
ECS / EKS
OpenTelemetry
Prometheus
Grafana
```

O domínio e os casos de uso não precisam conhecer essas diferenças.

As implementações de infraestrutura são isoladas através das portas e adapters definidos pela arquitetura.

---

# 67. Escalabilidade

A API é stateless.

É possível executar múltiplas instâncias:

```text
API 1
API 2
API 3
```

Consumers também podem ser escalados:

```text
Consumer 1
Consumer 2
Consumer 3
```

A consistência é garantida por:

- PostgreSQL;
- transações;
- locks apropriados;
- constraints;
- Inbox;
- idempotência.

Não existe um mutex global para controlar o saldo de todas as wallets.

---

# 68. Consistência Financeira

A operação financeira principal deve ser transacional:

```text
BEGIN
    │
    ├── lock wallet
    ├── validate balance
    ├── update wallet
    ├── create transaction
    ├── append ledger
    └── create outbox
    │
COMMIT
```

Em caso de erro:

```text
ROLLBACK
```

A wallet, transaction, ledger e eventos correspondentes devem respeitar a atomicidade definida pelo caso de uso.

---

# 69. Eventual Consistency

A parte assíncrona é eventualmente consistente:

```text
Transaction
    │
    ▼
Outbox
    │
    ▼
SQS
    │
    ▼
Consumer
    │
    ▼
Inbox
```

Isso é intencional.

A operação financeira principal é consistente dentro da transação do banco.

O processamento assíncrono ocorre posteriormente.

---

# 70. Exactly Once Effect

O sistema não depende de entrega exactly-once do broker.

O modelo é:

```text
At-Least-Once Delivery

          +

Idempotent Processing

          =

Effectively Once Business Effect
```

Os mecanismos envolvidos são:

- Idempotency;
- Inbox;
- database constraints;
- transactions;
- Outbox.

---

# 71. Por que usar Outbox?

Sem Outbox:

```text
DB COMMIT
   │
   ▼
Publish SQS
   │
   X
```

Existe risco de o banco confirmar a operação e o evento não ser publicado.

Com Outbox:

```text
BEGIN

Wallet
Transaction
Ledger
Outbox

COMMIT
```

Depois:

```text
Outbox
   │
   ▼
SQS
```

A operação e o registro do evento são persistidos atomicamente no banco antes da publicação assíncrona.

---

# 72. Por que usar Inbox?

O SQS pode entregar uma mensagem novamente.

Sem Inbox:

```text
Message
   │
   ▼
Debit
   │
   ▼
Redelivery
   │
   ▼
Debit AGAIN
```

Com Inbox:

```text
Message
   │
   ▼
Inbox
   │
   ▼
Debit
   │
   ▼
PROCESSED
```

Na redelivery:

```text
Message
   │
   ▼
Inbox
   │
   ▼
Already Processed
```

O efeito financeiro não é repetido.

---

# 73. Por que usar Ledger?

A wallet representa o estado atual.

O ledger representa o histórico.

```text
Wallet
------
Balance = 90.00

Ledger
------
+100.00
-10.00
```

O ledger é imutável e append-only, melhorando auditoria e rastreabilidade das movimentações financeiras.

---

# 74. Observabilidade de Negócio

Além de métricas técnicas, a aplicação mede comportamento de negócio:

```text
wager_transactions_total
```

Isso permite observar:

- volume;
- processadas;
- rejeitadas.

E também:

```text
wager_processing_duration_seconds
```

para medir a duração do processamento.

---

# 75. Observabilidade de Mensageria

O consumer expõe:

```text
sqs_messages_received_total

sqs_messages_processed_total

sqs_messages_deleted_total

sqs_message_processing_duration_seconds
```

Isso permite identificar diferenças entre mensagens recebidas, processadas e removidas da fila, auxiliando na investigação de falhas e redelivery.

---

# 76. Observabilidade da Inbox

As métricas:

```text
inbox_messages_received_total

inbox_messages_processed_total
```

ajudam a identificar o comportamento do consumidor idempotente.

---

# 77. CI / Quality Gate

A validação mínima recomendada é:

```bash
go test ./...
go test -race ./...
go build ./...
```

Se houver Docker disponível:

```bash
docker compose build
docker compose up -d
```

E executar os testes E2E.

---

# 78. Comandos Úteis

Status:

```bash
git status
```

Histórico:

```bash
git log --oneline --decorate -10
```

Testes:

```bash
go test ./...
```

Race:

```bash
go test -race ./...
```

Build:

```bash
go build ./...
```

Format:

```bash
find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w
```

Verificar formatação:

```bash
gofmt -l $(find . -name '*.go' -not -path './vendor/*')
```

Docker:

```bash
docker compose up -d

docker compose down

docker compose ps

docker compose logs -f
```

---

# 79. Parar o Ambiente

Parar containers:

```bash
docker compose down
```

Parar e remover volumes:

```bash
docker compose down -v
```

Atenção: `-v` remove os volumes persistentes do ambiente local, incluindo dados do PostgreSQL e outros serviços configurados com volumes.

---

# 80. Rebuild Completo

Quando houver alteração no código ou Dockerfile:

```bash
docker compose down

docker compose build --no-cache

docker compose up -d
```

Verificar:

```bash
docker compose ps
```

---

# 81. Limpeza

Para remover containers:

```bash
docker compose down
```

Para remover também volumes:

```bash
docker compose down -v
```

Para remover imagens não utilizadas:

```bash
docker image prune
```

---

# 82. Teste Completo Recomendado

Uma validação completa pode ser executada nesta ordem:

## Infraestrutura

```bash
docker compose up -d

docker compose ps
```

## OIDC

Obter access token:

```bash
TOKEN=$(curl -s \
  -X POST \
  "http://localhost:8081/realms/backend-challenge/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=backend-api" \
  -d "client_secret=backend-api-secret" \
  | jq -r '.access_token')
```

## Health

Sem token:

```bash
curl -i http://localhost:8080/health
```

Esperado:

```text
401 Unauthorized
```

Com token:

```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/health
```

Readiness:

```bash
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/health/ready
```

## Testes automatizados

```bash
go test ./...
```

## Race detector

```bash
go test -race ./...
```

## Build

```bash
go build ./...
```

## Docker

```bash
docker compose build
docker compose up -d
```

## Observabilidade

Abrir:

```text
http://localhost:3000
```

```text
http://localhost:9090
```

```text
http://localhost:16686
```

---

# 83. Acceptance Checklist

## Backend

- [x] Go
- [x] HTTP API
- [x] Uber Fx
- [x] PostgreSQL
- [x] Keycloak/OIDC
- [x] Exact Money
- [x] Wallet
- [x] Wager Transaction
- [x] Ledger
- [x] Idempotency
- [x] Inbox
- [x] Outbox
- [x] SQS
- [x] Consumer
- [x] DLQ
- [x] Redelivery
- [x] Pending Reference Worker

## Concurrency

- [x] Transactional wallet update
- [x] No global lock
- [x] Double-spend protection
- [x] Concurrent processing tests
- [x] Race detector

## Observability

- [x] OpenTelemetry
- [x] Metrics
- [x] Prometheus
- [x] Grafana
- [x] Jaeger
- [x] HTTP metrics
- [x] Wager metrics
- [x] SQS metrics
- [x] Inbox metrics
- [x] Processing latency

## Tests

- [x] Unit tests
- [x] Integration tests
- [x] E2E
- [x] `go test ./...`
- [x] `go test -race ./...`

## Infrastructure

- [x] Docker
- [x] Docker Compose
- [x] MiniStack
- [x] PostgreSQL
- [x] Keycloak
- [x] Grafana
- [x] Prometheus
- [x] Jaeger
- [x] OTel Collector

---

# 84. Testes E2E Validados

Os principais cenários funcionais validados incluem:

## Autenticação

Sem token:

```text
Request
   ↓
401 Unauthorized
```

Com token válido:

```text
Bearer Access Token
    ↓
API
    ↓
200
```

---

## Wager

```text
Wallet = 100

Bet = 10

Result:

Wallet = 90
Transaction = PROCESSED
```

---

## Idempotency Replay

```text
Request 1
    ↓
Processed

Request 2
same Idempotency-Key
same payload
    ↓
Replay
```

Sem novo débito.

---

## Idempotency Conflict

```text
Request 1

Key = X
Amount = 10

Request 2

Key = X
Amount = 20
```

Resultado:

```text
409 Conflict
```

Sem novo efeito financeiro.

---

## Insufficient Balance

```text
Balance < Bet
```

Resultado:

```text
REJECTED
```

Sem débito.

---

## Concurrent Wagers

```text
Balance = 90

Bet A = 50
Bet B = 50
```

Resultado:

```text
One processed
One rejected

Final balance = 40
```

---

## SQS Consumer

Mensagem publicada:

```text
SQS
 ↓
Consumer
 ↓
Inbox
 ↓
Business Processing
 ↓
Delete Message
```

Após processamento normal, a mensagem é removida da fila.

Em caso de falha antes da conclusão, a mensagem pode ser redeliverada e a Inbox evita a duplicação do efeito financeiro.

---

# 85. Considerações de Produção

O ambiente deste desafio é local e voltado para avaliação/desenvolvimento.

Em produção, recomenda-se adicionalmente:

- TLS;
- Secrets Manager;
- IAM;
- private subnets;
- security groups;
- backups;
- disaster recovery;
- autoscaling;
- alertas;
- DLQ alarms;
- SQS visibility timeout adequado;
- database connection pooling;
- rate limiting;
- WAF;
- centralized logging;
- secret rotation;
- image scanning;
- vulnerability scanning;
- CI/CD com gates.

---

# 86. Decisões Arquiteturais

| Problema | Solução |
|---|---|
| Precisão financeira | Money decimal exato |
| Concorrência | PostgreSQL transaction/locking |
| Double spend | Atomicidade + locking |
| Retry HTTP | Idempotency |
| Redelivery SQS | Inbox |
| DB + evento | Outbox |
| Mensageria | SQS |
| Falhas permanentes | DLQ |
| Processamento assíncrono | Consumer |
| Dependências | Interfaces / Ports |
| DI | Uber Fx |
| Auth | OIDC / Keycloak |
| Métricas | OpenTelemetry / Prometheus |
| Dashboards | Grafana |
| Tracing | Jaeger |
| Ambiente local | Docker Compose |
| AWS local | MiniStack |

---

# 87. Documentação do Projeto

A documentação foi separada para facilitar a avaliação técnica e a manutenção do projeto.

## Requisitos

[REQUIREMENTS.md](REQUIREMENTS.md)

Contém os requisitos funcionais e não funcionais do sistema.

## Regras de Negócio

[BUSINESS_RULES.md](BUSINESS_RULES.md)

Contém as regras financeiras, invariantes, idempotência, concorrência, Inbox, Outbox e demais regras de negócio.

## Arquitetura

[ARCHITECTURE.md](ARCHITECTURE.md)

Contém detalhes da arquitetura, componentes, padrões, decisões, persistência, mensageria, observabilidade, segurança e escalabilidade.

## Rastreabilidade

[TRACEABILITY.md](TRACEABILITY.md)

Relaciona requisitos e regras de negócio com implementação, testes e cenários de aceitação.

## Casos de Uso

[docs/use-cases.md](docs/use-cases.md)

Descreve os principais fluxos funcionais do sistema.

## Contrato da API

[docs/api-contract.md](docs/api-contract.md)

Documenta autenticação, endpoints, requests, responses, validações e comportamento de idempotência.

## Eventos

[docs/events.md](docs/events.md)

Documenta eventos, Outbox, Inbox, SQS, redelivery, DLQ e processamento assíncrono.

---

# 88. Conclusão

O projeto foi desenvolvido com foco em um cenário realista de processamento financeiro distribuído.

Os principais mecanismos de confiabilidade são:

```text
Exact Money

     +

Database Transactions

     +

Concurrency Control

     +

Idempotency

     +

Inbox

     +

Outbox

     +

SQS

     +

DLQ

     +

Observability

     +

Automated Tests
```

O resultado é uma arquitetura modular, testável e preparada para evolução.

O domínio permanece desacoplado da infraestrutura, permitindo substituir componentes como:

```text
SQS
  ↕
RabbitMQ

PostgreSQL
  ↕
Outro adapter de persistência

MiniStack
  ↕
AWS
```

sem alterar as regras centrais do negócio.

As instruções completas para subir e validar o ambiente, bem como a documentação técnica e funcional necessária, estão neste README e nos documentos vinculados.

---

# 89. Comando Final de Validação

Antes de entregar:

```bash
find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w

gofmt -l $(find . -name '*.go' -not -path './vendor/*')

go test ./...

go test -race ./...

go build ./...

docker compose build

docker compose up -d

docker compose ps
```

Depois validar:

```text
Health sem token → 401

Health com token → 200

OIDC

Wager

Idempotency

Conflict

Insufficient Balance

Concurrency

Inbox

Outbox

SQS

Redelivery

DLQ

Pending Reference Worker

Grafana

Prometheus

Jaeger

OpenTelemetry
```

Com isso, o backend pode ser avaliado tanto pela funcionalidade quanto pelos aspectos arquiteturais, de consistência, concorrência, resiliência e observabilidade.