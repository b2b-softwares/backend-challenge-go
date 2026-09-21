# ARCHITECTURE.md

# Backend Challenge — Distributed Wager Processing

## 1. Visão Geral

Este projeto implementa uma plataforma de processamento distribuído de apostas
com foco em consistência financeira, idempotência, processamento assíncrono,
concorrência segura, observabilidade e separação entre domínio e infraestrutura.

A arquitetura foi construída utilizando princípios de:

- Clean Architecture
- Hexagonal Architecture
- Ports and Adapters
- SOLID
- DDD Lite
- Dependency Inversion
- Transactional Integrity
- Inbox Pattern
- Outbox Pattern
- Idempotent Consumer
- Event-Driven Architecture

O objetivo principal é permitir que as regras de negócio permaneçam independentes
de HTTP, PostgreSQL, SQS, Uber Fx, OpenTelemetry ou qualquer outro mecanismo de
infraestrutura.

---

# 2. Objetivos Arquiteturais

Os principais objetivos são:

1. Garantir consistência financeira.
2. Impedir saldo negativo.
3. Impedir double spend.
4. Garantir idempotência de requisições.
5. Garantir idempotência no processamento assíncrono.
6. Permitir redelivery de mensagens.
7. Permitir recuperação após falhas.
8. Garantir persistência transacional.
9. Evitar locks globais na aplicação.
10. Permitir processamento concorrente seguro.
11. Separar domínio de infraestrutura.
12. Permitir substituição de adapters.
13. Permitir execução local e futura execução em AWS.
14. Garantir observabilidade.
15. Facilitar testes unitários e de integração.
16. Permitir escala horizontal.

---

# 3. Visão de Alto Nível

```text
                         ┌──────────────────────┐
                         │       Client         │
                         └──────────┬───────────┘
                                    │
                                    │ HTTP
                                    ▼
                         ┌──────────────────────┐
                         │      HTTP API        │
                         │  Authentication/OIDC │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │ Application Layer    │
                         │                      │
                         │ WagerService         │
                         │ ReversalService      │
                         │ Consumer              │
                         │ Pending Worker        │
                         └──────────┬───────────┘
                                    │
                         ┌──────────┴───────────┐
                         │                      │
                         ▼                      ▼
                ┌─────────────────┐    ┌─────────────────┐
                │ Domain          │    │ Ports           │
                │                 │    │                 │
                │ Money           │    │ Repository      │
                │ Wallet          │    │ SQS             │
                │ Transaction     │    │ Transaction     │
                │ Ledger          │    │ Manager         │
                │ Rules           │    │                 │
                └─────────────────┘    └────────┬────────┘
                                                 │
                                  ┌──────────────┼──────────────┐
                                  │              │              │
                                  ▼              ▼              ▼
                              PostgreSQL       SQS          Observability
                                             /MiniStack       OTel
```

---

# 4. Organização do Projeto

A estrutura segue separação clara entre aplicação, domínio e infraestrutura.

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
└── README.md
```

---

# 5. Clean Architecture

A aplicação segue a ideia de dependências apontando para dentro.

```text
                Infrastructure
                     │
                     ▼
                Application
                     │
                     ▼
                  Domain
```

O domínio não conhece:

- PostgreSQL
- SQS
- HTTP
- JSON
- Docker
- Uber Fx
- OpenTelemetry
- AWS
- MiniStack

A camada de aplicação conhece interfaces.

Os adapters implementam essas interfaces.

---

# 6. Hexagonal Architecture

A aplicação é organizada em torno de portas.

```text
                     ┌───────────────────┐
                     │      Domain       │
                     │                   │
                     │ Business Rules    │
                     └─────────┬─────────┘
                               │
                        Application
                               │
              ┌────────────────┼────────────────┐
              │                │                │
              ▼                ▼                ▼
          HTTP Port       Repository Port    Queue Port
              │                │                │
              ▼                ▼                ▼
          HTTP Adapter     PostgreSQL        SQS Adapter
```

Isso permite substituir uma implementação sem alterar o domínio.

Por exemplo:

```text
SQS
 │
 └── QueuePort

RabbitMQ
 │
 └── QueuePort
```

O caso de uso continua o mesmo.

---

# 7. Dependency Inversion

O código de negócio não depende diretamente de implementações concretas.

Exemplo conceitual:

```go
type WalletRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error)
    UpdateBalance(ctx context.Context, wallet *domain.Wallet) error
}
```

O application service depende dessa interface.

O PostgreSQL implementa a interface:

```text
Application
     │
     ▼
WalletRepository
     ▲
     │
PostgresWalletRepository
```

Isso facilita:

- testes unitários;
- mocks;
- troca de banco;
- troca de infraestrutura;
- evolução arquitetural.

---

# 8. Domain Layer

O domínio concentra regras financeiras.

Entre os principais conceitos:

- Wallet
- Money
- Wager Transaction
- Ledger Entry
- Transaction Status
- Business Errors

O domínio deve ser determinístico e independente de infraestrutura.

---

# 9. Money

Valores financeiros não utilizam `float32` ou `float64`.

A representação utiliza decimal exato.

Exemplo:

```text
R$ 10,00
R$ 0,01
R$ 100,50
R$ 999999,99
```

A criação do valor financeiro ocorre a partir de representação decimal/string.

Isso evita problemas de precisão binária.

Exemplo conceitual:

```go
money, err := domain.NewMoney("10.50")
```

Operações financeiras devem permanecer determinísticas.

---

# 10. Regras Financeiras

Uma aposta deve:

1. possuir valor válido;
2. possuir moeda válida;
3. possuir wallet existente;
4. possuir saldo suficiente;
5. debitar a wallet;
6. registrar a transação;
7. registrar o ledger;
8. produzir evento para processamento assíncrono quando aplicável.

Saldo insuficiente não deve gerar:

- saldo negativo;
- ledger de débito;
- side effect parcial.

---

# 11. Wallet e Concorrência

O sistema não utiliza um mutex global para proteger wallets.

Isso seria um anti-pattern para uma aplicação distribuída.

Um lock global:

```text
Request A ─┐
Request B ─┼── Global Lock
Request C ─┘
```

criaria um gargalo.

Em vez disso, a consistência é garantida no banco utilizando transações
e mecanismos de concorrência apropriados.

Conceitualmente:

```text
Transaction
    │
    ▼
SELECT wallet ... FOR UPDATE
    │
    ▼
Validate balance
    │
    ▼
Update wallet
    │
    ▼
Insert transaction
    │
    ▼
Insert ledger
    │
    ▼
Commit
```

Duas operações concorrentes sobre a mesma wallet são serializadas pelo banco.

---

# 12. Double Spend

Considere:

```text
Saldo = R$ 90

Request A = R$ 50
Request B = R$ 50
```

As duas requisições não podem resultar em:

```text
Saldo = -R$ 10
```

O banco controla a concorrência.

Uma operação consegue realizar o débito.

A outra encontra saldo insuficiente.

Resultado esperado:

```text
Request A → PROCESSED
Request B → REJECTED

Saldo final = R$ 40
```

Essa garantia é feita na transação de banco, e não por lock global da aplicação.

---

# 13. Transaction Manager

O acesso transacional é abstraído.

A aplicação não precisa conhecer diretamente o mecanismo específico do PostgreSQL.

Conceitualmente:

```go
type TransactionManager interface {
    WithinTransaction(
        ctx context.Context,
        fn func(ctx context.Context) error,
    ) error
}
```

Isso mantém a aplicação desacoplada da implementação concreta.

---

# 14. Atomicidade

A operação financeira deve ser atômica.

Exemplo:

```text
BEGIN

Wallet update
      +
Transaction insert
      +
Ledger insert
      +
Outbox insert

COMMIT
```

Se qualquer operação falhar:

```text
ROLLBACK
```

O sistema não deve chegar a estados como:

```text
wallet debitado
transaction inexistente
ledger inexistente
event perdido
```

---

# 15. Idempotência HTTP

O endpoint exige:

```http
Idempotency-Key: <key>
```

A chave é persistida.

A primeira requisição:

```text
Idempotency-Key = abc123
       │
       ▼
Processamento
       │
       ▼
Persistência
```

Uma repetição:

```text
Idempotency-Key = abc123
       │
       ▼
Registro existente
       │
       ▼
Replay
```

A operação não é executada novamente.

---

# 16. Idempotency Conflict

Idempotência não significa aceitar qualquer payload com a mesma chave.

Exemplo:

Primeira requisição:

```text
Idempotency-Key: abc123
Amount: 10.00
```

Segunda:

```text
Idempotency-Key: abc123
Amount: 50.00
```

Isso deve ser considerado conflito.

Resposta:

```http
409 Conflict
```

Nenhum novo efeito financeiro deve ocorrer.

---

# 17. Payload Hash

A requisição é associada a um hash do payload.

Conceitualmente:

```text
Idempotency Key
       +
Payload Hash
       │
       ▼
Idempotency Record
```

Isso permite distinguir:

```text
mesma requisição
```

de:

```text
mesma chave + payload diferente
```

---

# 18. Inbox Pattern

O processamento assíncrono utiliza Inbox persistente.

Fluxo:

```text
SQS Message
     │
     ▼
Inbox Ensure
     │
     ├── Already Processed ──► Delete SQS
     │
     └── New Message
              │
              ▼
           Claim
              │
              ▼
         Processing
```

A Inbox registra que a mensagem foi recebida e seu estado de processamento.

---

# 19. Estados da Inbox

Conceitualmente:

```text
RECEIVED
   │
   ▼
PROCESSING
   │
   ▼
PROCESSED
```

Em caso de falha:

```text
PROCESSING
   │
   ▼
retry / redelivery
```

O processamento pode ser retomado sem duplicar o efeito financeiro.

---

# 20. Inbox e Idempotência

São mecanismos diferentes.

### Idempotency

Protege a entrada HTTP.

```text
Client
  │
  ▼
HTTP
  │
  ▼
Idempotency
```

### Inbox

Protege o processamento assíncrono.

```text
SQS
 │
 ▼
Inbox
 │
 ▼
Consumer
```

Eles podem coexistir.

---

# 21. Outbox Pattern

Eventos de negócio são persistidos na Outbox dentro da mesma transação
da operação financeira.

```text
BEGIN

Wallet
Transaction
Ledger
Outbox

COMMIT
```

Somente depois do commit o publisher envia o evento para SQS.

---

# 22. Por que Outbox?

Sem Outbox:

```text
DB COMMIT
   │
   ▼
Publish SQS
   │
   X
   └── falha
```

O banco teria sido atualizado, mas o evento seria perdido.

Com Outbox:

```text
DB Transaction
      │
      ├── Wallet
      ├── Transaction
      ├── Ledger
      └── Outbox
             │
             ▼
           COMMIT
             │
             ▼
       Outbox Publisher
             │
             ▼
            SQS
```

A persistência do evento faz parte da mesma transação.

---

# 23. Outbox Publisher

O publisher busca eventos pendentes.

Conceitualmente:

```text
OUTBOX
  │
  ├── PENDING
  │
  ▼
Publish SQS
  │
  ▼
MARK PUBLISHED
```

Se SQS estiver indisponível:

```text
OUTBOX
  │
  ▼
PENDING
```

O evento permanece persistido.

Após recuperação da infraestrutura:

```text
PENDING
   │
   ▼
Retry
   │
   ▼
SQS
   │
   ▼
PUBLISHED
```

---

# 24. SQS

O projeto utiliza uma abstração de fila.

A aplicação não depende diretamente de uma implementação específica.

```text
QueuePort
   ▲
   │
   ├── SQS Adapter
   │
   └── RabbitMQ Adapter
```

No ambiente local:

```text
Application
    │
    ▼
SQS Adapter
    │
    ▼
MiniStack
```

Em AWS:

```text
Application
    │
    ▼
SQS Adapter
    │
    ▼
Amazon SQS
```

---

# 25. FIFO Queue

A fila utiliza características de FIFO quando necessárias.

Isso ajuda a manter ordenação dentro do grupo de mensagens.

O agrupamento pode utilizar uma chave relacionada ao domínio,
como wallet ou entidade financeira.

A aplicação, entretanto, não deve depender exclusivamente da ordenação da fila
para garantir consistência.

A consistência financeira continua sendo garantida pelo banco.

---

# 26. Consumer

O consumer executa:

```text
SQS Receive
      │
      ▼
Validate Message
      │
      ▼
Inbox Ensure
      │
      ▼
Inbox Claim
      │
      ▼
Business Transaction
      │
      ├── WagerService
      └── Inbox MarkProcessed
      │
      ▼
COMMIT
      │
      ▼
Delete SQS Message
```

A mensagem só é removida da fila após o processamento transacional.

---

# 27. Falha Antes do Commit

Se ocorrer:

```text
SQS Receive
      │
      ▼
Business Processing
      │
      X
   Failure
```

A mensagem não deve ser considerada processada.

Ela permanece disponível para redelivery.

---

# 28. Falha Depois do Commit

Se:

```text
DB COMMIT
   │
   ▼
Delete SQS
   │
   X
Failure
```

A mensagem poderá retornar.

Nesse cenário a Inbox detecta:

```text
Already Processed
```

e o consumer pode remover a mensagem sem repetir o efeito financeiro.

Esse é um dos objetivos principais do padrão Idempotent Consumer.

---

# 29. DLQ

Mensagens que falham repetidamente podem ser encaminhadas para uma Dead Letter Queue.

Conceitualmente:

```text
SQS
 │
 ├── retry 1
 ├── retry 2
 ├── retry 3
 │
 ▼
DLQ
```

A DLQ impede que uma mensagem permanentemente inválida bloqueie indefinidamente
o processamento normal.

---

# 30. Redelivery

Redelivery é tratado como comportamento esperado.

A aplicação não assume:

```text
"uma mensagem será entregue apenas uma vez"
```

Em vez disso assume:

```text
at-least-once delivery
```

e garante idempotência no consumidor.

---

# 31. Pending Reference Worker

O projeto possui worker para referências pendentes.

A ideia é tratar situações onde uma entidade necessária para completar o processamento
ainda não esteja disponível.

Fluxo:

```text
Pending Reference
       │
       ▼
Worker
       │
       ▼
Check Reference
       │
       ├── Available ──► Continue
       │
       └── Missing ────► Retry
```

Esse mecanismo permite trabalhar com consistência eventual.

---

# 32. HTTP Adapter

O adapter HTTP é responsável por:

- parsing;
- validação de entrada;
- autenticação;
- headers;
- JSON;
- status HTTP;
- serialização da resposta.

Ele não deve conter regras financeiras.

Exemplo:

```text
HTTP Handler
    │
    ▼
Request DTO
    │
    ▼
Application Service
    │
    ▼
Domain
```

---

# 33. Authentication

A API utiliza OIDC/Keycloak.

O cliente utiliza:

```text
client_credentials
```

Fluxo:

```text
Client
   │
   │ credentials
   ▼
Keycloak
   │
   │ access token
   ▼
Client
   │
   │ Authorization: Bearer
   ▼
API
```

A API valida o token antes de executar o caso de uso.

---

# 34. Segurança

A autenticação é separada da regra de negócio.

O domínio não conhece:

- JWT;
- Keycloak;
- OAuth;
- OIDC;
- Authorization header.

Esses mecanismos pertencem à infraestrutura/interface.

---

# 35. Observabilidade

A observabilidade utiliza:

```text
OpenTelemetry
       │
       ├── Traces
       └── Metrics
              │
              ▼
       OTel Collector
              │
       ┌──────┴──────┐
       ▼             ▼
    Jaeger       Prometheus
                     │
                     ▼
                  Grafana
```

---

# 36. OpenTelemetry

O código utiliza OpenTelemetry para instrumentação.

Os principais sinais utilizados são:

- traces;
- metrics.

A configuração fica centralizada em:

```text
internal/observability/observability.go
```

Essa implementação fornece:

- TracerProvider;
- MeterProvider;
- OTLP exporters;
- Resource attributes;
- service.name;
- deployment.environment.

---

# 37. Tracing

Traces permitem acompanhar uma requisição através das camadas.

Exemplo:

```text
HTTP Request
     │
     ▼
WagerService
     │
     ├── PostgreSQL
     │
     ├── Outbox
     │
     └── SQS
```

Em ambiente distribuído isso permite investigar:

- latência;
- falhas;
- dependências;
- gargalos;
- chamadas externas.

---

# 38. Metrics

As principais métricas incluem:

```text
http_server_request_duration_seconds

wager_transactions_total

wager_transactions_processed_total

wager_processing_duration_seconds

sqs_messages_received_total

sqs_messages_processed_total

sqs_messages_deleted_total

sqs_message_processing_duration_seconds

inbox_messages_received_total

inbox_messages_processed_total
```

---

# 39. HTTP Metrics

As métricas HTTP permitem observar:

- volume;
- erros;
- latência;
- distribuição de duração.

Exemplo de taxa de requisições:

```promql
sum(rate(http_server_request_duration_seconds_count[5m]))
```

Taxa de erro:

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

# 40. Histogramas

Latência não deve ser observada somente pela média.

O projeto utiliza histogramas para permitir percentis.

Exemplo P95:

```promql
histogram_quantile(
  0.95,
  sum by (le) (
    rate(http_server_request_duration_seconds_bucket[5m])
  )
)
```

Isso permite detectar tail latency.

---

# 41. Wager Metrics

A aplicação mede operações de apostas.

Exemplo:

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

# 42. Wager Processing Latency

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

# 43. SQS Metrics

O consumer mede:

```text
sqs_messages_received_total
sqs_messages_processed_total
sqs_messages_deleted_total
sqs_message_processing_duration_seconds
```

Isso permite acompanhar:

```text
Received
   │
   ▼
Processed
   │
   ▼
Deleted
```

Diferenças entre esses valores podem indicar:

- falhas;
- redelivery;
- processamento lento;
- mensagens pendentes.

---

# 44. Inbox Metrics

A Inbox possui métricas para:

```text
inbox_messages_received_total
inbox_messages_processed_total
```

Essas métricas ajudam a observar o comportamento do consumidor idempotente.

---

# 45. Grafana

O projeto possui dashboard:

```text
Backend Challenge - Distributed Wager Processing
```

O dashboard possui painéis para:

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

---

# 46. Stack de Observabilidade

Ambiente local:

```text
Grafana
http://localhost:3000

Prometheus
http://localhost:9090

Jaeger
http://localhost:16686

OTel Collector
http://localhost:8889/metrics
```

API:

```text
http://localhost:8080
```

Keycloak:

```text
http://localhost:8081
```

MiniStack:

```text
http://localhost:4566
```

---

# 47. Persistência

PostgreSQL é utilizado como banco transacional.

A persistência inclui entidades relacionadas a:

- wallets;
- wager transactions;
- ledger;
- idempotency;
- inbox;
- outbox.

As alterações estruturais são realizadas por migrations.

---

# 48. Ledger

O ledger é append-only.

Isso significa que eventos financeiros não devem ser simplesmente sobrescritos.

Conceitualmente:

```text
Ledger
----------------------------
ENTRY 1  +100.00
ENTRY 2  -10.00
ENTRY 3  -20.00
ENTRY 4  +50.00
```

O histórico representa as movimentações.

Isso melhora:

- auditoria;
- rastreabilidade;
- reconciliação;
- investigação de incidentes.

---

# 49. Transação Financeira

Uma operação típica:

```text
POST /wagering/transactions
             │
             ▼
        Authenticate
             │
             ▼
       Validate Input
             │
             ▼
       Idempotency Check
             │
             ▼
       BEGIN TRANSACTION
             │
             ├── Wallet Lock
             │
             ├── Validate Balance
             │
             ├── Debit Wallet
             │
             ├── Create Transaction
             │
             ├── Append Ledger
             │
             └── Create Outbox
             │
             ▼
           COMMIT
             │
             ▼
          Response
```

---

# 50. Rejected Wager

Saldo insuficiente é uma rejeição de negócio.

Exemplo:

```text
Balance = 10.00
Bet     = 50.00
```

Resultado:

```text
Transaction = REJECTED
Balance     = 10.00
Ledger      = sem débito
```

Esse comportamento é diferente de um erro técnico.

---

# 51. Business Error vs Technical Error

### Business Error

Exemplo:

```text
insufficient balance
```

Pode resultar em:

```http
409 Conflict
```

ou status equivalente definido pelo contrato.

### Technical Error

Exemplo:

```text
PostgreSQL unavailable
SQS unavailable
```

Deve ser tratado como falha técnica e não como rejeição financeira normal.

Essa distinção é importante para observabilidade e retry.

---

# 52. Error Handling

Os erros devem manter semântica.

Exemplo:

```text
Domain Error
     │
     ▼
Application
     │
     ▼
HTTP Adapter
     │
     ▼
HTTP Status
```

O domínio não deve conhecer códigos HTTP.

---

# 53. Testabilidade

A arquitetura facilita testes porque os casos de uso dependem de interfaces.

Exemplo:

```text
WagerService
    │
    ├── WalletRepository
    ├── TransactionRepository
    ├── LedgerRepository
    ├── IdempotencyRepository
    ├── OutboxRepository
    └── TransactionManager
```

Nos testes essas interfaces podem ser substituídas por mocks/fakes.

---

# 54. Unit Tests

Os testes unitários validam:

- Money;
- regras de domínio;
- validações;
- idempotência;
- insuficiência de saldo;
- estados;
- application services.

Executar:

```bash
go test ./tests/unit/...
```

---

# 55. Integration Tests

Os testes de integração validam componentes reais.

Exemplos:

- PostgreSQL;
- transações;
- concorrência;
- repositories;
- Inbox;
- Outbox.

Executar:

```bash
go test ./tests/integration/...
```

---

# 56. Race Detector

O projeto deve ser validado com:

```bash
go test -race ./...
```

O race detector é especialmente importante porque existem:

- consumers;
- workers;
- goroutines;
- processamento concorrente;
- acesso compartilhado;
- operações financeiras simultâneas.

---

# 57. Concorrência

O cenário crítico é:

```text
Wallet = 90

Goroutine A → 50
Goroutine B → 50
```

Resultado esperado:

```text
A → success
B → rejected

Final = 40
```

O teste deve validar tanto o resultado lógico quanto ausência de data races.

---

# 58. E2E

O teste end-to-end percorre a arquitetura completa:

```text
Client
 │
 ▼
Keycloak
 │
 ▼
HTTP API
 │
 ▼
Application
 │
 ▼
PostgreSQL
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
 │
 ▼
Application
 │
 ▼
PostgreSQL
```

Isso valida integração entre os principais componentes.

---

# 59. LocalStack / MiniStack

Para desenvolvimento local, o projeto utiliza MiniStack para simular serviços AWS.

O objetivo é permitir:

```text
Local Development
       │
       ▼
MiniStack
       │
       ▼
SQS-compatible API
```

Sem necessidade de depender de uma conta AWS para testes locais.

---

# 60. Local vs AWS

A arquitetura permite:

```text
LOCAL

PostgreSQL
MiniStack
Keycloak
Grafana
Prometheus
Jaeger
OTel Collector
```

e posteriormente:

```text
AWS

RDS / PostgreSQL
SQS
Keycloak ou IdP corporativo
CloudWatch / Grafana
OpenTelemetry
ECS / EKS
```

O domínio não precisa ser alterado para essa migração.

---

# 61. Configuration Driven Architecture

Configurações devem ser externas ao código.

Exemplos:

```text
DATABASE_URL
SQS_ENDPOINT
SQS_QUEUE_NAME
SQS_DLQ_NAME
KEYCLOAK_URL
OTEL_EXPORTER_OTLP_ENDPOINT
SERVICE_NAME
ENVIRONMENT
```

Isso permite mudar o ambiente sem alterar regras de negócio.

---

# 62. Uber Fx

Uber Fx é utilizado como composition root.

O `main.go` é responsável por montar a aplicação.

Conceitualmente:

```text
main.go
   │
   ├── Config
   ├── Observability
   ├── Database
   ├── Repositories
   ├── Services
   ├── HTTP
   ├── SQS
   └── Workers
```

O Fx resolve as dependências.

A lógica de negócio não deve depender diretamente do Fx.

---

# 63. Composition Root

O composition root conhece as implementações concretas.

```text
cmd/app/main.go
       │
       ├── PostgreSQL Adapter
       ├── SQS Adapter
       ├── HTTP Server
       ├── Observability
       └── Application Services
```

Essa é uma decisão importante para evitar acoplamento do domínio ao framework.

---

# 64. Lifecycle

Workers e servidores são registrados no lifecycle da aplicação.

Conceitualmente:

```text
Application Start
       │
       ├── HTTP
       ├── Outbox Publisher
       ├── Wager Consumer
       └── Pending Worker
       │
       ▼
Application Running
       │
       ▼
Application Shutdown
       │
       ├── Stop HTTP
       ├── Stop Consumer
       ├── Stop Workers
       └── Flush Observability
```

---

# 65. Graceful Shutdown

A aplicação deve encerrar de forma controlada.

Objetivos:

- não interromper transações em andamento de forma abrupta;
- interromper workers;
- finalizar operações em andamento;
- liberar conexões;
- flush de telemetry;
- fechar recursos.

---

# 66. Scalability

A aplicação foi desenhada para escala horizontal.

Exemplo:

```text
             Load Balancer
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
      API-1     API-2     API-3
        │         │         │
        └─────────┼─────────┘
                  ▼
              PostgreSQL
```

Consumers:

```text
             SQS
              │
       ┌──────┼──────┐
       ▼      ▼      ▼
   Consumer1 Consumer2 Consumer3
```

A consistência é garantida pela persistência e idempotência.

---

# 67. Stateless API

A API não deve depender de estado local em memória para garantir consistência.

Isso permite:

```text
API instance A
API instance B
API instance C
```

processarem requisições simultaneamente.

Estado crítico permanece em PostgreSQL.

---

# 68. No Global Lock

Não é utilizado:

```go
var globalMutex sync.Mutex
```

para proteger todas as wallets.

Isso impediria escala horizontal e criaria gargalo.

A coordenação financeira ocorre no banco.

---

# 69. Eventual Consistency

O processamento assíncrono naturalmente possui etapas:

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

Essas etapas podem não acontecer no mesmo instante.

O sistema assume consistência eventual onde apropriado.

A operação financeira principal, entretanto, permanece transacional.

---

# 70. Observabilidade como Parte da Arquitetura

Observabilidade não é apenas monitoramento externo.

Ela é integrada ao fluxo de aplicação.

Exemplo:

```text
Request
  │
  ├── Trace
  ├── Metrics
  │
  ▼
WagerService
  │
  ├── Metrics
  ├── Trace
  │
  ▼
Database / SQS
```

Isso permite correlacionar comportamento de negócio e infraestrutura.

---

# 71. Diagnóstico de Incidentes

Uma investigação típica pode seguir:

```text
Grafana
   │
   ▼
Latency / Error Rate
   │
   ▼
Trace ID
   │
   ▼
Jaeger
   │
   ▼
Application Span
   │
   ▼
Database / SQS
```

Isso reduz o tempo necessário para identificar gargalos.

---

# 72. Failure Scenarios

### PostgreSQL indisponível

Resultado:

```text
Transaction fails
```

Nenhum débito parcial deve permanecer.

---

### SQS indisponível

A Outbox permanece:

```text
PENDING
```

e pode ser publicada posteriormente.

---

### Consumer falha

A mensagem pode ser redeliverada.

A Inbox impede duplicação do efeito.

---

### Delete SQS falha

A mensagem pode retornar.

A Inbox detecta:

```text
PROCESSED
```

e evita novo processamento financeiro.

---

### API reinicia

O estado permanece no PostgreSQL.

A aplicação pode voltar a processar.

---

# 73. Anti-Patterns Evitados

A arquitetura evita:

### Regra de negócio no Handler

```text
HTTP Handler
 ├── SQL
 ├── wallet calculation
 ├── ledger
 └── business rules
```

Não recomendado.

---

### Domínio dependendo de PostgreSQL

Não recomendado.

---

### Domínio dependendo de SQS

Não recomendado.

---

### Float para dinheiro

Não permitido.

---

### Global Mutex

Não utilizado para consistência financeira.

---

### Estado financeiro somente em memória

Não permitido.

---

### Publicar SQS antes do commit

Evita-se devido ao Outbox.

---

### Deletar SQS antes do commit

Evita-se porque poderia perder mensagem.

---

### Consumidor sem Inbox

Evita-se porque redelivery poderia gerar efeitos duplicados.

---

# 74. Princípios SOLID

## Single Responsibility

Cada componente possui uma responsabilidade clara.

Exemplos:

```text
HTTP Handler
    → HTTP

WagerService
    → Use case

Repository
    → Persistence

SQS Adapter
    → Messaging

Observability
    → Telemetry
```

---

## Open/Closed

Adapters podem ser adicionados sem alterar o domínio.

Exemplo:

```text
SQS
RabbitMQ
Kafka
```

podem implementar a mesma porta.

---

## Liskov Substitution

Uma implementação de uma porta deve respeitar o contrato definido pela interface.

---

## Interface Segregation

Interfaces são pequenas e específicas.

---

## Dependency Inversion

Application e domínio dependem de abstrações.

---

# 75. DDD Lite

O projeto não tenta implementar DDD completo.

Utiliza conceitos que agregam valor ao problema:

- entidades;
- value objects;
- regras de domínio;
- serviços de aplicação;
- repositories;
- eventos;
- estados.

O objetivo é manter o domínio expressivo sem adicionar complexidade desnecessária.

---

# 76. Aggregate Wallet

A wallet pode ser tratada como unidade de consistência financeira.

Operações relacionadas ao saldo precisam respeitar a consistência desse agregado.

A persistência e concorrência são coordenadas pelo banco.

---

# 77. Transaction Lifecycle

Uma transação pode passar por estados como:

```text
RECEIVED
    │
    ▼
PROCESSING
    │
    ├── success ──► PROCESSED
    │
    └── business failure ──► REJECTED
```

Estados devem ser persistidos para auditoria e idempotência.

---

# 78. Auditability

A combinação:

```text
Transaction
+
Ledger
+
Inbox
+
Outbox
```

permite reconstruir a trajetória de uma operação.

Isso é importante para sistemas financeiros.

---

# 79. Data Consistency

Existem dois tipos principais de consistência:

### Strong Consistency

Utilizada na operação financeira principal.

Exemplo:

```text
wallet + transaction + ledger
```

### Eventual Consistency

Utilizada em processamento assíncrono.

Exemplo:

```text
outbox → SQS → consumer
```

Essa distinção é intencional.

---

# 80. Performance

A arquitetura evita gargalos artificiais.

Exemplos:

- API stateless;
- processamento concorrente;
- ausência de global lock;
- SQS para desacoplamento;
- workers independentes;
- banco como coordenador de concorrência;
- observabilidade baseada em métricas.

---

# 81. Resilience

A arquitetura suporta:

- retries;
- redelivery;
- idempotência;
- Inbox;
- Outbox;
- DLQ;
- transações;
- graceful shutdown.

Isso permite recuperar-se de falhas transitórias.

---

# 82. Test Pyramid

A estratégia de testes segue:

```text
              E2E
             /   \
        Integration
          /       \
         Unit Tests
```

A maior quantidade de testes deve estar próxima do domínio e application layer.

Testes de infraestrutura validam integração real.

E2E valida o fluxo completo.

---

# 83. Regression

Antes de considerar uma alteração concluída:

```bash
go test ./...
```

e:

```bash
go test -race ./...
```

devem passar.

Quando Docker estiver disponível, também devem ser validados:

```bash
docker compose up -d
```

seguido pelos testes de integração/E2E aplicáveis.

---

# 84. Quality Gates

Os principais gates são:

```text
Compilation
    │
    ▼
Unit Tests
    │
    ▼
Integration Tests
    │
    ▼
Race Detector
    │
    ▼
E2E
    │
    ▼
Observability
    │
    ▼
Documentation
```

---

# 85. Production Considerations

Para produção, recomenda-se:

- secrets manager;
- TLS;
- IAM;
- database credentials fora do código;
- network segmentation;
- least privilege;
- autoscaling;
- SQS visibility timeout adequado;
- DLQ monitoring;
- database backups;
- migrations controladas;
- alertas;
- dashboards;
- distributed tracing.

---

# 86. AWS Mapping

Uma possível evolução:

```text
Local                    AWS

PostgreSQL      →       RDS PostgreSQL
MiniStack       →       Amazon SQS
Keycloak        →       OIDC Provider
Docker Compose  →       ECS / EKS
OTel Collector  →       AWS / Managed Backend
Prometheus      →       Managed Prometheus
Grafana         →       Grafana
Jaeger          →       Trace backend
```

A arquitetura permite essa evolução sem alterar o domínio.

---

# 87. Deployment Model

Uma implantação pode ser:

```text
                    Internet
                       │
                       ▼
                Load Balancer
                       │
              ┌────────┴────────┐
              ▼                 ▼
            API 1             API 2
              │                 │
              └────────┬────────┘
                       ▼
                   PostgreSQL

                       │
                       ▼
                      SQS
                       │
                ┌──────┴──────┐
                ▼             ▼
            Consumer 1    Consumer 2
```

---

# 88. Security Boundaries

As fronteiras principais são:

```text
External Client
      │
      ▼
Authentication
      │
      ▼
HTTP Adapter
      │
      ▼
Application
      │
      ▼
Persistence / Messaging
```

Cada camada possui responsabilidades distintas.

---

# 89. Repository Pattern

Repositories escondem detalhes de persistência.

Exemplo conceitual:

```go
type LedgerRepository interface {
    Append(ctx context.Context, entry *domain.LedgerEntry) error
}
```

O application service não precisa saber:

```sql
INSERT INTO ledger ...
```

Esse detalhe pertence ao adapter.

---

# 90. Messaging Port

A mesma filosofia é utilizada para filas.

Exemplo conceitual:

```go
type MessageQueue interface {
    Publish(ctx context.Context, message Message) error
    Receive(ctx context.Context) ([]Message, error)
    Delete(ctx context.Context, message Message) error
}
```

A implementação concreta pode ser SQS.

---

# 91. Evolução para RabbitMQ

Se futuramente a infraestrutura utilizar RabbitMQ:

```text
Antes:

Application
    │
    ▼
QueuePort
    │
    ▼
SQS Adapter


Depois:

Application
    │
    ▼
QueuePort
    │
    ▼
RabbitMQ Adapter
```

A aplicação não precisa ser reescrita.

---

# 92. Evolução para Kafka

O mesmo conceito pode ser utilizado para Kafka quando o modelo de entrega
e processamento justificar essa tecnologia.

O importante é preservar:

- contrato;
- idempotência;
- consistência;
- observabilidade.

---

# 93. Transactional Boundaries

Cada caso de uso define sua própria fronteira transacional.

Isso evita transações gigantes que envolvam:

```text
HTTP
+
SQS
+
Database
+
External API
```

A transação de banco deve ser curta e determinística.

---

# 94. Não Existe Transação Distribuída

A arquitetura não depende de 2PC entre:

```text
PostgreSQL
+
SQS
```

Em vez disso utiliza:

```text
PostgreSQL Transaction
+
Outbox
+
Asynchronous Publishing
```

Esse modelo reduz acoplamento e melhora resiliência.

---

# 95. Exactly Once vs At Least Once

SQS normalmente deve ser tratado como mecanismo de entrega que pode resultar
em redelivery.

Portanto:

```text
Delivery = at least once
```

A aplicação busca:

```text
Effect = effectively once
```

por meio de:

- Inbox;
- Idempotency;
- database constraints;
- transactions.

---

# 96. Idempotent Consumer

O consumidor não confia na fila para evitar duplicação.

Ele utiliza persistência:

```text
Message ID / Payload Hash
            │
            ▼
          Inbox
            │
            ▼
       Business Effect
```

Assim uma mesma mensagem pode ser recebida novamente sem duplicar o efeito.

---

# 97. Database as Consistency Coordinator

Em operações financeiras concorrentes, PostgreSQL atua como coordenador da
consistência.

Isso é preferível a manter estado crítico em memória da aplicação.

---

# 98. Horizontal Scaling

Com múltiplas instâncias:

```text
API 1
API 2
API 3
```

todas compartilham o mesmo banco.

Consumers podem ser escalados:

```text
Consumer 1
Consumer 2
Consumer 3
Consumer 4
```

A Inbox e as transações impedem efeitos duplicados.

---

# 99. Operational Visibility

A equipe pode acompanhar:

```text
HTTP
 │
 ├── throughput
 ├── errors
 └── latency

Wager
 │
 ├── processed
 ├── rejected
 └── latency

SQS
 │
 ├── received
 ├── processed
 └── deleted

Inbox
 │
 ├── received
 └── processed
```

Isso fornece visão operacional do sistema.

---

# 100. Architectural Decision Summary

Principais decisões:

| Decisão | Motivo |
|---|---|
| Clean Architecture | Separação de responsabilidades |
| Hexagonal Architecture | Desacoplamento de infraestrutura |
| Interfaces | Testabilidade e substituição de adapters |
| Exact Money | Precisão financeira |
| PostgreSQL | Consistência transacional |
| Database locking | Concorrência financeira |
| Idempotency | Proteção contra retries HTTP |
| Inbox | Idempotência assíncrona |
| Outbox | Atomicidade entre DB e eventos |
| SQS | Processamento assíncrono |
| DLQ | Isolamento de mensagens problemáticas |
| OpenTelemetry | Observabilidade |
| Prometheus | Métricas |
| Grafana | Dashboards |
| Jaeger | Distributed tracing |
| Uber Fx | Dependency Injection |
| Keycloak/OIDC | Authentication |
| Docker Compose | Ambiente local reproduzível |

---

# 101. Fluxo Completo

O fluxo completo pode ser representado como:

```text
                    CLIENT
                       │
                       ▼
                ┌─────────────┐
                │  Keycloak   │
                └──────┬──────┘
                       │ token
                       ▼
                ┌─────────────┐
                │ HTTP API    │
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
                └──────┬──────┘
                       │
                       ▼
                ┌─────────────┐
                │ PostgreSQL  │
                │             │
                │ Wallet      │
                │ Transaction │
                │ Ledger      │
                │ Outbox      │
                └──────┬──────┘
                       │ commit
                       ▼
                ┌─────────────┐
                │Outbox Worker│
                └──────┬──────┘
                       │
                       ▼
                ┌─────────────┐
                │     SQS     │
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
                └──────┬──────┘
                       │
                       ▼
                ┌─────────────┐
                │ PostgreSQL  │
                └─────────────┘


Observability:

        Application
             │
             ▼
       OpenTelemetry
             │
             ▼
       OTel Collector
         │          │
         ▼          ▼
    Prometheus     Jaeger
         │
         ▼
      Grafana
```

---

# 102. Architectural Validation

A arquitetura atende aos principais requisitos do desafio:

- processamento distribuído;
- processamento assíncrono;
- exact money;
- persistência;
- concorrência;
- idempotência;
- Inbox;
- Outbox;
- SQS;
- DLQ;
- retries;
- redelivery;
- OIDC;
- observabilidade;
- testes;
- Docker;
- modularidade;
- separação de responsabilidades.

---

# 103. Estado Esperado do Sistema

Em operação normal:

```text
HTTP
    │
    ▼
Authenticated
    │
    ▼
Validated
    │
    ▼
Idempotent
    │
    ▼
Transactional
    │
    ▼
Ledgered
    │
    ▼
Outboxed
    │
    ▼
Published
    │
    ▼
Consumed
    │
    ▼
Inbox Protected
```

---

# 104. Conclusão

A arquitetura foi projetada para tratar o processamento de apostas como um
problema de consistência financeira distribuída, e não apenas como uma API HTTP.

Os principais pilares são:

```text
Domain Rules
      +
Transactional Consistency
      +
Idempotency
      +
Inbox
      +
Outbox
      +
Asynchronous Messaging
      +
Concurrency Control
      +
Observability
      +
Automated Tests
```

Essa combinação permite que o sistema seja executado localmente, testado de
forma determinística e posteriormente evoluído para uma infraestrutura AWS
sem necessidade de alterar as regras centrais do domínio.

A principal característica arquitetural é o desacoplamento:

```text
Business Rules
      ≠
Infrastructure
```

Isso permite evoluir banco, mensageria, autenticação, observabilidade e ambiente
de execução mantendo estáveis os casos de uso e o domínio.