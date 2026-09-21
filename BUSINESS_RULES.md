# Business Rules

## 1. Objetivo

Este documento define as regras de negócio e invariantes financeiras
do sistema de processamento distribuído de apostas.

As regras deste documento devem permanecer válidas independentemente
da tecnologia utilizada na infraestrutura.

---

# 2. Regras financeiras

## BR-001 — Saldo não pode ficar negativo

Uma carteira não pode apresentar saldo inferior a zero como resultado
de uma aposta.

---

## BR-002 — Débito representa aposta processada

Uma aposta somente pode produzir débito financeiro quando a operação
for efetivamente processada.

---

## BR-003 — Saldo insuficiente não gera débito

Quando o saldo for insuficiente:

- a carteira permanece inalterada;
- não deve existir débito correspondente;
- a transação deve registrar a rejeição.

---

## BR-004 — Dinheiro não utiliza ponto flutuante

Valores financeiros devem ser representados exatamente.

Operações com dinheiro não podem utilizar `float32` ou `float64`.

---

## BR-005 — Precisão monetária

Valores BRL devem possuir duas casas decimais.

---

## BR-006 — Moeda deve ser explícita

Toda operação financeira deve possuir uma moeda identificável.

---

# 3. Idempotência

## BR-007 — Uma chave não pode gerar dois efeitos financeiros

Uma mesma Idempotency-Key não pode resultar em dois débitos para
a mesma operação lógica.

---

## BR-008 — Replay deve retornar o resultado existente

Quando uma requisição idêntica for reenviada após processamento,
o sistema deve retornar o resultado existente.

---

## BR-009 — Payload diferente é conflito

Se a mesma Idempotency-Key for utilizada com conteúdo diferente,
a operação deve ser rejeitada.

Nenhum novo efeito financeiro deve ocorrer.

---

## BR-010 — Idempotência deve ser persistente

A garantia de idempotência não pode depender somente de memória.

O estado deve sobreviver a:

- restart;
- crash;
- múltiplas instâncias;
- redelivery.

---

# 4. Concorrência

## BR-011 — Concorrência deve preservar saldo

Duas operações simultâneas sobre uma mesma carteira não podem
corromper o saldo.

---

## BR-012 — Não utilizar lock global

A aplicação não deve utilizar um mutex global para proteger
todas as carteiras.

A sincronização deve permitir concorrência entre carteiras distintas.

---

## BR-013 — Carteiras independentes podem processar em paralelo

Uma operação sobre a carteira A não deve bloquear desnecessariamente
uma operação sobre a carteira B.

---

## BR-014 — Decisão financeira deve ser transacional

A decisão:

```text
saldo suficiente?
        ↓
debitar
        ↓
persistir
```

deve ocorrer dentro da unidade transacional apropriada.

---

# 5. Ledger

## BR-015 — Ledger é append-only

Um lançamento financeiro já registrado não deve ser alterado para
modificar o histórico.

---

## BR-016 — Correções devem gerar novos lançamentos

Quando uma operação precisar ser corrigida ou revertida, deve ser
registrado novo lançamento em vez de alterar o lançamento original.

---

## BR-017 — Ledger deve permitir auditoria

O histórico deve permitir identificar:

- operação;
- carteira;
- valor;
- moeda;
- tipo;
- momento;
- referência;
- relação com operações anteriores.

---

# 6. Inbox

## BR-018 — Mensagem recebida deve possuir identidade persistente

Cada mensagem processável deve possuir uma identificação persistente.

---

## BR-019 — Mensagem processada não deve executar novamente

Uma mensagem já marcada como processada não deve repetir a operação
de negócio.

---

## BR-020 — Inbox deve sobreviver ao restart

O estado do Inbox não pode ser perdido quando o consumer for reiniciado.

---

# 7. Outbox

## BR-021 — Evento deve ser persistido antes da publicação

Um evento necessário para integração externa deve ser registrado
na Outbox na mesma unidade transacional da operação de negócio.

---

## BR-022 — Falha de publicação não desfaz operação financeira

Uma indisponibilidade temporária do broker não deve desfazer uma
transação financeira já confirmada.

---

## BR-023 — Evento pode ser publicado novamente

Consumidores externos devem ser preparados para duplicidade eventual
da publicação.

---

# 8. Mensageria

## BR-024 — Mensagem só deve ser removida após processamento seguro

O consumer não deve excluir uma mensagem da fila antes de confirmar
que o processamento foi concluído de forma segura.

---

## BR-025 — Falha deve permitir redelivery

Falhas durante o processamento devem permitir nova tentativa.

---

## BR-026 — Mensagens poison devem chegar à DLQ

Mensagens que excederem a política de tentativas devem ser encaminhadas
para DLQ.

---

# 9. Reversões

## BR-027 — Reversão não apaga o histórico

Uma reversão não deve modificar ou excluir o lançamento original.

---

## BR-028 — Reversão deve possuir referência

Uma reversão deve indicar a operação que está sendo revertida.

---

## BR-029 — Reversão deve ser idempotente

Uma mesma solicitação de reversão não deve produzir múltiplos efeitos.

---

# 10. Estados

## BR-030 — Estados devem representar transições válidas

Transações devem evoluir somente através de estados permitidos.

Exemplo:

```text
PENDING
   ↓
PROCESSING
   ↓
PROCESSED
```

ou:

```text
PENDING
   ↓
PROCESSING
   ↓
REJECTED
```

---

## BR-031 — Estado financeiro não pode ser inferido somente da fila

O estado definitivo da operação deve estar persistido.

---

# 11. Segurança

## BR-032 — Endpoint protegido exige autenticação

Operações protegidas não devem ser processadas sem credencial válida.

---

## BR-033 — Token inválido deve ser rejeitado

Token ausente, inválido ou não autorizado não deve permitir
processamento financeiro.

---

# 12. Rastreabilidade

## BR-034 — Operação deve ser rastreável

Uma operação deve poder ser relacionada através de:

```text
HTTP Request
   ↓
Idempotency-Key
   ↓
Transaction
   ↓
Wallet
   ↓
Ledger
   ↓
Outbox
   ↓
SQS Message
   ↓
Inbox
```

---

## BR-035 — Identificadores devem permanecer disponíveis

Identificadores externos e internos necessários para investigação
não devem ser descartados prematuramente.

---

# 13. Recuperação

## BR-036 — Falha não pode gerar perda silenciosa

Uma operação confirmada não pode depender de uma etapa posterior
não persistida para ser recuperável.

---

## BR-037 — Estados persistentes devem permitir recuperação

Após restart, o sistema deve conseguir identificar operações:

- processadas;
- pendentes;
- em processamento;
- falhadas.

---

# 14. Invariantes principais

As seguintes condições devem permanecer verdadeiras:

```text
1. Saldo nunca deve ficar negativo.

2. Uma operação idempotente não pode gerar efeitos duplicados.

3. Ledger existente não deve ser alterado para apagar histórico.

4. Falha de publicação não pode apagar uma operação financeira confirmada.

5. Mensagem não pode ser removida antes de processamento seguro.

6. Concorrência não pode gerar saldo incorreto.

7. Reversão não pode apagar a operação original.

8. Estado persistido deve permitir recuperação.

9. Dinheiro não pode depender de ponto flutuante.

10. Domínio não deve depender de infraestrutura.
```

---

# 15. Prioridade das regras

Em caso de conflito entre uma otimização técnica e uma regra financeira,
a consistência financeira deve prevalecer.

A implementação pode mudar.

As regras de negócio não.