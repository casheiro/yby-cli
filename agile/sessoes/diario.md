# Diário de sessões

> Uma entrada por sessão, a mais recente no topo. Escrito pela skill `/encerrar`.
> Formato: data · branch · o que foi feito · decisões · o que ficou pendente · próximo passo.

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
