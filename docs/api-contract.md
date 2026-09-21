# API Contract

## 1. Autenticação

Endpoints protegidos devem utilizar:

```http
Authorization: Bearer <access-token>
```

O token é obtido através do fluxo OAuth2 `client_credentials`.

---

# 2. Health

## GET /health

Indica se a aplicação está operacional.

### Resposta

```json
{
  "status": "UP"
}
```

---

# 3. Readiness

## GET /health/ready

Indica se a aplicação está pronta para operação.

### Resposta

```json
{
  "status": "UP"
}
```

---

# 4. Criar/processar aposta

## POST /transactions/wager

### Headers

```http
Authorization: Bearer <token>
Content-Type: application/json
Idempotency-Key: <unique-key>
```

### Request

```json
{
  "externalTransactionId": "external-001",
  "providerId": "provider-001",
  "walletId": "00000000-0000-0000-0000-000000000001",
  "playerId": "00000000-0000-0000-0000-000000000002",
  "roundId": "round-001",
  "gameId": "game-001",
  "amount": "10.00",
  "currency": "BRL"
}
```

---

# 5. Resposta de sucesso

```json
{
  "transactionId": "transaction-id",
  "status": "PROCESSED",
  "balance": "90.00",
  "currency": "BRL",
  "idempotentReplay": false
}
```

---

# 6. Resposta de replay

Quando a mesma operação for enviada novamente:

```json
{
  "transactionId": "transaction-id",
  "status": "PROCESSED",
  "balance": "90.00",
  "currency": "BRL",
  "idempotentReplay": true
}
```

---

# 7. Conflito de idempotência

Quando a mesma chave for utilizada com payload diferente:

```http
409 Conflict
```

A operação não deve produzir novo efeito financeiro.

---

# 8. Saldo insuficiente

Quando não existir saldo suficiente:

```http
409 Conflict
```

A transação deve registrar a rejeição.

---

# 9. Erros de autenticação

Sem Authorization:

```http
401 Unauthorized
```

Token inválido ou não autorizado:

```http
401 Unauthorized
```

---

# 10. Validação

Payload inválido deve ser rejeitado.

Exemplos:

- JSON inválido;
- UUID inválido;
- amount inválido;
- currency ausente;
- providerId ausente;
- externalTransactionId ausente;
- Idempotency-Key ausente.

---

# 11. Limite de payload

A API deve limitar o tamanho máximo do corpo da requisição para
evitar payloads excessivos.

O limite implementado atualmente é:

```text
1 MB
```

---

# 12. Campos desconhecidos

Campos não previstos no contrato devem ser rejeitados.

Exemplo:

```json
{
  "amount": "10.00",
  "unexpectedField": true
}
```

---

# 13. Dinheiro

O campo `amount` deve ser enviado como string.

Correto:

```json
{
  "amount": "10.00"
}
```

Não utilizar:

```json
{
  "amount": 10.00
}
```

A representação textual evita perda de precisão durante a desserialização.

---

# 14. Idempotency-Key

A chave deve identificar exclusivamente a tentativa lógica da operação.

Exemplo:

```http
Idempotency-Key: wager-123456
```

A mesma chave com o mesmo payload deve permitir replay.

A mesma chave com payload diferente deve gerar conflito.

---

# 15. Contrato de resposta

Campos:

| Campo | Tipo | Descrição |
|---|---|---|
| transactionId | string | Identificador da transação |
| status | string | Resultado da operação |
| balance | string | Saldo após operação |
| currency | string | Moeda da carteira |
| idempotentReplay | boolean | Indica replay idempotente |

---

# 16. Observabilidade HTTP

As requisições devem permitir correlação através de request ID.

O sistema deve disponibilizar métricas de:

- quantidade;
- duração;
- status HTTP;
- tamanho de request;
- tamanho de response.

---

# 17. Compatibilidade

O contrato HTTP deve permanecer independente da implementação interna.

Alterações em:

```text
PostgreSQL
SQS
RabbitMQ
Keycloak
```

não devem alterar o contrato funcional da API sem mudança explícita
de requisito.
