Backend Challenge — Distributed Wager Processing

Backend para processamento distribuído de transações de apostas, desenvolvido em Go, com foco em consistência financeira, idempotência, concorrência, processamento assíncrono, resiliência, observabilidade e desacoplamento arquitetural.

1. Visão geral

O sistema recebe transações de apostas por HTTP e também suporta processamento assíncrono através de SQS.

A arquitetura separa claramente:

domínio;
casos de uso;
portas/interfaces;
adapters de infraestrutura;
transporte HTTP;
mensageria;
persistência;
autenticação;
observabilidade;
composição da aplicação.

O objetivo é permitir evolução e substituição de componentes de infraestrutura sem alterar as regras de negócio.

Por exemplo:

SQS
  ↓
MessageConsumer
  ↓
Application

pode futuramente ser substituído por:

RabbitMQ
  ↓
MessageConsumer
  ↓
Application

sem alterar o domínio ou os casos de uso.

2. Principais requisitos atendidos

O projeto contempla:

Go 1.25
Uber Fx
HTTP REST
Keycloak / OpenID Connect
OAuth 2.0 Client Credentials
PostgreSQL
AWS SQS
filas FIFO
DLQ
MiniStack para AWS local
Docker Compose
migrations
Money com precisão exata
idempotência persistente
Inbox Pattern
Outbox Pattern
Ledger append-only
controle de concorrência da wallet
prevenção de double spend
processamento assíncrono
retry/redelivery
pending reference worker
OpenTelemetry
Prometheus
Grafana
Jaeger
testes unitários
testes de integração
testes de concorrência
go test -race ./...
validação E2E.
3. Arquitetura

A aplicação segue uma arquitetura em camadas baseada em Ports and Adapters / Hexagonal Architecture, combinada com princípios de Clean Architecture.

                         ┌──────────────────────┐
                         │       Clients        │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │     HTTP Adapter     │
                         │   REST / OIDC Auth   │
                         └──────────┬───────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────┐
│                    APPLICATION LAYER                     │
│                                                          │
│  WagerService                                            │
│  ReversalService                                         │
│  WagerConsumer                                           │
│  PendingReferenceWorker                                  │
│                                                          │
│  Use cases + orchestration                               │
└───────────────┬──────────────────────────┬───────────────┘
                │                          │
                ▼                          ▼
       ┌─────────────────┐       ┌─────────────────────┐
       │     DOMAIN      │       │       PORTS         │
       │                 │       │                     │
       │ Money           │       │ WalletRepository    │
       │ Wallet          │       │ TransactionRepo     │
       │ Wager           │       │ LedgerRepository    │
       │ Ledger          │       │ InboxRepository     │
       │ Errors          │       │ OutboxRepository    │
       │ Rules           │       │ MessageConsumer     │
       └─────────────────┘       │ MessagePublisher    │
                                 │ TransactionManager  │
                                 └──────────┬──────────┘
                                            │
                         ┌──────────────────┼──────────────────┐
                         │                  │                  │
                         ▼                  ▼                  ▼
                 ┌──────────────┐   ┌──────────────┐   ┌──────────────┐
                 │  PostgreSQL  │   │     SQS      │   │   Keycloak   │
                 │    Adapter   │   │    Adapter   │   │    Adapter   │
                 └──────────────┘   └──────────────┘   └──────────────┘


                    OBSERVABILITY CROSS-CUTTING

Application
    │
    ├── OpenTelemetry Traces ──► OTel Collector ──► Jaeger
    │
    └── OpenTelemetry Metrics ─► OTel Collector ──► Prometheus
                                                       │
                                                       ▼
                                                    Grafana
4. Princípios arquiteturais
4.1 Dependency Inversion

O domínio não conhece:

PostgreSQL;
SQS;
HTTP;
Keycloak;
Uber Fx;
OpenTelemetry.

Esses componentes dependem das abstrações definidas pela aplicação.

4.2 Ports and Adapters

Interfaces definem as fronteiras entre aplicação e infraestrutura.

Exemplos conceituais:

Application
    │
    ├── WalletRepository
    ├── WagerTransactionRepository
    ├── LedgerRepository
    ├── IdempotencyRepository
    ├── InboxRepository
    ├── OutboxRepository
    ├── TransactionManager
    ├── MessageConsumer
    └── MessagePublisher

Implementações ficam nos adapters.

4.3 Single Responsibility

Cada componente possui responsabilidade específica.

Exemplos:

WagerService
    → regras/orquestração da aposta

WagerConsumer
    → consumo e lifecycle da mensagem

OutboxPublisher
    → publicação dos eventos persistidos

PendingReferenceWorker
    → processamento de referências pendentes

HTTP Handler
    → transporte, parsing e validação de entrada

PostgreSQL repositories
    → persistência

SQS adapters
    → mensageria
4.4 Composition Root

Uber Fx é utilizado para composição da aplicação.

A infraestrutura é montada em:

cmd/app

O domínio não depende de Fx.

5. Fluxo síncrono

Fluxo principal:

Client
  │
  │ POST /wagering/transactions
  ▼
HTTP Handler
  │
  ├── Authorization
  ├── Idempotency-Key
  ├── JSON validation
  ├── UUID validation
  ├── Currency validation
  └── Money parsing
  │
  ▼
WagerService
  │
  ├── Idempotency
  ├── Wallet
  ├── Transaction
  ├── Ledger
  └── Outbox
  │
  ▼
PostgreSQL Transaction
  │
  └── COMMIT
  │
  ▼
HTTP Response

A operação financeira e seus registros relacionados são persistidos de forma transacional.

6. Fluxo assíncrono

O processamento assíncrono utiliza SQS.

Producer
   │
   ▼
wager-transactions.fifo
   │
   ▼
WagerConsumer
   │
   ▼
Inbox
   │
   ├── duplicate
   │      └── delete/replay
   │
   └── new message
          │
          ▼
       Claim
          │
          ▼
     WagerService
          │
          ▼
     DB Transaction
          │
          ├── Wallet
          ├── Transaction
          ├── Ledger
          ├── Idempotency
          └── Inbox
          │
          ▼
        COMMIT
          │
          ▼
     Delete SQS

Em caso de erro antes do commit:

SQS message
    ↓
processing error
    ↓
message NOT deleted
    ↓
redelivery
    ↓
retry
7. Idempotência

A idempotência é persistente.

A chave é associada ao payload recebido.

O sistema calcula um hash do payload.

Fluxo:

Idempotency-Key
       │
       ▼
Existe?
 ┌─────┴─────┐
 │           │
 NÃO         SIM
 │           │
 ▼           ▼
Processa   Compara payload hash
             │
       ┌─────┴─────┐
       │           │
      igual      diferente
       │           │
       ▼           ▼
     Replay       409

Isso impede:

processamento duplicado;
débito duplicado;
inconsistência financeira;
reutilização indevida de uma chave para outro payload.
8. Money

Valores monetários não utilizam float.

Exemplo:

10.00
50.00
100.00

são tratados com precisão exata.

Isso evita erros clássicos de representação binária de valores financeiros.

9. Wallet e concorrência

A atualização da wallet ocorre de forma transacional e concorrente-safe.

O sistema não utiliza global mutex para controlar saldo.

A consistência é garantida no banco.

Exemplo validado:

Saldo inicial: R$90,00

Aposta A: R$50,00
Aposta B: R$50,00

Resultado:

A → PROCESSED
B → REJECTED

Saldo final: R$40,00

Isso comprova que duas requisições concorrentes não conseguem consumir o mesmo saldo.

10. Ledger

O ledger é append-only.

As operações financeiras geram registros que permitem rastrear os movimentos.

O ledger não deve ser tratado como um simples campo de saldo.

A wallet representa o estado atual.

O ledger representa o histórico financeiro.

11. Inbox Pattern

O Inbox Pattern garante idempotência no consumo de mensagens.

A mensagem recebida é identificada através do payload/hash e persistida antes do processamento.

Estados conceituais:

RECEIVED
   ↓
PROCESSING
   ↓
PROCESSED

Em caso de erro:

PROCESSING
   ↓
erro
   ↓
mensagem permanece na fila
12. Outbox Pattern

Eventos de negócio não são publicados diretamente antes do commit do banco.

Eles são persistidos no Outbox dentro da mesma transação.

Business operation
      │
      ├── Database changes
      │
      └── Outbox event
             │
             ▼
           COMMIT
             │
             ▼
       Outbox Publisher
             │
             ▼
           SQS

Isso evita o problema clássico:

Banco commitou
MAS
mensagem não foi publicada
13. Autenticação

A API utiliza Keycloak como Identity Provider.

Fluxo:

Client
   │
   │ client_credentials
   ▼
Keycloak
   │
   │ access_token
   ▼
Client
   │
   │ Authorization: Bearer <token>
   ▼
API
   │
   ▼
Token validation
   │
   ▼
Endpoint

Realm:

backend-challenge

Client:

backend-api
14. Observabilidade

A aplicação utiliza OpenTelemetry.

API
 │
 ├── traces
 │
 └── metrics
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
15. Métricas principais

Métricas HTTP:

http_server_request_duration_seconds
http_server_request_body_size_bytes
http_server_response_body_size_bytes

Métricas Wager:

wager_processing_duration_seconds
wager_transactions_processed_total
wager_transactions_total

Métricas SQS:

sqs_message_processing_duration_seconds
sqs_messages_received_total
sqs_messages_processed_total
sqs_messages_deleted_total

Métricas Inbox:

inbox_messages_received_total
inbox_messages_processed_total
16. Grafana

Dashboard:

Backend Challenge - Distributed Wager Processing

URL local:

http://localhost:3000/dashboards/f/dfyx604lqnmkgf/backend-challenge

Painéis:

HTTP Latency
  P95
  P50

Wager Transactions
  Total
  Processed

Wager Processing Latency
  P95
  P50

Wager Business Outcomes
  Processed

SQS Consumer
  Received
  Processed
  Deleted

SQS Processing Latency
  P95

Inbox / Idempotent Consumer
  Received
  Processed
17. Estrutura de infraestrutura
Docker Compose
│
├── postgres
│
├── ministack
│
├── sqs-init
│
├── keycloak
│
├── jaeger
│
├── otel-collector
│
├── prometheus
│
├── grafana
│
└── api
18. Pré-requisitos

Para execução local:

Docker
Docker Compose
Go 1.25
Git
curl
jq

O projeto foi validado em ambiente Docker Desktop/Linux.

19. Subindo o ambiente

Na raiz:

cd /media/data/_GIT_/backend-challenge-go

Subir infraestrutura:

docker compose up -d

Verificar containers:

docker compose ps

Esperado:

postgres
ministack
keycloak
jaeger
otel-collector
prometheus
grafana
api
20. Logs da aplicação
docker logs -f backend-challenge-api

Na inicialização devem aparecer componentes como:

database migrations completed
OpenTelemetry initialized
outbox publisher started
wager consumer started
pending reference worker started
HTTP server listening on :8080
21. Health check

Sem autenticação:

curl -i http://localhost:8080/health/ready

Deve retornar:

401 Unauthorized

Isso confirma que o endpoint protegido está exigindo autenticação.

22. Obtendo token Keycloak
TOKEN=$(curl -s \
  -X POST \
  "http://localhost:8081/realms/backend-challenge/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=client_credentials" \
  -d "client_id=backend-api" \
  -d "client_secret=backend-api-secret" \
  | jq -r '.access_token')

Validar:

echo "${#TOKEN}"

O resultado deve ser maior que zero.

23. Health check autenticado
curl -i \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/health/ready

Resultado esperado:

{
  "status": "UP"
}
24. Testes automatizados

Executar toda a suíte:

go test ./...

Executar com race detector:

go test -race ./...

O -race é especialmente importante neste desafio porque existem:

processamento concorrente;
workers;
consumidores;
acesso concorrente a wallet;
publicação assíncrona;
operações transacionais.
25. Testes por camada
25.1 Testes unitários
go test ./tests/unit/...

Objetivo:

domínio;
regras;
serviços;
validações;
idempotência;
comportamento de negócio.
25.2 Testes de integração PostgreSQL
go test ./tests/integration/postgres/...

Objetivo:

repositories;
transações;
concorrência;
wallet;
idempotência;
persistência.
25.3 Race detector
go test -race ./...

Objetivo:

detectar data races em toda a aplicação.

26. Teste manual de aposta HTTP

Primeiro obtenha:

TOKEN=...

Crie/obtenha uma wallet de teste conforme os dados de ambiente do desafio.

Depois:

curl -i \
  -X POST \
  http://localhost:8080/wagering/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: e2e-test-001" \
  -d '{
    "externalTransactionId": "e2e-ext-001",
    "providerId": "e2e-provider",
    "walletId": "<WALLET_UUID>",
    "playerId": "<PLAYER_UUID>",
    "roundId": "e2e-round-001",
    "gameId": "e2e-game-001",
    "amount": "10.00",
    "currency": "BRL"
  }'

Resultado esperado:

{
  "transactionId": "...",
  "status": "PROCESSED",
  "balance": "...",
  "currency": "BRL",
  "idempotentReplay": false
}
27. Teste de idempotência

Execute exatamente a mesma requisição novamente, mantendo:

Idempotency-Key
payload

iguais.

Esperado:

{
  "idempotentReplay": true
}

A transação não deve ser criada novamente.

28. Teste de conflito de idempotência

Mantenha a mesma:

Idempotency-Key

mas altere:

amount

ou outro campo do payload.

Esperado:

HTTP 409

Com mensagem equivalente a:

idempotency key was already used with a different payload

O saldo não deve ser alterado pela segunda requisição.

29. Teste de saldo insuficiente

Utilize uma wallet com saldo menor que o valor da aposta.

Esperado:

HTTP 409

e:

insufficient balance

A transação deve ser registrada como rejeitada, sem lançamento financeiro indevido.

30. Teste de concorrência

O cenário crítico é:

saldo = R$90

aposta A = R$50
aposta B = R$50

As duas operações devem ser executadas simultaneamente.

Resultado esperado:

uma PROCESSADA
uma REJECTED

saldo final = R$40

Executar a suíte de concorrência:

go test -race ./...
31. Teste do Outbox

Depois de processar uma aposta:

consultar a tabela:

outbox_events

Verificar:

event_id;
event_type;
aggregate_id;
correlation_id;
payload;
attempts;
published_at;
last_error.

Um evento publicado corretamente deve possuir:

published_at != NULL
last_error = NULL
32. Teste do Inbox

Enviar uma mensagem válida para a fila:

wager-transactions.fifo

Verificar:

Inbox RECEIVED
Inbox PROCESSED

Depois verificar que a mensagem foi removida da fila.

A segunda entrega da mesma mensagem não deve gerar novo efeito financeiro.

33. Teste de processamento SQS

Verificar logs:

docker logs backend-challenge-api | grep -Ei \
'inbox|sqs|wager|consumer'

Verificar métricas:

curl -s http://localhost:8889/metrics | grep -Ei \
'wager|sqs|inbox'
34. Teste de Outbox Publisher

Verificar:

docker logs backend-challenge-api | grep -Ei \
'outbox|published'

Eventos publicados devem apresentar logs equivalentes a:

outbox event ... published successfully
35. Verificação das métricas

Collector:

curl -s http://localhost:8889/metrics

Filtrar:

curl -s http://localhost:8889/metrics | grep '^# HELP' | grep -Ei \
'outbox|dlq|wallet|wager|http|sqs|inbox'

Verificar métricas Wager:

curl -s http://localhost:8889/metrics | grep '^wager_'

Verificar SQS:

curl -s http://localhost:8889/metrics | grep '^sqs_'

Verificar Inbox:

curl -s http://localhost:8889/metrics | grep '^inbox_'
36. Prometheus

Abrir:

http://localhost:9090

Exemplos de consultas:

sum(rate(wager_transactions_total[5m]))
sum(rate(wager_transactions_total{status="processed"}[5m]))
sum(rate(sqs_messages_received_total[5m]))
sum(rate(sqs_messages_processed_total[5m]))
sum(rate(inbox_messages_received_total[5m]))
37. Grafana

Abrir:

http://localhost:3000

Dashboard:

Backend Challenge - Distributed Wager Processing

URL:

http://localhost:3000/dashboards/f/dfyx604lqnmkgf/backend-challenge

Após gerar tráfego, observar:

HTTP
P95;
P50.
Wager
total;
processadas;
taxa de processamento;
latência.
SQS
mensagens recebidas;
processadas;
deletadas;
latência.
Inbox
mensagens recebidas;
processadas.
38. Jaeger

Abrir:

http://localhost:16686

Selecionar:

backend-challenge-api

Usar para investigar:

latência;
chamadas HTTP;
operações de negócio;
comportamento distribuído;
correlação temporal de operações.
39. Verificação completa de observabilidade

Fluxo recomendado:

1. Fazer uma requisição HTTP
        ↓
2. API processa
        ↓
3. Métricas são geradas
        ↓
4. OTel Collector recebe
        ↓
5. Prometheus armazena
        ↓
6. Grafana apresenta

Para SQS:

1. Mensagem entra na fila
        ↓
2. Consumer recebe
        ↓
3. Inbox é atualizado
        ↓
4. Wager é processado
        ↓
5. Mensagem é deletada
        ↓
6. Métricas são exportadas
        ↓
7. Grafana apresenta
40. Troubleshooting
Containers
docker compose ps
Logs API
docker logs -f backend-challenge-api
Logs Grafana
docker logs -f backend-challenge-grafana
Logs Prometheus
docker logs -f backend-challenge-prometheus
Logs OTel
docker logs -f backend-challenge-otel-collector
Reiniciar ambiente
docker compose restart
Derrubar ambiente
docker compose down

Para remover também volumes:

docker compose down -v

Atenção: down -v remove dados persistidos do ambiente local, incluindo PostgreSQL, Prometheus, Jaeger, Grafana e estado do MiniStack.

41. Verificação de migrations

As migrations são executadas automaticamente durante o startup da API.

No log:

database migrations completed

deve aparecer durante a inicialização.

42. Segurança

O sistema utiliza:

OIDC;
Bearer Token;
Keycloak;
client credentials;
validação de autenticação antes dos endpoints protegidos;
validação de entrada;
limite de leitura do body;
rejeição de campos JSON desconhecidos;
validação de UUID;
validação de moeda;
validação de Money.

Segredos usados no ambiente local são apenas credenciais de desenvolvimento.

Em produção, devem ser substituídos por mecanismos apropriados de secret management.

43. Configuração

A infraestrutura é configurável por environment variables.

Principais:

HTTP_PORT
DATABASE_URL

AWS_REGION
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
AWS_ENDPOINT_URL

SQS_QUEUE_URL
SQS_EVENTS_QUEUE_URL
SQS_DLQ_URL

OIDC_ISSUER_URL
OIDC_CLIENT_ID
OIDC_AUDIENCE

OTEL_SERVICE_NAME
OTEL_EXPORTER_OTLP_ENDPOINT
OTEL_EXPORTER_OTLP_PROTOCOL
OTEL_RESOURCE_ATTRIBUTES

Isso permite executar localmente com MiniStack e adaptar a configuração para ambientes AWS.

44. Desenvolvimento

Formatar código:

gofmt -w .

Executar testes:

go test ./...

Executar race detector:

go test -race ./...

Verificar compilação:

go build ./...
45. Critérios de aceitação validados
Financeiro

Money exato

sem float

saldo consistente

insufficient balance

ledger

transações atômicas

Concorrência

wallet concorrente

prevenção de double spend

sem global lock

race detector

Idempotência

persistent idempotency

replay

payload hash

conflict detection

Inbox

Mensageria

SQS

FIFO

consumer

Outbox

redelivery

DLQ

delete após sucesso

Segurança

Keycloak

OIDC

client credentials

Bearer token

endpoint protegido

Observabilidade

OpenTelemetry

Collector

Prometheus

Grafana

Jaeger

métricas reais

dashboard

Qualidade

testes unitários

integração

concorrência

E2E

race detector

46. Estado atual

O projeto encontra-se funcionalmente concluído.

Foram validados:

HTTP
OIDC
PostgreSQL
Wallet
Money
Idempotência
Concorrência
Ledger
Inbox
Outbox
SQS
Consumer
Pending Reference Worker
OpenTelemetry
Prometheus
Grafana
Jaeger
Docker Compose
go test
go test -race
E2E

O dashboard do Grafana também foi validado e está carregando corretamente.

47. Próximos passos

O próximo passo natural é realizar apenas uma revisão final de entrega:

conferir git status;
conferir arquivos modificados;
executar gofmt;
executar go test ./...;
executar go test -race ./...;
revisar README;
revisar ARCHITECTURE.md;
criar commit;
fazer push.

Não é recomendado introduzir novas mudanças arquiteturais depois desta etapa sem uma nova exigência funcional.

48. Comandos finais de validação
git status
gofmt -w .
go test ./...
go test -race ./...
docker compose ps
docker logs --tail=100 backend-challenge-api
docker logs --tail=100 backend-challenge-grafana
49. Conclusão

Este projeto implementa um backend distribuído para processamento de apostas com consistência financeira, idempotência persistente, processamento assíncrono, concorrência segura, mensageria, autenticação e observabilidade.

A arquitetura foi desenhada para manter o domínio independente das tecnologias de infraestrutura, permitindo evolução e substituição de adapters sem alteração das regras de negócio.

O ambiente local reproduz os principais componentes necessários para validação da solução:

Go
+
PostgreSQL
+
SQS/MiniStack
+
Keycloak
+
OpenTelemetry
+
Prometheus
+
Grafana
+
Jaeger

O estado atual foi validado por testes automatizados, testes de concorrência, testes E2E e validação operacional dos componentes de observabilidade.