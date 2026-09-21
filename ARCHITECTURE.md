# ARCHITECTURE.md

# Backend Challenge — Distributed Wager Processing

## 1. Visão Geral

Este projeto implementa uma plataforma de processamento distribuído de apostas com foco em consistência financeira, idempotência, processamento assíncrono, concorrência segura, observabilidade e separação entre domínio e infraestrutura.

A arquitetura foi construída utilizando princípios de:

- **Clean Architecture** — organiza o sistema em camadas com dependências direcionadas para o domínio.
- **Hexagonal Architecture** — separa o núcleo da aplicação das tecnologias externas através de portas e adapters.
- **Ports and Adapters** — permite substituir implementações de infraestrutura sem alterar os casos de uso.
- **SOLID** — promove responsabilidades bem definidas, baixo acoplamento e facilidade de evolução.
- **DDD Lite** — utiliza conceitos de domínio relevantes sem adicionar complexidade desnecessária.
- **Dependency Inversion** — faz aplicação e domínio dependerem de abstrações.
- **Transactional Integrity** — garante atomicidade das operações financeiras críticas.
- **Inbox Pattern** — controla o processamento persistente de mensagens recebidas.
- **Outbox Pattern** — garante persistência confiável dos eventos antes da publicação externa.
- **Idempotent Consumer** — evita efeitos financeiros duplicados em redelivery.
- **Event-Driven Architecture** — utiliza eventos e mensageria para desacoplar processamento síncrono e assíncrono.

O objetivo principal é permitir que as regras de negócio permaneçam independentes de HTTP, PostgreSQL, SQS, Uber Fx, OpenTelemetry ou qualquer outro mecanismo de infraestrutura.

---

# 2. Objetivos Arquiteturais

Os principais objetivos são:

1. **Garantir consistência financeira** — operações de saldo, aposta e ledger devem permanecer atomicamente consistentes.
2. **Impedir saldo negativo** — uma aposta não pode consumir recursos financeiros inexistentes.
3. **Impedir double spend** — duas operações concorrentes não podem consumir o mesmo saldo.
4. **Garantir idempotência de requisições** — retries HTTP não devem produzir novos efeitos financeiros.
5. **Garantir idempotência no processamento assíncrono** — redelivery de mensagens não deve duplicar efeitos.
6. **Permitir redelivery de mensagens** — falhas durante o processamento não devem causar perda da mensagem.
7. **Permitir recuperação após falhas** — operações pendentes devem poder ser processadas novamente.
8. **Garantir persistência transacional** — alterações financeiras relacionadas devem ser confirmadas ou revertidas em conjunto.
9. **Evitar locks globais na aplicação** — a coordenação financeira deve ocorrer de forma compatível com múltiplas instâncias.
10. **Permitir processamento concorrente seguro** — diferentes operações podem ser processadas simultaneamente sem comprometer a consistência.
11. **Separar domínio de infraestrutura** — regras de negócio não conhecem tecnologias externas.
12. **Permitir substituição de adapters** — implementações como SQS e PostgreSQL podem ser substituídas através de interfaces.
13. **Permitir execução local e futura execução em AWS** — o mesmo núcleo pode operar em ambientes diferentes através de configuração.
14. **Garantir observabilidade** — métricas e traces permitem acompanhar comportamento técnico e de negócio.
15. **Facilitar testes unitários e de integração** — interfaces e separação de responsabilidades reduzem dependências desnecessárias.
16. **Permitir escala horizontal** — múltiplas instâncias podem processar requisições e mensagens simultaneamente.

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
                         │ Consumer             │
                         │ Pending Worker       │
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

O fluxo principal separa claramente:

- entrada HTTP;
- autenticação;
- casos de uso;
- regras de domínio;
- persistência;
- mensageria;
- observabilidade.

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

Responsabilidades principais:

- `cmd/app` — composição e inicialização da aplicação.
- `application` — casos de uso e orquestração das operações.
- `domain` — regras e conceitos financeiros.
- `ports` — contratos entre aplicação e infraestrutura.
- `adapters` — implementações concretas dos contratos.
- `http` — adaptação das requisições HTTP para os casos de uso.
- `config` — configuração externa da aplicação.
- `observability` — instrumentação de métricas e traces.
- `migrations` — evolução controlada do schema.
- `tests` — validação unitária, integração e ponta a ponta.
- `infra` — configuração dos componentes de infraestrutura local.

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

- PostgreSQL — persistência é detalhe externo.
- SQS — mensageria é detalhe externo.
- HTTP — transporte é detalhe externo.
- JSON — serialização pertence à interface.
- Docker — ambiente de execução é externo.
- Uber Fx — composição pertence à infraestrutura.
- OpenTelemetry — instrumentação é externa ao domínio.
- AWS — cloud provider não deve influenciar as regras financeiras.
- MiniStack — ferramenta de desenvolvimento local não pertence ao núcleo.

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

A porta define o contrato; o adapter define como esse contrato é implementado.

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

- **testes unitários** — repositories podem ser substituídos por mocks ou fakes;
- **mocks** — dependências podem ser controladas durante os testes;
- **troca de banco** — a aplicação não depende diretamente do PostgreSQL;
- **troca de infraestrutura** — adapters podem ser substituídos;
- **evolução arquitetural** — novas implementações podem ser adicionadas sem alterar o domínio.

---

# 8. Domain Layer

O domínio concentra regras financeiras.

Entre os principais conceitos:

- **Wallet** — representa a carteira e o saldo financeiro atual.
- **Money** — representa valores monetários com precisão decimal.
- **Wager Transaction** — representa uma operação de aposta e seu ciclo de vida.
- **Ledger Entry** — representa um lançamento financeiro persistente e imutável.
- **Transaction Status** — representa os estados possíveis de uma aposta.
- **Business Errors** — representam condições de negócio, como saldo insuficiente.

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

1. **Possuir valor válido** — o valor deve respeitar as regras de precisão monetária.
2. **Possuir moeda válida** — a operação deve identificar corretamente a moeda.
3. **Possuir wallet existente** — a carteira precisa estar disponível para processamento.
4. **Possuir saldo suficiente** — o débito não pode gerar saldo negativo.
5. **Debitar a wallet** — o saldo deve refletir a operação aprovada.
6. **Registrar a transação** — a operação deve possuir histórico próprio.
7. **Registrar o ledger** — a movimentação financeira deve ser auditável.
8. **Produzir evento para processamento assíncrono quando aplicável** — eventos devem ser persistidos através da Outbox.

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

Em vez disso, a consistência é garantida no banco utilizando transações e mecanismos de concorrência apropriados.

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

O Transaction Manager concentra a fronteira transacional sem expor detalhes do banco aos casos de uso.

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

A atomicidade garante que as alterações críticas sejam confirmadas ou revertidas em conjunto.

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

A idempotência protege principalmente contra retries do cliente, timeouts e repetição de requisições.

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

O hash funciona como uma segunda camada de proteção da idempotência.

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

Isso permite tratar redelivery sem repetir efeitos financeiros.

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

Os estados permitem identificar em qual etapa o processamento da mensagem se encontra.

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

Eles podem coexistir porque protegem fronteiras diferentes.

---

# 21. Outbox Pattern

Eventos de negócio são persistidos na Outbox dentro da mesma transação da operação financeira.

```text
BEGIN

Wallet

Transaction

Ledger

Outbox

COMMIT
```

Somente depois do commit o publisher envia o evento para SQS.

A Outbox reduz o risco de inconsistência entre banco e mensageria.

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

O Publisher permite separar o commit financeiro da disponibilidade momentânea da mensageria.

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

O objetivo é manter o mecanismo de mensageria substituível.

---

# 25. FIFO Queue

A fila utiliza características de FIFO quando necessárias.

Isso ajuda a manter ordenação dentro do grupo de mensagens.

O agrupamento pode utilizar uma chave relacionada ao domínio, como wallet ou entidade financeira.

A aplicação, entretanto, não deve depender exclusivamente da ordenação da fila para garantir consistência.

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

Isso evita confirmar uma mensagem antes de seu efeito estar persistido.

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

O objetivo é preservar a possibilidade de recuperação sem perda da operação.

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

A DLQ impede que uma mensagem permanentemente inválida bloqueie indefinidamente o processamento normal.

Também permite investigação posterior das mensagens problemáticas.

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

Essa abordagem é importante em sistemas distribuídos porque entrega duplicada deve ser considerada uma possibilidade normal.

---

# 31. Pending Reference Worker

O projeto possui worker para referências pendentes.

A ideia é tratar situações onde uma entidade necessária para completar o processamento ainda não esteja disponível.

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

Esse mecanismo permite trabalhar com consistência eventual sem bloquear indefinidamente o fluxo principal.

---

# 32. HTTP Adapter

O adapter HTTP é responsável por:

- **Parsing** — interpretar a requisição recebida.
- **Validação de entrada** — verificar campos obrigatórios e formatos.
- **Autenticação** — validar o acesso do cliente.
- **Headers** — processar informações como Authorization e Idempotency-Key.
- **JSON** — converter requests e responses.
- **Status HTTP** — mapear resultados para códigos HTTP.
- **Serialização da resposta** — devolver o resultado no contrato da API.

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

- **JWT** — formato do token utilizado na camada de autenticação.
- **Keycloak** — provedor de identidade utilizado externamente.
- **OAuth** — protocolo de autorização.
- **OIDC** — protocolo utilizado para identidade.
- **Authorization header** — detalhe do transporte HTTP.

Esses mecanismos pertencem à infraestrutura/interface.

Outras proteções incluem:

- **validação de payload** — impede entrada estruturalmente inválida.
- **limite de tamanho da requisição** — reduz risco de payloads excessivos.
- **validação de UUID** — garante identificadores no formato esperado.
- **validação monetária** — garante valores financeiros válidos.
- **rejeição de campos desconhecidos** — evita aceitar contratos não previstos.
- **idempotência** — impede efeitos duplicados.
- **tratamento controlado de erros** — evita exposição desnecessária de detalhes internos.

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
       │             │
       ▼             ▼
    Jaeger       Prometheus

                     │

                     ▼

                  Grafana
```

A observabilidade permite acompanhar:

- **disponibilidade** — verificar se os componentes estão operacionais.
- **volume** — acompanhar quantidade de requisições e mensagens.
- **erros** — identificar falhas técnicas e resultados de negócio.
- **latência** — acompanhar tempo de processamento.
- **processamento das apostas** — observar o fluxo financeiro.
- **resultados de negócio** — diferenciar operações processadas e rejeitadas.
- **comportamento do SQS** — acompanhar o processamento assíncrono.
- **processamento da Inbox** — identificar duplicidades e falhas.
- **investigação de incidentes** — correlacionar métricas, traces e persistência.

---

# 36. OpenTelemetry

O código utiliza OpenTelemetry para instrumentação.

Os principais sinais utilizados são:

- **Traces** — permitem acompanhar uma operação entre diferentes componentes.
- **Metrics** — permitem acompanhar volume, erros, latência e resultados agregados.

A configuração fica centralizada em:

```text
internal/observability/observability.go
```

Essa implementação fornece:

- **TracerProvider** — gerencia a geração e exportação de traces.
- **MeterProvider** — gerencia a geração e exportação de métricas.
- **OTLP exporters** — enviam telemetria para o Collector.
- **Resource attributes** — adicionam contexto aos dados observados.
- **service.name** — identifica o serviço responsável pela telemetria.
- **deployment.environment** — identifica o ambiente de execução.

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

- **latência** — tempo gasto em cada etapa.
- **falhas** — ponto onde uma operação apresentou erro.
- **dependências** — componentes envolvidos no processamento.
- **gargalos** — etapas que concentram maior tempo de execução.
- **chamadas externas** — interação com banco ou mensageria.

---

# 38. Metrics

As métricas são utilizadas para acompanhar tanto aspectos técnicos da aplicação quanto resultados relacionados ao processamento de apostas.

Os principais grupos são:

- **HTTP** — acompanha volume, erros e comportamento de latência da API.
- **Wager** — acompanha volume, resultado e tempo de processamento das apostas.
- **SQS** — acompanha recebimento, processamento, remoção e latência das mensagens.
- **Inbox** — acompanha processamento idempotente e mensagens duplicadas.

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

- **volume** — quantidade de requisições processadas.
- **erros** — proporção de respostas HTTP de erro.
- **latência** — tempo de processamento das requisições.
- **distribuição de duração** — comportamento da latência em diferentes percentis.

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

O P95 mostra o comportamento de uma parcela mais lenta das requisições, enquanto o P50 representa a mediana.

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

Essas métricas permitem diferenciar volume operacional de resultado de negócio.

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

O P50 representa o comportamento típico do processamento.

O P95 ajuda a identificar operações mais lentas e possíveis gargalos.

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

- **falhas** — mensagens recebidas que não chegaram ao processamento concluído.
- **redelivery** — mensagens entregues novamente após falhas.
- **processamento lento** — aumento do tempo necessário para processar mensagens.
- **mensagens pendentes** — diferença persistente entre recebimento e conclusão.

---

# 44. Inbox Metrics

A Inbox possui métricas para:

```text
inbox_messages_received_total

inbox_messages_processed_total

inbox_messages_failed_total

inbox_duplicate_total
```

Essas métricas ajudam a observar o comportamento do consumidor idempotente.

Em especial, `inbox_duplicate_total` permite identificar situações em que uma mesma mensagem foi recebida novamente.

---

# 45. Grafana

O projeto possui dashboard:

```text
Backend Challenge - Distributed Wager Processing
```

O dashboard possui os seguintes painéis:

1. **HTTP Request Rate**  
   Mede a quantidade de requisições HTTP processadas por unidade de tempo, permitindo acompanhar o volume de tráfego da API.

2. **HTTP Error Rate**  
   Mede a proporção de requisições HTTP que resultaram em erros, permitindo identificar aumento de falhas na API.

3. **HTTP Latency P95**  
   Mostra o percentil 95 da latência HTTP, permitindo identificar degradações que podem não aparecer na média.

4. **HTTP Latency P50**  
   Mostra a mediana da latência HTTP, representando o comportamento típico das requisições.

5. **Wager Transactions**  
   Mostra o volume de transações de apostas e seus estados, permitindo acompanhar o fluxo das operações de negócio.

6. **Wager Processing Latency**  
   Mede o tempo necessário para processar uma aposta, permitindo identificar lentidão no fluxo de negócio.

7. **Wager Business Outcomes**  
   Mostra os resultados de negócio das apostas, diferenciando operações processadas com sucesso das operações rejeitadas.

8. **SQS Consumer**  
   Acompanha mensagens recebidas, processadas e removidas pelo consumidor SQS, permitindo verificar o funcionamento do processamento assíncrono.

9. **SQS Processing Latency**  
   Mede o tempo necessário para processar mensagens SQS, permitindo identificar gargalos no processamento assíncrono.

10. **Inbox / Idempotent Consumer**  
    Acompanha mensagens recebidas, processadas e duplicadas, permitindo verificar o funcionamento da proteção contra processamento duplicado.

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

Cada ferramenta possui uma responsabilidade:

- **Grafana** — visualização de dashboards e indicadores.
- **Prometheus** — armazenamento e consulta de métricas.
- **Jaeger** — visualização e investigação de traces.
- **OTel Collector** — coleta, processamento e encaminhamento da telemetria.

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

- **wallets** — representam as carteiras dos jogadores e armazenam o saldo financeiro atual.
- **wager transactions** — registram as apostas, seus identificadores, valores e estados de processamento.
- **ledger** — mantém o histórico financeiro append-only das movimentações realizadas.
- **idempotency** — armazena chaves e informações necessárias para impedir processamento duplicado de requisições.
- **inbox** — registra mensagens recebidas e seu estado de processamento.
- **outbox** — armazena eventos que precisam ser publicados externamente.

As alterações estruturais são realizadas por migrations.

As migrations permitem evoluir o schema de forma versionada e reproduzível.

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

- **auditoria** — permite verificar os lançamentos financeiros realizados.
- **rastreabilidade** — permite acompanhar a origem e sequência das movimentações.
- **reconciliação** — permite comparar movimentações com o estado da carteira.
- **investigação de incidentes** — permite reconstruir o histórico de uma operação.

O saldo da Wallet representa o estado atual.

O Ledger representa o histórico financeiro.

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

Essa fronteira garante que as alterações financeiras críticas sejam tratadas atomicamente.

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

A rejeição representa uma decisão válida do domínio.

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

Business errors representam condições esperadas do domínio.

Technical errors representam falhas de infraestrutura ou execução.

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

Cada camada traduz o erro apenas quando necessário para seu contexto.

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

Isso permite testar regras sem iniciar toda a infraestrutura.

---

# 54. Unit Tests

Os testes unitários validam:

- **Money** — precisão e regras dos valores monetários.
- **regras de domínio** — comportamento financeiro esperado.
- **validações** — entrada e invariantes.
- **idempotência** — repetição de operações.
- **insuficiência de saldo** — rejeição sem efeito financeiro.
- **estados** — transições válidas das entidades.
- **application services** — coordenação dos casos de uso.

Executar:

```bash
go test ./tests/unit/...
```

---

# 55. Integration Tests

Os testes de integração validam componentes reais.

Exemplos:

- **PostgreSQL** — persistência real.
- **transações** — atomicidade e rollback.
- **concorrência** — operações simultâneas sobre recursos financeiros.
- **repositories** — comportamento das implementações de persistência.
- **Inbox** — processamento e deduplicação.
- **Outbox** — persistência e publicação de eventos.

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

- **consumers** — processamento concorrente de mensagens.
- **workers** — execução assíncrona em background.
- **goroutines** — execução paralela.
- **processamento concorrente** — múltiplas apostas podem ocorrer simultaneamente.
- **acesso compartilhado** — componentes podem operar sobre os mesmos recursos.
- **operações financeiras simultâneas** — cenário crítico para double spend.

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

O E2E representa o comportamento mais próximo do fluxo real da aplicação.

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

Isso facilita desenvolvimento, testes e reprodução do ambiente.

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

Keycloak ou OIDC Provider

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

O lifecycle centraliza inicialização e encerramento controlado dos componentes.

---

# 65. Graceful Shutdown

A aplicação deve encerrar de forma controlada.

Objetivos:

- **não interromper transações em andamento de forma abrupta** — reduz risco de operações incompletas.
- **interromper workers** — evita iniciar novos processamentos durante shutdown.
- **finalizar operações em andamento** — permite conclusão controlada.
- **liberar conexões** — encerra recursos de infraestrutura.
- **flush de telemetry** — evita perda de traces e métricas.
- **fechar recursos** — garante encerramento adequado dos componentes.

---

# 66. Scalability

A arquitetura foi desenhada para escala horizontal.

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

Estado crítico permanece no PostgreSQL.

Essa característica permite adicionar ou remover instâncias sem migrar estado local.

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

A observabilidade também suporta:

- auditoria;
- rastreabilidade;
- diagnóstico;
- análise de performance;
- investigação de incidentes.

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

Outros componentes podem ser consultados durante a investigação:

- **Ledger** — histórico financeiro.
- **Wager Transaction** — estado da operação.
- **Inbox** — estado do processamento assíncrono.
- **Outbox** — eventos pendentes de publicação.
- **Logs** — contexto detalhado do processamento.

---

# 72. Failure Scenarios

### PostgreSQL indisponível

Resultado:

```text
Transaction fails
```

Nenhum débito parcial deve permanecer.

### SQS indisponível

A Outbox permanece:

```text
PENDING
```

e pode ser publicada posteriormente.

### Consumer falha

A mensagem pode ser redeliverada.

A Inbox impede duplicação do efeito.

### Delete SQS falha

A mensagem pode retornar.

A Inbox detecta:

```text
PROCESSED
```

e evita novo processamento financeiro.

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

As regras pertencem ao domínio/application layer.

### Domínio dependendo de PostgreSQL

Não recomendado.

### Domínio dependendo de SQS

Não recomendado.

### Float para dinheiro

Não permitido.

### Global Mutex

Não utilizado para consistência financeira.

### Estado financeiro somente em memória

Não permitido.

### Publicar SQS antes do commit

Evita-se devido ao Outbox.

### Deletar SQS antes do commit

Evita-se porque poderia perder mensagem.

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

## Open/Closed

Adapters podem ser adicionados sem alterar o domínio.

Exemplo:

```text
SQS

RabbitMQ

Kafka
```

podem implementar a mesma porta.

## Liskov Substitution

Uma implementação de uma porta deve respeitar o contrato definido pela interface.

## Interface Segregation

Interfaces são pequenas e específicas, evitando contratos maiores do que a responsabilidade necessária.

## Dependency Inversion

Application e domínio dependem de abstrações.

---

# 75. DDD Lite

O projeto não tenta implementar DDD completo.

Utiliza conceitos que agregam valor ao problema:

- **entidades** — representam objetos com identidade e ciclo de vida.
- **value objects** — representam conceitos como Money.
- **regras de domínio** — concentram invariantes financeiras.
- **serviços de aplicação** — coordenam casos de uso.
- **repositories** — abstraem persistência.
- **eventos** — representam fatos que precisam ser propagados.
- **estados** — representam o ciclo de vida das operações.

O objetivo é manter o domínio expressivo sem adicionar complexidade desnecessária.

---

# 76. Aggregate Wallet

A wallet pode ser tratada como unidade de consistência financeira.

Operações relacionadas ao saldo precisam respeitar a consistência desse agregado.

A persistência e concorrência são coordenadas pelo banco.

Isso permite que operações concorrentes sejam serializadas no nível do registro da carteira.

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

Cada estado representa uma etapa funcional do ciclo de vida da aposta.

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

A rastreabilidade pode ser complementada por:

- **request ID** — identifica a requisição HTTP.
- **trace ID** — correlaciona spans de uma operação distribuída.
- **transaction ID** — identifica a transação financeira.
- **external transaction ID** — identifica a operação no sistema externo.
- **message ID** — identifica a mensagem assíncrona.

Além disso:

- **auditoria** — capacidade de verificar o histórico das operações e lançamentos.
- **rastreabilidade** — capacidade de seguir uma operação entre HTTP, aplicação, banco, Outbox, SQS e Inbox.
- **reconciliação** — comparação entre Wallet, Ledger e Wager Transactions para detectar divergências.
- **investigação de incidentes** — suporte à análise de falhas usando logs, métricas, traces e registros persistidos.

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

- **API stateless** — permite múltiplas instâncias.
- **processamento concorrente** — operações independentes podem executar simultaneamente.
- **ausência de global lock** — evita serialização artificial da aplicação.
- **SQS para desacoplamento** — separa produção e consumo.
- **workers independentes** — permite processamento assíncrono.
- **banco como coordenador de concorrência** — mantém consistência sem estado global da aplicação.
- **observabilidade baseada em métricas** — permite detectar degradação.

---

# 81. Resilience

A arquitetura suporta:

- **retries** — repetição controlada de operações que podem se recuperar.
- **redelivery** — reentrega de mensagens não concluídas.
- **idempotência** — evita efeitos duplicados.
- **Inbox** — controla processamento assíncrono.
- **Outbox** — evita perda de eventos.
- **DLQ** — isola mensagens que excederam as tentativas.
- **transações** — garantem atomicidade.
- **graceful shutdown** — reduz interrupções abruptas.

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

Cada etapa valida uma dimensão diferente da qualidade:

- **compilação** — garante que o código pode ser construído.
- **comportamento** — validado pelos testes unitários.
- **integração** — confirma interação entre componentes reais.
- **concorrência** — validada pelo race detector e testes concorrentes.
- **fluxo completo** — validado pelos testes E2E.
- **operação** — validada através da observabilidade.
- **documentação** — registra decisões e funcionamento do sistema.

---

# 85. Production Considerations

Para produção, recomenda-se:

- **secrets manager** — armazenar credenciais fora do código.
- **TLS** — proteger comunicação em trânsito.
- **IAM** — controlar permissões de infraestrutura.
- **database credentials fora do código** — evitar exposição de credenciais.
- **network segmentation** — restringir comunicação entre componentes.
- **least privilege** — conceder somente as permissões necessárias.
- **autoscaling** — ajustar capacidade conforme demanda.
- **SQS visibility timeout adequado** — evitar redelivery prematuro.
- **DLQ monitoring** — acompanhar mensagens que falharam repetidamente.
- **database backups** — permitir recuperação de dados.
- **migrations controladas** — evoluir schema de forma segura.
- **alertas** — detectar problemas automaticamente.
- **dashboards** — acompanhar indicadores operacionais.
- **distributed tracing** — investigar fluxos distribuídos.

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

A API pode escalar horizontalmente, enquanto os consumers podem aumentar conforme o volume de mensagens.

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

A separação evita que mecanismos externos contaminem as regras de negócio.

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

O Repository Pattern também facilita testes e substituição de infraestrutura.

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

A aplicação depende do contrato e não do SDK específico.

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
```

Depois:

```text
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

O mesmo conceito pode ser utilizado para Kafka quando o modelo de entrega e processamento justificar essa tecnologia.

O importante é preservar:

- **contrato** — mantém a interface esperada pela aplicação.
- **idempotência** — evita efeitos duplicados.
- **consistência** — preserva as regras financeiras.
- **observabilidade** — mantém visibilidade sobre o processamento.

A tecnologia de transporte não deve alterar as regras centrais do domínio.

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

Isso reduz contenção e facilita recuperação de falhas.

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

SQS deve ser tratado como mecanismo de entrega que pode resultar em redelivery.

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

O objetivo não é assumir exactly-once delivery, mas garantir que o efeito financeiro seja idempotente.

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

Em operações financeiras concorrentes, PostgreSQL atua como coordenador da consistência.

Isso é preferível a manter estado crítico em memória da aplicação.

O banco fornece:

- **transações** — garantem atomicidade das operações relacionadas.
- **locks por registro** — coordenam concorrência sobre uma wallet específica.
- **constraints** — reforçam invariantes diretamente no banco.
- **persistência durável** — mantém o estado após reinícios.

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

Além dos indicadores técnicos, os resultados de negócio permitem avaliar se o processamento está ocorrendo conforme esperado.

---

# 100. Architectural Decision Summary

Principais decisões:

| Decisão | Motivo |
|---|---|
| Clean Architecture | Mantém dependências direcionadas para o domínio e separa responsabilidades. |
| Hexagonal Architecture | Isola o núcleo da aplicação das tecnologias externas. |
| Interfaces | Permitem testes e substituição de implementações. |
| Exact Money | Evita erros de precisão em operações financeiras. |
| PostgreSQL | Fornece consistência transacional e mecanismos de concorrência. |
| Database locking | Protege operações concorrentes sobre a mesma carteira. |
| Idempotency | Evita efeitos duplicados em retries HTTP. |
| Inbox | Garante processamento idempotente de mensagens. |
| Outbox | Evita perda de eventos entre banco e mensageria. |
| SQS | Fornece processamento assíncrono e desacoplamento. |
| DLQ | Isola mensagens que falham repetidamente. |
| OpenTelemetry | Padroniza instrumentação de métricas e traces. |
| Prometheus | Permite armazenamento e consulta de métricas. |
| Grafana | Fornece visualização operacional e dashboards. |
| Jaeger | Permite investigação de traces distribuídos. |
| Uber Fx | Centraliza composição e Dependency Injection. |
| Keycloak/OIDC | Fornece autenticação baseada em padrões. |
| Docker Compose | Reproduz o ambiente local de forma controlada. |

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

- **processamento distribuído** — API e consumers podem operar em múltiplas instâncias.
- **processamento assíncrono** — SQS desacopla produção e consumo.
- **exact money** — valores financeiros não dependem de floating point.
- **persistência** — estado financeiro é armazenado de forma durável.
- **concorrência** — banco coordena operações simultâneas.
- **idempotência** — retries não duplicam efeitos.
- **Inbox** — protege o processamento assíncrono contra duplicidade.
- **Outbox** — protege eventos contra perda após commit.
- **SQS** — fornece transporte assíncrono.
- **DLQ** — isola mensagens problemáticas.
- **retries** — permitem recuperação de falhas transitórias.
- **redelivery** — mensagens não concluídas podem ser processadas novamente.
- **OIDC** — fornece autenticação.
- **observabilidade** — métricas e traces permitem monitoramento.
- **testes** — unitários, integração, E2E e race detector.
- **Docker** — fornece ambiente local reproduzível.
- **modularidade** — interfaces isolam infraestrutura.
- **separação de responsabilidades** — cada camada possui uma função definida.

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

Esse fluxo representa a combinação entre segurança, validação, consistência financeira, mensageria e idempotência.

---

# 104. Conclusão

A arquitetura foi projetada para tratar o processamento de apostas como um problema de consistência financeira distribuída, e não apenas como uma API HTTP.

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

Essa combinação permite que o sistema seja executado localmente, testado de forma determinística e posteriormente evoluído para uma infraestrutura AWS sem necessidade de alterar as regras centrais do domínio.

A principal característica arquitetural é o desacoplamento:

```text
Business Rules

      ≠

Infrastructure
```

Isso permite evoluir banco, mensageria, autenticação, observabilidade e ambiente de execução mantendo estáveis os casos de uso e o domínio.

O resultado é uma arquitetura preparada para:

- **processamento financeiro consistente** — garante que saldo, transações e lançamentos permaneçam coerentes.
- **execução distribuída** — permite múltiplas instâncias da API e dos workers.
- **processamento assíncrono** — desacopla produção e consumo através da mensageria.
- **recuperação de falhas** — utiliza retries, redelivery, Inbox, Outbox e DLQ.
- **escala horizontal** — permite aumentar a capacidade sem alterar as regras de negócio.
- **observabilidade operacional** — fornece métricas, traces e dashboards para acompanhamento do sistema.
- **auditoria** — mantém histórico das operações e movimentações financeiras.
- **rastreabilidade** — permite acompanhar uma operação entre seus diferentes componentes.
- **reconciliação** — possibilita comparar o estado das entidades financeiras.
- **testes automatizados** — valida regras, integração, concorrência e fluxo completo.
- **evolução de infraestrutura** — permite substituir implementações como SQS por RabbitMQ ou Kafka sem alterar o domínio.