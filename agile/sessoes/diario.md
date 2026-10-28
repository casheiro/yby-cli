# Diário de sessões

> Uma entrada por sessão, a mais recente no topo. Escrito pela skill `/encerrar`.
> Formato: data · branch · o que foi feito · decisões · o que ficou pendente · próximo passo.

---

## 27/10/2026 · `feat/correcoes-plugins` → trilhas `feat/correcoes-*` → `develop`

**Feito**
- Plano de correções dos plugins completo (77/77), em ondas por workflow (implementador e revisor adversarial por tarefa, um worktree por trilha): onda 0 (#220, `7f8337c`), onda 1 com Sentinel, Viz, Atlas, Synapstor e Bard (#221–#225), contratos (#226, `fbd0b02`) e fecho (#227, `43b5f36`).
- Releases v5.8.1, v5.8.2 (CHANGELOG pelo #231, `d81add9`) e v5.8.3.
- Wiki da v5.8.2 (P02, `6d450f9`): seis páginas conferidas contra o código e a CLI-Reference gerada de novo. A conferência achou a Fase 8, no #233 (`9251014`): P04 (a descoberta rodava o `manifest` de plugin não confiável), P05, S16 (métricas do `investigate` sem ambiente) e S17 (namespace padrão do Sentinel, por decisão do usuário).
- #235 (`1e6f21d`): seção da v5.8.3 no CHANGELOG e prazo de 30 s no `coletar` dos testes do Bard, que estourava com `-race` no CI.
- Wiki da v5.8.3 (`9e06685`) e ponteiro do submódulo neste registro. Worktrees e branches locais das ondas removidos.

**Decisões**
- S17: sem `-n`, o Sentinel usa o namespace do ambiente pedido, depois o do contexto do kubeconfig e por fim `default`.
- A descoberta confia no carimbo (caminho, tamanho e mtime) do registro; o executor sempre recalcula o hash.

**Pendente**
- Fechos para a `main`: #228, #232 e #236.
- Limites da confiança e achados soltos, em Atenção no `estado.md`.

**Próximo passo**
- O usuário escolhe a próxima frente ("Fora do escopo" do plano ou o `ROADMAP.md`).

---

## 26/10/2026 · `feat/pedido-grande-por-arquivo` → `feat/bard-relatorio-sentinel` → `feat/correcoes-plugins`

**Feito**
- v5.8.0 publicada (#209), com as notas da release limpas (#211, #213) e o bump calculado desde a última tag (#215, porque a tag fica fora da develop).
- PR #216: `plugin list` sem duplicatas quando o yby roda na home.
- PR #217: o pedido grande vai por arquivo (`YBY_PLUGIN_REQUEST_FILE`). Corrige o "argument list too long" no `yby synapstor --help` do anhumas-nexus. Testado em Linux num contêiner descartável.
- PR #218: o Bard recebe os achados do Sentinel, que escreve o relatório sem o envelope `{"data":…}`, e o `sentinel_investigate` pede `-o json` (`ed2dfd1`). O `sentinel scan --no-ai` evita a segunda IA quando o Bard chama o scan (`fca00ec`).
- Revisão dos cinco plugins com um agente por plugin. Os achados graves foram conferidos no código e viraram o plano `agile/planning/correcoes-plugins/tasks.md`, com cerca de 60 tarefas em 8 fases. `scripts/falhas.sh` entrou no repo.

**Decisões**
- Escopo das correções: tudo o que é erro ou está errado em relação à proposta. Features e perfumaria ficam fora, listadas no fim do plano.
- Um PR por fase, com push direto, porque são correções.
- A wiki só ganha a linha do `--no-ai` depois da release.

**Pendente**
- Merge do #216, do #217 e do #218, nesta ordem, e depois a release v5.8.1.

**Próximo passo**
- `git merge origin/develop` na `feat/correcoes-plugins` e começar pela C01.

## 24–25/10/2026 · `feat/tui-viz` → `feat/tui-cloud` → `feat/tui-plugins`

**Feito**
- Fase 3 (Viz) aprovada e no PR #204 (`6b64dd7`). Release v5.7.0 (core e Bard) publicada por disparo manual do `release.yaml`; PR #205 para a main aberto.
- CI sem GH_PAT: `release.yaml` chamado pelo `release-pipeline.yaml`, back-merge só com diferença, job da wiki removido (`49e4aae`, PR #206).
- Fase 4 (Cloud): T029, T030 e o checkpoint (`d93da8e`), PR #207. Correções do cloud contra a documentação oficial e o gcloud real sem faturamento (`30061e1` … `50c9055`).
- Fase 5: T031–T034, T039 e o checkpoint (`79dd9e5`), PR #208. Cloud em beta (`6486e3f`). T035: CHANGELOG, README, wiki, referência da CLI em português pelo `gen-docs`, prancha marcada como implementada (`5f65fbe`); wiki publicada (`b8dc3f7`) e submódulo apontado (`3ae29eb`).

**Decisões**
- Sem contas pagas de nuvem: o `yby cloud` fica em beta até o teste com clusters reais (lista no ROADMAP).
- Login SSO pelo navegador no painel, coluna de nós e saída 0 de `cloud refresh`/`list` ficam para a saída do beta.
- Quadros do Bard com o rótulo na cor da borda, causa técnica do client-go na linha "causa" e scan sem modo de falha para CI ficam como estão.
- A referência da wiki passa a ser gerada em português da mesma ajuda do `--help`.

**Pendente**
- Merges: #206, #205, #204 → #207 → #208.
- Pendências menores listadas em Atenção no `estado.md`.

**Próximo passo**
- Release da próxima versão depois dos merges.

---

## 23/10/2026 (2ª sessão) · `feat/tui-carta`

**Feito**
- Workflow de agentes Sonnet em ondas para a fase 0 (tabela no `tasks.md`); especificação local da prancha em `docs/design/tui/especificacao-pkg-ui.md` (`936c7cb`).
- Fase 0 completa, T001–T010: `pkg/ui` com tokens, glifos, tema, teclas, moldura, abas, filtro, tabela, quadro, etapas, cota, rota, spinner, toast, confirmar, estados, tema huh e saídas de comando (`260b880` … `0094a88`); catálogo `tools/ui-catalogo`; 99,1% de cobertura.
- Conferência do catálogo pelo usuário no Windows Terminal (escuro, claro, `NO_COLOR`, `YBY_ASCII`) achou dois desvios, corrigidos: texto e seleção sem Tinta (`c8f6b91`) e teclas Unicode no ASCII (`396c6b3`).
- Checkpoint da fase 0 aprovado: lint 2.15 sem achados nos pacotes novos (`076d546`).

**Decisões**
- Sem push/PR até o usuário testar o CLI real localmente; catálogo não conta como teste (memória salva).
- Proposta pendente: não abrir PR só com a lib; começar pela fase 1 (T023 + T026) por ser testável sem cluster nem IA.

**Pendente**
- Aval do usuário para a proposta acima.
- Falhas de teste antigas no Windows em 10 pacotes (mesma lista antes da fase).

**Próximo passo**
- T023 + T026 e guiar o teste real: `task build` → `yby doctor` e um erro qualquer.

---

## 23/10/2026 · `feat/multicloud-support` → `feat/tui-carta`

**Feito**
- Ambiente Windows preparado: Go 1.27 instalado, `task build` gerando `.exe`, plugins nativos instalados a partir de `bin/`.
- Correções para Windows em `feat/multicloud-support` (sem push): `.exe` no Taskfile (`12d783b`), `.gitattributes` e modo executável dos scripts (`4b712af`), descoberta de plugins `.exe` (`353d7cf`), `OLLAMA_HOST` sem esquema causando panic (`372b6be`), download do modelo de embeddings sem symlinks (`04036c6`).
- Análise do estado do multicloud: entregue nas releases v5.6.0 e v5.6.1; lacunas principais são o CI sem testes com as tags `aws/azure/gcp` e o `yby up` sem validação de token cloud.
- Redesign "Carta do terminal": inventário das TUIs, estudo do site, prancha com 13 telas aprovada, briefing, plano de migração e plano de tarefas (`1bd2bde` em `feat/tui-carta`).
- Controle de sessão criado: `scripts/sessao.sh`, `agile/sessoes/`, skills `/retomar` e `/encerrar`, hook `SessionStart`.

**Decisões**
- Escopo total do redesign, metáfora "carta" com intensidade média, tema adaptativo, pacote `pkg/ui`.
- Ordem das fases: 0 → 2 → 3 → 1 → 4 → 5; um commit por tarefa; um PR por fase.
- `feat/tui-carta` separada do multicloud, a partir de `origin/main`.

**Pendente**
- Push de `feat/multicloud-support` (decisão do usuário).

**Próximo passo**
- T001: `pkg/ui/tokens.go` e `pkg/ui/glyphs.go`.
