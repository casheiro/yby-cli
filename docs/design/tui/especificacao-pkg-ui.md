# Especificação de `pkg/ui` (fase 0)

> Extraída da prancha aprovada (https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk), folhas 01 a 04 e "Saídas de comando".
> Este arquivo basta para implementar a fase 0; não é preciso abrir a prancha.
> Decisões tomadas na extração estão marcadas com **(decisão)**.

## 1. Paleta (`tokens.go`)

Seis papéis, cada um um `lipgloss.CompleteAdaptiveColor` (`Dark` = mapa noturno, `Light` = kraft).

| Papel | Escuro truecolor | Escuro 256 | Escuro 16 **(decisão)** | Claro truecolor | Claro 256 | Claro 16 **(decisão)** | Uso |
|---|---|---|---|---|---|---|---|
| `Tinta` | `#DDD0B5` | 187 | 15 | `#33261A` | 236 | 0 | texto, títulos |
| `TintaSuave` | `#A1937A` | 144 | 7 | `#5C4A36` | 240 | 8 | rótulos, apoio |
| `Fio` | `#4A4232` | 238 | 8 | `#A88E6A` | 137 | 7 | bordas, réguas, trecho restante |
| `Ok` | `#A7BC72` | 143 | 10 | `#4E5F2E` | 58 | 2 | ok, ativo, rodando |
| `Aviso` | `#D9AE5F` | 179 | 11 | `#7D5A10` | 94 | 3 | aviso |
| `Erro` | `#D98361` | 173 | 9 | `#86331A` | 88 | 1 | erro, destrutivo |

- O fundo é do terminal: nenhum componente pinta fundo, exceto a seleção (tinta invertida: fundo `Tinta`, texto sem cor de fundo explícita → usar `Reverse(true)` ou fundo `Tinta` + frente igual ao fundo do tema; preferir `Reverse(true)`, que funciona com `NO_COLOR`).
- **Contraste (decisão):** o teste WCAG vale para os papéis de texto (`Tinta`, `TintaSuave`, `Ok`, `Aviso`, `Erro`), ≥ 4,5:1, contra fundos típicos de terminal: escuro `#000000`, `#0C0C0C`, `#1E1E1E`, `#1F1C15`; claro `#FFFFFF`, `#FAFAFA`, `#F2F2F2`. `Fio` é decorativo e fica fora. Observação: contra o fundo kraft `#D8C8AA` da prancha, `Ok` (4,3:1) e `Aviso` (3,8:1) não passam; esse fundo é só da amostra, as TUIs não pintam fundo.
- Com `NO_COLOR`: sem cores; hierarquia só por negrito, `Reverse` e glifos.

## 2. Glifos (`glyphs.go`)

| Nome | Unicode | ASCII **(decisão)** | Significado |
|---|---|---|---|
| `Ativo` | `◆` | `*` | ativo, foco, etapa atual, marca |
| `Pendente` | `◇` | `o` | pendente, não visitado |
| `Andamento` | `◈` | `+` | em andamento |
| `Feito` | `✓` | `v` | concluído, saudável |
| `Falha` | `✕` | `x` | erro, falhou; também o "remover" do chip |
| `Alerta` | `▲` | `!` | aviso |
| `Rodando` | `●` | `*` | rodando |
| `Parado` | `○` | `o` | parado, vazio |
| `Cheio` | `━` | `=` | trecho concluído (cota, rota) |
| `Tracejado` | `┄` | `-` | trecho restante |
| `Caminho` | `▸` | `>` | separador de caminho (`staging ▸ payments`) |
| `Linha` | `─` | `-` | fio fino da régua |
| `Prompt` | `›` | `>` | prompt de campo de texto |
| `Reticencias` | `…` | `~` **(1 célula, como o original)** | truncamento |

Teclas exibidas: `↵ ⇥ ⇧ esc ␣ ↑↓`; em ASCII: `enter tab shift esc space up/down`.
Spinner: `◇ ◈ ◆ ◈` (ASCII `o + * +`).

## 3. Tema e modo (`theme.go`)

- `YBY_THEME=auto|dark|light` (padrão `auto`, que usa a detecção do lipgloss; na dúvida, escuro).
- `NO_COLOR` (qualquer valor não vazio) desliga cor.
- `YBY_ASCII=1` força glifos ASCII; também ASCII quando o terminal não declara UTF-8 (`LANG`/`LC_ALL`/`LC_CTYPE` sem `UTF-8`, e no Windows quando não é Windows Terminal (`WT_SESSION`) nem VS Code (`TERM_PROGRAM=vscode`)).
- Config `~/.yby/config.yaml`: `ui.theme`, `ui.ascii`. Precedência: flags > env > config > padrão.

## 4. Hierarquia de texto

Quatro níveis, sem tamanho de fonte: marca + instrumento (`◆ YBY  VIZ`, negrito, caixa-alta) · título (negrito, caixa-alta) · rótulo (`TintaSuave`, caixa-alta) · dado (`Tinta`) · apoio (`TintaSuave`) · selecionado (tinta invertida).

## 5. Moldura (`frame.go`, folha 02)

Linhas fixas, de cima para baixo:

1. **Masthead:** à esquerda `◆ YBY  <INSTRUMENTO>` em negrito + rótulo em `TintaSuave` (`INSTR. 05 · OBSERVAÇÃO`); à direita o contexto (`staging ▸ payments   ● ao vivo · 2 s`, caminho em `TintaSuave` exceto o último segmento).
2. **Rule:** fio horizontal em `Fio` na largura toda.
3. Abas (`tabs.go`), filtros (`filtro.go`), corpo.
4. **Rule.**
5. **StatusLine:** à esquerda uma frase (com glifo de status), à direita `atualizado HH:MM:SS` em `TintaSuave`.
6. **KeyHints:** tecla em negrito, ação em `TintaSuave`, separadas por dois espaços; `q sair` sempre no canto direito; só teclas válidas na tela.

Responsivo:
- ≥ 120 colunas: lista e detalhe lado a lado (60/40).
- 80–119: uma coluna; o detalhe abre por cima da lista.
- < 80: compacto; abas viram números (`1 Pods · 2 Deploy…`), a legenda mostra 3 teclas + `? mais`.
- < 60×16: tela única "aumente a janela".

## 6. Componentes (folha 03)

- **Tabs:** ativa `◆ Pods 8` em negrito (◆ em `Ok`); outras `◇ Deployments 4` em `TintaSuave`; `⇥ mais` quando não cabem; teclas 1–9.
- **Filtro:** `filtros  [status: com problema ✕]  [app=api ✕]` à esquerda, `8 de 42` à direita em `TintaSuave`.
- **Quadro:** painel com borda em `Fio` e rótulo em caixa-alta no fio de cima (`┌ DETALHE · POD ───┐`); com foco o fio vira `Tinta`. Variante destrutiva com fio `Erro`.
- **Table:** rótulos de coluna em `TintaSuave` caixa-alta; largura em células (`lipgloss.Width` / `ansi.StringWidth`); truncar com `…`; colunas com prioridade somem primeiro em terminal estreito; linha selecionada em tinta invertida (+ negrito no nome); itens concluídos em `TintaSuave`.
- **Status:** sempre glifo + cor + palavra: `● Running` (Ok), `✕ CrashLoopBackOff` (Erro), `▲ Pending` (Aviso), `✓ Completed` e `○ Parado` (TintaSuave).
- **Etapas:** linhas `✓ contexto do projeto   0,3 s` (✓ em Ok), `◈ kubectl get pvc -A   2,1 s` (atual, negrito), `◇ resposta` (TintaSuave), `✕` em Erro. Tempo à direita em `TintaSuave`, formato `0,3 s`, `38 s`, `1 min 12 s` (vírgula decimal).
- **Cota:** `argo cd  ━━━━━━━━━━━━━━┄┄┄┄┄┄┄┄  64%` — cheio em `Tinta`, tracejado em `Fio`; à direita percentual ou tempo restante (tempo em `Aviso` quando perto do fim).
- **Rota:** `◆━━━◆━━━◆┄┄┄◇┄┄┄◇┄┄┄◇  03/06`; marcos feitos `◆` em `Ok`, o atual `◆` negrito em `Tinta`, futuros `◇` em `TintaSuave`, trechos feitos `━`, futuros `┄` em `Fio`; abaixo os rótulos em caixa-alta separados por ` · `, o atual em negrito `Tinta`.
- **Spinner:** quadros `◇ ◈ ◆ ◈`; sem animação (quadro fixo `◈`) com `NO_COLOR` ou fora de TTY.
- **Toast:** na linha de estado, `✓ Deployment api reiniciado · 3 réplicas` ou `✕ sem permissão para escalar · RBAC`; some em 6 s.
- **Confirmar:** destrutivo = Quadro com fio `Erro`, rótulo `EXCLUIR POD`, `digite  <nome em negrito>`, campo `› ` com cursor, `esc cancelar`; ↵ só confirma com o nome exato. Reversível = ↵ confirma, esc cancela.
- **Estados:** carregando `◈ lendo pods de payments`; vazio `○ nenhum pod com esse filtro · esc limpa`; erro `✕ sem acesso ao cluster · r tenta de novo`.
- **Campo (tema huh):** título em caixa-alta negrito, `› ` + valor, descrição em `TintaSuave`.

## 7. Saídas de comando (`msg.go`, folha "Saídas de comando")

```
◆ YBY  DOCTOR                                   23/10 12:10     ← Title(instrumento, direita)

FERRAMENTAS                                                       ← Section (TintaSuave, caixa-alta)
  ✓ docker      27.3                                              ← Check(ok, nome, versão, nota)
  ▲ helm        3.12      recomendado 3.14 ou mais · yby setup atualiza
  ✕ kubectl     ausente   winget install Kubernetes.kubectl
  ○ az          ausente   opcional                                 ← Check(neutro) todo em TintaSuave

RESUMO  4 ok · 1 aviso · 1 erro  · o erro impede yby up          ← Summary
```

Colunas de `Check`: nome com 12 células e versão com 10; quando o texto enche a coluna, segue com 2 espaços. Para alinhar um relatório com nomes longos (ex.: `Prometheus Operator`), `CheckCom(w, ColunasPara(nomes...), …)` alarga a coluna do nome até o mais largo + 2.

`Steps` (yby up): `  ✓ cluster yby-local   criado   38 s` / `  ◈ argo cd   ━━━━┄┄ 64%   25 s` / `  ◇ root-app`.

Erro (`RenderError(*errors.YbyError)`):

```
✕ Não deu para falar com o cluster staging            ERR_CLUSTER_OFFLINE
  causa     o token do EKS expirou há 12 min
  faça      yby cloud refresh --provider aws          ← hint, em negrito
  detalhes  rode de novo com --log-level=debug
```

## 8. Teclas (`keys.go`, folha 04)

Global (toda TUI): `?` ajuda · `esc` fecha camada de cima / limpa filtros / cancela · `q` sai (só no nível de cima) · `ctrl+c` sai de qualquer lugar · `r` recarrega.

Listas (Viz, Cloud): `↑↓ j k` mover · `pgup pgdn` página · `g G` início e fim · `⇥ ⇧⇥ 1–9` aba · `/` buscar por nome · `f` filtrar · `n` namespace · `↵` abrir · `x` ações · `l` logs · `y` yaml · `e` eventos · `b` perguntar ao Bard.

Texto (Bard, assistentes): `↵` enviar/continuar · `⇧↵` nova linha · `/` comandos (início da linha) · `↑` editar última mensagem · `pgup pgdn` rolar · `esc` interrompe a resposta · `⇧⇥` voltar uma etapa · `␣` marcar · `a` marcar todos.

Níveis: Quadro →(↵) Detalhe →(x) Ações →(d) Confirmação; `esc` sobe um nível. Token do Cloud em `t`; reiniciar em `x ▸ r`.

## 9. Tom

Português com acento, verbos diretos, Kubernetes com os nomes originais. Exemplos: `◈ gerando resposta · 4 s`, `◈ lendo pods de payments`, `✕ cluster fora do ar · token expirado`, `digite api-7f9c-x2k para excluir`.
