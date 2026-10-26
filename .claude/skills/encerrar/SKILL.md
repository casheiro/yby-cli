---
name: encerrar
description: Fecha a sessão de trabalho no yby-cli deixando tudo pronto para a próxima — atualiza o plano, o estado e o diário de sessões e faz o commit do registro. Use quando o usuário disser "encerrar", "fechar a sessão", "parar por hoje", "vou sair" ou antes de um /clear.
---

# Encerrar sessão

O objetivo é que a próxima sessão, aberta do zero, saiba exatamente o que aconteceu e o que fazer em seguida, sem depender desta conversa.

## Passos

1. **Levantar o que mudou nesta sessão.**
   - `git status`.
   - `git log --oneline` desde o último commit citado no diário.
   - As tarefas que passaram por `[~]` ou `[x]` no plano.

2. **Fechar as tarefas.**
   - **Concluída e verificada:** já deve estar `[x]` com o commit dela. Se faltar, faça agora.
   - **Começada e não terminada:** continua `[~]`. No Histórico do plano, registre o que falta.
   - **Código pela metade:**
     - se compila e os testes passam, faça um commit `wip(Txxx): <o que já está feito>`;
     - se não, deixe sem commit e liste os arquivos em **Atenção** no `estado.md`.
   - **Bloqueada:** `[!]` com o motivo.

3. **Reescrever a seção "Estado atual"** de `agile/sessoes/estado.md`:
   - **Formato:** mantenha `- **Nome:** \`valor\`` nos campos Projeto ativo, Plano e Branch (o script lê esses três).
   - **Próxima ação:** uma frase concreta, com o ID da tarefa e os arquivos.
   - **Aguardando o usuário:** decisões ou aprovações pendentes; "nada" se não houver.
   - **Atenção:** branch, arquivos sem commit, dependências entre frentes.
   - **Outras seções:** atualize "Ambiente" e "Outras frentes abertas" só se algo mudou.

4. **Registrar a sessão** no topo de `agile/sessoes/diario.md`, abaixo do `---`:
   - **Título:** `## DD/MM/AAAA · <branch>`.
   - **Corpo:** blocos **Feito** (com hashes), **Decisões**, **Pendente** e **Próximo passo**.
   - **Tamanho:** curto, o que importa para retomar, não um relato completo.

5. **Commit do registro:** `chore(sessao): registra sessão de DD/MM/AAAA`, com `estado.md`, `diario.md` e o plano.
   - **Autorização:** commits do fluxo de sessão foram autorizados pelo usuário em 23/10/2026.
   - **Push:** só se o usuário pedir.

6. **Mostrar ao usuário em até 5 linhas:**
   - tarefas concluídas;
   - o que ficou em andamento;
   - a próxima ação;
   - o que espera a decisão dele;
   - o hash do commit.

## Cuidados

- Nunca apague entradas antigas do diário nem linhas do Histórico do plano.
- Se a sessão mudou de assunto (outro plano ou outra branch), atualize **Projeto ativo**, **Plano** e **Branch** e mova o assunto anterior para "Outras frentes abertas".
