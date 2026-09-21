Architecture — Distributed Wager Processing
1. Objetivo

Este documento descreve a arquitetura do Backend Challenge — Distributed Wager Processing.

O sistema foi projetado para processar transações financeiras de apostas com:

consistência financeira;
idempotência;
segurança contra processamento duplicado;
controle de concorrência;
processamento síncrono e assíncrono;
resiliência a falhas;
mensageria;
rastreabilidade;
observabilidade;
testabilidade;
baixo acoplamento com infraestrutura.

A arquitetura utiliza princípios e padrões consolidados de engenharia de software:

Clean Architecture;
Hexagonal Architecture;
Ports and Adapters;
Dependency Inversion;
SOLID;
Domain-Driven Design em escala apropriada;
Transactional Outbox;
Inbox Pattern;
Event-Driven Architecture;
Repository Pattern;
Unit of Work / Transaction Manager;
Dependency Injection;
Composition Root.
2. Princípios arquiteturais

Os principais princípios adotados são:

2.1 Dependency Inversion

As regras de negócio não dependem de infraestrutura.

O domínio não conhece:

PostgreSQL;
SQS;
HTTP;
Keycloak;
OpenTelemetry;
Grafana;
Uber Fx.

As implementações de infraestrutura dependem das abstrações definidas pelas camadas internas.

                 ┌─────────────────┐
                 │     Domain      │
                 └────────┬────────┘
                          │
                          ▼
                 ┌─────────────────┐
                 │   Application  │
                 └────────┬────────┘
                          │
                          ▼
                    Ports / Interfaces
                          ▲
                          │
              ┌───────────┴───────────┐
              │                       │
       PostgreSQL Adapter        SQS Adapter
3. Clean Architecture

A organização conceitual segue:

┌──────────────────────────────────────────────────────────────┐
│                     INFRASTRUCTURE                           │
│                                                              │
│ HTTP │ PostgreSQL │ SQS │ Keycloak │ OTel │ Fx              │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                         ADAPTERS                             │
│                                                              │
│ repositories │ consumers │ publishers │ handlers             │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                       APPLICATION                            │
│                                                              │
│ use cases │ orchestration │ workflows                        │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                          DOMAIN                              │
│                                                              │
│ entities │ value objects │ business rules │ errors           │
│                                                              │
└──────────────────────────────────────────────────────────────┘

A regra fundamental é:

Dependências apontam para dentro.

4. Hexagonal Architecture

A aplicação é tratada como um núcleo cercado por adapters.

                     ┌───────────────┐
                     │    HTTP       │
                     │    Adapter    │
                     └───────┬───────┘
                             │
                             ▼
                 ┌──────────────────────┐
                 │                      │
                 │    APPLICATION       │
                 │                      │
                 │     DOMAIN           │
                 │                      │
                 └──────────────────────┘
                    ▲       ▲       ▲
                    │       │       │
              ┌─────┘       │       └─────┐
              │             │             │
       PostgreSQL         SQS          OIDC
        Adapter          Adapter       Adapter

As portas representam contratos.

Os adapters implementam esses contratos.

5. Camadas
5.1 Domain

Responsável por:

entidades;
value objects;
regras de negócio;
erros de domínio;
invariantes.

Exemplo importante:

Money

é um conceito financeiro e não deve depender de PostgreSQL, HTTP ou SQS.

5.2 Application

Responsável por:

casos de uso;
orquestração;
transações;
coordenação entre repositories;
processamento de mensagens;
workflows.

Componentes principais:

WagerService
ReversalService
WagerConsumer
PendingReferenceWorker
OutboxPublisher
5.3 Ports

As portas definem as dependências que a aplicação precisa.

Exemplos conceituais:

WalletRepository
WagerTransactionRepository
LedgerRepository
IdempotencyRepository
InboxRepository
OutboxRepository
TransactionManager
MessageConsumer
MessagePublisher

Essas interfaces representam contratos, não tecnologias.

5.4 Adapters

Adapters implementam as portas.

Exemplos:

PostgreSQL
SQS
HTTP
Keycloak
OpenTelemetry
6. Composition Root

Uber Fx é utilizado como mecanismo de composição.

A composição ocorre no entrypoint da aplicação:

cmd/app

O papel do Fx é:

construir dependências;
resolver interfaces;
registrar lifecycle;
iniciar workers;
iniciar HTTP;
iniciar observabilidade;
executar shutdown.

O domínio não depende de Fx.

7. Estrutura do projeto

Estrutura conceitual:

cmd/
└── app/

internal/
├── application/
│   ├── wager_service.go
│   ├── wager_consumer.go
│   ├── reversal_service.go
│   └── pending_reference_worker.go
│
├── domain/
│
├── ports/
│
├── adapters/
│   ├── postgres/
│   ├── sqs/
│   └── oidc/
│
├── http/
│
└── observability/

migrations/

infra/
├── grafana/
├── prometheus/
├── otel-collector/
├── keycloak/
└── ministack/

tests/
├── unit/
└── integration/
8. Wager Use Case

O caso de uso principal é o processamento de uma aposta.

Fluxo:

Request
   │
   ▼
Validation
   │
   ▼
Idempotency
   │
   ▼
Wallet
   │
   ▼
Transaction
   │
   ▼
Ledger
   │
   ▼
Outbox
   │
   ▼
Commit

A operação é realizada dentro de uma transação quando necessário para manter consistência.

9. Modelo financeiro

O sistema diferencia:

Wallet Balance

de:

Ledger

A wallet representa o estado atual.

O ledger representa o histórico.

                 WALLET
              balance=90.00
                   │
                   │
                   ▼
                LEDGER
        ┌────────────────────┐
        │ BET -10.00         │
        │ BET -50.00         │
        │ CREDIT +100.00     │
        └────────────────────┘

Essa separação permite auditoria e rastreabilidade.

10. Money como Value Object

Valores financeiros são tratados sem float.

Conceitualmente:

Money
 ├── amount
 └── currency

Invariantes:

valor válido;
precisão definida;
moeda definida;
operações matemáticas exatas;
ausência de floating-point arithmetic.

Isso é especialmente importante em operações financeiras.

11. Wallet concurrency

A wallet é protegida por controle transacional no banco.

Não existe global mutex para proteger todas as wallets.

O objetivo é permitir:

Wallet A → operação concorrente
Wallet B → operação concorrente
Wallet C → operação concorrente

sem serializar artificialmente todo o sistema.

12. Prevenção de double spend

Considere:

Saldo = R$90

Request A = R$50
Request B = R$50

As duas requisições não podem consumir o mesmo saldo.

O banco é utilizado como mecanismo de sincronização da wallet.

Resultado esperado:

Request A → PROCESSING → PROCESSED
Request B → REJECTED

Saldo final = R$40

A consistência é garantida pela transação e pelo controle concorrente da atualização.

13. Por que não utilizar global lock?

Um mutex global seria simples, mas criaria um gargalo:

Wallet A
Wallet B
Wallet C
Wallet D

       ↓

   GLOBAL LOCK

       ↓

uma operação por vez

Isso não escala.

O modelo utilizado permite concorrência no nível do recurso financeiro.

14. Idempotência

A idempotência é persistente.

O sistema utiliza:

Idempotency-Key
+
Payload Hash

Fluxo:

                 Request
                    │
                    ▼
             Idempotency Key
                    │
                    ▼
              Repository
                    │
          ┌─────────┴─────────┐
          │                   │
       inexistente          existente
          │                   │
          ▼                   ▼
       processar        comparar payload
                              │
                       ┌──────┴──────┐
                       │             │
                     igual       diferente
                       │             │
                       ▼             ▼
                    replay          409

Isso impede que a mesma intenção financeira seja executada duas vezes.

15. Inbox Pattern

O Inbox protege o processamento de mensagens.

A mensagem recebida é registrada persistentemente.

Estados conceituais:

RECEIVED
   │
   ▼
PROCESSING
   │
   ▼
PROCESSED

Em caso de mensagem duplicada:

SQS
 │
 ▼
Inbox
 │
 └── já processada
        │
        ▼
     sem efeito financeiro
        │
        ▼
     delete message
16. Separação entre Inbox e Idempotency

Embora os conceitos sejam relacionados, possuem responsabilidades diferentes.

Idempotency

Protege a operação de negócio contra repetição de uma requisição.

Inbox

Protege o consumidor contra redelivery/reprocessamento da mesma mensagem.

HTTP
 │
 ▼
Idempotency
 │
 ▼
WagerService


SQS
 │
 ▼
Inbox
 │
 ▼
WagerService

Essa separação permite tratar corretamente tanto duplicidade HTTP quanto duplicidade de mensagens.

17. Outbox Pattern

Eventos de negócio são persistidos junto com a transação principal.

Exemplo:

BEGIN TRANSACTION

UPDATE wallet

INSERT wager_transaction

INSERT ledger

INSERT outbox_event

COMMIT

Somente depois:

Outbox Publisher
       │
       ▼
      SQS
18. Problema solucionado pelo Outbox

Sem Outbox:

DB COMMIT
   ↓
publish SQS
   ↓
ERROR

Resultado:

Banco atualizado
Mensagem perdida

Com Outbox:

DB COMMIT
   ↓
Outbox persisted
   ↓
Publisher
   ↓
SQS

Se SQS falhar, o evento permanece persistido para retry.

19. Outbox Publisher

O publisher busca eventos pendentes.

Fluxo:

outbox_events
      │
      ▼
 claim
      │
      ▼
 publish
      │
 ┌────┴────┐
 │         │
OK       ERROR
 │         │
 ▼         ▼
published retry

O evento possui informações como:

event_id;
event_type;
aggregate_id;
correlation_id;
causation_id;
occurred_at;
version;
payload;
attempts;
available_at;
claimed_by;
claimed_at;
published_at;
last_error.
20. SQS

O sistema utiliza SQS através de uma abstração.

O application layer não conhece detalhes específicos da AWS.

Application
     │
     ▼
MessagePublisher
MessageConsumer
     │
     ▼
SQS Adapter
     │
     ▼
AWS SQS / MiniStack

Isso permite substituir a implementação por outro broker.

21. FIFO

As filas de transações utilizam FIFO.

Isso é importante para preservar características de ordenação e deduplicação do mecanismo de mensageria.

A consistência final, entretanto, não depende exclusivamente da FIFO.

A aplicação também possui:

Inbox;
Idempotency;
transações;
controle de concorrência.
22. DLQ

Existe uma fila de Dead Letter Queue.

Fluxo:

Message
  │
  ▼
Consumer
  │
  ├── sucesso → delete
  │
  └── erro
        │
        ▼
      retry
        │
        ▼
      retry
        │
        ▼
       DLQ

A DLQ evita que mensagens permanentemente inválidas bloqueiem indefinidamente o processamento normal.

23. Falhas e redelivery

A regra fundamental do consumer é:

Não deletar a mensagem antes de o processamento necessário estar confirmado.

Fluxo de sucesso:

Receive
  ↓
Inbox
  ↓
Business Transaction
  ↓
COMMIT
  ↓
Delete SQS

Fluxo de falha:

Receive
  ↓
Business error
  ↓
No delete
  ↓
SQS redelivery
24. Consumer

O WagerConsumer possui responsabilidade de:

receber mensagem;
validar envelope;
validar payload;
calcular hash;
verificar Inbox;
realizar claim;
chamar caso de uso;
marcar Inbox como processado;
apagar mensagem somente após sucesso.

Ele não contém as regras financeiras fundamentais.

Essas regras permanecem no application/domain layer.

25. Pending Reference Worker

Existe um worker separado para processamento de referências pendentes.

Essa separação evita colocar todos os workflows assíncronos dentro do consumer principal.

Conceitualmente:

Pending References
       │
       ▼
PendingReferenceWorker
       │
       ▼
Business processing
26. HTTP Adapter

O HTTP handler possui responsabilidade de:

receber request;
validar método;
validar headers;
validar JSON;
validar campos;
converter tipos;
chamar application service;
converter resultado para HTTP response.

Ele não deve conter regra financeira.

Exemplo:

HTTP
 │
 ▼
Handler
 │
 ▼
WagerService
27. Autenticação

Keycloak atua como Identity Provider.

Fluxo:

Client
   │
   │ client_credentials
   ▼
Keycloak
   │
   │ access token
   ▼
Client
   │
   │ Bearer token
   ▼
API

A API valida o token antes de executar operações protegidas.

28. OAuth2 Client Credentials

O fluxo utilizado é apropriado para comunicação machine-to-machine.

Não existe login interativo de usuário no fluxo principal do desafio.

Service A
   │
   │ client_id + client_secret
   ▼
Keycloak
   │
   ▼
access_token
   │
   ▼
Service B
29. Observabilidade

Observabilidade é tratada como preocupação transversal.

São utilizados:

traces;
metrics;
logs.

Fluxo:

Application
     │
     ├──────── traces ────────┐
     │                        │
     └──────── metrics ────┐  │
                           │  │
                           ▼  ▼
                       OTel Collector
                         │       │
                         ▼       ▼
                    Prometheus  Jaeger
                         │
                         ▼
                      Grafana
30. OpenTelemetry

A aplicação cria:

TracerProvider
MeterProvider

e envia dados via OTLP para:

otel-collector:4317

O service name utilizado é:

backend-challenge-api
31. Métricas

As métricas de negócio permitem acompanhar:

Wager processing
SQS processing
Inbox processing
HTTP latency
HTTP errors

Exemplos:

wager_transactions_total
wager_transactions_processed_total
wager_processing_duration_seconds

sqs_messages_received_total
sqs_messages_processed_total
sqs_messages_deleted_total
sqs_message_processing_duration_seconds

inbox_messages_received_total
inbox_messages_processed_total
32. Histograms

Latências são expostas como histograms.

Isso permite calcular percentis no Prometheus.

Exemplo:

histogram_quantile(
  0.95,
  sum(
    rate(
      wager_processing_duration_seconds_bucket[5m]
    )
  ) by (le)
)

Isso permite visualizar P95 sem armazenar apenas uma média.

33. Grafana

O dashboard foi construído sobre as métricas realmente exportadas pelo sistema.

Dashboard:

Backend Challenge - Distributed Wager Processing

Painéis:

HTTP Latency
Wager Transactions
Wager Processing Latency
Wager Business Outcomes
SQS Consumer
SQS Processing Latency
Inbox / Idempotent Consumer
34. Persistência

PostgreSQL é utilizado como fonte de consistência transacional.

A aplicação utiliza repositories para abstrair acesso ao banco.

Conceitualmente:

Application
     │
     ▼
Repository Interface
     │
     ▼
PostgreSQL Adapter
     │
     ▼
PostgreSQL

O domínio não conhece SQL.

35. Transaction Manager

Operações que exigem atomicidade utilizam uma abstração de transaction manager.

Isso permite que o caso de uso controle a unidade transacional sem acoplar-se diretamente à implementação específica do banco.

Conceitualmente:

TransactionManager
       │
       ▼
BEGIN
       │
       ├── WalletRepository
       ├── TransactionRepository
       ├── LedgerRepository
       ├── IdempotencyRepository
       └── OutboxRepository
       │
       ▼
COMMIT

Em caso de erro:

ROLLBACK
36. Atomicidade

Uma operação de aposta deve manter consistência entre:

Wallet
Transaction
Ledger
Idempotency
Outbox

Quando necessário, essas alterações fazem parte da mesma unidade transacional.

Isso evita estados parciais.

37. Correlation e causation

Eventos possuem informações para rastreabilidade.

Conceitos:

correlation_id
causation_id

permitindo relacionar:

HTTP request
    ↓
business transaction
    ↓
outbox event
    ↓
SQS message
    ↓
consumer

Isso é importante para diagnóstico em sistemas distribuídos.

38. Resiliência

A arquitetura possui múltiplas camadas de proteção.

HTTP
 │
 ├── authentication
 ├── validation
 └── idempotency
       │
       ▼
Application
 │
 ├── transaction
 ├── concurrency
 └── business rules
       │
       ▼
Outbox
 │
 └── retry
       │
       ▼
SQS
 │
 ├── redelivery
 └── DLQ
39. Estratégia de falha

A aplicação diferencia:

Erros de negócio

Exemplo:

insufficient balance

Resultado:

REJECTED

A operação pode ser registrada como resultado de negócio.

Erros técnicos

Exemplos:

database unavailable
SQS unavailable
unexpected error

Esses erros não devem ser tratados como sucesso.

No processamento assíncrono, a mensagem permanece disponível para retry.

40. Separação entre erro de negócio e erro técnico

Essa distinção é importante.

Insufficient balance
       ↓
Business rejection
       ↓
Transaction REJECTED

Enquanto:

Database error
       ↓
Technical failure
       ↓
Rollback
       ↓
Retry/redelivery

Isso evita transformar falhas de infraestrutura em resultados financeiros falsos.

41. Testabilidade

A arquitetura foi projetada para permitir testes sem depender sempre de infraestrutura real.

As interfaces permitem mocks/fakes.

Exemplo:

WagerService
    │
    ├── MockWalletRepository
    ├── MockTransactionRepository
    ├── MockLedgerRepository
    ├── MockIdempotencyRepository
    └── MockOutboxRepository

Isso permite testes unitários rápidos.

42. Testes de integração

Os testes de integração validam:

PostgreSQL;
transações;
repositories;
concorrência;
idempotência;
wallet.
43. Race Detector

O projeto utiliza:

go test -race ./...

O objetivo é detectar condições de corrida em:

goroutines;
workers;
consumers;
processamento concorrente;
estruturas compartilhadas.

O comando foi executado com sucesso no estado atual do projeto.

44. Testes E2E

A validação E2E cobre o fluxo real:

Keycloak
   ↓
HTTP
   ↓
API
   ↓
PostgreSQL
   ↓
Outbox
   ↓
SQS
   ↓
Consumer
   ↓
Inbox

Também foram validados:

replay idempotente;
conflito de idempotência;
saldo insuficiente;
concorrência;
processamento SQS;
métricas;
Grafana.
45. Estratégia de deploy

A aplicação foi desenhada para funcionar localmente com:

Docker Compose
+
MiniStack

e permitir adaptação para AWS real através de configuração.

O objetivo é evitar dependência estrutural do ambiente local.

46. Local versus AWS

Local:

MiniStack
PostgreSQL
Docker Compose
Keycloak
OTel
Prometheus
Grafana
Jaeger

AWS:

AWS SQS
AWS IAM
AWS RDS / PostgreSQL
AWS ECS
AWS CloudWatch / observability stack

A camada de aplicação não deve precisar conhecer essas diferenças.

47. Configuração por ambiente

As URLs, credenciais e endpoints são configuráveis através de environment variables.

Isso permite:

Local
   ↓
MiniStack

Staging
   ↓
AWS

Production
   ↓
AWS

sem alterar o domínio.

48. Segurança arquitetural

A API utiliza autenticação antes do processamento.

Além disso:

inputs são validados;
JSON desconhecido é rejeitado;
body possui limite;
UUIDs são validados;
moeda é normalizada/validada;
valores monetários não utilizam float.

As credenciais locais são exclusivamente para ambiente de desenvolvimento.

49. Performance

Algumas decisões favorecem escalabilidade:

Concorrência por wallet

Não existe lock global.

Workers

Processamento assíncrono pode escalar horizontalmente.

SQS

Desacopla produtores e consumidores.

Outbox

Desacopla transação financeira da publicação externa.

PostgreSQL

Mantém consistência onde ela é necessária.

50. Escalabilidade horizontal

A arquitetura permite múltiplas instâncias da API.

             Load Balancer
                   │
       ┌───────────┼───────────┐
       ▼           ▼           ▼
     API-1       API-2       API-3
       │           │           │
       └───────────┼───────────┘
                   ▼
              PostgreSQL

Para processamento assíncrono:

             SQS
              │
       ┌──────┼──────┐
       ▼      ▼      ▼
     Worker Worker Worker

A idempotência persistente e o controle transacional são fundamentais para esse modelo.

51. Consistência versus disponibilidade

Operações financeiras priorizam consistência.

Para uma aposta:

consistência financeira
        >
processamento parcial

É preferível rejeitar/reprocessar uma operação do que permitir:

saldo incorreto

ou:

double spend
52. Eventual consistency

Nem todo componente precisa ser síncrono.

O fluxo financeiro principal mantém consistência transacional.

Eventos derivados podem ser publicados de forma assíncrona:

Transaction
    ↓
COMMIT
    ↓
Outbox
    ↓
SQS

Isso permite desacoplar consumidores secundários do request original.

53. Decisões arquiteturais importantes
PostgreSQL para consistência financeira

Escolhido por:

transações;
locking;
constraints;
durabilidade;
suporte a operações concorrentes.
SQS para processamento assíncrono

Escolhido por:

desacoplamento;
retry;
DLQ;
escalabilidade;
modelo assíncrono.
Inbox

Escolhido para garantir processamento idempotente de mensagens.

Outbox

Escolhido para evitar inconsistência entre banco e broker.

Keycloak

Escolhido para separar autenticação da aplicação.

OpenTelemetry

Escolhido para observabilidade vendor-neutral.

54. Anti-patterns evitados

A arquitetura evita:

Global lock
mutex global

porque limita concorrência.

Float para dinheiro

Porque pode gerar erros de precisão.

Publish-before-commit

Porque pode publicar eventos de operações que posteriormente sofreriam rollback.

Delete-before-commit

Porque pode perder mensagens.

Idempotência apenas em memória

Porque reinício destruiria o estado.

Regra de negócio no HTTP handler

Porque acoplaria domínio ao transporte.

Regra de negócio no SQS consumer

Porque acoplaria negócio à infraestrutura de mensageria.

Dependência direta de AWS no domínio

Porque dificultaria testes e substituição de infraestrutura.

55. Lifecycle da aplicação

O startup é controlado pelo Fx.

Fluxo conceitual:

Application start
      │
      ▼
Configuration
      │
      ▼
Database
      │
      ▼
Migrations
      │
      ▼
Observability
      │
      ▼
HTTP
      │
      ├── Wager Consumer
      │
      ├── Outbox Publisher
      │
      └── Pending Reference Worker

Shutdown:

SIGTERM
   ↓
stop accepting new work
   ↓
stop workers
   ↓
flush telemetry
   ↓
close resources
   ↓
exit
56. Observabilidade como arquitetura transversal

Observabilidade não é responsabilidade exclusiva de um componente.

Ela atravessa:

HTTP
Application
Database
Messaging
Workers

Isso permite acompanhar o ciclo completo da operação.

57. Diagnóstico operacional

Para investigar uma operação:

1. Correlation ID
       ↓
2. HTTP trace
       ↓
3. Wager processing
       ↓
4. Database transaction
       ↓
5. Outbox
       ↓
6. SQS
       ↓
7. Consumer
       ↓
8. Inbox

As métricas mostram comportamento agregado.

Os traces ajudam a investigar uma execução específica.

Os logs fornecem detalhes operacionais.

58. Modelo de observabilidade
                ┌─────────────┐
                │ Application │
                └──────┬──────┘
                       │
          ┌────────────┼────────────┐
          │            │            │
          ▼            ▼            ▼
       Metrics       Traces        Logs
          │            │            │
          ▼            ▼            ▼
    Prometheus       Jaeger      Container Logs
          │
          ▼
       Grafana
59. Critérios de qualidade

A implementação foi validada em diferentes dimensões:

Correctness
    ↓
Transactions
    ↓
Concurrency
    ↓
Idempotency
    ↓
Messaging
    ↓
Security
    ↓
Observability
    ↓
Testing
60. Estado atual

O projeto encontra-se funcionalmente concluído.

Foram validados:

✓ Go 1.25
✓ Uber Fx
✓ HTTP
✓ Keycloak / OIDC
✓ PostgreSQL
✓ Money
✓ Wallet
✓ Concurrency
✓ Double-spend protection
✓ Idempotency
✓ Inbox
✓ Ledger
✓ Outbox
✓ SQS
✓ FIFO
✓ Consumer
✓ DLQ
✓ Redelivery
✓ Pending Reference Worker
✓ OpenTelemetry
✓ Prometheus
✓ Grafana
✓ Jaeger
✓ Docker Compose
✓ Unit tests
✓ Integration tests
✓ E2E tests
✓ Race detector
61. Validação arquitetural final

O sistema atende aos principais princípios definidos para o desafio:

Domain independent from infrastructure
                ✓

Application independent from HTTP
                ✓

Application independent from SQS
                ✓

Persistence behind interfaces
                ✓

Messaging behind interfaces
                ✓

Idempotent financial processing
                ✓

Transactional consistency
                ✓

Concurrency-safe wallet
                ✓

Observable distributed processing
                ✓

Testable application layer
                ✓
62. Resumo arquitetural

A solução pode ser resumida como:

                         CLIENT
                           │
                           ▼
                    ┌──────────────┐
                    │    HTTP      │
                    │   Adapter    │
                    └──────┬───────┘
                           │
                           ▼
                  ┌──────────────────┐
                  │  APPLICATION     │
                  │                  │
                  │ WagerService     │
                  │ ReversalService  │
                  │ WagerConsumer    │
                  │ Workers          │
                  └────────┬─────────┘
                           │
                    ┌──────┴──────┐
                    │    PORTS     │
                    └──────┬───────┘
                           │
             ┌─────────────┼─────────────┐
             │             │             │
             ▼             ▼             ▼
        PostgreSQL        SQS         Keycloak
             │             │
             │             │
             └──────┬──────┘
                    │
                    ▼
             EVENT PROCESSING
                    │
             ┌──────┴──────┐
             ▼             ▼
          Inbox          Outbox
             │             │
             └──────┬──────┘
                    │
                    ▼
              OBSERVABILITY
                    │
          ┌─────────┼─────────┐
          ▼         ▼         ▼
      Prometheus  Jaeger    Grafana
63. Conclusão

A arquitetura foi construída para manter as regras financeiras e os casos de uso independentes dos detalhes de infraestrutura.

Os principais mecanismos de confiabilidade são:

Money exato
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
SQS Retry
     +
DLQ

Os principais mecanismos de observabilidade são:

OpenTelemetry
     +
OTel Collector
     +
Prometheus
     +
Grafana
     +
Jaeger

Os principais mecanismos de desacoplamento são:

Ports
     +
Adapters
     +
Dependency Inversion
     +
Composition Root

Com isso, o sistema consegue processar operações financeiras de forma consistente, suportar processamento síncrono e assíncrono, tolerar redelivery, evitar duplicidade e double spend, além de oferecer observabilidade suficiente para operação e diagnóstico.

O projeto encontra-se em estado de implementação funcional concluída e validada.