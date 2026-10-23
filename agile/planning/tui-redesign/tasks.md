# Tasks: Carta do terminal (redesign das TUIs e saídas do Yby CLI)

> **Fonte da verdade do progresso.** Toda sessão de trabalho começa e termina neste arquivo.
> Briefing: [docs/design/tui/briefing.md](../../../docs/design/tui/briefing.md) · Plano: [docs/design/tui/plano-migracao.md](../../../docs/design/tui/plano-migracao.md) · Prancha aprovada: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk
> **Ordem:** fase 0 → 2 → 3 → 1 → 4 → 5. Dentro de cada fase, na ordem dos IDs.
> **Verificação geral:** `go test ./pkg/... ./plugins/... ./cmd/...` + `go test -tags k8s ./plugins/sentinel/...` + `golangci-lint run` + `task build`.

## Como retomar

Use `/retomar` ao abrir a sessão e `/encerrar` ao parar. Elas seguem os passos abaixo e mantêm `agile/sessoes/estado.md` e `agile/sessoes/diario.md` atualizados.

1. A próxima tarefa é a primeira marcada `[~]` (em andamento). Se não houver nenhuma, é a primeira `[ ]` na ordem deste arquivo.
2. Antes de mexer no código, leia o "Verificar" da tarefa e a folha correspondente da prancha.
3. Ao terminar uma tarefa, faça **um commit** com o código, a caixa marcada `[x]` e uma linha no Histórico (fim do arquivo).
4. Ao terminar uma fase, rode o **Checkpoint** da fase e só então abra o PR.

**Legenda:** `[ ]` pendente · `[~]` em andamento · `[x]` concluída · `[!]` bloqueada (motivo no Histórico)

**Branch:** `feat/tui-carta`, criada a partir de `origin/main`, separada do multicloud. As correções para Windows (`.exe`, descoberta de plugins, `OLLAMA_HOST`, download do modelo de embeddings) estão em `feat/multicloud-support` e chegam aqui quando ela entrar na `main`; antes da fase 2 (Bard), atualizar esta branch com a `main`. Um PR por fase.

---

### Fase 0 · Fundação `pkg/ui`

**Execução com agentes** (especificação: [docs/design/tui/especificacao-pkg-ui.md](../../../docs/design/tui/especificacao-pkg-ui.md)):

| Onda | Tarefas | Agente | Modo |
|---|---|---|---|
| 0 | T001 + T002 + helpers de teste | 1 Sonnet | na branch, em sequência; define a API usada pelas outras |
| 1 | A: T003 + T004 · B: T005 (abas, filtro) + T006 · C: T007 + `quadro.go` + T008 · D: T009 sem `Steps` | 4 Sonnet | em paralelo, cada um num worktree, um commit por tarefa, sem tocar neste arquivo |
| 2 | `Steps` do T009 + T010 | 1 Sonnet | na branch, depois da integração da onda 1 (`Steps` usa etapas e cota do C) |
| — | integração, verificação, registro no plano | sessão principal | cherry-pick por tarefa, testes da fase |

Só a onda 1 C mexe em `go.mod` (teatest). `pkg/ui/helpers_test.go` é da onda 0; as outras não o editam.

- [x] T001 Criar `pkg/ui/tokens.go` e `pkg/ui/glyphs.go`.
  - Paleta como `lipgloss.CompleteAdaptiveColor` nas variantes mapa noturno e kraft (truecolor + ANSI 256 + ANSI 16), com os papéis `Tinta`, `TintaSuave`, `Fio`, `Ok`, `Aviso` e `Erro` e os valores da folha 01 da prancha.
  - Glifos `◆ ◇ ◈ ✓ ✕ ▲ ● ○ ━ ┄ ▸` com fallback ASCII.
  - Verificar: testes com perfil de cor forçado retornam os códigos esperados; contraste ≥ 4,5:1 nos dois temas (teste com a fórmula WCAG).
  - Arquivos: `pkg/ui/tokens.go`, `pkg/ui/glyphs.go`.
- [x] T002 Criar `pkg/ui/theme.go` com a detecção de tema e modo.
  - Respeitar `NO_COLOR`, `YBY_THEME=auto|dark|light`, `YBY_ASCII=1`, e os campos `ui.theme`/`ui.ascii` em `pkg/config` (precedência flags > env > config > padrão).
  - Fallback ASCII quando o terminal não declara UTF-8.
  - Padrão: mapa noturno.
  - Verificar: tabela de testes cobrindo cada combinação de variáveis.
  - Arquivos: `pkg/ui/theme.go`, `pkg/config/`.
- [x] T003 Criar `pkg/ui/keys.go` com o keymap padrão.
  - `bubbles/key` com as teclas da folha "Navegação e atalhos" (global, listas, texto) e integração com `bubbles/help` para a tela `?`.
  - Verificar: nenhuma tecla aparece em dois grupos com ações diferentes (teste).
  - Arquivos: `pkg/ui/keys.go`.
- [x] T004 Criar `pkg/ui/frame.go` com a moldura comum.
  - `Masthead` (marca, instrumento, rótulo, contexto à direita), `Rule`, `StatusLine` (mensagem + hora), `KeyHints` (`q sair` sempre à direita) e o layout responsivo (≥120 lado a lado; 80–119 por cima; <80 compacto; <60×16 tela "aumente a janela").
  - Verificar: golden files em 3 larguras e 2 temas.
  - Arquivos: `pkg/ui/frame.go`, `pkg/ui/testdata/`.
- [x] T005 Criar `pkg/ui/tabs.go`, `pkg/ui/filtro.go` e `pkg/ui/quadro.go`.
  - Abas com ◆/◇ e contagem, com "⇥ mais" quando não cabem.
  - Chips de filtro com ✕ e "N de M".
  - Painel com rótulo no fio de cima; o foco troca o fio para tinta.
  - Verificar: golden files.
- [x] T006 Criar `pkg/ui/table.go`.
  - Larguras em células (`lipgloss.Width`), nunca em bytes; truncamento com `…` respeitando UTF-8.
  - Colunas com prioridade, que somem em terminal estreito.
  - Linha selecionada em tinta invertida; itens concluídos em tinta suave.
  - Verificar: testes com nomes acentuados e glifos largos; nenhuma linha passa da largura.
- [x] T007 Criar `pkg/ui/etapas.go`, `pkg/ui/cota.go`, `pkg/ui/rota.go` e `pkg/ui/spinner.go`.
  - Etapas (✓ ◈ ◇ ✕) com tempo.
  - Cota `━━┄┄` com percentual ou tempo restante.
  - Rota `◆━━◆┄┄◇` com rótulos.
  - Spinner `◇ ◈ ◆ ◈`.
  - Verificar: golden files; spinner sem animação com `NO_COLOR` ou fora de TTY.
- [x] T008 Criar `pkg/ui/toast.go`, `pkg/ui/confirmar.go` e `pkg/ui/estados.go` (modelos Bubbletea).
  - Toast que some em 6 s.
  - Confirmação destrutiva por nome (↵ só com o nome certo) e reversível por ↵/esc.
  - Estados vazio, carregando e erro com textos diferentes.
  - Verificar: testes com `teatest` para digitar o nome errado, o certo, e esc.
- [~] T009 Criar `pkg/ui/huh.go` e `pkg/ui/msg.go`.
  - `HuhTheme()`.
  - Saída sem TUI: `Title`, `Section`, `Check(ok|aviso|erro|neutro)`, `Summary`, `Steps`.
  - `RenderError(*errors.YbyError)` com título, causa, "faça" (hint) e código.
  - Verificar: golden files do doctor, do up e do erro da folha "Saídas de comando".
- [ ] T010 Criar o catálogo visual `tools/ui-catalogo/main.go` e documentar.
  - `go run ./tools/ui-catalogo [--tema claro|escuro] [--largura N]` mostra todos os componentes, para conferir no terminal real.
  - Seção "Interface" no `CLAUDE.md`: toda saída para pessoas usa `pkg/ui`.
  - Verificar: catálogo conferido no Windows Terminal (escuro e claro), com `NO_COLOR=1` e com `YBY_ASCII=1`.

**Checkpoint fase 0:** cobertura de `pkg/ui` ≥ 80%, lint limpo, catálogo conferido com a prancha → PR "feat(ui): pacote pkg/ui da carta do terminal".

---

### Fase 2 · Bard (`plugins/bard`)

- [ ] T011 Refatorar o fluxo de resposta em eventos.
  - O `sendMessage` atual (`tui/model.go`) vira um pipeline que emite `EtapaMsg` (contexto, intenção, ferramenta, resposta), `TrechoMsg` (pedaços da resposta) e `FimMsg` por canal.
  - Cancelamento por contexto; retry igual ao do REPL antigo (`main.go`).
  - Verificar: testes com provider falso que emite pedaços e erro 429.
- [ ] T012 Implementar a moldura e as mensagens novas.
  - Cabeçalho com o provider (preencher `Config.AIModel` em `main.go`).
  - Mensagens em blocos (VOCÊ · hora / ◆ BARD · hora · ferramentas · tempo).
  - Glamour com largura do terminal e estilo derivado dos tokens; tabelas em largura total.
  - Verificar: golden das telas "conversa" e "tema kraft".
- [ ] T013 Corrigir teclado e rolagem.
  - O histórico só rola com pgup/pgdn e a roda do mouse; digitar não rola mais.
  - `↑` edita a última mensagem; `⇧↵` faz nova linha; `esc` interrompe a resposta.
  - Indicador "↓ N mensagens novas" quando a tela está rolada para cima.
  - Verificar: `teatest` digitando "jkdufb " sem mudar a rolagem.
- [ ] T014 Mostrar etapas e ferramentas.
  - `ui.Etapas` durante o trabalho, com ↵ para ver a saída de uma ferramenta.
  - Bloco "COMANDO · c copia" (copiar via OSC 52).
  - Linha de estado "◈ gerando resposta · N s".
  - Verificar: golden da tela "trabalhando".
- [ ] T015 Implementar os comandos `/` com lista filtrável.
  - `/sessões`, `/limpar`, `/contexto`, `/ferramentas`, `/modelo`, `/copiar`, `/exportar`, `/ajuda`, reaproveitando `history.go`.
  - Verificar: `teatest` para filtrar, ⇥ completar, ↵ executar e esc fechar; golden da tela "comandos".
- [ ] T016 Migrar one-shot e batch e fazer a limpeza.
  - `-p` e batch passam a usar `pkg/ui/msg.go`.
  - Remover o REPL antigo (`YBY_BARD_LEGACY_UI`), `maxToolIterations` e `promptStyle`.
  - Verificar: `echo "..." | yby bard` e `yby bard -p` funcionam sem TTY; nada interativo trava.

**Checkpoint fase 2:** as 4 telas do Bard batem com a prancha no terminal real → PR "feat(bard): nova TUI da carta do terminal".

---

### Fase 3 · Viz (`plugins/viz`)

- [ ] T017 Atualizar dependências, sem mudança visual (commit isolado).
  - `go.mod` passa a exigir `github.com/casheiro/yby-cli` com `replace => ../../`.
  - Bubbletea sobe de v0.25.0 para v1.3.6 e o lipgloss é alinhado.
  - Verificar: testes atuais do Viz passam e `go build -C ./plugins/viz` compila.
- [ ] T018 Corrigir os bugs antes do visual.
  - `selectedResourceInfo` (`internal/ui/model.go:352-394`) passa a usar a lista filtrada pela busca.
  - Scale e restart de StatefulSet param de chamar as funções de Deployment (`model.go:934-948`).
  - Lista vazia por filtro deixa de mostrar "Carregando".
  - Verificar: testes de regressão para cada bug.
- [ ] T019 Implementar moldura, abas e tabela.
  - Abas com contagem e ◆.
  - `ui.Table` com ordenação "problemas primeiro".
  - Chips de filtro com "N de M".
  - Verificar: golden da tela "pods com detalhe" (lista).
- [ ] T020 Aplicar as teclas novas.
  - `/` busca por nome, `n` troca namespace, `f` filtra status e label, `r` recarrega, `x` abre ações; mouse ativo; tela `?`.
  - Quando uma tecla antiga for usada (`d`, `s`, `L`, `S`), a linha de estado avisa onde a ação foi parar.
  - Verificar: `teatest` do mapa de teclas.
- [ ] T021 Implementar o detalhe responsivo.
  - Painel ao lado (≥120 colunas) ou por cima (<120) que acompanha a seleção.
  - Logs, YAML e eventos dentro de `ui.Quadro`.
  - Verificar: golden em 2 larguras.
- [ ] T022 Implementar o menu de ações e a confirmação.
  - Menu `x` contextual por tipo de recurso, confirmação por nome para excluir, ↵ para reiniciar e escalar, resultado em `ui.Toast`.
  - Verificar: teste "com busca ativa, a ação atinge a linha selecionada na tela"; golden da tela "ações e confirmação".

**Checkpoint fase 3:** as 2 telas do Viz batem com a prancha; rodado contra um cluster k3d local → PR "feat(viz): nova TUI da carta do terminal".

---

### Fase 1 · Core: erros, prompts e saídas (`cmd/`, `pkg/`)

- [ ] T023 Usar `ui.RenderError` em `handleExecutionError` (`cmd/root.go`).
  - Com `--log-format json`, a saída fica idêntica à de hoje.
  - Verificar: testes de erro com e sem hint, em texto e em JSON.
- [ ] T024 Unificar os prompts.
  - `cmd/prompt.go` aplica `ui.HuhTheme()`.
  - `env create` (`fmt.Scanln`) e `uninstall` (`bufio` y/N) passam a usar o `Prompter`.
  - `destroy` confirma por nome.
  - O opt-in de telemetria não trava sem TTY: nesse caso fica desligado.
  - Verificar: comandos com stdin não interativo não travam (teste com pipe).
- [ ] T025 Transformar o init num único `huh.Form` com grupos e `ui.Rota`.
  - Permitir voltar (⇧⇥) e não gravar nada antes do fim.
  - Verificar: golden do assistente; `--non-interactive` igual ao de hoje.
- [ ] T026 Migrar as saídas de doctor, status, setup e validate.
  - Verificar: golden; fim de "✅ ✅" e de strings vazias de `SetString`.
- [ ] T027 Migrar as saídas de bootstrap vps/cluster, up, access, destroy, uninstall e upgrade.
  - `pkg/executor` e `pkg/mirror` passam a usar `ui.Steps`.
  - Verificar: golden do `up` (etapas e cota).
- [ ] T028 Migrar as saídas restantes e remover os estilos antigos.
  - secret (todos), uki, env, plugin, chart, logs e cloud list/status/audit/refresh.
  - Remover `cmd/style.go`.
  - Verificar: `grep` sem emoji nem `lipgloss.Color(` fora de `pkg/ui` em `cmd/` e `pkg/`; saídas JSON e export inalteradas.

**Checkpoint fase 1:** todos os comandos do core conferidos no terminal real e com `NO_COLOR` → PR "feat(cli): erros, prompts e saídas da carta do terminal".

---

### Fase 4 · Cloud Dashboard (`cmd/cloud_dashboard.go`)

- [ ] T029 Construir o painel novo.
  - Moldura comum e resumo antes da tabela.
  - Validade do token como `ui.Cota`, com aviso abaixo de 5 min.
  - Detalhe com identidade e auditoria recente (`pkg/cloud/audit.go`).
  - Teclas: `t` renova, `r` recarrega, `esc` só fecha o detalhe.
  - Verificar: golden; teste da cota em função do tempo restante.
- [ ] T030 Fazer ↵ abrir o cluster escolhido no Viz.
  - O Viz passa a aceitar contexto e namespace iniciais pelos args do plugin.
  - Verificar: teste do comando montado; manual com um cluster.

**Checkpoint fase 4:** tela confere com a prancha → PR "feat(cloud): painel da carta do terminal".

---

### Fase 5 · Plugins de saída, ligações e documentação

- [ ] T031 Migrar a saída e a ajuda do Sentinel.
  - `investigate` e `scan` usam `ui.Quadro`, `ui.Check` e `ui.Steps`.
  - Acentos corrigidos.
  - O relatório markdown não muda.
  - Verificar: `go test -tags k8s ./plugins/sentinel/...`; golden.
- [ ] T032 Migrar a saída e a ajuda do Synapstor (capture, study, search, index, quality, export) e do Atlas.
  - Verificar: golden; `go test` em `plugins/synapstor` e `plugins/atlas`.
- [ ] T033 Fazer `b` no Viz abrir o Bard com a pergunta preenchida sobre o recurso selecionado.
  - Verificar: teste do request montado.
- [ ] T034 Criar a guarda de regressão.
  - Teste ou script no CI que falha se aparecer emoji de status ou `lipgloss.Color(` fora de `pkg/ui`.
  - Verificar: pipeline `feature-pipeline.yaml` roda a guarda.
- [ ] T035 Atualizar a documentação.
  - Capturas novas com fitas do VHS; wiki e README atualizados.
  - Changelog com as mudanças de teclas do Viz e do Cloud.
  - Prancha marcada como implementada.
  - Verificar: links e capturas conferidos.

**Checkpoint final:** todas as caixas `[x]`; verificação geral passando; revisão das telas reais contra a prancha.

---

## Dependências

```
T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009 → T010   (fase 0, base de tudo)
T010 → T011 → T012 → T013 → T014 → T015 → T016                        (fase 2, Bard)
T010 → T017 → T018 → T019 → T020 → T021 → T022                        (fase 3, Viz)
T010 → T023 → T024 → T025 → T026 → T027 → T028                        (fase 1, core)
T010 → T029 ; T022 → T030                                             (fase 4, Cloud; T030 precisa do Viz novo)
T028 → T031 → T032 ; T016 + T022 → T033 ; T028 → T034 → T035           (fase 5)
```

## Histórico

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 23/10/2026 | — | Plano criado a partir do briefing e do plano de migração aprovados | — |
| 23/10/2026 | T001 | `tokens.go` e `glyphs.go`: paleta adaptativa (truecolor/256/16), `Contraste` WCAG, glifos Unicode/ASCII com mesma largura; 100% de cobertura | `260b880` |
| 23/10/2026 | T002 | `theme.go`: `ResolverOpcoes` (flags > env > config), `Configurar`, `S()`, `Colorir`; `NO_COLOR` mantém negrito e reverse; `ui.theme`/`ui.ascii` em `pkg/config`. Entregue junto da T001 (arquivos interdependentes) | `260b880` |
| 23/10/2026 | T009 | `huh.go` (`HuhTheme`) e `msg.go` (`Title`, `Section`, `Check`, `Summary`, `RenderError`); golden do doctor e do erro. `Steps` fica para a onda 2 | `1b9e493` |
| 23/10/2026 | T003 | `keys.go`: grupos Global, Lista e Texto com `bubbles/key`, `ShortHelp`/`FullHelp`, `NovoAjuda` | `fc7d96b` |
| 23/10/2026 | T004 | `frame.go`: `Masthead`, `Rule`, `StatusLine`, `KeyHints`, `Moldura` responsiva (lado a lado, coluna, compacto, janela pequena); golden 60/100/140 × 2 temas | `4110330` |
| 23/10/2026 | T005 | `tabs.go` e `filtro.go` (`29f499f`); `quadro.go` entrou no commit da T008 | `29f499f` · `37e7698` |
| 23/10/2026 | T006 | `table.go`: colunas por prioridade, truncamento em células, `Status`; testes com acentos, CJK e emoji de 1 a 200 colunas | `5aab19b` |
| 23/10/2026 | T007 | `etapas.go`, `cota.go`, `rota.go`, `spinner.go`; `FormatarDuracao` em pt-BR; spinner parado sem TTY ou sem cor | `e0b089c` |
| 23/10/2026 | T008 | `quadro.go`, `toast.go`, `confirmar.go`, `estados.go`; teatest em `pkg/ui/uiteatest` (a flag `-update` do teatest colide com a do pacote) | `37e7698` |
