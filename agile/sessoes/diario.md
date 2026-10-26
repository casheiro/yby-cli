# Diário de sessões

> Uma entrada por sessão, a mais recente no topo. Escrito pela skill `/encerrar`.
> Formato: data · branch · o que foi feito · decisões · o que ficou pendente · próximo passo.

---

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
