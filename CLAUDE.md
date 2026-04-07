# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Idioma

Todo conteúdo deve ser produzido em português do Brasil (PT-BR): respostas, comentários, docstrings, mensagens de erro, documentação. Exceções: identificadores de código (variáveis, funções, classes) e palavras-chave da linguagem.

## Comandos de Desenvolvimento

```bash
# Build (CLI + todos os plugins)
task build

# Testes unitários
task test

# Teste unitário de um pacote específico
go test -v ./pkg/services/bootstrap/...

# Teste unitário de uma função específica
go test -v -run TestNomeDaFuncao ./pkg/services/bootstrap/...

# Testes E2E (requer Docker)
task test:e2e

# Limpar binários
task clean

# Lint
golangci-lint run
```

**Build tags importantes:**
- `k8s` — necessária para compilar `plugins/sentinel/cli` e `plugins/sentinel/agent`
- `e2e` — isola testes E2E em `test/e2e/` (não rodam no `task test`)
- `integration` — isola testes que fazem chamadas reais a APIs externas (ex: Gemini). Rodar localmente com: `go test -tags=integration ./pkg/ai/...`. **NÃO rodar no CI.**

## Arquitetura

**Yby CLI** é um assistente de infraestrutura Kubernetes escrito em Go 1.26. Combina scaffolding interativo, automação de clusters (k3d local / VPS remoto), IA multi-provider e um sistema de plugins baseado em processos.

### Camadas

```
cmd/              → Camada CLI (Cobra). Cada arquivo = um comando/subcomando.
                    root.go: descoberta dinâmica de plugins, telemetria, tratamento de erros.

pkg/              → Lógica de negócio e utilitários:
  services/       → Serviços com injeção de dependência via interfaces (shared.Runner, shared.Filesystem)
    bootstrap/    → Bootstrap K8s (Argo CD, secrets, configs) — usa K8sClient interface
    environment/  → Orquestração do "up" (k3d local vs. remote) — usa ClusterManager, MirrorService
    network/      → Port-forward e credenciais — usa ClusterNetworkManager, LocalContainerManager
    secrets/      → Gestão de secrets
    logs/         → Serviço de logs de pods (wrapper kubectl logs com detecção de namespace)
    doctor/       → Diagnósticos
    shared/       → Interfaces Runner/Filesystem + adaptadores reais (RealRunner, RealFilesystem)
  config/         → Configuração global (~/.yby/config.yaml), carregamento com precedência flags > env > config > defaults
  ai/             → Providers de IA (Ollama > Gemini > OpenAI), factory com auto-detect, vector store
  plugin/         → Sistema de plugins: Manager (discover/install), Executor, Types (manifesto/request/response)
  context/        → Contexto de projeto (CoreContext via Synapstor/README) e ambientes (.yby/environments.yaml)
  scaffold/       → Engine de templates (Go text/template), filtros por topology/workflow/features
  errors/         → YbyError com código estruturado, wrapping e contexto diagnóstico
  executor/       → Execução de comandos (local + SSH)
  mirror/         → Git mirror server no cluster + túnel + sync loop
  retry/          → Exponential backoff (cenkalti/backoff)
  logger/         → slog estruturado (text/json)
  telemetry/      → Coleta de métricas por comando, persistência em ~/.yby/telemetry.jsonl
  filesystem/     → Composite FS (overlay de múltiplos fs.FS)
  testutil/       → MockRunner, MockFilesystem, exec_mock

plugins/          → Plugins nativos (processos separados, comunicação JSON via STDIN/ENV):
  atlas/          → Descoberta de recursos e scanning de blueprints
  bard/           → Assistente IA interativo (TUI)
  sentinel/       → Monitoramento K8s e segurança (build tag: k8s)
  synapstor/      → Gestão de conhecimento
  viz/            → Visualização de infraestrutura

test/e2e/         → Testes E2E com godog/Cucumber (build tag: e2e)
```

### Protocolo de Plugins

Plugins são binários independentes que se comunicam via JSON:
- **Descoberta:** Manager escaneia `~/.yby/plugins/` e `./.yby/plugins/`
- **Hooks:** `manifest` (capabilities), `context` (contribui dados), `command` (execução interativa), `assets` (arquivos estáticos)
- **Execução:** `Run()` captura STDOUT/STDERR parseando JSON; `RunInteractive()` passa contexto via env var `YBY_PLUGIN_REQUEST` e conecta TTY

### Tratamento de Erros

Usar `pkg/errors.YbyError` com códigos padronizados:
- `errors.New(code, message)` — erro sem causa
- `errors.Wrap(cause, code, message)` — wrapping com causa
- `.WithContext(key, value)` — adiciona contexto diagnóstico
- `.WithHint(hint)` — adiciona sugestão de correção exibida ao usuário (ex: "Rode 'yby doctor' para verificar dependências")
- Registry de hints automáticos em `pkg/errors/hints.go` — mapeia códigos de erro para sugestões padrão
- Códigos: `ERR_IO`, `ERR_NETWORK_TIMEOUT`, `ERR_CLUSTER_OFFLINE`, `ERR_PLUGIN`, `ERR_VALIDATION`, `ERR_CONFIG`, `ERR_SCAFFOLD_FAILED`, etc.

### Padrão de Serviços

Todos os serviços usam **injeção de dependência via construtor** com as interfaces `shared.Runner` e `shared.Filesystem`. Para testes, usar `testutil.MockRunner` e `testutil.MockFilesystem`.

### IA

Factory (`pkg/ai/factory.go`) auto-detecta providers na ordem configurável via `ai.priority` em `~/.yby/config.yaml`. Ordem padrão: Ollama (local) → Claude Code CLI → Gemini CLI → Gemini API → OpenAI API. Idioma padrão via `YBY_AI_LANGUAGE` (default: `pt-BR`). Modelo selecionável via `YBY_AI_MODEL` (aplica-se a qualquer provider).

**Providers CLI:** `ClaudeCLIProvider` (`pkg/ai/claude_cli.go`) e `GeminiCLIProvider` (`pkg/ai/gemini_cli.go`) usam os CLIs `claude -p` e `gemini -p` como providers de IA. Suportam `Completion` e `StreamCompletion`, não suportam embeddings. Valores aceitos em `ai.provider`: `ollama`, `gemini`, `openai`, `claude-cli`, `gemini-cli`.

**Prioridade configurável:** `ai.priority` em config.yaml define a ordem de tentativa. `GetAllAvailableProviders()` retorna todos em cascata para retry automático.

**Cadeia de decorators** (aplicados em `wrapProvider`):
```
Raw Provider → CachedEmbeddingProvider → TokenAwareProvider → CostTrackingProvider → RateLimitProvider → RetryProvider
```

- `CachedEmbeddingProvider` — cache LRU (SHA-256 key, 1000 entries, TTL 1h) para embeddings, evita chamadas redundantes
- `TokenAwareProvider` — valida se o prompt cabe no context window antes de enviar (estimativa ~4 chars/token)
- `CostTrackingProvider` — extrai `usage` (prompt_tokens, completion_tokens) das respostas OpenAI/Gemini e loga custo estimado via `slog.Info("ai.usage", ...)`
- `RateLimitProvider` — token bucket (`golang.org/x/time/rate`) + circuit breaker (closed/open/half-open). Respeita header `Retry-After`. Config: `ai.rate_limit.requests_per_second`
- `RetryProvider` — retry com backoff exponencial (cenkalti/backoff) para erros 429/502/503

**VectorStore** (`pkg/ai/vector_store.go`): wrapper ChromemDB com `AddDocuments`, `Search`, `DeleteDocuments`, `DeleteByMetadata`, `Count`. O indexer do Synapstor usa `DeleteByMetadata` para limpar embeddings de arquivos removidos durante reindexação.

**Ollama batch embeddings**: usa `/api/embed` (batch nativo, Ollama v0.5+) com fallback automático para `/api/embeddings` (sequencial) em versões antigas.

### Configuração de Ambientes

Arquivo `.yby/environments.yaml` define ambientes (local/remote) com tipo, valores, kubeconfig e namespace. Contexto ativo via flag `--context` ou env var `YBY_ENV`.

### Configuração Global

Arquivo `~/.yby/config.yaml` (`pkg/config/`) persiste preferências do usuário:
- `ai.provider` — provider de IA preferido (ollama, gemini, openai)
- `ai.model` — modelo específico (ex: gpt-4-turbo, gemini-pro)
- `ai.language` — idioma das respostas de IA (padrão: pt-BR)
- `log.level` — nível de log (debug, info, warn, error)
- `log.format` — formato de log (text, json)
- `telemetry.enabled` — habilita/desabilita coleta de métricas

**Precedência:** flags > variáveis de ambiente > config.yaml > defaults

### Comando `yby logs`

`yby logs [pod] [-n namespace] [--follow] [--tail N]` — wrapper inteligente para visualização de logs de pods. Detecta namespace automaticamente a partir do contexto ativo. Implementado em `cmd/logs.go` com serviço em `pkg/services/logs/`.

### Comando `yby upgrade`

`yby upgrade [--check] [--force] [--version <tag>]` — self-update do CLI via GitHub releases. Verifica checksum SHA256, faz rollback em caso de falha. Implementado em `cmd/upgrade.go`.

### Comando `yby telemetry export`

`yby telemetry export` — exporta eventos de telemetria persistidos em `~/.yby/telemetry.jsonl` para stdout. Implementado em `cmd/telemetry.go`.

### Scaffold Merge (`yby init --update`)

`yby init --update` faz merge inteligente entre templates novos e customizações do usuário, usando hash tracking (SHA-256) no manifest (`.yby/project.yaml`). Mutuamente exclusivo com `--force`. Resolve conflitos interativamente (survey) ou com marcadores estilo Git (`--non-interactive`). Implementado em `pkg/scaffold/merge.go`.

### Plugin Bard — Assistente IA (v1.0.0)

- **Tool Calling**: sistema de tools (kubectl get/logs/events/describe, sentinel scan, atlas blueprint) com loop de execução (max 5 iterações)
- **TUI**: interface Bubbletea com viewport Glamour, input multiline, status bar. Modo legado via `YBY_BARD_LEGACY_UI=1`
- **Context Awareness**: auto-injeta namespace, pods e eventos no system prompt via kubectl
- **Guardrails**: validação de operações perigosas antes de executar tools
- **Sessões e Batch**: histórico com SessionID, modo batch non-TTY

### Plugin Atlas — Scanner de Topologia (v1.0.0)

- **Analyzers de infraestrutura**: Helm (`Chart.yaml` + templates), K8s manifests (YAML com `apiVersion/kind`), Docker Compose, Kustomize, Terraform (regex)
- **Diagrama Mermaid**: `yby atlas diagram` gera `.yby/atlas-diagram.mmd` com topologia real. Overview (padrão, max 25 nós) e full (`--detail full`)
- **Refinamento IA**: diagrama programático enviado ao LLM para reorganização e nomes legíveis. Desativável com `--no-ai`
- **Filtros inteligentes**: exclui templates Helm não renderizados (`{{ }}`), secrets SOPS encriptados, nós isolados no overview
- **Detecção de relações**: Service→Pod (selects), Ingress→Service (routes), ArgoCD App→Chart (syncs), Helm deps (depends_on)

### Plugin Sentinel — Auditoria K8s (v1.0.0)

- **Backends reais**: Polaris SDK (pod security, best practices) + OPA SDK (RBAC, network) em vez de checks artesanais
- **Agrupamento inteligente**: findings deduplicados por check, mostra "Deployment/api (+5)" em vez de repetir por workload
- **Allowlists**: RBAC ignora automaticamente `system:*`, controllers conhecidos (argocd, cert-manager, traefik, etc.)
- **Investigacao IA inteligente**: verifica saude do pod antes de chamar IA — so aciona quando detecta problemas reais
- **Remediacao**: `--fix-dry-run` e `--fix` com strategic-merge patches
- **Relatorios**: resumo no terminal + relatorio completo em `~/.yby/reports/sentinel-scan-{namespace}-{data}.md`
- **Cache**: investigacoes em `~/.yby/sentinel/cache/` (TTL 1h), nao polui diretorio do projeto
- **Fallback**: checks artesanais internos quando backends nao estao disponiveis

### Plugin Synapstor — Gestão de Conhecimento (v1.0.0)

- **Capture**: transforma texto livre em UKI estruturado via IA
- **Study**: analisa codigo do projeto e gera documentacao tecnica via IA
- **Search**: busca semantica nos UKIs via embeddings (Ollama local ou API)
- **Index**: indexacao incremental com SHA-256 tracking e embeddings configuráveis por provider
- **Quality Scoring**: score 0-100 por UKI (contexto, exemplos, headers, links, metadata)
- **Knowledge Decay**: deteccao de UKIs stale (>90 dias sem atividade git)
- **Export multi-formato**: Docusaurus, Obsidian, Markdown puro

### Plugin Viz — Observabilidade TUI (v1.0.0)

- **Dashboard TUI**: visualizacao de Pods, Deployments, StatefulSets, Services em tempo real via Bubbletea
- **Real K8s Client**: conexao direta ao cluster via client-go (nao usa kubectl)
- **RetryClient**: reconexao com backoff exponencial, preserva ultimo estado durante reconexao
- **Filtros**: namespace (`/`), label (`L`), status (`S`) com filtro server-side via `ListFilter`
- **Scroll**: PgUp/PgDn, Home/End, indicador de posicao na status bar
- **Tabs**: navegacao entre tipos de recurso (Pods, Deployments, StatefulSets, Services)
- **Acoes**: delete, scale, restart direto da TUI com confirmacao
- **Detail view**: YAML viewer para inspecao de recursos
- **Logs**: visualizacao de logs de pods em tempo real
- **Search**: busca por nome de recurso com highlight

### Enterprise Overrides

Sistema de customização enterprise via arquivo YAML (`pkg/scaffold/overrides.go`):
- **Struct `EnterpriseOverrides`** com sub-structs: Registry, Cloud, Namespaces, Ingress, TLS, Helm, Images, Git, Profiles, Observability
- **Carregamento**: `LoadOverrides(paths...)` com precedência `--config` > `.yby/overrides.yaml` (projeto) > `~/.yby/overrides.yaml` (global) > defaults vazios
- **Funções Resolve***: `ResolveImage()`, `ResolveNamespace()`, `ResolveStorageClass()`, `ResolveIngressClass()`, `ResolveHelmRepo()`, `ResolveChartVersion()`, `ResolveTLSIssuer()`, `ResolveGitProvider()`, `ResolveObservability()`, `ResourceProfile()`
- **Integração com templates**: `contextFuncMap(ctx)` em `engine.go` disponibiliza todas as funções Resolve* nos templates `.tmpl`
- **BlueprintContext**: campo `Overrides *EnterpriseOverrides` passa overrides para toda a cadeia de rendering
- **Bootstrap**: `BootstrapOptions.Overrides` parametriza Helm repos, versões e namespaces
- **Backward-compat**: sem overrides, todas as funções retornam o valor original (zero breaking change)

## Convenções

- **Versionamento:** ldflags injetam `Version`, `commit`, `date` via GoReleaser
- **Linting:** golangci-lint com gofmt, govet, ineffassign, revive (exclui vendor, testdata, test/)
- **Logs:** `log/slog` estruturado — nunca usar `fmt.Println` para output de diagnóstico
- **Erros em comandos:** retornar `error` (nunca chamar `os.Exit` diretamente nos RunE)
- **Testes:** mocks via `testutil/`, testes extras em `*_extra_test.go`, E2E separados por build tag
