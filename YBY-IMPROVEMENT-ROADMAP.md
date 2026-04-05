# Yby CLI — Roadmap de Melhorias

Auditoria completa do codebase realizada em 2026-04-05. Este documento cataloga todas as oportunidades de melhoria identificadas, organizadas por area, priorizadas e com evidencias do codigo-fonte.

**Versao do CLI auditada:** v4.7.0+
**Cobertura da analise:** cmd/, pkg/, plugins/ (atlas, bard, sentinel, synapstor, viz)

---

## Indice

1. [Sistema de IA (pkg/ai/)](#1-sistema-de-ia)
2. [Plugin Atlas](#2-plugin-atlas)
3. [Plugin Bard](#3-plugin-bard)
4. [Plugin Synapstor](#4-plugin-synapstor)
5. [Plugin Viz](#5-plugin-viz)
6. [Plugin Sentinel](#6-plugin-sentinel)
7. [CLI UX e Comandos](#7-cli-ux-e-comandos)
8. [Mirror e Sync](#8-mirror-e-sync)
9. [Telemetria](#9-telemetria)
10. [Scaffold e Templates](#10-scaffold-e-templates)
11. [Configuracao Global](#11-configuracao-global)

---

## 1. Sistema de IA

**Localizacao:** `pkg/ai/`
**Estado:** Funcional com 3 providers (Ollama, Gemini, OpenAI). Factory com auto-detect. Vector store com ChromemDB.

### 1.1 Streaming do Gemini e fake (CRITICO)

**Problema:** `gemini.go` `StreamCompletion()` nao faz streaming real — chama `Completion()` inteira e escreve tudo de uma vez.

**Evidencia:** `pkg/ai/gemini.go` — StreamCompletion delega para Completion, aguarda resposta completa, escreve no writer de uma vez.

**Impacto:** Respostas longas congelam a UI por minutos sem feedback visual.

**Solucao:** Implementar streaming real via API generateContent com `alt=sse` ou usar a SDK oficial do Gemini com callback.

### 1.2 Retry apenas no Gemini

**Problema:** Apenas o Gemini tem retry (3x exponential backoff para 429/503). OpenAI e Ollama falham imediatamente.

**Evidencia:** `pkg/ai/gemini.go` implementa retry loop. `pkg/ai/openai.go` e `pkg/ai/ollama.go` nao tem nenhum retry.

**Impacto:** Em rede instavel ou rate-limit, OpenAI/Ollama falham sem recuperacao.

**Solucao:** Extrair logica de retry do Gemini em middleware generico e aplicar a todos os providers.

### 1.3 Sem token counting nem context window management [CONCLUIDO em 2026-04-05]

**Problema:** Nenhum provider verifica se o prompt cabe no context window antes de enviar.

**Evidencia:** Nenhuma chamada a token counting em `pkg/ai/openai.go`, `gemini.go` ou `ollama.go`. Prompts longos (ex: `study` com 100KB de codigo) podem exceder silenciosamente.

**Impacto:** Overflow silencioso, custos imprevisveis (OpenAI), comportamento erratico.

**Solucao:** Adicionar `CountTokens(prompt)` na interface Provider ou usar estimativa (4 chars ~= 1 token). Truncar antes de enviar.

### 1.4 Sem rate limiting

**Problema:** Sem respeito aos headers `Retry-After`, sem queue de requisicoes, sem circuit breaker.

**Evidencia:** Nenhum codigo em `pkg/ai/` verifica headers de rate-limit nas respostas HTTP.

**Impacto:** Em alta concorrencia (ex: indexacao de 200 documentos), requisicoes sao rejeitadas.

**Solucao:** Implementar circuit breaker pattern com adaptive backoff.

### 1.5 Modelos hardcoded [CONCLUIDO em 2026-04-05]

**Problema:** OpenAI usa `gpt-4o-mini` fixo. Ollama pega o primeiro modelo disponivel. So Gemini permite override via `GEMINI_MODEL`.

**Evidencia:** `pkg/ai/openai.go` — modelo `gpt-4o-mini` hardcoded. `pkg/ai/ollama.go` — seleciona primeiro modelo de `/api/tags`.

**Impacto:** Impossivel usar gpt-4-turbo, Claude, ou modelos especificos.

**Solucao:** Adicionar env var `YBY_AI_MODEL` ou flag `--model` para override.

### 1.6 Sem cost tracking

**Problema:** Nenhum logging de tokens consumidos ou estimativa de custo.

**Evidencia:** Nenhum campo de usage nas respostas e processado em nenhum provider.

**Impacto:** Impossivel gerenciar custos em producao com OpenAI.

**Solucao:** Extrair `usage.prompt_tokens` e `usage.completion_tokens` das respostas e logar via slog.

### 1.7 Embeddings sequenciais no Ollama

**Problema:** Ollama processa embeddings um por um (N documentos = N requests HTTP).

**Evidencia:** `pkg/ai/ollama.go` — loop sequencial em `EmbedDocuments`, faz 1 request por documento.

**Impacto:** Indexacao de 1000 documentos = 1000 requests de rede. Extremamente lento.

**Solucao:** Agrupar embeddings com goroutines limitadas (semaforo) ou implementar batch manual.

### 1.8 Sem cache de embeddings

**Problema:** Cada busca semantica gera novo embedding da query, mesmo que seja identica a anterior.

**Evidencia:** `pkg/ai/vector_store.go` — `Search()` chama `EmbedDocuments` a cada query sem cache.

**Impacto:** Mesma query 10x = 10x chamadas a API.

**Solucao:** Cache LRU local para embeddings com TTL.

### 1.9 Vector store sem delete

**Problema:** `VectorStore` suporta apenas `AddDocuments` e `Search`. Nao tem `Delete` ou `Update`.

**Evidencia:** `pkg/ai/vector_store.go` — apenas 2 metodos publicos.

**Impacto:** Documentos obsoletos permanecem no indice para sempre.

**Solucao:** Expor `DeleteDocuments(ids)` via ChromemDB.

---

## 2. Plugin Atlas

**Localizacao:** `plugins/atlas/`
**Estado:** Funcional. Discovery de componentes e relacoes via hooks context/manifest. Testes reais.

### 2.1 Cobertura limitada de linguagens [CONCLUIDO em 2026-04-05]

**Problema:** Regras de discovery so detectam Go (`go.mod`), Node (`package.json`), Helm (`Chart.yaml`), Kustomize, Docker.

**Evidencia:** `plugins/atlas/discovery/rules.go` — `DefaultRules` lista apenas 8 patterns.

**Impacto:** Projetos Python, Java, Rust, .NET nao sao descobertos.

**Solucao:** Adicionar regras para: `pyproject.toml`, `requirements.txt` (Python), `pom.xml`, `build.gradle` (Java), `Cargo.toml` (Rust), `docker-compose.yml`.

### 2.2 Deteccao de relacoes limitada

**Problema:** Relacoes so sao detectadas em 3 cenarios: Go `replace` directives, Docker `COPY`/`ADD`, Helm `file://` deps.

**Evidencia:** `plugins/atlas/discovery/scanner.go` — funcao `detectRelations` com 3 parsers.

**Impacto:** Nao detecta: imports reais de Go/Python/Node, `FROM` multi-stage em Docker, deps remotas de Helm.

**Solucao:** Adicionar parser de imports para Go (`import` blocks), detectar `FROM` em Dockerfiles, e deps remotas em Chart.yaml.

### 2.3 Ignores com string matching ingenuo

**Problema:** Ignores usam `strings.Contains()` que pode causar false positives.

**Evidencia:** `plugins/atlas/discovery/scanner.go` — `ShouldIgnore` usa `strings.Contains(path, ignoredDir)`.

**Impacto:** Diretorio `my-vendor-lib` seria ignorado se `vendor` estiver na lista de ignores.

**Solucao:** Usar `filepath.Base()` para comparar apenas o nome do diretorio, ou integrar com `.gitignore`.

### 2.4 Sem cache de descoberta

**Problema:** Cada execucao do hook `context` faz full scan do filesystem.

**Evidencia:** `plugins/atlas/discovery/scanner.go` — `Scan()` sempre faz `filepath.WalkDir` completo.

**Impacto:** Em projetos grandes, pode levar segundos a cada execucao de qualquer comando.

**Solucao:** Cache baseado em hash de diretorio com invalidacao incremental.

---

## 3. Plugin Bard

**Localizacao:** `plugins/bard/`
**Estado:** Funcional. Chat TUI com RAG via Synapstor vector store. Historico persistido. Multi-provider.

### 3.1 Sem token counting [CONCLUIDO em 2026-04-05]

**Problema:** Bard envia system prompt + historico + contexto RAG + pergunta sem verificar se cabe no context window.

**Evidencia:** `plugins/bard/main.go` — `formatHistoryContext()` injeta ate 20 entradas de historico no prompt sem contagem.

**Impacto:** Prompts muito longos causam truncamento silencioso ou erro do provider.

**Solucao:** Calcular tokens estimados antes de enviar. Truncar historico ou contexto RAG se necessario.

### 3.2 Historico flat sem estrutura conversacional

**Problema:** Historico e uma lista linear de `{role, content, timestamp}`. Sem sessoes, sem threading.

**Evidencia:** `plugins/bard/history.go` — struct `HistoryEntry` com 3 campos apenas.

**Impacto:** Impossivel distinguir conversas diferentes. `/clear` apaga tudo.

**Solucao:** Adicionar `sessionID` ao HistoryEntry. Permitir navegar entre sessoes.

### 3.3 RAG read-only

**Problema:** Bard consome o vector store do Synapstor mas nao pode indexar novos documentos.

**Evidencia:** `plugins/bard/main.go` — apenas `vectorStore.Search()`, sem `AddDocuments`.

**Impacto:** Usuario precisa rodar `yby synapstor index` separadamente para atualizar a base.

**Solucao:** Auto-indexar documentos referenciados na conversa, ou trigger de re-index apos `capture`.

### 3.4 Sem modo batch

**Problema:** Bard so funciona interativamente (stdin/stdout loop). Sem suporte a input de arquivo ou pipe.

**Evidencia:** `plugins/bard/main.go` — `bufio.NewScanner(os.Stdin)` em loop interativo.

**Impacto:** Impossivel usar em scripts ou pipelines de CI.

**Solucao:** Detectar se stdin e TTY. Se nao for, processar input como batch (uma pergunta por linha).

### 3.5 Sem hot-reload de configuracao

**Problema:** `.yby/bard.yaml` e lido apenas na inicializacao.

**Evidencia:** `plugins/bard/config.go` — `LoadConfig()` chamado uma vez no startup.

**Impacto:** Mudar `top_k` ou `relevance_threshold` requer reiniciar o bard.

**Solucao:** Watch no arquivo com `fsnotify` ou comando `/reload`.

---

## 4. Plugin Synapstor

**Localizacao:** `plugins/synapstor/`
**Estado:** Funcional. Captura/estudo via IA, indexacao incremental com ChromemDB. Integrado com Bard.

### 4.1 Nao implementa hook `context`

**Problema:** Synapstor declara apenas hook `command`. Nao enriquece o blueprint context para outros plugins.

**Evidencia:** `plugins/synapstor/main.go` — manifesto tem `Hooks: []string{"command"}` apenas.

**Impacto:** Outros plugins (alem do Bard que acessa direto) nao se beneficiam do conhecimento indexado.

**Solucao:** Implementar hook `context` que injeta metadados do indice (quantidade de UKIs, temas, ultima indexacao).

### 4.2 Scanner usa apenas string matching

**Problema:** `scanner.Scan()` usa `strings.Contains` para matching. Sem relevancia ou scoring.

**Evidencia:** `plugins/synapstor/internal/scanner/scanner.go` — `strings.Contains(content, query)`.

**Impacto:** Busca por "deploy" retorna QUALQUER arquivo que mencione a palavra, sem ranking.

**Solucao:** Implementar TF-IDF ou BM25 basico para scoring de relevancia.

### 4.3 Sem comando `search` no CLI [CONCLUIDO em 2026-04-05]

**Problema:** O vector store existe e funciona mas nao ha CLI para busca direta.

**Evidencia:** Comandos do Synapstor: `capture`, `study`, `index`. Nenhum `search`.

**Impacto:** So o Bard consegue consultar o indice. Usuario nao pode buscar UKIs diretamente.

**Solucao:** Adicionar `yby synapstor search "query"` que retorna top-K resultados com score e preview.

### 4.4 Sem metricas de indexacao

**Problema:** Comando `index` nao reporta quantos arquivos processados, chunks gerados, embeddings criados, tempo decorrido.

**Evidencia:** `plugins/synapstor/internal/indexer/indexer.go` — funcao `Index()` retorna apenas error.

**Impacto:** Usuario nao sabe se indexacao foi efetiva ou se houve problemas.

**Solucao:** Retornar struct `IndexReport{FilesScanned, ChunksGenerated, EmbeddingsCreated, Duration, Skipped}`.

---

## 5. Plugin Viz

**Localizacao:** `plugins/viz/`
**Estado:** Funcional mas MUITO basico. Apenas lista pods. Sem metricas, sem filtros, sem drill-down.

### 5.1 Monitora apenas Pods (CRITICO)

**Problema:** Dashboard mostra APENAS pods. Nenhum outro recurso Kubernetes.

**Evidencia:** `plugins/viz/internal/monitor/client.go` — `GetPods()` e o unico metodo. Nao ha `GetDeployments`, `GetServices`, etc.

**Impacto:** Viz cobre ~5% dos recursos que um operador precisa monitorar.

**Solucao:** Adicionar views para: Deployments (replicas), Services (endpoints), Ingresses, Nodes (capacity), Jobs, StatefulSets.

### 5.2 Sem metricas de recursos [CONCLUIDO em 2026-04-05]

**Problema:** CPU mostra "N/A" sempre. Sem memoria, rede ou disco.

**Evidencia:** `plugins/viz/internal/monitor/client.go` — struct `Pod` tem `CPU: "N/A"` hardcoded.

**Impacto:** Dashboard nao mostra informacoes de consumo de recursos.

**Solucao:** Integrar com metrics-server (`metrics.k8s.io/v1beta1`) para CPU e memoria.

### 5.3 Sem filtros ou busca

**Problema:** Lista TODOS os pods de TODOS os namespaces. Sem filtro por namespace, label ou status.

**Evidencia:** `plugins/viz/internal/monitor/client.go` — `Pods("").List(...)` sem ListOptions.

**Impacto:** Em clusters grandes, lista e inutilizavel.

**Solucao:** Adicionar: filtro por namespace (tab/dropdown), label selector (input), status filter (Running/Failed).

### 5.4 Sem scroll nem paginacao

**Problema:** Se ha mais pods que linhas no terminal, lista e cortada.

**Evidencia:** `plugins/viz/internal/ui/model.go` — `View()` renderiza lista sem viewport.

**Impacto:** Informacao e perdida em clusters com muitos pods.

**Solucao:** Usar `viewport` do Bubbletea ou `list` component com scroll.

### 5.5 Sem reconexao automatica

**Problema:** Se conexao com cluster cair, viz mostra erro e nao tenta reconectar.

**Evidencia:** `plugins/viz/internal/monitor/client.go` — sem retry em `GetPods()`.

**Impacto:** Usuario precisa reiniciar viz apos qualquer desconexao.

**Solucao:** Retry com backoff exponencial em caso de erro de rede.

### 5.6 Refresh hardcoded em 2s

**Problema:** Intervalo de refresh e fixo em 2 segundos.

**Evidencia:** `plugins/viz/internal/ui/model.go` — `time.Sleep(2 * time.Second)` no tick.

**Impacto:** Sem controle do usuario. Pode ser muito rapido (custo de CPU) ou muito lento.

**Solucao:** Flag `--interval` ou tecla de atalho para ajustar.

### 5.7 Testes superficiais

**Problema:** Testes do Viz cobrem apenas serialization e struct fields. Nenhum teste de integracao ou UI lifecycle.

**Evidencia:** `plugins/viz/internal/ui/model_test.go` — 3 testes basicos. `monitor/client_test.go` — apenas FakeClient.

**Impacto:** Regressoes em UI ou monitor podem passar despercebidas.

**Solucao:** Testes com FakeClient simulando cenarios reais (cluster grande, pods em transicao, reconexao).

---

## 6. Plugin Sentinel

**Localizacao:** `plugins/sentinel/`
**Estado:** Funcional. Build tag `k8s`. Testes unitarios presentes.

### 6.1 Testes unitarios em andamento

**Problema:** Testes unitarios do sentinel (cache, report, scan) foram adicionados recentemente na branch `feat/sentinel-unit-tests` e podem nao estar completos.

**Evidencia:** Branch atual, arquivos `cache_test.go`, `integration_test.go`.

**Impacto:** Cobertura pode estar abaixo do padrao do projeto (90%+).

**Solucao:** Completar testes e medir cobertura com `go test -cover -tags=k8s ./plugins/sentinel/cli/...`

---

## 7. CLI UX e Comandos

### 7.1 Comando `yby logs` documentado mas nao implementado [CONCLUIDO em 2026-04-05]

**Problema:** `docs/wiki/CLI-Reference.md` menciona `yby logs` mas o comando nao existe.

**Evidencia:** Nenhum arquivo `cmd/logs.go`. Grep por "logsCmd" retorna zero.

**Impacto:** Inconsistencia entre documentacao e realidade.

**Solucao:** Implementar `yby logs [pod]` como wrapper de `kubectl logs` com deteccao de namespace, ou remover da documentacao.

### 7.2 Sem `yby upgrade` para CLI ou templates

**Problema:** Nao existe mecanismo para atualizar o binario do CLI ou os templates embed.

**Evidencia:** Nenhum arquivo `cmd/upgrade.go`. Templates sao embed no binario.

**Impacto:** Atualizacao requer download manual ou rebuild.

**Solucao:** Implementar `yby upgrade` que verifica releases no GitHub e faz self-update, ou pelo menos um `yby version --check`.

### 7.3 Exemplos faltando em alguns comandos

**Problema:** `yby access`, `yby env check`, `yby chart create` nao tem campo `Example` definido.

**Evidencia:** `cmd/access.go` — sem campo Example no cobra.Command.

**Impacto:** `yby access --help` nao mostra exemplos de uso.

**Solucao:** Adicionar Examples com cenarios reais.

### 7.4 Erros genericos sem sugestoes de fix [CONCLUIDO em 2026-04-05]

**Problema:** Muitos erros retornam mensagem tecnica sem sugerir o que o usuario pode fazer.

**Evidencia:** `pkg/errors/errors.go` tem codigos estruturados mas sem campo `Suggestion`.

**Impacto:** Usuario precisa debugar sozinho.

**Solucao:** Adicionar campo `Hint string` ao YbyError. Exemplos: "Rode 'yby setup' para instalar dependencias", "Verifique se o cluster esta rodando com 'yby doctor'".

### 7.5 `cmd/exec.go` residual

**Problema:** Arquivo `cmd/exec.go` existe apenas para manter `var lookPath`. E um residuo da migracao para DI.

**Evidencia:** `cmd/exec.go` tem apenas 8 linhas com `var lookPath = exec.LookPath`.

**Impacto:** Confusao para desenvolvedores novos. Nao segue o padrao DI.

**Solucao:** Migrar usos de `lookPath` para `runner.LookPath` nos 2 arquivos restantes (seal.go, status.go) e remover `exec.go`.

---

## 8. Mirror e Sync

**Localizacao:** `pkg/mirror/`

### 8.1 Port-forward sem reconexao automatica

**Problema:** Se o port-forward cair (pod reiniciou, rede instavel), nao ha reconexao automatica.

**Evidencia:** `pkg/mirror/forwarder.go` — `Start()` cria port-forward uma vez. Sem health check ou reconnect.

**Impacto:** `yby up` perde sincronizacao se port-forward cair. `yby access` desconecta sem aviso.

**Solucao:** Wrapper com health check periodico (ping) e reconexao com backoff.

### 8.2 SyncLoop sem backoff em caso de erro

**Problema:** O loop de sincronizacao repete a cada tick fixo mesmo apos erros consecutivos.

**Evidencia:** `pkg/mirror/mirror.go` — SyncLoop usa `ticker.C` sem ajustar intervalo apos falha.

**Impacto:** Em caso de erro persistente, gera spam de logs a cada 5 segundos.

**Solucao:** Backoff exponencial em caso de falhas consecutivas. Reset ao sucesso.

---

## 9. Telemetria

**Localizacao:** `pkg/telemetry/`

### 9.1 Dados nao persistidos

**Problema:** Metricas sao coletadas em memoria e descartadas ao final da execucao. So aparecem com `--log-level debug`.

**Evidencia:** `pkg/telemetry/metrics.go` — `var events []Event` em memoria. `Flush()` escreve em slog.Debug.

**Impacto:** Impossivel analisar performance historica ou diagnosticar problemas recorrentes.

**Solucao:** Persistir em `~/.yby/telemetry.jsonl` com rotacao. Ou exportar via flag `--telemetry-export`.

### 9.2 Sem opt-in/opt-out explicito

**Problema:** Telemetria e sempre coletada. Nao ha flag para desabilitar.

**Evidencia:** `pkg/telemetry/metrics.go` — `Track()` sempre executa. Sem check de configuracao.

**Impacto:** Sem controle do usuario sobre coleta de dados.

**Solucao:** Flag `--no-telemetry` ou config `telemetry.enabled: false` em `~/.yby/config.yaml`.

---

## 10. Scaffold e Templates

### 10.1 Sem versionamento de templates

**Problema:** Templates sao embed no binario. Nao ha como saber qual versao gerou um scaffold.

**Evidencia:** `pkg/templates/fs.go` — `embed.FS` sem metadata de versao.

**Impacto:** Ao atualizar o CLI, impossivel saber se templates do projeto estao desatualizados.

**Solucao:** Adicionar campo `ybyVersion` ao `.yby/project.yaml` (ja implementado no manifest). Comando `yby init --check` para comparar.

### 10.2 Sem mecanismo de merge para re-init

**Problema:** `yby init --force` sobrescreve tudo. Nao ha merge inteligente entre templates novos e customizacoes do usuario.

**Evidencia:** `cmd/init.go` — `scaffold.Apply()` sobrescreve arquivos sem diff.

**Impacto:** Customizacoes do usuario sao perdidas ao re-inicializar.

**Solucao:** Implementar `yby init --update` que gera diff e aplica apenas templates novos/modificados, preservando customizacoes.

---

## 11. Configuracao Global

### 11.1 Sem arquivo de configuracao global [CONCLUIDO em 2026-04-05]

**Problema:** Nao existe `~/.yby/config.yaml` para configuracoes persistentes do CLI.

**Evidencia:** Nenhum codigo carrega `~/.yby/config.yaml`. Tudo via flags ou env vars.

**Impacto:** Usuario precisa passar `--log-level`, `--ai-provider` etc. a cada execucao.

**Solucao:** Criar `~/.yby/config.yaml` com: `ai.provider`, `ai.model`, `log.level`, `log.format`, `telemetry.enabled`. Carregar no `cmd/root.go` via viper ou yaml direto.

---

## Resumo por Prioridade

### P0 — Criticos (bloqueiam uso em producao)

| # | Area | Melhoria | Impacto | Status |
|---|------|----------|---------|--------|
| 1.1 | IA | Gemini streaming real | UX congelada em respostas longas | ✅ |
| 1.2 | IA | Retry universal (nao so Gemini) | Falhas em rede instavel | ✅ |
| 5.1 | Viz | Expandir alem de pods | Dashboard inutil para operacao real | ✅ |
| 8.1 | Mirror | Port-forward com reconexao | Sync quebra silenciosamente | ✅ |

### P1 — Importantes (impactam experiencia significativamente)

| # | Area | Melhoria | Impacto | Status |
|---|------|----------|---------|--------|
| 1.3 | IA | Token counting | Overflow silencioso, custos | ✅ |
| 1.5 | IA | Model selection configuravel | Impossivel usar modelos especificos | ✅ |
| 2.1 | Atlas | Mais linguagens (Python, Java, Rust) | Projetos nao-Go/Node ignorados | ✅ |
| 3.1 | Bard | Token counting | Prompts truncados | ✅ |
| 4.3 | Synapstor | Comando search | Conhecimento inacessivel diretamente | ✅ |
| 5.2 | Viz | Metricas CPU/Memory | Dashboard sem dados de recursos | ✅ |
| 7.1 | CLI | Implementar yby logs | Documentacao inconsistente | ✅ |
| 7.4 | CLI | Sugestoes de fix nos erros | UX de erro ruim | ✅ |
| 11.1 | Config | Arquivo ~/.yby/config.yaml | Flags repetitivas | ✅ |

### P2 — Melhorias (qualidade de vida)

| # | Area | Melhoria | Impacto |
|---|------|----------|---------|
| 1.4 | IA | Rate limiting | Concorrencia alta |
| 1.6 | IA | Cost tracking | Gestao de custos |
| 1.7 | IA | Ollama batch embeddings | Performance indexacao |
| 1.8 | IA | Cache de embeddings | Chamadas redundantes |
| 1.9 | IA | Vector store delete | Documentos obsoletos |
| 2.2 | Atlas | Deteccao de relacoes | Analise incompleta |
| 2.3 | Atlas | Ignores com filepath.Base | False positives |
| 3.2 | Bard | Historico com sessoes | Conversas misturadas |
| 3.4 | Bard | Modo batch | Integracao CI |
| 4.1 | Synapstor | Hook context | Integracao cross-plugin |
| 4.2 | Synapstor | Scanner com scoring | Busca sem relevancia |
| 4.4 | Synapstor | Metricas de indexacao | Feedback ausente |
| 5.3 | Viz | Filtros e busca | Clusters grandes |
| 5.4 | Viz | Scroll e paginacao | UI cortada |
| 5.5 | Viz | Reconexao automatica | Resiliencia |
| 7.2 | CLI | yby upgrade | Self-update |
| 7.3 | CLI | Exemplos faltantes | Ajuda incompleta |
| 8.2 | Mirror | SyncLoop com backoff | Spam de logs |
| 9.1 | Telemetria | Persistencia | Historico perdido |
| 10.2 | Scaffold | Merge em re-init | Customizacoes perdidas |

---

*Ultima atualizacao: 2026-04-05 — 9 itens P1 concluidos (1.3, 1.5, 2.1, 3.1, 4.3, 5.2, 7.1, 7.4, 11.1)*
