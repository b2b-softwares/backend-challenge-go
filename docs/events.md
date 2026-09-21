# Events and Messaging

## 1. Objetivo

Este documento descreve os eventos e mensagens utilizados pelo
processamento distribuído.

O contrato de mensageria deve ser independente da implementação
específica do broker.

---

# 2. WagerTransactionRequested

Mensagem utilizada para solicitar processamento de uma aposta.

### Tipo

```text
WagerTransactionRequested
```

### Kind

```text
BET
```

---

# 3. Estrutura conceitual

```text
{
    type,
    kind,
    transaction,
    wallet,
    player,
    round,
    game,
    amount,
    currency
}
```

A implementação concreta deve permanecer compatível com o contrato
utilizado pelo consumer.

---

# 4. Identidade da mensagem

A mensagem deve possuir informações suficientes para permitir:

- deduplicação;
- rastreamento;
- auditoria;
- correlação;
- processamento idempotente.

---

# 5. Payload hash

O consumer calcula hash do payload para auxiliar na identificação
de alterações ou duplicidades.

O hash utilizado é SHA-256.

---

# 6. Fluxo de processamento

```text
Producer
   |
   v
SQS
   |
   v
WagerConsumer
   |
   v
Inbox
   |
   v
WagerService
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
Inbox PROCESSED
   |
   v
Delete SQS
```

---

# 7. Inbox

O Inbox representa o controle persistente da mensagem recebida.

Estados conceituais:

```text
RECEIVED
   |
   v
PROCESSING
   |
   v
PROCESSED
```

Em caso de erro, a mensagem pode permanecer disponível para redelivery.

---

# 8. Redelivery

Uma mensagem não deve ser excluída quando ocorrer erro antes da
conclusão segura do processamento.

```text
Message
  |
  v
Consumer
  |
  X
  |
  v
Visibility timeout
  |
  v
Redelivery
```

---

# 9. Delete

A exclusão da mensagem deve ocorrer somente após:

1. processamento concluído;
2. estado persistido;
3. Inbox atualizado;
4. commit concluído.

---

# 10. Dead Letter Queue

Mensagens que falharem repetidamente devem ser encaminhadas
para a DLQ conforme a política configurada no ambiente.

A DLQ permite:

- investigação;
- diagnóstico;
- replay controlado;
- identificação de mensagens inválidas.

---

# 11. Outbox

Eventos produzidos pela aplicação devem primeiro ser persistidos
na Outbox.

```text
Business Transaction
       |
       +--> Outbox
       |
     COMMIT
       |
       v
Publisher
       |
       v
Broker
```

---

# 12. Garantia de publicação

O publisher deve buscar eventos pendentes na Outbox e tentar publicá-los.

Uma falha temporária de publicação não deve apagar o evento.

---

# 13. Reprocessamento

Se a publicação falhar:

```text
OUTBOX PENDING
      |
      v
retry
      |
      +---- success --> published
      |
      +---- failure --> pending
```

---

# 14. Eventos de negócio

Os eventos devem representar fatos relevantes do domínio.

Exemplos conceituais:

```text
WagerTransactionRequested
WagerProcessed
WagerRejected
WagerReversed
```

A existência de um evento deve ser tratada como contrato de integração,
não como detalhe de infraestrutura.

---

# 15. Independência do broker

O domínio e a aplicação não devem depender diretamente de:

```text
AWS SDK
SQS SDK
RabbitMQ SDK
```

A aplicação deve depender de uma abstração de mensageria.

Exemplo conceitual:

```text
Application
    |
    v
MessagePublisher
    |
    +---- SQS Adapter
    |
    +---- RabbitMQ Adapter
```

---

# 16. Observabilidade de mensagens

O processamento deve gerar métricas relacionadas a:

- mensagens recebidas;
- mensagens processadas;
- mensagens com falha;
- mensagens deletadas;
- tempo de processamento;
- mensagens duplicadas;
- Inbox processado.

---

# 17. Requisitos de consumidores

Consumidores externos devem considerar que eventos podem ser
entregues mais de uma vez.

A idempotência deve ser tratada como requisito de integração.
