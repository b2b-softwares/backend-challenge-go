# Requirements Traceability Matrix

## 1. Objetivo

Este documento relaciona requisitos, regras de negócio, componentes
de implementação e testes.

A matriz permite responder:

> Onde este requisito foi implementado?

e:

> Qual teste comprova que esta regra funciona?

---

# 2. Requisitos → implementação → testes

| ID | Requisito | Implementação | Evidência/Teste |
|---|---|---|---|
| REQ-001 | Receber aposta | HTTP Handler + WagerService | testes HTTP/application |
| REQ-002 | Autenticação | OIDC/Keycloak middleware | E2E |
| REQ-003 | Validar entrada | HTTP Handler | testes HTTP |
| REQ-004 | Dinheiro exato | `domain.Money` | testes unitários |
| REQ-005 | Processar aposta | `WagerService.PlaceBet` | testes application/integration |
| REQ-006 | Saldo insuficiente | `WagerService` + wallet repository | testes de rejeição |
| REQ-007 | Concorrência | PostgreSQL transaction/locking | concurrency integration |
| REQ-008 | Idempotência | IdempotencyRepository | idempotency tests |
| REQ-009 | Conflito | IdempotencyRepository + service | conflict tests |
| REQ-010 | Inbox | Inbox repository + consumer | consumer/integration |
| REQ-011 | Outbox | Outbox repository | outbox tests |
| REQ-012 | Publicação assíncrona | Outbox publisher | integration |
| REQ-013 | SQS | SQS adapter/port | SQS integration |
| REQ-014 | Redelivery | consumer | SQS integration |
| REQ-015 | DLQ | SQS infrastructure | E2E/integration |
| REQ-016 | Pending worker | PendingReferenceWorker | worker tests |
| REQ-017 | Reversão | ReversalService | reversal tests |
| REQ-018 | Ledger | LedgerRepository | integration tests |
| REQ-019 | Transação | TransactionManager | integration tests |
| REQ-020 | Health | HTTP handler | HTTP tests |
| REQ-021 | Readiness | HTTP handler | HTTP/E2E |
| REQ-022 | Observabilidade | `internal/observability` | metrics/tracing |
| REQ-023 | OpenTelemetry | OTel providers | integration |
| REQ-024 | Prometheus | OTel Collector/Prometheus | observability |
| REQ-025 | Grafana | dashboard JSON | Grafana |
| REQ-026 | Tracing | OTel instrumentation | Jaeger |
| REQ-027 | Desacoplamento | domain/application/ports | architecture review |
| REQ-028 | Troca de infraestrutura | interfaces/adapters | architecture review |
| REQ-029 | Fx | `cmd/app/main.go` | application startup |
| REQ-030 | Docker | `docker-compose.yml` | environment |
| REQ-031 | PostgreSQL | postgres adapter + migrations | integration |
| REQ-032 | Testes | `tests/` | test suite |
| REQ-033 | Race detector | Go race detector | `go test -race ./...` |
| REQ-034 | Recuperação | Inbox/Outbox/SQS | integration |

---

# 3. Regras de negócio → implementação → testes

| Regra | Implementação | Evidência |
|---|---|---|
| BR-001 | Wallet transaction | concurrency tests |
| BR-002 | WagerService | wager tests |
| BR-003 | Insufficient balance handling | wager tests |
| BR-004 | Money | money tests |
| BR-005 | Money validation | money tests |
| BR-006 | Transaction/currency validation | service tests |
| BR-007 | Idempotency repository | idempotency tests |
| BR-008 | Replay handling | idempotency tests |
| BR-009 | Payload hash | idempotency conflict tests |
| BR-010 | Persistent repository | PostgreSQL tests |
| BR-011 | Database concurrency | concurrency integration |
| BR-012 | Repository transaction | architecture/code review |
| BR-013 | Per-wallet transaction scope | concurrency integration |
| BR-014 | DB transaction | integration tests |
| BR-015 | Ledger repository | persistence tests |
| BR-016 | ReversalService | reversal tests |
| BR-017 | Ledger data | audit tests |
| BR-018 | Inbox repository | consumer tests |
| BR-019 | Inbox state | consumer integration |
| BR-020 | PostgreSQL Inbox | integration |
| BR-021 | Outbox transaction | outbox integration |
| BR-022 | Publisher separation | outbox tests |
| BR-023 | Publisher behavior | integration |
| BR-024 | SQS delete-after-success | consumer integration |
| BR-025 | Consumer error path | consumer tests |
| BR-026 | Queue/DLQ configuration | infrastructure |
| BR-027 | Reversal append-only | reversal tests |
| BR-028 | Reversal reference | reversal tests |
| BR-029 | Reversal idempotency | reversal tests |
| BR-030 | Transaction states | domain/application tests |
| BR-031 | Persistent transaction | integration |
| BR-032 | HTTP/OIDC | E2E |
| BR-033 | Authentication validation | E2E |
| BR-034 | Correlation identifiers | observability |
| BR-035 | Transaction/ledger/outbox IDs | persistence |
| BR-036 | Inbox/Outbox | integration |
| BR-037 | Persistent states | recovery tests |

---

# 4. Cenários críticos

## RT-001 — Aposta normal

```text
REQ-005
  ↓
BR-002
  ↓
WagerService.PlaceBet
  ↓
Wallet + Transaction + Ledger + Outbox
  ↓
Integration tests
```

---

## RT-002 — Saldo insuficiente

```text
REQ-006
  ↓
BR-001
BR-003
  ↓
WagerService
  ↓
Transaction REJECTED
  ↓
No financial debit
  ↓
Insufficient balance test
```

---

## RT-003 — Idempotência

```text
REQ-008
REQ-009
  ↓
BR-007
BR-008
BR-009
  ↓
IdempotencyRepository
  ↓
Replay / Conflict
  ↓
Idempotency tests
```

---

## RT-004 — Concorrência

```text
REQ-007
  ↓
BR-011
BR-012
BR-013
BR-014
  ↓
PostgreSQL transaction
  ↓
Concurrent integration tests
  ↓
go test -race ./...
```

---

## RT-005 — Processamento assíncrono

```text
REQ-010
REQ-013
REQ-014
REQ-015
  ↓
BR-018
BR-019
BR-024
BR-025
BR-026
  ↓
Inbox + SQS Consumer + DLQ
  ↓
Integration/E2E
```

---

## RT-006 — Outbox

```text
REQ-011
REQ-012
  ↓
BR-021
BR-022
BR-023
  ↓
OutboxRepository
  ↓
OutboxPublisher
  ↓
Outbox tests
```

---

# 5. Critérios de aceitação rastreáveis

## AC-001

Dado saldo suficiente, uma aposta válida deve produzir uma transação
processada e atualizar o saldo.

Relacionamento:

```text
REQ-005
BR-002
```

---

## AC-002

Dado saldo insuficiente, a aposta deve ser rejeitada sem alterar
financeiramente a carteira.

Relacionamento:

```text
REQ-006
BR-001
BR-003
```

---

## AC-003

Dada uma mesma Idempotency-Key e payload idêntico, uma segunda
solicitação deve ser tratada como replay.

Relacionamento:

```text
REQ-008
BR-007
BR-008
```

---

## AC-004

Dada uma mesma Idempotency-Key e payload diferente, a segunda
solicitação deve ser rejeitada.

Relacionamento:

```text
REQ-009
BR-009
```

---

## AC-005

Duas apostas concorrentes sobre uma carteira devem preservar
a consistência do saldo.

Relacionamento:

```text
REQ-007
BR-011
BR-014
```

---

## AC-006

Uma mensagem já processada não deve executar novamente a operação.

Relacionamento:

```text
REQ-010
BR-019
```

---

## AC-007

Uma falha no processamento não deve remover prematuramente
a mensagem da fila.

Relacionamento:

```text
REQ-014
BR-024
BR-025
```

---

## AC-008

Uma operação financeira confirmada deve continuar recuperável
mesmo se a publicação externa falhar.

Relacionamento:

```text
REQ-011
REQ-012
BR-021
BR-022
```

---

# 6. Verificação final

Comandos principais:

```bash
go test ./...
```

```bash
go test -race ./...
```

```bash
docker compose up -d
```

A documentação deve ser considerada consistente com o código quando
os requisitos e regras descritos aqui forem encontrados nos testes
e componentes correspondentes.