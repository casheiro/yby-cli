---
name: retomar
description: Retoma o trabalho no yby-cli de onde a última sessão parou — lê o estado da sessão, confere branch e ambiente e apresenta a próxima tarefa do plano. Use no início de uma sessão ou quando o usuário disser "retomar", "continuar", "onde paramos", "o que falta".
---

# Retomar sessão

O estado do trabalho vive em três lugares, nesta ordem de autoridade:

1. `agile/sessoes/estado.md` — projeto ativo, plano, branch, próxima ação, alertas e ambiente.
2. O plano apontado em **Plano** (hoje `agile/planning/tui-redesign/tasks.md`) — tarefas com `[ ]`, `[~]`, `[x]`, `[!]` e o Histórico.
3. `agile/sessoes/diario.md` — o que aconteceu nas sessões anteriores (entrada mais recente no topo).

## Passos

1. **Ler o estado.** Rode `bash scripts/sessao.sh`. Se o hook `SessionStart` já trouxe essa saída nesta sessão, não rode de novo.
2. **Ler o contexto da próxima tarefa.** Leia só o bloco da próxima tarefa no plano e a entrada mais recente do diário. Leia `docs/design/tui/briefing.md` ou a prancha apenas se a tarefa pedir.
3. **Conferir antes de mexer em qualquer coisa:**
   - **Branch.** Se a atual for diferente da esperada, avise. Nunca troque de branch com arquivos alterados sem perguntar.
   - **Arquivos alterados de uma sessão anterior.** Mostre quais são e pergunte se continuam, viram commit ou são descartados. Nunca descarte por conta própria.
   - **Go no PATH.** Se o script avisar que está fora, prefixe todo comando Go com `export PATH="$PATH:/c/Program Files/Go/bin" &&`.
   - **Tarefa `[~]`.** Ela está em andamento; continue dela, não comece outra.
4. **Apresentar ao usuário em até 6 linhas:** onde paramos, progresso (N/M), a próxima tarefa, alertas (branch, arquivos alterados, pendências que esperam o usuário) e a pergunta "sigo com a Txxx?". Se o usuário já pediu para continuar, comece direto.
5. **Ao começar uma tarefa,** marque-a `[~]` no plano. Esse passo não gera commit.

## Regras durante a sessão

- **Uma tarefa = um commit** com o código, a caixa marcada `[x]` e uma linha no Histórico do plano (data, tarefa, o que foi feito, hash do commit).
- **Antes do commit,** rode o "Verificar" da tarefa. Se não passar, a tarefa continua `[~]`.
- **Tarefa bloqueada:** marque `[!]` e registre o motivo no Histórico.
- **Mudança de direção pedida pelo usuário:** registre no diário e, se mudar o plano, edite o plano no mesmo commit.
- **Push e PR** só quando o usuário pedir.
