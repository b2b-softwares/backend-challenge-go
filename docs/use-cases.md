# Use Cases

## 1. UC-001 — Processar aposta

### Objetivo

Processar uma aposta válida, atualizar a carteira e registrar
os efeitos financeiros.

### Ator

Cliente autenticado.

### Pré-condições

- cliente autenticado;
- carteira existente;
- Idempotency-Key presente;
- payload válido.

### Fluxo

```text
Cliente
  |
  | POST wager
  v
HTTP Handler
  |
  v
WagerService
  |
  +--> Idempotency
  |
  +--> Wallet
  |
  +--> Transaction
  |
  +--> Ledger
  |
  +--> Outbox
  |
  v
Response
```

### Resultado

A aposta é processada e o novo saldo é retornado.

---

# 2. UC-002 — Repetir aposta idempotente

### Objetivo

Processar novamente uma requisição já processada sem duplicar
o efeito financeiro.

### Fluxo

```text
Request
  |
  v
IdempotencyRepository
  |
  +--> existente
        |
        v
      replay
        |
        v
resultado anterior
```

### Resultado

A operação original é retornada.

---

# 3. UC-003 — Conflito de idempotência

### Objetivo

Impedir reutilização de uma chave com payload diferente.

### Fluxo

```text
Idempotency-Key
      |
      v
Existing payload hash
      |
      v
Hash diferente
      |
      v
Conflict
```

### Resultado

A operação é rejeitada sem efeito financeiro adicional.

---

# 4. UC-004 — Aposta com saldo insuficiente

### Objetivo

Rejeitar aposta quando o saldo não é suficiente.

### Fluxo

```text
Aposta
  |
  v
Wallet
  |
  v
Saldo insuficiente
  |
  v
REJECTED
```

### Resultado

Nenhum débito financeiro é realizado.

---

# 5. UC-005 — Apostas concorrentes

### Objetivo

Preservar consistência quando múltiplas apostas atingem
a mesma carteira simultaneamente.

### Exemplo

```text
Saldo = 90

A = 50
B = 50

      Wallet
        |
   +----+----+
   |         |
   A         B
   |         |
   50        50
   |         |
PROCESSADO  REJEITADO
```

### Resultado

Somente uma operação consegue utilizar o saldo disponível.

---

# 6. UC-006 — Consumir mensagem SQS

### Objetivo

Processar uma solicitação recebida através da fila.

### Fluxo

```text
SQS
 |
 v
Consumer
 |
 v
Inbox Ensure
 |
 v
Claim
 |
 v
WagerService
 |
 v
Inbox MarkProcessed
 |
 v
Delete SQS
```

### Regra

A mensagem somente deve ser removida depois de processamento seguro.

---

# 7. UC-007 — Redelivery

### Objetivo

Permitir recuperação após falha de processamento.

### Fluxo

```text
SQS
 |
 v
Consumer
 |
 X erro
 |
 +--> mensagem permanece
 |
 v
Redelivery
 |
 v
Consumer
```

---

# 8. UC-008 — Outbox

### Objetivo

Garantir persistência de eventos antes da publicação externa.

### Fluxo

```text
Business Transaction
 |
 +--> Wallet
 +--> Transaction
 +--> Ledger
 +--> Outbox
 |
 COMMIT
 |
 v
Outbox Publisher
 |
 v
SQS
```

---

# 9. UC-009 — Reversão

### Objetivo

Registrar reversão de uma operação anterior sem apagar
o histórico original.

### Fluxo

```text
Original transaction
        |
        v
ReversalService
        |
        +--> reversal transaction
        |
        +--> ledger entry
        |
        +--> outbox event
```

---

# 10. UC-010 — Processar referência pendente

### Objetivo

Tentar novamente operações que permaneceram pendentes.

### Fluxo

```text
PendingReferenceWorker
        |
        v
Find pending
        |
        v
Process
        |
   +----+----+
   |         |
success     error
   |         |
   v         v
complete   retry
```

---

# 11. UC-011 — Autenticar cliente

### Fluxo

```text
Client
 |
 | client_credentials
 v
Keycloak
 |
 v
Access Token
 |
 v
API
 |
 v
Authorization
```

---

# 12. UC-012 — Observar operação

Uma operação deve permitir acompanhamento através de:

- logs;
- métricas;
- traces;
- request ID;
- transaction ID;
- idempotency key;
- mensagem SQS;
- Inbox;
- Outbox.

---

# 13. Resumo dos casos de uso

| ID | Caso de uso |
|---|---|
| UC-001 | Processar aposta |
| UC-002 | Replay idempotente |
| UC-003 | Conflito de idempotência |
| UC-004 | Saldo insuficiente |
| UC-005 | Concorrência |
| UC-006 | Consumir SQS |
| UC-007 | Redelivery |
| UC-008 | Outbox |
| UC-009 | Reversão |
| UC-010 | Referência pendente |
| UC-011 | Autenticação |
| UC-012 | Observabilidade |
