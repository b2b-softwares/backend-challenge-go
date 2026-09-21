# Requirements Specification

## 1. Objetivo

Este documento especifica os requisitos funcionais e não funcionais do
Backend Challenge — Processamento Distribuído de Apostas em Go.

O objetivo é registrar de forma rastreável o que o sistema deve fazer,
independentemente da implementação tecnológica utilizada.

---

# 2. Escopo

O sistema deve processar transações financeiras relacionadas a apostas,
mantendo consistência de saldo, idempotência, rastreabilidade e
processamento distribuído.

O sistema contempla:

- criação e processamento de apostas;
- validação de solicitações;
- controle de saldo;
- persistência transacional;
- ledger financeiro append-only;
- idempotência;
- Inbox;
- Outbox;
- processamento assíncrono;
- AWS SQS;
- redelivery;
- DLQ;
- processamento de referências pendentes;
- reversões;
- autenticação OIDC;
- observabilidade;
- métricas;
- tracing;
- testes concorrentes.

---

# 3. Requisitos funcionais

## REQ-001 — Receber uma solicitação de aposta

O sistema deve disponibilizar uma API HTTP para receber uma solicitação
de processamento de aposta.

A solicitação deve conter:

- externalTransactionId;
- providerId;
- walletId;
- playerId;
- roundId;
- gameId;
- amount;
- currency.

A operação deve exigir um `Idempotency-Key`.

---

## REQ-002 — Autenticação

As operações protegidas devem exigir um token OAuth2/OIDC válido.

O ambiente local deve utilizar Keycloak como Identity Provider.

O fluxo esperado é:

```text
Client
  |
  | client_credentials
  v
Keycloak
  |
  | access_token
  v
Client
  |
  | Authorization: Bearer
  v
API
```

---

## REQ-003 — Validar entrada

A API deve validar:

- método HTTP;
- presença do Idempotency-Key;
- Authorization;
- JSON válido;
- campos obrigatórios;
- UUIDs;
- moeda;
- valor monetário;
- tamanho máximo do payload;
- campos desconhecidos.

Entradas inválidas devem ser rejeitadas antes da operação financeira.

---

## REQ-004 — Representar dinheiro sem ponto flutuante

Valores monetários não podem utilizar `float32` ou `float64`.

A representação monetária deve preservar exatamente duas casas decimais
para BRL.

Exemplos válidos:

```text
10.00
50.00
100.00
0.01
```

Exemplos inválidos:

```text
10.001
-10.00
abc
```

---

## REQ-005 — Processar aposta

Uma aposta válida deve:

1. identificar a carteira;
2. verificar saldo;
3. reservar/debitar o valor;
4. registrar a transação;
5. registrar o lançamento financeiro;
6. registrar o evento necessário;
7. preservar idempotência;
8. retornar o novo saldo.

As alterações financeiras devem ocorrer atomicamente.

---

## REQ-006 — Impedir saldo insuficiente

Uma aposta não pode ser processada quando a carteira não possui
saldo suficiente.

Nesse caso:

- a carteira não deve sofrer débito;
- o ledger não deve registrar débito financeiro da aposta;
- a transação deve registrar o resultado da operação;
- a API deve informar a rejeição.

---

## REQ-007 — Concorrência por carteira

O sistema deve suportar apostas concorrentes sobre a mesma carteira.

A consistência deve ser garantida pelo mecanismo transacional do banco,
sem utilização de lock global da aplicação.

Exemplo:

```text
Saldo = R$ 90,00

Aposta A = R$ 50,00
Aposta B = R$ 50,00

Resultado:

A → PROCESSING
B → REJECTED

Saldo final = R$ 40,00
```

---

## REQ-008 — Idempotência

Uma mesma operação lógica não pode produzir efeitos financeiros
duplicados.

A mesma combinação de:

```text
Idempotency-Key
+
operação
+
payload
```

deve produzir no máximo um efeito financeiro.

Uma repetição válida deve retornar o resultado anteriormente produzido.

---

## REQ-009 — Conflito de idempotência

Quando uma Idempotency-Key já utilizada for enviada com payload diferente,
a operação deve ser rejeitada.

Nenhuma alteração financeira adicional pode ocorrer.

---

## REQ-010 — Inbox persistente

Mensagens recebidas da fila devem possuir controle persistente de Inbox.

O Inbox deve permitir identificar:

- mensagem recebida;
- mensagem em processamento;
- mensagem processada;
- mensagem duplicada;
- falha de processamento.

O controle não pode depender somente da memória da aplicação.

---

## REQ-011 — Outbox persistente

Eventos que precisam ser publicados externamente devem ser registrados
em Outbox de forma transacional.

A publicação externa não deve ser necessária para confirmar a transação
financeira.

---

## REQ-012 — Publicação assíncrona

Eventos persistidos na Outbox devem ser publicados de forma assíncrona.

Falhas temporárias de publicação não devem causar perda do evento.

---

## REQ-013 — Processamento SQS

O sistema deve consumir mensagens de aposta através de uma abstração de
mensageria.

A implementação local utiliza SQS disponibilizado pelo MiniStack.

O domínio não deve depender diretamente do SDK da AWS.

---

## REQ-014 — Redelivery

Quando o processamento de uma mensagem falhar, a mensagem deve permanecer
disponível para redelivery.

O consumidor não deve excluir a mensagem antes da conclusão segura
do processamento.

---

## REQ-015 — Dead Letter Queue

Mensagens que excederem a política de tentativas devem ser encaminhadas
para uma DLQ.

O mecanismo deve permitir investigação posterior da mensagem.

---

## REQ-016 — Worker de referências pendentes

O sistema deve possuir processamento assíncrono para referências que
não puderam ser concluídas imediatamente.

O worker deve:

- localizar referências pendentes;
- tentar processá-las novamente;
- respeitar estados;
- evitar duplicidade;
- preservar rastreabilidade.

---

## REQ-017 — Reversão

O sistema deve permitir reversão de operações financeiras quando aplicável.

A reversão deve:

- possuir identificação própria;
- preservar o histórico original;
- registrar lançamento correspondente;
- não apagar o lançamento original;
- respeitar idempotência.

---

## REQ-018 — Ledger financeiro

Operações financeiras devem produzir registros em um ledger append-only.

O ledger deve permitir reconstruir o histórico financeiro da carteira.

Registros financeiros existentes não devem ser alterados para esconder
operações anteriores.

---

## REQ-019 — Persistência transacional

Operações que alteram estado financeiro devem utilizar transação de banco.

A atualização da carteira e seus registros financeiros relacionados
devem possuir comportamento atômico.

---

## REQ-020 — API de health

A aplicação deve disponibilizar endpoint de health para verificar se
a aplicação está operacional.

---

## REQ-021 — API de readiness

A aplicação deve disponibilizar endpoint de readiness para indicar
se está pronta para receber/processar operações.

---

## REQ-022 — Observabilidade

A aplicação deve fornecer:

- métricas;
- logs;
- traces;
- correlação de requisições;
- métricas de processamento;
- métricas de mensageria;
- métricas de idempotência.

---

## REQ-023 — OpenTelemetry

A instrumentação deve utilizar OpenTelemetry.

O sistema deve permitir exportação de traces e métricas para o
OpenTelemetry Collector.

---

## REQ-024 — Prometheus

Métricas devem ser disponibilizadas de forma compatível com Prometheus.

---

## REQ-025 — Grafana

O projeto deve possuir dashboard para acompanhamento operacional.

O dashboard deve permitir visualizar, entre outros:

1. taxa de requisições HTTP;
2. taxa de erros HTTP;
3. latência HTTP P95;
4. latência HTTP P50;
5. transações de aposta;
6. latência de processamento;
7. resultados de negócio;
8. consumidor SQS;
9. latência SQS;
10. Inbox/idempotência.

---

## REQ-026 — Tracing distribuído

Operações relevantes devem ser rastreáveis através de spans.

O tracing deve permitir acompanhar o fluxo:

```text
HTTP
 |
Application
 |
Database
 |
Outbox
 |
SQS
 |
Consumer
 |
Application
 |
Database
```

---

## REQ-027 — Arquitetura desacoplada

O domínio não deve depender diretamente de:

- HTTP;
- Fx;
- PostgreSQL;
- SQS;
- AWS SDK;
- Keycloak;
- OpenTelemetry.

Essas tecnologias devem ser acessadas através de interfaces/adapters
quando fizer sentido arquitetural.

---

## REQ-028 — Troca de infraestrutura

Deve ser possível substituir implementações de infraestrutura sem alterar
as regras de negócio.

Exemplos:

```text
SQS       ↔ RabbitMQ
Postgres  ↔ outro storage compatível
Keycloak  ↔ outro OIDC Provider
```

---

## REQ-029 — Injeção de dependências

A composição das dependências deve ser realizada utilizando Uber Fx.

O domínio e os casos de uso não devem depender de Fx diretamente.

---

## REQ-030 — Docker

O sistema deve ser executável através de Docker Compose no ambiente local.

O ambiente deve disponibilizar os componentes necessários para execução,
teste e observabilidade.

---

## REQ-031 — Banco de dados

O sistema deve utilizar PostgreSQL para persistência.

A criação/evolução do schema deve utilizar migrations versionadas.

---

## REQ-032 — Testes

O projeto deve possuir testes:

- unitários;
- integração;
- concorrência;
- idempotência;
- mensageria;
- persistência;
- HTTP;
- E2E quando aplicável.

---

## REQ-033 — Race detector

O projeto deve passar:

```bash
go test -race ./...
```

---

## REQ-034 — Recuperação

Falhas de componentes externos não devem provocar perda silenciosa
de operações confirmadas.

O desenho deve permitir recuperação através de:

- Inbox;
- Outbox;
- redelivery;
- DLQ;
- estados persistentes.

---

# 4. Requisitos não funcionais

## NFR-001 — Consistência

Operações financeiras devem preservar consistência transacional.

## NFR-002 — Idempotência

Reprocessamentos não devem gerar efeitos financeiros duplicados.

## NFR-003 — Concorrência

O sistema deve suportar concorrência sem race conditions.

## NFR-004 — Disponibilidade operacional

Falhas transitórias de componentes externos devem ser recuperáveis.

## NFR-005 — Observabilidade

Operações críticas devem ser mensuráveis e rastreáveis.

## NFR-006 — Segurança

Operações protegidas devem exigir autenticação.

## NFR-007 — Manutenibilidade

O código deve possuir separação clara entre domínio, aplicação e
infraestrutura.

## NFR-008 — Testabilidade

Casos de uso devem poder ser testados sem depender obrigatoriamente
de infraestrutura externa.

## NFR-009 — Configurabilidade

Configurações de infraestrutura devem ser externas ao domínio.

## NFR-010 — Auditabilidade

Operações financeiras devem possuir histórico suficiente para
investigação e reconciliação.

---

# 5. Critérios gerais de aceite

O sistema é considerado funcionalmente completo quando:

- apostas válidas são processadas;
- saldo é atualizado corretamente;
- saldo insuficiente é rejeitado;
- operações duplicadas são tratadas de forma idempotente;
- conflitos de idempotência são rejeitados;
- operações concorrentes preservam o saldo;
- ledger é mantido;
- Inbox é persistente;
- Outbox é persistente;
- mensagens podem sofrer redelivery;
- DLQ é suportada;
- reversões são rastreáveis;
- autenticação funciona;
- observabilidade está operacional;
- migrations funcionam;
- Docker Compose sobe o ambiente;
- testes passam;
- `go test -race ./...` passa.