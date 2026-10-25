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
| `Vazio` | `—` | `-` | campo sem valor numa tabela ou num detalhe |

Teclas exibidas: `↵ ⇥ ⇧ esc ␣ ↑↓`; em ASCII: `enter tab shift esc space up/down`.
Spinner: `◇ ◈ ◆ ◈` (ASCII `o + * +`).

### Marca (`marca.go`, folha "marca")

- `Marca(largura, altura)` devolve a logo que cabe em largura×altura células (`altura <= 0` não limita), na cor `Tinta`: a grande (64×12) ou a compacta (42×8), em meio-blocos (` ▀▄█`, que todo terminal desenha). Com `YBY_ASCII=1`, o desenho a traço (60×12). Sem espaço para nenhum, a marca em texto `◆ YBY`.
- `Logo(w)` escreve a logo que cabe na largura do terminal, com recuo de 2 e uma linha em branco depois. Fora de um terminal não escreve nada.
- Os desenhos ficam em `pkg/ui/marca/*.txt` e foram feitos à mão, pixel a pixel, a partir de `docs/assets/brand/yby-logo.svg`: folha com fenda, caule até o nó da base, de cada lado a trilha em S (nó de cima ao nó da borda) e a diagonal solta com o nó de baixo, nenhuma encostando no caule; o B do YBY aberto em cima. Ao mudar a logo oficial, redesenhe e confira contra o SVG; não converta por limiar, que desmancha os traços finos.
- Uso: só nos momentos de apresentação (`yby` sem argumentos, boas-vindas do Bard). Nas telas de trabalho o cabeçalho continua `◆ YBY`.

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
6. **KeyHints:** tecla em negrito, ação em `TintaSuave`, separadas por dois espaços; `q sair` sempre no canto direito; só teclas válidas na tela. Nas camadas em que q é letra ou não sai (campo, menu, ajuda), `Partes.Sair` troca a tecla do canto direito (o Viz usa `ctrl+c sair`). Abaixo de 80 colunas a legenda fecha com `? mais`; nas camadas em que o ? não abre a ajuda (campo de texto, confirmação, a própria tela ?), `Partes.SemAjuda` tira o `? mais`.

**Moldura de texto** (`moldura_texto.go`), para telas com campo de texto (Bard): `MolduraTexto(largura, altura, PartesTexto)` monta masthead, régua, corpo alinhado ao topo, régua, campo (n linhas), régua, estado e legenda. A tecla de sair é configurável (o Bard usa ctrl+c, porque q é digitado). Abaixo de 60×16 mostra a tela "aumente a janela". Junto: `AlturaCorpoTexto(altura, linhasCampo)`, `StatusLineTexto(largura, mensagem, direita)` (direita em `TintaSuave`) e `KeyHintsCom(largura, sair, teclas...)`.

Responsivo:
- ≥ 120 colunas: lista e detalhe lado a lado (60/40).
- 80–119: uma coluna; o detalhe abre por cima da lista.
- < 80: compacto; abas viram números (`1 Pods · 2 Deploy…`), a legenda mostra 3 teclas + `? mais`.
- < 60×16: tela única "aumente a janela".

Quem desenha a lista e o detalhe usa as medidas da moldura: `LargurasLadoALado(largura)` devolve as larguras da lista (60%) e do detalhe (40%) sem o fio entre elas, e `LimiteDetalheEmpilhado(h)` a altura máxima do detalhe por cima da lista num corpo de h linhas. `TeclasDoPainel(largura, teclas...)` escreve as teclas do rodapé de um painel no formato da legenda, sem `q sair` nem `? mais`.

## 6. Componentes (folha 03)

- **Tabs:** ativa `◆ Pods 8` em negrito (◆ em `Ok`); outras `◇ Deployments 4` em `TintaSuave`; `⇥ mais` quando não cabem; teclas 1–9.
- **Filtro:** `filtros  [status: com problema ✕]  [app=api ✕]` à esquerda, `8 de 42` à direita em `TintaSuave`.
- **Quadro:** painel com borda em `Fio` e rótulo em caixa-alta no fio de cima (`┌ DETALHE · POD ───┐`); com foco o fio vira `Tinta`. Variante destrutiva com fio `Erro`.
- **Table:** rótulos de coluna em `TintaSuave` caixa-alta; largura em células (`lipgloss.Width` / `ansi.StringWidth`); truncar com `…`; colunas com prioridade somem primeiro em terminal estreito; linha selecionada em tinta invertida (+ negrito no nome: as colunas com `Nome`, ou a primeira); `LargurasDasColunas` dá a largura que a tabela daria a cada coluna, para ajustar o conteúdo de uma célula antes de desenhar; itens concluídos em `TintaSuave`.
- **Status:** sempre glifo + cor + palavra: `● Running` (Ok), `✕ CrashLoopBackOff` (Erro), `▲ Pending` (Aviso), `✓ Completed` e `○ Parado` (TintaSuave).
- **Etapas:** linhas `✓ contexto do projeto   0,3 s` (✓ em Ok), `◈ kubectl get pvc -A   2,1 s` (atual, negrito), `◇ resposta` (TintaSuave), `✕` em Erro. Tempo à direita em `TintaSuave`, formato `0,3 s`, `38 s`, `1 min 12 s` (vírgula decimal).
- **Cota:** `argo cd  ━━━━━━━━━━━━━━┄┄┄┄┄┄┄┄  64%` — cheio em `Tinta`, tracejado em `Fio`; à direita percentual ou tempo restante (tempo em `Aviso` quando perto do fim). `CotaRestante` é a cota de uma validade (o token do painel do Cloud): a barra é o que ainda resta, o texto do tempo vem pronto e quem chama decide o aviso; sem rótulo, começa na barra.
- **Rota:** `◆━━━◆━━━◆┄┄┄◇┄┄┄◇┄┄┄◇  03/06`; marcos feitos `◆` em `Ok`, o atual `◆` negrito em `Tinta`, futuros `◇` em `TintaSuave`, trechos feitos `━`, futuros `┄` em `Fio`; abaixo os rótulos em caixa-alta separados por ` · `, o atual em negrito `Tinta`.
- **Spinner:** quadros `◇ ◈ ◆ ◈`; sem animação (quadro fixo `◈`) com `NO_COLOR` ou fora de TTY.
- **Toast:** na linha de estado, `✓ Deployment api reiniciado · 3 réplicas` ou `✕ sem permissão para escalar · RBAC`; some em 6 s. `MostrarComNota(tipo, texto, nota)` põe a nota depois de ` · `, em `TintaSuave` (o `RBAC` da prancha).
- **Confirmar:** destrutivo = Quadro com fio `Erro`, rótulo `EXCLUIR POD`, `digite  <nome em negrito>`, campo `› ` com cursor, `esc cancelar`; ↵ só confirma com o nome exato. `ComDetalhes(resumo, explicacao, verbo)` dá o diálogo completo da folha VizConfirmar: o resumo do recurso, a explicação do que acontece, `Digite <nome> para confirmar`, `faltam N letras` (ou `✓ confere`, `▲ não confere`) ao lado do campo, o campo num quadro de fio (só com a explicação; sem ela, quando falta altura, o campo é uma linha só) e os botões `[ esc cancelar ]   [ ↵ <verbo> ]`, o de confirmar apagado até o nome conferir. Reversível = ↵ confirma, esc cancela.
- **Copiar (OSC 52):** `CopiarOSC52(texto)` manda o texto para a área de transferência do terminal (embrulhado no tmux e no screen); o terminal não responde, então a frase diz "enviado", não "copiado". Usado pelo ctrl+y do Bard e pelo `x ▸ c` do Viz.
- **Quadro nomeado:** `QuadroNomeado(rotulo, nome, corpo, largura, variante)` põe o nome do recurso depois do rótulo sem caixa-alta (`┌ POD · worker-5d8b6-q7tvn ───┐`), porque os nomes do Kubernetes são em minúsculas.
- **Quadro com tecla** (`quadro_tecla.go`): `QuadroComTecla(rotulo, tecla, acao, corpo, largura, variante)` põe `ROTULO · tecla ação` no fio de cima (`┌ SAÍDA · ctrl+y copia ─┐`); se não couber, vira o Quadro comum.
- **Aviso de rolagem** (`rolagem.go`): `AvisoRolagem(SetaAcima|SetaAbaixo, texto)` marca conteúdo fora da janela em `TintaSuave`, sem recuo: `↑ 2 mensagens anteriores`, `↓ 3 mensagens novas` (`^`/`v` no ASCII).
- **Markdown** (`pkg/ui/uimarkdown`, pacote separado para o core não levar o glamour): `Novo(largura)` e `Renderizar(md)` com estilo tirado dos tokens: texto em Tinta, h1/h2 em negrito e caixa-alta, código inline em negrito sem fundo, links em TintaSuave sublinhados, régua em Fio, listas com • (- no ASCII). Tabelas viram `ui.Tabela` na largura toda (status do Kubernetes vira `ui.Status`), blocos cercados viram Quadro com o nome da linguagem (shell vira `COMANDO · ctrl+y copia`) e linhas longas de código quebram dentro do quadro. Sem cor fica só o negrito; no perfil Ascii, texto puro. `Comandos(md)` devolve os blocos de shell.
- **Estados:** carregando `◈ lendo pods de payments`; vazio `○ nenhum pod com esse filtro · esc limpa`; erro `✕ sem acesso ao cluster · r tenta de novo`.
- **Campo (tema huh):** título em caixa-alta negrito, `› ` + valor (sem o `> ` do huh), descrição em `TintaSuave`.
- **Assistente** (`assistente.go`, folha "assistente"): `NovoAssistente(largura, passos...)` com `PassoAssistente{Marco, Campos, Ocultar}` monta um único `huh.Form`, um grupo por passo, com a Rota em cima marcando o passo do campo em foco. Passos seguidos com o mesmo marco dividem o marco; passo sem campos fica de fora; `Ocultar` pula o grupo. `⇧⇥` volta um campo e, no primeiro campo do passo, o passo anterior; `esc` e `ctrl+c` cancelam (as listas não abrem filtro com `/`, para o `esc` sempre cancelar). Legenda `↵ continuar · ⇧⇥ voltar · esc cancelar`, que perde itens em janela estreita. Largura até 64; altura a da janela menos 7 linhas. Ao terminar, a view fica vazia e quem roda lê `Concluido` e `Cancelado`. Usado pelo `yby init`, que não grava nada antes da revisão.

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

Largura do terminal **(decisão)**: quando `w` é um terminal (`ui.Largura(w) > 0`) mais estreito que a linha, a nota de `Check` desce inteira para a linha de baixo, recuada sob o nome (4 células), e só quebra entre palavras se ainda não couber; assim caminhos com espaço (`C:\Program Files\…`) não se partem. `Title` se ajusta à largura do terminal e, se os dois lados não cabem, o da direita desce; em `Summary` a frase desce, alinhada sob a contagem. Fora do terminal (pipe, arquivo) nada quebra. `ui.Quebrar(texto, largura)` faz a quebra (não quebra abaixo de 16 células nem parte palavras).

`Steps` (yby up): `  ✓ cluster yby-local   criado   38 s` / `  ◈ argo cd   ━━━━┄┄ 64%   25 s` / `  ◇ root-app`.

`Progresso` (`progresso.go`, folha "progresso") conduz os `Steps` dos comandos longos (up, bootstrap vps/cluster, destroy, upgrade). No terminal, as etapas terminadas ficam fixas e a atual e as pendentes se redesenham no lugar; quando uma etapa falha, as pendentes saem da tela (o erro que vem a seguir diz onde o comando parou) e voltam se outra etapa começar. Fora do terminal, cada etapa sai uma vez ao terminar e as pendentes não aparecem. API: `NovoProgresso(w, rótulos...)` e `NovoProgressoCom(w, OpcoesProgresso{Terminal, Relogio, Largura}, …)`; `Iniciar`, `Avancar` (a fração vira cota), `Concluir`, `Falhar`, `Avisar` (▲ abaixo da etapa), `Detalhe` (texto recuado 4 células), `Animar(intervalo)`, `Escritor(destino)` (escreve linhas acima do trecho vivo, usado para o slog), `Pausar` (tira o trecho vivo da tela, por exemplo durante um pedido de senha do sudo), `Etapas` e `Quadro`. Os serviços recebem a interface `shared.Etapas` (ou `EtapasMudas`) e não escrevem na tela; os comandos externos rodam por `runnerQuieto`/`runnerAoVivo` (`cmd/progresso.go`), que mandam a saída deles para o log em debug.

Listagens, campos e passos (`listagem.go`, folha "listagem, campos e passos"):

- `Listagem(w, []Coluna, []Linha)`: cabeçalho em `TintaSuave` e caixa-alta, recuo de 2, cada coluna na largura da célula mais larga (com `Max` e reticências), separadas por 2 espaços, sem espaço à direita; célula que já traz ANSI não é retingida; em terminal estreito encolhe como a Tabela. Ex.: `env list`, com uma coluna de marca vazia e `◆` em `Ok` no ativo.
- `Campos(w, Campo{Rotulo, Valor}...)`: rótulos alinhados pelo mais largo + 2; valor vazio não aparece. Ex.: `env show`, `env dump`.
- `Passos(w, ...)`: lista numerada `1. texto` para os próximos passos (`secret init-eso`, `seal`).

Convenção dos comandos curtos: `Title` com o subcomando à direita, uma linha em branco e o corpo; `◈ texto` antes de etapas lentas; linhas `Check` com o estado na coluna de versão (`salvo`, `criado`, `ativo`, `falhou`); quando a operação é rápida, o título só sai depois dela, para o erro não ficar com duas linhas em branco. Saídas para máquina (`--export`, `telemetry export`) sem dados deixam o stdout vazio e avisam em stderr.

O `yby init` usa as seções `PROJETO`, `ARQUIVOS` (campos críticos vazios, como `repoURL` sem `--git-repo`, saem como ▲), `VALIDAÇÃO` (no `--update`, `PLANO DE MERGE`) e fecha com o `Summary` "próximo passo: yby env list".

Erro (`RenderError(*errors.YbyError)`):

```
✕ Não deu para falar com o cluster staging            ERR_CLUSTER_OFFLINE
  causa     o token do EKS expirou há 12 min
  faça      yby cloud refresh --provider aws          ← hint, em negrito
  detalhes  rode de novo com --log-level=debug
```

`WriteError(w, e)` escreve o mesmo bloco ajustado ao terminal: causa, faça e detalhes quebram com recuo de 12 células; se o título não cabe com o código à direita, ele quebra e o código vai para uma linha `código` no fim. A linha `detalhes` com a dica de depuração só aparece quando o erro tem causa (é o que o debug detalha) e o hint ainda não manda depurar. Os comandos escrevem o erro depois de uma linha em branco.

Título e hint seguem o tom da seção 9: título em minúscula, sem ponto; hint no imperativo, em minúscula e sem ponto final (`confira o caminho passado em --file`). Na causa, o último nível, quando é a saída de um comando externo, fica só com a linha de erro (`errors.ResumirSaida`: sem "exit status N", sem o prefixo `error:` e sem o rodapé do kubectl). Cada erro dos comandos traz o próprio hint; as dicas padrão de `pkg/errors/hints.go`, usadas quando não há hint, seguem o mesmo tom.

### Ajuda (`ajuda.go`, folha "ajuda")

`EscreverAjuda(w, Ajuda{...})` escreve a ajuda de um comando no formato das saídas: `Title` com o caminho (`◆ YBY  ENV CREATE`, "ajuda" à direita; na raiz, `◆ YBY  AJUDA`), a descrição e as seções USO (com "também yby gen" para os apelidos), COMANDOS, EXEMPLOS, FLAGS e FLAGS GLOBAIS; seção vazia não aparece. Nomes em `Tinta` numa coluna com a largura do mais largo (até 30) + 2, descrições em `TintaSuave` quebradas sob a coluna; nome mais largo fica sozinho e a descrição desce. Nos exemplos, linhas `#` em `TintaSuave`; num terminal estreito, comando longo continua com recuo de 4 e comentário continua com `# `. Rodapé `yby env <comando> --help mostra os detalhes de cada um`.

O core liga a ajuda ao cobra em `cmd/ajuda.go` (`SetHelpFunc` e `SetUsageFunc` no root, herdados por todos os comandos): flags como `-c, --context texto` (tipos em português: `texto`, `n`, `lista`, `duração`), `(padrão info)` quando o padrão diz algo, `-h, --help  mostra esta ajuda`, e o `help` padrão do cobra traduzido. Grupos que só mostram a ajuda (bootstrap, cloud, generate, plugin) não têm linha de uso própria. Com `--help` o cobra pula o `PersistentPreRun`, então a ajuda resolve o modo visual por conta própria. `yby` sem argumentos mostra a logo no lugar do título. Os plugins passam a usar `EscreverAjuda` nas tarefas deles (T016, T031, T032).

## 8. Teclas (`keys.go`, folha 04)

Global (toda TUI): `?` ajuda · `esc` fecha camada de cima / limpa filtros / cancela · `q` sai (só no nível de cima) · `ctrl+c` sai de qualquer lugar · `r` recarrega.

Listas (Viz, Cloud): `↑↓ j k` mover · `pgup pgdn` página · `g G` início e fim · `⇥ ⇧⇥ 1–9` aba · `/` buscar por nome · `f` filtrar · `n` namespace · `↵` abrir · `x` ações · `l` logs · `y` yaml · `e` eventos · `b` perguntar ao Bard.

Texto (Bard, assistentes): `↵` enviar/continuar · `⇧↵` nova linha · `/` comandos (início da linha) · `↑` editar última mensagem · `pgup pgdn` rolar · `esc` interrompe a resposta · `⇧⇥` voltar uma etapa · `␣` marcar · `a` marcar todos.

No Bard: a nova linha é `ctrl+j` (e `alt+enter`, que a maioria dos terminais manda no ⇧↵), porque o Bubble Tea v1 não separa ⇧↵ de ↵ no console do Windows, e a legenda mostra `ctrl+j`. Só pgup, pgdn e a roda do mouse rolam; o resto do teclado é do campo. Com o campo vazio, `↵` abre a saída da ferramenta e `ctrl+y` copia o último comando de shell da resposta (OSC 52) e, com a saída aberta, a saída inteira; a prancha mostra `c copia`, mas no Bard um `c` solto comeria a primeira letra da pergunta. `/` abre a lista de comandos sobre o campo: texto filtra · `↑↓` escolher · `⇥` completar · `↵` executar · `esc` fechar e apagar a barra; os seletores de `/sessões` e `/modelo` usam `↑↓ escolher · ↵ retomar|usar · esc fechar`.

Níveis: Quadro →(↵) Detalhe →(x) Ações →(d) Confirmação; `esc` sobe um nível. Token do Cloud em `t`; reiniciar em `x ▸ r`.

## 9. Tom

Português com acento, verbos diretos, Kubernetes com os nomes originais. Exemplos: `◈ gerando resposta · 4 s`, `◈ lendo pods de payments`, `✕ cluster fora do ar · token expirado`, `digite api-7f9c-x2k para excluir`.
