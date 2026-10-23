# Diário de sessões

> Uma entrada por sessão, a mais recente no topo. Escrito pela skill `/encerrar`.
> Formato: data · branch · o que foi feito · decisões · o que ficou pendente · próximo passo.

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
