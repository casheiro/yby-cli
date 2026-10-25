# Tasks: Carta do terminal (redesign das TUIs e saídas do Yby CLI)

> **Fonte da verdade do progresso.** Toda sessão de trabalho começa e termina neste arquivo.
> Briefing: [docs/design/tui/briefing.md](../../../docs/design/tui/briefing.md) · Plano: [docs/design/tui/plano-migracao.md](../../../docs/design/tui/plano-migracao.md) · Prancha aprovada: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk
> **Ordem:** fase 0 → 1 → 2 → 3 → 4 → 5. Dentro de cada fase, na ordem dos IDs. Exceção na fase 1: T023 e T026 vêm primeiro (decisão de 24/10/2026).
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
- [x] T009 Criar `pkg/ui/huh.go` e `pkg/ui/msg.go`.
  - `HuhTheme()`.
  - Saída sem TUI: `Title`, `Section`, `Check(ok|aviso|erro|neutro)`, `Summary`, `Steps`.
  - `RenderError(*errors.YbyError)` com título, causa, "faça" (hint) e código.
  - Verificar: golden files do doctor, do up e do erro da folha "Saídas de comando".
- [x] T010 Criar o catálogo visual `tools/ui-catalogo/main.go` e documentar.
  - `go run ./tools/ui-catalogo [--tema claro|escuro] [--largura N]` mostra todos os componentes, para conferir no terminal real.
  - Seção "Interface" no `CLAUDE.md`: toda saída para pessoas usa `pkg/ui`.
  - Verificar: catálogo conferido no Windows Terminal (escuro e claro), com `NO_COLOR=1` e com `YBY_ASCII=1`.

**Checkpoint fase 0:** cobertura de `pkg/ui` ≥ 80%, lint limpo, catálogo conferido com a prancha. Sem PR próprio (decisão de 24/10/2026): `pkg/ui` sobe no PR da fase 1.

---

### Fase 1 · Core: erros, prompts e saídas (`cmd/`, `pkg/`)

- [x] T023 Usar `ui.RenderError` em `handleExecutionError` (`cmd/root.go`).
  - Com `--log-format json`, a saída fica idêntica à de hoje.
  - Verificar: testes de erro com e sem hint, em texto e em JSON.
- [x] T024 Unificar os prompts.
  - `cmd/prompt.go` aplica `ui.HuhTheme()`.
  - `env create` (`fmt.Scanln`) e `uninstall` (`bufio` y/N) passam a usar o `Prompter`.
  - `destroy` confirma por nome.
  - O opt-in de telemetria não trava sem TTY: nesse caso fica desligado.
  - Verificar: comandos com stdin não interativo não travam (teste com pipe).
- [x] T025 Transformar o init num único `huh.Form` com grupos e `ui.Rota`.
  - Permitir voltar (⇧⇥) e não gravar nada antes do fim.
  - Verificar: golden do assistente; `--non-interactive` igual ao de hoje.
- [x] T026 Migrar as saídas de doctor, status, setup e validate.
  - Verificar: golden; fim de "✅ ✅" e de strings vazias de `SetString`.
- [x] T027 Migrar as saídas de bootstrap vps/cluster, up, access, destroy, uninstall e upgrade.
  - `pkg/executor` e `pkg/mirror` passam a usar `ui.Steps`.
  - Verificar: golden do `up` (etapas e cota).
- [x] T028 Migrar as saídas restantes e remover os estilos antigos.
  - secret (todos), uki, env, plugin, chart, logs e cloud list/status/audit/refresh.
  - Remover `cmd/style.go`.
  - Verificar: `grep` sem emoji nem `lipgloss.Color(` fora de `pkg/ui` em `cmd/` e `pkg/`; saídas JSON e export inalteradas.

**Checkpoint fase 1:** todos os comandos do core conferidos no terminal real e com `NO_COLOR` → PR "feat(cli): pkg/ui e erros, prompts e saídas da carta do terminal" (leva também a fase 0).

---

### Fase 2 · Bard (`plugins/bard`)

- [x] T011 Refatorar o fluxo de resposta em eventos.
  - O `sendMessage` atual (`tui/model.go`) vira um pipeline que emite `EtapaMsg` (contexto, intenção, ferramenta, resposta), `TrechoMsg` (pedaços da resposta) e `FimMsg` por canal.
  - Cancelamento por contexto; retry igual ao do REPL antigo (`main.go`).
  - Verificar: testes com provider falso que emite pedaços e erro 429.
- [x] T012 Implementar a moldura e as mensagens novas.
  - Cabeçalho com o provider (preencher `Config.AIModel` em `main.go`).
  - Mensagens em blocos (VOCÊ · hora / ◆ BARD · hora · ferramentas · tempo).
  - Glamour com largura do terminal e estilo derivado dos tokens; tabelas em largura total.
  - Verificar: golden das telas "conversa" e "tema kraft".
- [x] T013 Corrigir teclado e rolagem.
  - O histórico só rola com pgup/pgdn e a roda do mouse; digitar não rola mais.
  - `↑` edita a última mensagem; `⇧↵` faz nova linha; `esc` interrompe a resposta.
  - Indicador "↓ N mensagens novas" quando a tela está rolada para cima.
  - Verificar: `teatest` digitando "jkdufb " sem mudar a rolagem.
- [x] T014 Mostrar etapas e ferramentas.
  - `ui.Etapas` durante o trabalho, com ↵ para ver a saída de uma ferramenta.
  - Bloco "COMANDO · c copia" (copiar via OSC 52).
  - Linha de estado "◈ gerando resposta · N s".
  - Verificar: golden da tela "trabalhando".
- [x] T015 Implementar os comandos `/` com lista filtrável.
  - `/sessões`, `/limpar`, `/contexto`, `/ferramentas`, `/modelo`, `/copiar`, `/exportar`, `/ajuda`, reaproveitando `history.go`.
  - Verificar: `teatest` para filtrar, ⇥ completar, ↵ executar e esc fechar; golden da tela "comandos".
- [x] T016 Migrar one-shot e batch e fazer a limpeza.
  - `-p` e batch passam a usar `pkg/ui/msg.go`.
  - Remover o REPL antigo (`YBY_BARD_LEGACY_UI`), `maxToolIterations` e `promptStyle`.
  - Verificar: `echo "..." | yby bard` e `yby bard -p` funcionam sem TTY; nada interativo trava.
- [x] T036 Desenhar a logo para o terminal e mostrá-la no `yby` sem argumentos.
  - `ui.Marca(largura, altura)` escolhe o desenho que cabe: grande (64×12) ou compacta (42×8) em meio-blocos, traço ASCII (60×12) com `YBY_ASCII=1`, e `◆ YBY` sem espaço. `ui.Logo(w)` só escreve num terminal.
  - Desenhos feitos à mão a partir de `docs/assets/brand/yby-logo.svg` e conferidos contra ele (decisão do usuário de 24/10/2026; antes o briefing dizia que a logo não ia para o terminal).
  - Verificar: goldens de `pkg/ui`, catálogo com a folha "marca" e `yby` sem argumentos capturado num ConPTY (larguras 100 e 56, ASCII e tema claro).
- [x] T038 Passar a ajuda dos comandos para o formato da carta e para o português.
  - `ui.EscreverAjuda` (título, descrição, USO, COMANDOS, EXEMPLOS, FLAGS, FLAGS GLOBAIS) e `cmd/ajuda.go` ligando ao cobra; `yby` sem argumentos com a logo no lugar do título.
  - Verificar: goldens de `pkg/ui`, testes de `cmd/ajuda_test.go` sem "Usage:", "Flags:" nem "help for", e capturas num ConPTY (raiz, `env create --help` em 56 colunas, tema claro e ASCII).
- [x] T037 Mostrar a logo nas boas-vindas do Bard.
  - Conversa vazia, antes da primeira pergunta: `ui.Marca` com a largura e a altura livres do histórico; some quando a conversa começa.
  - Verificar: golden da conversa vazia em 120×40 (grande), 64×20 (compacta) e 60×16 (sem logo: o cabeçalho já traz `◆ YBY  BARD`).

**Checkpoint fase 2:** as 4 telas do Bard batem com a prancha no terminal real → PR "feat(bard): nova TUI da carta do terminal".

---

### Fase 3 · Viz (`plugins/viz`)

- [x] T017 Atualizar dependências, sem mudança visual (commit isolado).
  - `go.mod` passa a exigir `github.com/casheiro/yby-cli` com `replace => ../../`.
  - Bubbletea sobe de v0.25.0 para v1.3.6 e o lipgloss é alinhado.
  - Verificar: testes atuais do Viz passam e `go build -C ./plugins/viz` compila.
- [x] T018 Corrigir os bugs antes do visual.
  - `selectedResourceInfo` (`internal/ui/model.go:352-394`) passa a usar a lista filtrada pela busca.
  - Scale e restart de StatefulSet param de chamar as funções de Deployment (`model.go:934-948`).
  - Lista vazia por filtro deixa de mostrar "Carregando".
  - Verificar: testes de regressão para cada bug.
- [x] T019 Implementar moldura, abas e tabela.
  - Abas com contagem e ◆.
  - `ui.Table` com ordenação "problemas primeiro".
  - Chips de filtro com "N de M".
  - Verificar: golden da tela "pods com detalhe" (lista).
- [x] T020 Aplicar as teclas novas.
  - `/` busca por nome, `n` troca namespace, `f` filtra status e label, `r` recarrega, `x` abre ações; mouse ativo; tela `?`.
  - Quando uma tecla antiga for usada (`d`, `s`, `L`, `S`), a linha de estado avisa onde a ação foi parar.
  - Verificar: `teatest` do mapa de teclas.
- [x] T021 Implementar o detalhe responsivo.
  - Painel ao lado (≥120 colunas) ou por cima (<120) que acompanha a seleção.
  - Logs, YAML e eventos dentro de `ui.Quadro`.
  - Verificar: golden em 2 larguras.
- [x] T022 Implementar o menu de ações e a confirmação.
  - Menu `x` contextual por tipo de recurso, confirmação por nome para excluir, ↵ para reiniciar e escalar, resultado em `ui.Toast`.
  - Verificar: teste "com busca ativa, a ação atinge a linha selecionada na tela"; golden da tela "ações e confirmação".

**Checkpoint fase 3:** as 2 telas do Viz batem com a prancha; rodado contra um cluster k3d local → PR "feat(viz): nova TUI da carta do terminal".

---

### Fase 4 · Cloud Dashboard (`cmd/cloud_dashboard.go`)

- [x] T029 Construir o painel novo.
  - Moldura comum e resumo antes da tabela.
  - Validade do token como `ui.Cota`, com aviso abaixo de 5 min.
  - Detalhe com identidade e auditoria recente (`pkg/cloud/audit.go`).
  - Teclas: `t` renova, `r` recarrega, `esc` só fecha o detalhe.
  - Verificar: golden; teste da cota em função do tempo restante.
  - Desvios e pendências (para o checkpoint da fase 4):
    - Coluna NÓS da prancha fora: o `cloud.ClusterInfo` não traz o número de nós e o painel não faz uma chamada a mais por cluster só para isso; para mostrar, cada `ListClusters` teria de trazer o dado.
    - "t renova pelo SSO (abre o navegador)" adiado: o `RefreshToken` da AWS só roda `aws eks get-token`; o login pelo navegador pede `aws sso login` com a TUI suspensa (`tea.ExecProcess`). Até lá, a linha de estado diz "rode aws sso login".
    - A validade vem do CLI de cada provider (`aws configure export-credentials`, `az account get-access-token`, `gcloud config config-helper`); chave de IAM fixa ou CLI antigo dão "desconhecido". Conferir com nuvem de verdade.
    - A AUDITORIA RECENTE só tem os registros que o yby grava: hoje o `t` do painel (com o método). Login e assume-role (`LogAuthentication`, `LogAssumeRole`) não são chamados por nenhum comando; os goldens mostram um login de exemplo.
    - "a auditoria completa → yby cloud audit" sai com `▸` (glifo `Caminho` do `pkg/ui`), não com `→`.
    - Sem teste manual num terminal de verdade (ConPTY) com o binário.
- [x] T030 Fazer ↵ abrir o cluster escolhido no Viz.
  - O Viz passa a aceitar contexto e namespace iniciais pelos args do plugin.
  - Verificar: teste do comando montado; manual com um cluster.
  - Desvios e pendências (para o checkpoint da fase 4):
    - Abaixo de 120 colunas o primeiro ↵ abre o detalhe por cima e o segundo abre no Viz; a prancha só mostra o painel largo, onde o ↵ abre direto.
    - No compacto (abaixo de 80) com o detalhe por cima, a legenda mostra `esc voltar  ↵ abrir no Viz  t renovar token` e tira o `↑↓ mover` (cabem três teclas).
    - Com o detalhe por cima (80 a 119 colunas), a legenda fica `↑↓ mover  esc voltar  ↵ abrir no Viz  t renovar token  r recarregar  ? ajuda`: o esc vem logo depois do mover para sobreviver ao corte em 80 colunas, também no ASCII.
    - O cluster fora do kubeconfig mostra na linha de estado `▲ <cluster> não está no kubeconfig: yby cloud connect …` quando cabe ao lado da hora; senão só `▲ yby cloud connect …`, e se nem assim cabe, a hora sai da linha (o comando inteiro cabe a partir de 72 colunas).
    - O ctrl+c dentro do Viz aberto pelo painel sai de tudo: o Viz sai com 130 (como o Bard) e o painel fecha junto; o q do Viz volta ao painel.
    - Se o Viz sai com erro depois de abrir, o painel mostra um toast com o código; o texto que o Viz escreveu no stderr some quando o painel volta à tela alternativa.
    - Teste manual com um `aws` falso (sem nuvem de verdade) e o k3s de teste: o contexto pelo ARN do EKS abriu o Viz e o painel voltou com as teclas e o mouse.

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
- [ ] T039 Fazer o `/modelo` do Bard trocar também o modelo dentro do provider.
  - Hoje troca só o provider (claude-cli, gemini...): a factory de `pkg/ai` lê o modelo só de `YBY_AI_MODEL` e do config. Precisa de uma forma de criar o provider com um modelo escolhido e de listar os modelos de cada provider.
  - Verificar: `/modelo` lista provider e modelos, a troca vale para a sessão e aparece no cabeçalho.

**Checkpoint final:** todas as caixas `[x]`; verificação geral passando; revisão das telas reais contra a prancha.

---

## Fora do plano

Achados durante o redesign que não fazem parte dele. Viram tarefa própria, fora desta branch.

- `yby setup` no Windows: instalar as ferramentas com winget, scoop ou choco (hoje só brew, apt e snap; no Windows o setup mostra os links de instalação). Confirmar o nome de cada ferramenta (kubectl, helm, k3d, direnv) em cada gerenciador e testar no Windows. Registrado em 24/10/2026, a pedido do usuário.

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
| 23/10/2026 | T009 | Ajustes: fio fino `─` na régua e prompt `›` nos campos (`015cd66`); `Steps` com etapas e cota, golden do `yby up` (`cd94087`) | `cd94087` |
| 23/10/2026 | T010 | `tools/ui-catalogo` (`--tema`, `--largura`, `--ascii`, `--sem-cor`) e seção "Interface (pkg/ui)" no `CLAUDE.md`. Falta a conferência manual no Windows Terminal | `0094a88` |
| 23/10/2026 | — | Verificação: `pkg/ui` 99,1%, `tools/ui-catalogo` 96,8%, vet/gofmt/build/`task build` ok. Falhas em `pkg/ai`, `ai/prompts`, `cloud`, `config`, `context`, `plugin`, `scaffold`, `services/secrets`, `services/setup` e `cmd` já existiam antes da fase (mesma lista em `a9b6498`, ambiente Windows). Sem `-race` (sem gcc) e sem golangci-lint | — |
| 23/10/2026 | T010 | Conferência no Windows Terminal (escuro e claro): texto de dados e seleção saíam na cor padrão do terminal; corrigido para Tinta, com teste `tinta_test.go` | `c8f6b91` |
| 23/10/2026 | T010 | Catálogo conferido pelo usuário no Windows Terminal: escuro, claro (One Half Light), `NO_COLOR=1` e `YBY_ASCII=1`; teclas com nome em texto no ASCII (`396c6b3`) | `0094a88` · `c8f6b91` · `396c6b3` |
| 23/10/2026 | Checkpoint fase 0 | `pkg/ui` 99,1% de cobertura; golangci-lint 2.15 (config do repo migrada para v2 só localmente) sem achados em `pkg/ui`, `tools`, `pkg/config`; catálogo conferido pelo usuário | — |
| 24/10/2026 | — | Ordem mudada pelo usuário: fase 1 logo após a fase 0, começando por T023 e T026; sem PR só com a lib, `pkg/ui` vai no PR da fase 1 | este commit |
| 24/10/2026 | T023 | `handleExecutionError` usa `ui.RenderError` em `saidaErro` (stderr) no formato text; json mantém os registros slog de antes; `initConfig` aplica `ui.Configurar`. `RenderError`: título sem código não tem espaços à direita, causa YbyError aninhada sem `[CODE]`, sem "detalhes" quando o hint já manda depurar. Conferido em `bin/yby.exe` (Unicode, `YBY_ASCII`, `NO_COLOR`, json) | este commit |
| 24/10/2026 | T026 | doctor, status, setup e validate com `ui.Title`, `Section`, `Check` e `Summary`; goldens em `cmd/testdata`; doctor sem despejar a saída de `docker info`/`kubectl` (`RunCombinedOutput`), `Opcional` no `CheckResult`, CRDs "sem cluster" quando a conexão falha; `ui.CheckCom`/`ColunasPara` para nomes longos. Conferido em `bin/yby.exe`. `yby setup` sem TTY trava no confirm (já travava antes; fica para a T024) | este commit |
| 24/10/2026 | T023 · T026 | Ajustes do teste do usuário no Windows Terminal: notas de `Check`, erros (`ui.WriteError`) e detalhes do `status` quebram na largura do terminal (`ui.Largura`, `ui.Quebrar`); erro depois de uma linha em branco; "detalhes · debug" só com causa; comando desconhecido, flag e argumentos inválidos viram `ERR_VALIDATION` em português com a dica de ajuda (e "você quis dizer"); `status` para nos nodes sem cluster e mostra o resto "sem cluster"; `validate` confere o helm antes; mensagens de doctor e status no tom da carta | este commit |
| 24/10/2026 | T023 · T026 | Segundo teste do usuário (janela estreita): nota de `Check` que não cabe desce inteira sob o nome (não parte `C:\Program Files\…`); `Title` na largura do terminal, direita desce se não couber; frase do `Summary` desce sob a contagem. Saída redirecionada conferida sem quebra nem cor | este commit |
| 24/10/2026 | T024 | `HuhPrompter` com `ui.HuhTheme()` (Sim/Não) e sem terminal na entrada devolve erro em vez de abrir o prompt (`Confirm` nunca devolve o padrão por engano); `ConfirmarNome` roda o `ui.Confirmar` destrutivo e o `destroy` de ambiente não-local usa ele (cancelar não destrói); `env create` e `uninstall` (novo `--yes`) usam o `Prompter`; opt-in de telemetria sem terminal não grava nada e fica desligado. Conferido com pipe no binário: setup, env create, uninstall, destroy e primeiro uso não travam. Os dois testes de setup que travavam no Windows passam | este commit |
| 24/10/2026 | T024 · T026 | Teste do usuário: `setup` só oferece instalar quando há gerenciador suportado (`SetupResult.Gerenciador`; no Windows não há) e mostra o link de instalação de cada ferramenta ausente; pergunta com margem própria (some com o formulário); `env create` avisa que não há projeto antes de pedir o nome. Destroy por nome e prompts no tema conferidos pelo usuário | este commit |
| 24/10/2026 | — | Instalação do `setup` no Windows (winget/scoop/choco) registrada em "Fora do plano", por decisão do usuário | este commit |
| 24/10/2026 | T025 | `ui.Assistente` (um `huh.Form` com um grupo por passo e a `ui.Rota` em cima; ⇧⇥ volta, esc/ctrl+c cancelam, `/` não abre filtro) e o init com os passos projeto, ambientes, cluster, módulos, IA e revisão; nada é gravado antes de "Gerar o projeto?", cancelar sai com "○ init cancelado" sem erro, sem terminal pede `--non-interactive`. Saída do init e do `--update` na carta (PROJETO, ARQUIVOS, VALIDAÇÃO, PLANO DE MERGE). Árvore do `--non-interactive` igual byte a byte à de a18740d em 7 combinações (`init_arvore_*.golden`, com o `\| default` dos templates contornado no teste); prompt do campo só "› ". Revisado por 3 lentes e conferido | este commit |
| 24/10/2026 | T027 | `ui.Progresso` (no terminal, as etapas feitas ficam e a atual e as pendentes se redesenham; fora dele, uma linha por etapa ao terminar) em up, bootstrap vps/cluster, destroy e upgrade; access, uninstall e upgrade na carta. Serviços recebem `shared.Etapas` (com `Avisar` ▲) e não escrevem na tela; comandos externos por `runnerQuieto`/`runnerAoVivo` (saída no log em debug, pedido de login do kubelogin aparece), slog desviado acima das etapas (`logger.Desviar`), `shared.CausaCurta` nas falhas do mirror e do sync. Port-forward do access avisa cada causa nova de falha em warn. Golden do up com etapas e cota; E2E ajustados (sem rodar, exige Docker). Revisado por 3 lentes e conferido | este commit |
| 24/10/2026 | T028 | secret (todos), seal, uki, env, plugin, chart, logs, gen-docs, telemetry, version, context dump e cloud list/status/audit/refresh/connect na carta, com `ui.Listagem`, `ui.Campos` e `ui.Passos` (novos em `pkg/ui/listagem.go`); hints próprios em minúscula e sem ponto, e o registro de `pkg/errors/hints.go` reescrito no mesmo tom; erro de argumento fora de `ValidArgs` traduzido. `cmd/style.go` removido: os estilos que só o `cloud_dashboard.go` usa foram para dentro dele até a T029. JSON e export iguais byte a byte (goldens `maquina/` gravados antes da mudança); `cloud audit --export` e `telemetry export` sem dados avisam em stderr. Varredura sem emoji nem `lipgloss.Color(` fora de `pkg/ui` (exceto o dashboard); testes com as falhas antigas do Windows puladas (61, iguais em a18740d), lint e build ok | este commit |
| 24/10/2026 | T025 | Bug antigo que impedia testar o init: o FuncMap do scaffold não tinha `default` (o init falhava em qualquer combinação) e a validação pós-render derrubava o scaffold com `repoURL` vazio. Agora `default` funciona como no Helm, campos críticos vazios viram ▲ no relatório (no `--offline` o `repoURL` não, o bootstrap aponta para o git-server), `domainBase` vazio é válido, o repositório digitado no assistente vai para o `repoURL` e, sem helm, a validação é uma linha só. Árvore do `--non-interactive` continua igual à linha de base com `scaffold.Apply` puro | este commit |
| 24/10/2026 | T026 · T027 · T028 | Checkpoint da fase 1 rodado pelo agente num ConPTY com capturas (xterm.js + Playwright), home e KUBECONFIG isolados. Ajustes: `status` sem cluster tira as linhas de log do client-go do detalhe (`shared.SemLogDoKlog`); `logs` e `access` mostram a mensagem do kubectl em vez de "exit status 1" (`shared.ComSaida`); secret e seal mostram o caminho relativo à pasta atual; `upgrade` consultava `casheiro-org/yby-cli` (404 sempre) e agora usa `casheiro/yby-cli`, com 404 como `ERR_VALIDATION`; descrição padrão dos ambientes em português ("ambiente local") | este commit |
| 24/10/2026 | T027 · T028 | Decisões do checkpoint, aprovado pelo usuário pelas capturas: no terminal, `Progresso.Falhar` tira as pendentes da tela (voltam se outra etapa começar); `destroy` de ambiente não local sem `YBY_CLUSTER_NAME` recusa em vez de apagar o cluster local `yby-local` (bug antigo); a causa dos erros resume a saída do comando externo (`errors.ResumirSaida`, usado por `ui.RenderError` e `shared.CausaCurta`: sem "exit status N", sem `error:` e sem o rodapé do kubectl); mensagens do `pkg/mirror` em português | este commit |
| 24/10/2026 | T025 | CI no Linux mostrou que, no Windows, o scaffold não achatava `.github/workflows/<padrão>/` (usava `filepath.Rel`, com `\`, num caminho de `fs.FS`): os goldens da árvore tinham sido gravados com esse defeito. Corrigido trabalhando com `/` e convertendo só no fim; os 7 goldens agora são iguais nos dois sistemas | este commit |
| 24/10/2026 | T036 | Logo oficial no terminal: `pkg/ui/marca.go` com `Marca` e `Logo` e os três desenhos em `pkg/ui/marca/` (feitos à mão pixel a pixel, julgados e lapidados contra o SVG; as trilhas não encostam no caule, como no original). `yby` sem argumentos mostra a logo antes da ajuda, só num terminal; `--help` continua sem. Folha "marca" no catálogo. O README já usa o SVG novo (commit do PR #199, trazido para esta branch) | este commit |
| 24/10/2026 | T038 | Ajuda de todos os comandos do core no formato da carta e em português (`ui.EscreverAjuda` + `cmd/ajuda.go`): USO, COMANDOS, EXEMPLOS, FLAGS e FLAGS GLOBAIS, tipos das flags e padrões traduzidos, `help` padrão do cobra traduzido, grupos sem linha de uso própria e exemplos que quebram com recuo. O `--help` resolve o modo visual sozinho (o cobra pula o `PersistentPreRun`), senão ignorava `YBY_THEME` e `YBY_ASCII`. `Title` sem texto à direita não deixa mais espaços no fim da linha. Os textos dos comandos (Short, Long, descrição das flags) continuam como estavam | este commit |
| 24/10/2026 | T011 | Fluxo de resposta em eventos (`plugins/bard/tui/fluxo.go`): `EtapaMsg` (contexto do projeto, entender o pedido, ferramenta, gerar resposta), `TrechoMsg` e `FimMsg` por canal; retry com o backoff do REPL antigo (2 s, 10 s, até 30 s) descartando o parcial; esc cancela por contexto sem fechar o Bard, mesmo com provider que ignora o contexto. Revisão: o histórico renderizado fica em cache e cada trecho só redesenha o parcial; panic no fluxo vira `FimMsg` com erro. Provider falso nos testes, inclusive 429 | este commit |
| 24/10/2026 | T012 | Moldura e mensagens da carta no Bard: cabeçalho com cluster ▸ namespace e provider (`Config.AIModel`), blocos VOCÊ / ◆ BARD com ferramentas e tempo, linha de estado e legenda da prancha. Novo `pkg/ui/uimarkdown` (glamour na largura do terminal com estilo dos tokens, tabelas em `ui.Tabela`, status do Kubernetes com glifo, shell em quadro), `ui.MolduraTexto` e `ui.QuadroComTecla`. Goldens "conversa" e "tema kraft". Revisão: blocos de código quebram dentro do quadro em vez de cortar | este commit |
| 24/10/2026 | T013 | Teclado e rolagem: letras vão só para o campo (bug "digitar rola a tela"), histórico rola com pgup/pgdn e a roda do mouse, "↓ N mensagens novas" (`ui.AvisoRolagem`), ↑ no campo vazio edita a última pergunta, esc interrompe, campo aceita a próxima pergunta durante a resposta. Nova linha com ctrl+j (e alt+enter): o Bubble Tea v1 não distingue ⇧↵ de ↵ no console do Windows, então a legenda mostra ctrl+j. teatest com "jkdufb " | este commit |
| 24/10/2026 | T014 | Etapas e ferramentas: bloco "◈ BARD · trabalhando · N s" com `ui.Etapas`, linha de estado "◈ gerando resposta · N s", ↵ no campo vazio abre a saída da ferramenta (camada com rolagem e "SAÍDA · c copia"), ctrl+y copia o último comando de shell por OSC 52 (o c no campo comia a primeira letra da pergunta). Relógio injetado para os goldens; golden "trabalhando" | este commit |
| 24/10/2026 | T015 | Comandos `/` com lista filtrável (prancha BardComandos): /sessões (retoma a conversa), /limpar, /contexto (quadro CONTEXTO DESTA SESSÃO), /ferramentas, /modelo (troca o provider da sessão), /copiar, /exportar e /ajuda; filtro sem acento, ⇥ completa, apelidos antigos digitados por inteiro, barra que não é comando vai como pergunta. teatest e golden "comandos" | este commit |
| 24/10/2026 | T016 | `-p` e batch pelo mesmo `tui.Fluxo` (`plugins/bard/direto.go`): só a resposta no stdout, etapas no stderr por `ui.Progresso` quando é terminal, erros por `ui.WriteError`, nada espera entrada sem TTY. Removidos o REPL antigo (`YBY_BARD_LEGACY_UI`), `maxToolIterations`, `promptStyle` e os estilos à mão. No core, o erro do plugin não sai mais duplicado e o código de saída do plugin (inclusive 128+sinal) é repassado (`plugin.SaidaDoPlugin`) | este commit |
| 24/10/2026 | T037 | Logo nas boas-vindas do Bard: na conversa vazia, a grande ou a compacta conforme a janela, alinhada ao topo como o corpo da moldura; sem espaço, o corpo fica vazio (a marca já está no cabeçalho). Some com a primeira pergunta. Goldens 120×40, 64×20 e 60×16 | este commit |
| 24/10/2026 | T016 | Checkpoint da fase 2 rodado pelo agente num ConPTY, com claude e kubectl falsos: uma linha de log no terminal durante a conversa (o download do modelo de embeddings) empurrava a moldura e deixava restos na tela. Agora o log vai para `~/.yby/logs/bard.log` enquanto a TUI está aberta, e a TUI só abre o índice do Synapstor se o projeto tiver um, como o `-p` (antes criava `.synapstor/` no projeto e baixava o modelo de 90 MB na primeira pergunta) | este commit |
| 24/10/2026 | T013 · T014 · T015 | Decisões do checkpoint da fase 2, aprovado pelo usuário pelas capturas: nova linha fica `ctrl+j` (e alt+enter), porque o Bubble Tea v1 não separa ⇧↵ de ↵ no Windows; copiar o comando fica `ctrl+y` em vez do `c` da prancha; a resposta interrompida com esc passa a ir para o histórico salvo com a marca *resposta interrompida*; a troca de modelo dentro do provider vira a T039 | este commit |
| 25/10/2026 | T017 | Dependências do Viz: `go.mod` exige `github.com/casheiro/yby-cli` com `replace => ../../` (o `main.go` usa os tipos do protocolo de `pkg/plugin` em vez de cópias locais); Bubble Tea v0.25.0 → v1.3.6, lipgloss e cobra alinhados com o core; o k8s sai do `replace` em v0.32.0 e sobe para v0.35.3, como no core, porque `pkg/plugin` já puxa o k8s. Sem mudança visual | este commit |
| 25/10/2026 | T018 | Bugs do Viz: a seleção (`selectedResourceInfo`) conta na lista da tela, com filtros e busca, e a ação guarda nome, namespace e tipo quando abre, então a lista pode mudar até a confirmação sem trocar o alvo; escalar e reiniciar StatefulSet usam as funções novas `ScaleStatefulSet`/`RestartStatefulSet`; a lista vazia diz "Carregando" só antes da primeira resposta, depois "Nenhum dos N passa nos filtros" ou "Nenhum recurso". As ações aceitam qualquer client com acesso direto (interface), o que permite testá-las com o clientset falso. Testes de regressão em `correcoes_test.go`, conferidos falhando no código antigo | este commit |
| 25/10/2026 | T019 | Viz na moldura (`ui.Moldura`): masthead `◆ YBY  VIZ` com `INSTR. 05 · OBSERVAÇÃO`, contexto do kubeconfig ▸ namespace (só o namespace sem contexto; `Contexto()` no `K8sClient` e no `RetryClient`) e `● ao vivo · 2 s`; abas na ordem da prancha (1 a 9) com ◆ e a contagem da lista visível, sem número antes da primeira leitura, compactas abaixo de 80 colunas; `ui.Tabela`/`ui.Status` com colunas em PT-BR por recurso (pods: NOME, OK, STATUS, REIN, IDADE, NÓ, IP; NAMESPACE só com todos os namespaces). Uma função por aba (`itensDaAba`) faz filtro, busca e a ordem "problemas primeiro" (falha, alerta, saudáveis, concluídos em `Linha.Concluida`; eventos ficam na ordem do tempo) e alimenta desenho, contagem, seleção e ações; a seleção acompanha o recurso quando a ordem muda. `monitor.Pod` ganha prontos, reinícios (e a hora do último), nó, IP e criação, com o STATUS como o kubectl (CrashLoopBackOff, Init:…, Terminating); Deployments e StatefulSets ganham réplicas atualizadas. Filtros em `ui.Chip` com "N de M" e, sem filtro, o resumo "8 pods · 2 com problema"; estados vazio, carregando e erro em `ui.Estado`; linha de estado com a frase mais importante e "atualizado HH:MM:SS". `styles.go`, `search.go` e os emojis saem; o plugin aplica o modo visual do core (`configurarVisual`) e o `NewK8sClient` não escreve mais na tela. Diferenças da prancha: o namespace fica só no masthead (não vira chip); a frase do pod em crash diz "reiniciou 14 vezes" sem "na última hora" (a API só dá o total); colunas de baixa prioridade somem antes de o nome ser cortado (passo no Viz, a `ui.Tabela` só tira coluna quando nem o mínimo cabe). Revisão: um relógio só (cada resposta do relógio agenda a próxima leitura, que relê a aba ativa e, em rodízio, uma aba já lida; as leituras avulsas não agendam; o detalhe não para o relógio; a lista que chega limpa o erro); label, status e busca são filtros da tela (o M do "N de M" é a lista inteira do namespace; o status só vale nos pods; label inválido não se aplica); trocar o namespace apaga as listas e as abas voltam sem número; o esc limpa os filtros e mantém o namespace; a seleção usa uma chave (o ID do Event nos eventos) e a atualização não desfaz a rolagem; pgup/pgdn levam a seleção; aviso "↓ N pods abaixo" quando a lista não cabe; células e frases numa linha só; Jobs pelas condições Complete/Failed; Services, Ingresses e ConfigMaps ordenados por nome; ctrl+c sai de qualquer nível; ? abre a lista inteira das teclas até a T020; `up/down` no ASCII; ação cancelada com glifo neutro e nomes em PT-BR. Goldens `TestTelaPods` em 100×30, 140×32, 80×24, 72×24, 60×16, 59×16, todos os namespaces (120×30) e em ASCII | este commit |
| 25/10/2026 | T020 | Teclas novas do Viz pelo `ui.Teclas` (`internal/ui/teclas.go`), nos níveis da folha Atalhos: `/` busca por nome filtrando enquanto digita (↵ confirma, esc volta ao termo de antes), `n` abre o campo do namespace, `f` filtra status ou label num campo só (com `=`, `!`, vírgula ou parênteses é label selector, senão status; vazio limpa os dois; a regra fica escrita ao lado do campo), `r` recarrega na hora ("◈ recarregando pods…" e depois "✓ pods recarregados", ou a falha), `x` abre um menu mínimo na linha de estado com as ações do tipo (r reiniciar, s escalar, d excluir), com a confirmação de hoje ("yes"); o menu com quadro e a confirmação por nome ficam para a T022; `↵` abre o YAML (nos eventos, os eventos do objeto) até o painel da T021; `g`/`G` vão ao início e ao fim. esc fecha a camada de cima (campo, ajuda, confirmação → menu → lista, detalhe) e, sem camada, limpa os filtros; q é letra no campo e nas outras camadas só avisa "q sai só da lista; esc volta"; ctrl+c sai de qualquer lugar. d, s, L e S não agem mais e a linha de estado diz onde a ação foi parar ("excluir agora fica em x ▸ d", "label agora fica em f (app=web)"); o r nos Deployments e StatefulSets lembra "reiniciar fica em x ▸ r". Mouse ligado (`tea.WithMouseCellMotion`): a roda rola a lista levando a seleção (ou o detalhe e a ajuda), o clique seleciona a linha e troca de aba, com a geometria tirada do que a moldura desenhou. Tela `?` num `ui.Quadro` no lugar da lista, com os grupos "em toda TUI" e "lista · <aba>" lado a lado quando cabem (só as teclas que valem na aba), por cima do menu e do detalhe. Legenda da prancha (↑↓ mover, ↵ abrir, / buscar, f filtrar, x ações, n namespace, ? ajuda, q sair); nas camadas o canto direito é `ctrl+c sair` (`Partes.Sair` novo na `ui.Moldura`, vazio mantém `q sair`). Revisão: a rolagem do modelo fica sempre dentro da que a tela desenha (redimensionar ou receber uma lista menor não desloca o clique); o detalhe que chega depois de abrir uma camada ou trocar de linha ou de aba não abre por baixo dela; sem conexão ou abaixo de 60×16, as teclas e o mouse não agem sobre a lista escondida (sem conexão valem q, ?, esc, r, n e as abas; na janela pequena, q e r); uma recarga por vez, com o aviso e a falha da aba que foi lida; abaixo de 80 colunas o campo, a confirmação e a tela ? não oferecem "? mais" (`Partes.SemAjuda` novo na `ui.Moldura`; `KeyHints` e `KeyHintsCom` passam a montar a legenda numa função só); teatest no go.mod como dependência direta. Testes teatest em `teclado_test.go` (cada tecla, g e G, níveis com a camada conferida antes do ctrl+c, teclas antigas, roda e clique), geometria do clique em 7 tamanhos e em ASCII, clique depois de redimensionar e de a lista encolher, detalhe atrasado, lista fora da tela, rolagem da ajuda e do detalhe, mouse nas camadas; goldens `TestTelaAjuda` (com campo, confirmação e menu a 72 colunas) e `TestTelaPods` regravados | este commit |
| 25/10/2026 | T021 | Detalhe responsivo do Viz (`internal/ui/detalhe.go`): painel `ui.QuadroNomeado` com `POD · <nome>` (o nome sem caixa-alta) pela `Partes.Detalhe` da moldura, ao lado (60/40) a partir de 120 colunas acompanhando a seleção e por cima da lista abaixo disso, aberto com ↵ e fechado com esc (q avisa, ↵ de novo abre o YAML; no compacto, esc vem primeiro na legenda). Campos por tipo (pod: status, motivo, nó, imagem, dono, recursos; deployment, statefulset, service, nó, job, ingress, configmap e evento com os seus), ÚLTIMOS EVENTOS (3, idade e glifo) e ÚLTIMA LINHA DE LOG (pods, com `panic:` em vermelho), teclas l y e no rodapé ao lado e na legenda por cima. Os extras só são lidos quando a seleção para (atraso de 300 ms por `tea.Tick` com id), com cache por recurso (validade 30 s, o relógio renova e o r relê) e descarte da resposta de um recurso que já não é o selecionado; `ui.Estado` carregando dentro do painel. l, y e e abrem logs, YAML e eventos inteiros num `ui.Quadro` que ocupa o corpo (`internal/ui/quadro.go`), com rolagem (↑↓, pgup/pgdn, g/G, roda) e `ui.AvisoRolagem`, troca entre os três e descarte de resposta velha; substitui o `ModeDetail`, o `detail.go` e o `pedirDetalhe`. Monitor: `LeitorDeDetalhe` (`EventosDoObjeto` por field selector, `LogsDoPod` com a execução anterior no CrashLoopBackOff, `YAMLDoRecurso`) no `K8sClient` e no `RetryClient`, e os campos novos lidos na mesma listagem (motivo, imagens, dono, pedidos e limites, estratégia, seletores, papéis, IP interno, endereço, chaves, kind e repetições dos eventos), com testes no clientset falso. No core: `QuadroNomeado`, `LargurasLadoALado`, `LimiteDetalheEmpilhado` e `TeclasDoPainel` em `pkg/ui`, com testes. Goldens `TestTelaPodsComDetalhe` (140×32 ao lado, 100×30 e 72×24 por cima depois do ↵, ASCII) e `TestTelaQuadroDeLogs`; `TestTelaPods` 140×32 e 120×30 regravados com o painel. Revisão: o log é pedido do contêiner do pod (`Pod.Conteiner`: o que está com problema, senão a anotação `kubectl.kubernetes.io/default-container`, senão o primeiro que não é sidecar; o quadro diz qual quando há mais de um), e as leituras do detalhe têm prazo de 10 s, para a trava de uma leitura por recurso sempre soltar; y na aba Eventos lê o YAML do Event; toda troca de recurso invalida os atrasos pendentes (o de antes não relê o recurso do cache); o r relê os extras mesmo com uma leitura em andamento e descarta a resposta de antes dele; por cima, sem espaço, saem o respiro e os campos secundários antes dos últimos eventos e da última linha de log, e a lista embaixo perde a linha em branco; esc só fecha o painel que está na tela (com a lista vazia ou a janela larga, limpa os filtros já no primeiro toque); o quadro vai até o fim do corpo, mostra `? ajuda`, relê com r mantendo a rolagem e traz a rolagem para o máximo novo quando a janela cresce. Testes do atraso com o agendador (e o valor do atraso), da renovação lendo o recurso da tela, do r no painel e no quadro, do aviso de rolagem no topo, no meio e no fim, do painel por cima em 60×16 e 80×24, do contêiner do log, da execução anterior pelo leitor de log injetável e do YAML do Event num servidor de teste; goldens 100×30 e 72×24 e do quadro regravados | este commit |
| 25/10/2026 | T022 | Menu de ações e confirmação do Viz (`internal/ui/actions.go`, folha VizConfirmar): `x` abre por cima da lista apagada (a linha da ação continua selecionada; o painel ao lado também apaga, o por cima sai) o quadro `AÇÕES · <Tipo> <nome>` com ◆ na ação focada e ◇ nas outras, tecla, rótulo e nota (`rollout restart, sem downtime`, `hoje N réplicas`, `pede confirmação`; excluir em cor de erro); ↑↓ e ↵ ou a letra escolhem, esc fecha. Contextual: deployments e statefulsets têm reiniciar, escalar, copiar nome e excluir; pods, jobs, services, configmaps e ingresses copiar nome e excluir (`monitor.DeleteResource` passa a apagar ingress); nós e eventos só copiar nome. Excluir usa `ui.NovoConfirmarDestrutivo` com o novo `ComDetalhes` do pkg/ui (resumo `worker em payments · 2 réplicas · 1 pod`, o que acontece, `Digite <nome> para confirmar`, `faltam N letras`, botões com o ↵ apagado até conferir); reiniciar usa `ui.NovoConfirmarReversivel`; escalar pede as réplicas num campo com o valor de hoje (a primeira tecla o troca) e só aceita inteiro de 0 para cima. esc volta ao menu; sem altura, a explicação sai. O alvo continua guardado quando a ação abre. O resultado vai para um `ui.Toast` na linha de estado (`✓ Deployment api reiniciado`, `✕ não deu para …`) e a lista é lida de novo; copiar o nome usa o OSC 52, que saiu do Bard para `ui.CopiarOSC52` (o ctrl+y do Bard usa o mesmo). Saem o `ActionMode`, o "yes" digitado e o `renderActionPrompt`. Testes com o clientset falso: a busca ativa atinge só a linha selecionada pelo caminho novo, nome errado não confirma, escalar e reiniciar StatefulSet e Deployment pelo menu, validação das réplicas, excluir por tipo, copiar nome; goldens `TestTelaAcoes` (menu e exclusão pela metade, e reiniciar, escalar, ASCII, 140, 72 e 60 colunas). Revisão: quando a lista sai da tela (janela abaixo de 60×16 ou conexão caída), o menu e a confirmação fecham ("ação cancelada: …" na linha de estado), e as teclas não confirmam mais uma exclusão escondida; uma ação no cluster por vez, com "◈ reiniciando Deployment api…" na linha de estado até a resposta, e as chamadas do monitor com prazo (`monitor.PrazoDaAcao`, 30 s); o Toast da falha é curto e em português ("✕ sem permissão para escalar · RBAC", "<Tipo> <nome> não existe mais", sem resposta, sem conexão; `ui.Toast.MostrarComNota` novo no pkg/ui) e o texto da API vai para o log em debug; com o painel por cima, a lista atrás do menu ocupa o corpo todo; o cabeçalho da tabela fica em tinta suave atrás do menu; as reticências dos rótulos seguem o modo (`G().Reticencias`, `~` no ASCII); o campo do nome fica num quadro de fio no diálogo completo (`ComDetalhes` do pkg/ui); a roda rola a ajuda aberta por cima do menu; os testes conferem as réplicas que chegam à API (0 e o valor de hoje), e o monitor ganha teste do ScaleDeployment e do ScaleStatefulSet | este commit |
| 25/10/2026 | Checkpoint fase 3 | Teste real do Viz num k3s em Docker (namespace payments com os dados da prancha), pelo ConPTY: com teclado ABNT2 o console manda `/` e `?` como AltGr e o Bubble Tea v1 entrega `alt+NUL` e `alt+/`; a nova `ui.TeclaAltGr` descarta o NUL e tira o alt dos símbolos (letra e dígito com alt seguem atalhos), aplicada no Viz. O Viz abre no namespace do contexto do kubeconfig, como o kubectl (`NamespaceDoContexto` no `K8sClient` e no `RetryClient`; esc não o limpa). Eventos não repetem a razão quando a mensagem já começa por ela ("Created Created container") | este commit |
| 25/10/2026 | Checkpoint fase 3 | Ajustes da revisão das capturas: as abas contam pelo watch (`monitor.Observador`, `internal/monitor/vigia.go`: um informer por tipo no namespace do filtro, lotes de no máximo 4 por segundo, `managedFields` fora do cache), sem o atraso do rodízio, que saiu; a contagem segue sendo a da lista que a aba mostraria, com label, status e busca da tela. O watch alimenta as listas das nove abas e reabre no `n`; sair (q, ctrl+c ou fim do programa) para tudo; tipo negado pelo RBAC deixa a aba sem número. O relógio de 2 s lê só a aba ativa e, com ela no watch, só confirma a conexão (o informer tenta de novo em silêncio quando a API cai). klog do client-go desligado no plugin. Janela abaixo de 60x16 cancela menu, confirmações e os campos `/`, `n` e `f` como o esc e avisa "ação cancelada: a janela ficou pequena demais" (`ui.Toast`) quando ela cresce; ajuda, quadros e detalhe por cima ficam; só ctrl+c e q (sem camada de leitura) valem nela. Seção do Viz no `CLAUDE.md` reescrita. Conferido contra o k3s: contagens do watch iguais às da listagem em payments e em todos os namespaces | este commit |
| 25/10/2026 | T029 | Painel do `yby cloud dashboard` na moldura (folha CloudPainel): masthead `◆ YBY  CLOUD` com `PAINEL DE CLUSTERS`, "N clusters · M contas" (a partir de 80 colunas) e "◈ próxima leitura em N s"; resumo antes da tabela só com os grupos que têm contagem (prontos, token expira em breve, token expirado, sem login, cluster não ativo, e "✕ aws sem resposta"); tabela CLOUD CLUSTER REGIÃO K8S STATUS TOKEN com CLOUD e CLUSTER em negrito na linha invertida (`ui.Coluna.Nome`, novo) e o nome do cluster sem corte; validade do token guardada como `time.Time` e desenhada na hora com relógio injetado: cota do que resta sobre 1 h (limitada a 0..1), ▲ atenção e tempo em Aviso abaixo de 5 min, "✕ token" e "expirou há N min" depois de vencer, "—" (`ui.G().Vazio`, novo, `-` no ASCII) e "desconhecido" sem informação (novo `ui.CotaRestante`: a `CotaTempo` avisa com a barra cheia); a barra encolhe para a coluna (`ui.LargurasDasColunas`, novo) e sai abaixo de 4 células, para o tempo nunca ser cortado. Validade lida do CLI de cada provider no `ValidateCredentials` (uma chamada a mais por provider, nunca por cluster; sem o dado, nil). Status do provider em português (criando, parado…). Provider que falha não some: os clusters dele ficam da leitura anterior como "✕ sem resposta" e a linha de estado diz o erro (com "rode aws sso login" no SSO vencido). Detalhe `ui.QuadroNomeado` (EKS/AKS/GKE · nome) com IDENTIDADE (conta mascarada, papel e método tirados do ARN, da credencial e do `.yby/environments.yaml`, token, ambiente) e AUDITORIA RECENTE (as 3 últimas do provider/cluster no `audit.log`, falha descrita em português, "SSO vencido"); o detalhe encolhe até caber (rodapé, registros, ambiente, auditoria, com o token subindo) e o quadro sempre fecha. A partir de 120 colunas fica abaixo da tabela quando cabem 3 linhas (com o aviso de rolagem) e o detalhe mais alto entre os visíveis, para o layout não mudar com a seleção; senão, ao lado; abaixo disso ↵ abre por cima com a altura que precisa, deixando à tabela a linha selecionada. Teclas: `t` renova em segundo plano com `ui.Toast` (registra no audit log com o método; renovação que termina durante uma leitura pede outra), `r` recarrega, `R` avisa que agora é `r`, esc só fecha o detalhe ou limpa a busca, `/` busca por nome, `?` ajuda com os grupos lado a lado ou empilhados e rolagem, q só no nível de cima, ctrl+c de qualquer lugar, AltGr, roda e clique do mouse. Tique de 1 s só redesenha; a leitura segue a cada 30 s. Sem terminal na entrada ou na saída, erro com a hint `yby cloud list`. `ReadEvents` do audit log lê linha a linha (uma linha cortada travava a leitura). Estilos antigos removidos. Goldens em 60, 70, 72, 80, 100, 120, 140 e 160 colunas (lista, detalhe e ajuda); tabela da cota com 59, 6, 4, 0, -12 min e desconhecido; testes do layout pela altura, do quadro sempre fechado, da seleção invertida em cor depois de rolar, do provider sem resposta e da releitura | este commit |
| 25/10/2026 | T030 | ↵ no painel do cloud abre o cluster no Viz e volta ao painel quando o Viz fecha (`tea.Exec` com um `tea.ExecCommand` que chama o hook command do Viz pelo `Manager`, com a verificação de confiança, como o `yby viz`; o mouse é religado na volta). O contexto vem, sem mexer no kubeconfig, do `kube_context` (e `namespace`) do ambiente de `.yby/environments.yaml` ligado ao cluster ou do nome que o provider dá (ARN no EKS, montado com conta e região quando o `describe-cluster` não traz, `gke_<projeto>_<região>_<nome>` no GKE, o nome do cluster no AKS), conferido nos contextos do kubeconfig; no GKE o projeto vem do `selfLink` do `gcloud container clusters list` e, sem ele, vale o único contexto `gke_*_<região>_<nome>`; o ambiente com `kube_config` próprio abre o Viz nesse arquivo (`--kubeconfig`) quando o `kube_context` está nele. Sem nenhum, a linha de estado diz `<cluster> não está no kubeconfig: yby cloud connect --provider … --region … --cluster …` (ou só o comando, que não é cortado). O ↵ reaproveita o `Manager` que o `Execute` já descobriu. A falha ao abrir vira toast com a dica do erro (ex.: `yby plugin trust`), e o ctrl+c no Viz (saída 130) fecha o painel também. Legenda e ajuda com `↵ abrir no Viz` (abaixo de 120 colunas, o primeiro ↵ abre o detalhe). O Viz passa a ler os args do pedido do core (`YBY_PLUGIN_REQUEST`): `--kubeconfig`, `--context`, `--namespace`/`-n`, ajuda no formato do core (`ui.EscreverAjuda`) e erros por `ui.WriteError`; o client usa o contexto pedido (KUBECONFIG com vários arquivos, como o kubectl), o namespace pedido vence o do contexto (`NamespaceDoContexto` vira `NamespaceInicial`) e contexto inexistente sai com `ERR_CONFIG_INVALID` e os contextos que existem na dica, sem abrir a TUI; o masthead mostra o nome do cluster (`staging ▸ payments`) e não o ARN nem o `gke_…`. Testes do alvo (ambiente, nome do provider, GKE sem projeto, ausente), da leitura do `environments.yaml`, do `kube_config` do ambiente, do ↵ largo e estreito com o comando montado rodando, da linha de estado em 80, 100, 120 e 160 colunas, do esc na legenda de 80 a 119 (também ASCII), da volta do Viz (mouse religado, toast, dica do trust, 130), das flags, da ajuda, do pedido e do contexto inválido; manual no ConPTY: `yby viz --context k3d-yby-local --namespace kube-system` abre em kube-system, e o painel com um `aws` falso abriu o Viz pelo ARN e voltou; na revisão, no ConPTY: o ctrl+c no Viz sai com 130 pelo core, o contexto inexistente com 1, e `--kubeconfig` com um arquivo cujo contexto é o ARN abriu em `staging ▸ payments` com o KUBECONFIG quebrado | este commit |
