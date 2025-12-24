---
description: Workflow para descobrir e estruturar novas demandas
---
# Workflow: Work Discovery

Este workflow é o ponto de partida para qualquer nova tarefa complexa no repositório.

## 1. Trigger
- Uma nova feature é solicitada.
- Um bug complexo precisa ser investigado.
- Uso do comando `/work-discovery`.

## 2. Passos

### Passo 1: Levantamento de Contexto
- Ler `.synapstor/00_PROJECT_OVERVIEW.md`.
- Ler `.synapstor/02_BACKLOG_AND_DEBT.md`.
- Pesquisar UKIs relacionadas em `.synapstor/.uki/`.

### Passo 2: Definição do Problema
- Perguntar ao usuário: "Qual o objetivo de sucesso dessa tarefa?"
- Perguntar se existem restrições (ex: não quebrar compatibilidade).

### Passo 3: Rascunho de Caminho
- Se for uma feature grande: Sugerir rodar `work-solution-design`.
- Se for implementação direta: Sugerir um plano de passos.

## 3. Saída Esperada
- Um resumo claro do que deve ser feito.
- Links para as UKIs que devem ser respeitadas.
