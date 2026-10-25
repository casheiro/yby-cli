# Plano de migração — Carta do terminal

> Proposta para o portão G3. Nada no código muda até a aprovação (portão G4).
> Referências: [briefing.md](briefing.md) · prancha aprovada: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk

## Resumo

São 6 fases, cada uma com um PR próprio. A fase 0 cria o pacote `pkg/ui`, e as fases seguintes trocam o código de cada interface para usar esse pacote. Cada fase pode ser entregue sem esperar as outras: o que ainda não migrou continua funcionando como hoje.

| Fase | Entrega | Depende de |
|---|---|---|
| 0 | `pkg/ui`: tokens, glifos, moldura, componentes, tema do huh, saída de comando | — |
| 1 | Erros, prompts e saída dos comandos do core | 0 |
| 2 | Bard: TUI nova, etapas, streaming, comandos `/` | 0 |
| 3 | Viz: atualização do Bubbletea, TUI nova, correção de bugs | 0 |
| 4 | Cloud Dashboard | 0 |
| 5 | Plugins de saída (Sentinel, Synapstor, Atlas), ligações entre instrumentos, docs | 1 a 4 |

As fases 1 a 4 podem andar em paralelo depois da fase 0. A ordem sugerida é 0 → 2 → 3 → 1 → 4 → 5, porque o Bard e o Viz são as telas mais usadas e as que têm mais bugs de UX.

---

## Fase 0 — Fundação `pkg/ui`

**Arquivos novos** em `pkg/ui/`:

| Arquivo | Conteúdo |
|---|---|
| `tokens.go` | Paleta como `lipgloss.CompleteAdaptiveColor` (truecolor + ANSI 256 + ANSI 16) nas variantes mapa noturno e kraft. Papéis: `Tinta`, `TintaSuave`, `Fio`, `Ok`, `Aviso`, `Erro`. |
| `glyphs.go` | `◆ ◇ ◈ ✓ ✕ ▲ ● ○ ━ ┄ ▸` com fallback ASCII (`* o + v x ! # . = - >`). |
| `theme.go` | Detecção de tema e modo: `NO_COLOR`, `YBY_THEME=auto\|dark\|light`, `YBY_ASCII=1`, `ui.theme` e `ui.ascii` em `~/.yby/config.yaml`. Estilos lipgloss derivados dos tokens. |
| `keys.go` | Keymap padrão (`bubbles/key`) com as teclas da folha de navegação e integração com `bubbles/help` para a tela `?`. |
| `frame.go` | `Masthead`, `Rule`, `StatusLine`, `KeyHints` e o layout responsivo: 120+ colunas com detalhe ao lado; 80–119 com detalhe por cima; abaixo de 80 compacto; abaixo de 60×16 pede para aumentar a janela. |
| `tabs.go`, `table.go`, `quadro.go`, `filtro.go` | Abas com contagem, tabela com largura em células (`lipgloss.Width`) e colunas por prioridade, painel com rótulo no fio, chips de filtro. |
| `etapas.go`, `cota.go`, `rota.go`, `spinner.go` | Etapas com tempo, barra de progresso como cota, progresso de assistente e spinner `◇ ◈ ◆ ◈`. |
| `toast.go`, `confirmar.go`, `estados.go` | Mensagem temporária na linha de estado, confirmação (destrutiva por nome, reversível por ↵/esc), estados vazio, carregando e erro. |
| `huh.go` | `ui.HuhTheme()`, o tema dos prompts. |
| `msg.go` | Saída de comando sem TUI: `Title`, `Section`, `Check(ok\|aviso\|erro\|neutro)`, `Summary`, `Steps`, e `RenderError(*errors.YbyError)` com causa, "faça" e código. |

**Também:**
- `pkg/config`: campos `ui.theme` e `ui.ascii`.
- `CLAUDE.md`: convenção "toda saída para pessoas usa `pkg/ui`; `fmt.Println` com emoji e `lipgloss.Color` fora de `pkg/ui` deixam de ser aceitos".

**Testes:**
- Render com o perfil de cor forçado (Ascii, ANSI256, TrueColor).
- Golden files das peças principais.
- `NO_COLOR` e `YBY_ASCII`.
- Largura com acentos e glifos largos.

**Aceite:** pacote com cobertura acima de 80%, `go vet` e `golangci-lint` limpos, e nenhum consumidor alterado ainda.

---

## Fase 1 — Core: erros, prompts e saídas de comando

**Erros:**
- Em `cmd/root.go`, `handleExecutionError` passa a usar `ui.RenderError`.
- O `slog` continua indo para stderr como log técnico.
- Com `--log-format json`, o formato atual fica igual.

**Prompts:**
- `cmd/prompt.go` aplica `ui.HuhTheme()` em todos os campos.
- O init vira um único `huh.Form` com grupos e `ui.Rota`, com ⇧⇥ para voltar.
- `env create` (`fmt.Scanln`) e `uninstall` (`bufio`, y/N) passam a usar o `Prompter`.
- `destroy` segue o padrão de confirmação por nome.
- O opt-in de telemetria deixa de travar quando a entrada não é terminal: nesse caso fica desligado.

**Saídas:**
- `cmd/style.go` é removido.
- Migram: doctor, status, setup, validate, bootstrap vps/cluster, secret (todos), uki, env, plugin, up, access, destroy, chart, logs, upgrade, uninstall e cloud list/status/audit/refresh.
- `pkg/executor` e `pkg/mirror` passam a usar `ui.Steps`.

**Bugs que somem com a fase:**
- `checkStyle.Render("✅ ...")` imprimindo "✅ ✅";
- `stepStyle.String()` e os estilos de `pkg/executor` imprimindo vazio;
- separadores de 36 a 80 caracteres diferentes;
- palavras sem acento ("Regiao", "Versao", "ATENCAO").

**Aceite:** nenhum emoji nem `lipgloss.Color` em `cmd/` e `pkg/` fora de `pkg/ui`; testes de comando atualizados; saídas JSON idênticas às de antes.

---

## Fase 2 — Bard

**TUI** (`plugins/bard/tui/`):
- moldura comum com o provider no cabeçalho (preencher `AIModel`, hoje nunca definido);
- mensagens em blocos com autor e hora;
- ferramentas usadas visíveis, com ↵ para ver a saída;
- glamour na largura do terminal e com estilo derivado dos tokens;
- tabela, comando com `c` para copiar e `esc` que interrompe a resposta (cancelamento por contexto).

**Teclado:**
- o histórico só recebe pgup/pgdn e a roda do mouse, então digitar deixa de rolar a tela (hoje `j`, `k`, `d`, `u`, `f`, `b` e espaço rolam);
- `↑` edita a última mensagem;
- `⇧↵` faz nova linha.

**Pipeline:**
- `sendMessage` passa a emitir mensagens de etapa (contexto, intenção, ferramenta, resposta) e a resposta em pedaços, num canal lido pelo Bubbletea. Hoje a resposta só aparece no fim.
- Retry igual ao do modo antigo.

**Comandos `/`:** `/sessões`, `/limpar`, `/contexto`, `/ferramentas`, `/modelo`, `/copiar`, `/exportar` e `/ajuda`, com lista filtrável.

**Saída fora da TUI:** one-shot (`-p`) e batch usam `pkg/ui/msg.go`. Feito na T016: os dois usam o mesmo `tui.Fluxo` da TUI.

**Limpeza:**
- remover o REPL antigo (`YBY_BARD_LEGACY_UI`) quando a TUI tiver tudo o que ele tem;
- remover `maxToolIterations` e `promptStyle`, que não são usados.

Feito na T016. O `promptStyle` já não existia; saíram também `ClassifyIntent` (duplicado da TUI), `filterByThreshold` e `clearHistory`.

**Aceite:** as 4 telas da prancha reproduzidas; testes com `teatest` para foco, rolagem, `esc` e comandos `/`.

---

## Fase 3 — Viz

**Base:**
- `plugins/viz/go.mod` passa a exigir `github.com/casheiro/yby-cli` com `replace => ../../`, como o synapstor já faz;
- Bubbletea sobe de v0.25.0 para v1.3.6 e o lipgloss é alinhado.

**TUI:**
- moldura comum;
- abas com contagem e ◆;
- ordenação "problemas primeiro";
- chips de filtro com "8 de 42";
- detalhe ao lado em 120+ colunas;
- menu de ações `x`;
- confirmação por nome;
- toast na linha de estado;
- mouse.

**Teclas:** `/` busca por nome; `n` troca o namespace (hoje é `/`); `f` filtra status e label; `r` recarrega; `d`, `s` e `r` soltos saem e vão para o menu `x`.

**Bugs corrigidos:**
- ação aplicada ao recurso errado quando há busca ativa (`selectedResourceInfo` indexa a lista sem o filtro);
- scale e restart de StatefulSet chamando as funções de Deployment;
- "Carregando..." aparecendo quando o filtro só retornou zero itens.

**Aceite:** as 2 telas da prancha; testes de seleção com filtro e de ação no recurso certo.

---

## Fase 4 — Cloud Dashboard

`cmd/cloud_dashboard.go` passa a ter:
- moldura comum;
- resumo antes da tabela;
- validade do token como cota, com aviso antes de expirar;
- detalhe com identidade e auditoria recente;
- `t` para renovar, `r` para recarregar e `esc` apenas fechando o detalhe;
- ↵ abrindo o Viz no cluster escolhido (exige que o Viz aceite contexto e namespace iniciais).

**Aceite:** a tela da prancha; testes de teclas e da cota em função do tempo restante.

---

## Fase 5 — Plugins de saída, ligações e documentação

- **Saídas dos plugins:**
  - Sentinel (investigate e scan): caixa e cores próprias trocadas por `ui.Quadro` e `ui.Check`; acentos corrigidos.
  - Synapstor (capture, study, search, index, quality, export): mesma migração.
  - Atlas: mesma migração.
  - O relatório em markdown do Sentinel não muda.
- **Ajuda dos plugins:** sai do `fmt.Println` escrito à mão e passa a usar o mesmo formato do help do core.
- **Ligação Viz → Bard:** `b` abre o Bard com a pergunta preenchida sobre o recurso.
- **Documentação:**
  - capturas novas geradas com fitas do VHS;
  - wiki e README atualizados;
  - seção "Interface" no `CLAUDE.md`.

---

## Riscos e como tratar

| Risco | Tratamento |
|---|---|
| Quem já usa o Viz estranha as teclas novas | A tela `?` lista tudo. Na primeira vez que uma tecla antiga for usada (`d`, `s`, `/` como namespace), a linha de estado avisa onde a ação foi parar. Registrar no changelog. |
| Scripts que leem a saída em texto dos comandos | Saídas legíveis por máquina (JSON, export, telemetria) não mudam. Comandos sem saída estruturada recebem nota no changelog. |
| Tema detectado errado (tmux, SSH, IDE) | Override por `YBY_THEME` e `ui.theme`. Na dúvida, o padrão é o mapa noturno. |
| Terminal sem Unicode (conhost antigo do Windows) | Fallback ASCII automático quando o terminal não declara UTF-8, ou forçado por `YBY_ASCII=1`. |
| Atualização do Bubbletea no Viz quebrar algo | É o primeiro commit da fase 3, isolado e com os testes atuais passando antes de qualquer mudança visual. |
| PRs grandes demais para revisar | Uma fase por PR; a fase 1 pode ser dividida em erros+prompts e saídas. |

## Como cada fase é verificada

1. Testes unitários e golden do que mudou, com `go test` sem tags e com as tags exigidas (`k8s` no Sentinel).
2. Rodar a TUI real no Windows Terminal (escuro e claro) e conferir com a prancha.
3. Rodar com `NO_COLOR=1` e `YBY_ASCII=1`.
4. Revisão contra o checklist: glifo + palavra em todo status; teclas do mapa; textos em PT-BR com acento.

## Decisão pedida (G4)

Posso começar pela **fase 0** (criar `pkg/ui`, só arquivos novos) e seguir na ordem 0 → 2 → 3 → 1 → 4 → 5? Ao fim de cada fase eu mostro o resultado (testes e a tela rodando) antes de passar para a próxima.
