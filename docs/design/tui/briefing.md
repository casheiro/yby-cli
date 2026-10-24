# Briefing — Carta do terminal (redesign das interfaces do Yby CLI)

> Status: **aprovado** (direção visual aprovada em 23/10/2026) · Rota: F, redesign completo · Prancha: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk

## Suposições (confirmadas ou assumidas na prancha)

- O escopo é **tudo**: as 3 TUIs (Bard, Viz, Cloud Dashboard), os prompts `huh` e a saída de todos os comandos e plugins, inclusive erros e progresso.
- A metáfora "carta topográfica" do site entra com **intensidade média**: paleta, `◆` como marcador, rótulos de instrumento e fios. Curvas de nível não aparecem nas telas de trabalho.
- Tema **adaptativo**: variante "mapa noturno" em fundo escuro e "kraft" em fundo claro. Nenhuma TUI pinta o fundo do terminal.
- Será criado um pacote compartilhado **`pkg/ui`**, usado pelo core e por todos os plugins.
- Ações do Viz passam para o menu `x`; `r` recarrega em todas as telas; o token do Cloud é renovado com `t`; no Viz, `/` busca por nome e `n` troca o namespace.
- Aviso usa **ocre** (`#D9AE5F` no escuro, `#7D5A10` no claro). O site não tem cor de aviso; esta foi derivada da mesma paleta.
- Dados mostrados na prancha são exemplos.

## Produto e objetivo

- **O que é:** Yby CLI, assistente de infraestrutura Kubernetes (scaffolding, clusters locais/remotos/cloud, GitOps com Argo CD e IA no terminal).
- **Objetivo do redesign:** que todas as interfaces de terminal do Yby pareçam um único produto, alinhado à identidade do site (https://yby.dev.br), e que cada tela deixe claro onde a pessoa está, o que está acontecendo e o que ela pode fazer.
- **Sinais de sucesso:**
  - uma única paleta e um único conjunto de glifos no código (hoje são 6 verdes, 4 vermelhos e 6 cinzas);
  - a mesma tecla faz a mesma coisa em todas as TUIs;
  - o Bard mostra cada etapa (contexto, ferramenta, resposta) e a resposta aparece enquanto é gerada;
  - nenhum erro chega ao usuário como `level=ERROR msg=...`;
  - as TUIs funcionam em terminal claro, escuro, com `NO_COLOR` e no Windows.

## Público

- Pessoas de plataforma, DevOps e desenvolvimento que operam clusters pelo terminal.
- Terminais: Windows Terminal, iTerm2, GNOME Terminal, terminais integrados de IDE (VS Code, Cursor), tmux e SSH. O tamanho vai de 80×24 até janelas largas.

## Ação principal por interface

| Interface | O que a pessoa precisa conseguir |
|---|---|
| Bard | perguntar sobre o cluster, ver o que o Bard consultou e copiar ou rodar o comando sugerido |
| Viz | achar o recurso com problema, entender o motivo e agir sobre ele com segurança |
| Cloud Dashboard | ver quais clusters estão acessíveis e renovar o acesso que expirou |
| Init e outros assistentes | responder às perguntas sabendo quanto falta, podendo voltar |
| Saídas de comando | saber se deu certo e, se não, o que fazer em seguida |

## Conteúdo

| Folha da prancha | Objetivo | Situação |
|---|---|---|
| 01 Fundamentos | princípios, paleta, glifos, hierarquia, tom | aprovado |
| 02 Anatomia | moldura comum e comportamento por largura | aprovado |
| 03 Biblioteca `pkg/ui` | componentes e estrutura do pacote | aprovado |
| Bard (4 telas) | conversa, trabalhando, comandos, tema claro | aprovado |
| Viz (2 telas) | lista com detalhe, ações e confirmação | aprovado |
| Cloud (1 tela) | painel multi-cluster | aprovado |
| Init, saídas, navegação | assistente, doctor/up/erro, mapa de teclas | aprovado |

## Tom de voz

- **3 palavras:** direto, técnico, calmo.
- **Regras:** português do Brasil com acento; verbos diretos nos atalhos ("abrir", "buscar", "sair"); nomes do Kubernetes como são (Pod, Deployment, namespace); o estado diz o que está acontecendo ("◈ lendo pods de payments"), nunca só "Carregando..."; erro diz a causa e o próximo passo.
- **Exemplos:** "✕ Não deu para falar com o cluster staging · causa: o token do EKS expirou há 12 min · faça: yby cloud refresh --provider aws"; "digite worker para confirmar".

## Referências

| Referência | O que levar | O que NÃO levar |
|---|---|---|
| Site yby.dev.br ("carta topográfica") | tokens `--fg --mu --ln --mk --er`, cor só como status, `◆` como marcador, rótulos em caixa-alta com espaçamento ("INSTR. 03 · ORIENTAÇÃO"), quadro com fio duplo, trilha tracejada | curvas de nível animadas, fontes Archivo e Martian Mono (o terminal usa a fonte do usuário), degradês |
| TUIs atuais do Yby | funcionalidades e fluxos existentes (abas do Viz, sessões do Bard, refresh do Cloud) | roxo `#7D56F4`, rosa 205, ciano, emojis como marcadores, teclas conflitantes |

## Restrições

- **Marca:** tokens do site (`yby-site/app/assets/css/main.css`). O logo não é desenhado no terminal; a marca aparece como `◆ YBY`.
- **Acessibilidade:** todo status tem glifo e palavra, além da cor; respeitar `NO_COLOR`; contraste mínimo de 4,5:1 para texto nos dois temas; fallback ASCII para terminais sem Unicode.
- **Stack:** Go 1.26, Bubbletea v1.3.6, Lipgloss, Bubbles, Glamour e Huh v1.0.0. O plugin Viz está em Bubbletea v0.25 com módulo próprio e precisa ser atualizado.
- **Não pode mudar:**
  - nomes de comandos, flags e subcomandos;
  - saídas legíveis por máquina (`--log-format json`, `yby telemetry export`, relatórios em markdown do Sentinel, `cloud audit --export`);
  - protocolo de plugins (`PluginRequest`/`PluginResponse`, `YBY_PLUGIN_REQUEST`);
  - comportamento não interativo (CI, pipes, `echo ... | yby bard`).

## Mídia

| Item | Status | Observação |
|---|---|---|
| Logo | não usado no terminal | marca vira `◆ YBY`; o SVG do site continua sendo a marca oficial |
| Fontes | não aplicável | terminal usa a fonte do usuário; a prancha usa JetBrains Mono só para ilustrar |
| Capturas para docs e site | pendente | gerar depois da migração (ex.: fitas do VHS) |

## Pendências

- Definir onde as capturas de tela novas vão aparecer (wiki, README, site).
- Revisar o texto da ajuda (`?`) de cada TUI quando as telas estiverem prontas.

## Histórico

| Data | Pedido | Decisão | Arquivos |
|---|---|---|---|
| 23/10/2026 | redesenhar todas as interfaces de terminal com a identidade do site | rota F; escopo total; metáfora média; tema adaptativo; `pkg/ui` | — |
| 23/10/2026 | prancha com telas, navegação e atalhos | 13 telas publicadas; rodapé da folha 01 corrigido (linhas grudadas) | prancha "Carta do Terminal Yby" |
| 23/10/2026 | aprovação | direção aprovada por inteiro | `docs/design/tui/briefing.md`, `docs/design/tui/plano-migracao.md` |
