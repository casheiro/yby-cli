# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Idioma

Todo conteúdo deve ser produzido em português do Brasil (PT-BR): respostas, comentários, docstrings, mensagens de erro, documentação. Exceções: identificadores de código (variáveis, funções, classes) e palavras-chave da linguagem.

## Comandos de Desenvolvimento

```bash
# Build (CLI + todos os plugins)
task build

# Build variante cloud (inclui SDKs AWS/Azure/GCP)
task build:cloud

# Testes unitários
task test

# Teste unitário de um pacote específico
go test -v ./pkg/services/bootstrap/...

# Teste unitário de uma função específica
go test -v -run TestNomeDaFuncao ./pkg/services/bootstrap/...

# Testes com build tags cloud
go test -tags aws ./pkg/ai/... -race
go test -tags aws ./pkg/cloud/... -race

# Testes E2E (requer Docker)
task test:e2e

# Limpar binários
task clean

# Lint
golangci-lint run
```

**Build tags importantes:**
- `k8s` — necessária para compilar `plugins/sentinel/cli` e `plugins/sentinel/agent`
- `aws` — habilita Amazon Bedrock AI provider e token generator EKS via SDK
- `azure` — habilita token generator AKS via azidentity SDK
- `gcp` — habilita token generator GKE via oauth2/google SDK
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
    environment/  → Orquestração do "up" (k3d local vs. remote vs. eks/aks/gke) — usa ClusterManager, MirrorService
    network/      → Port-forward e credenciais — usa ClusterNetworkManager, LocalContainerManager
    secrets/      → Gestão de secrets
    logs/         → Serviço de logs de pods (wrapper kubectl logs com detecção de namespace)
    doctor/       → Diagnósticos (inclui seção Cloud Providers)
    shared/       → Interfaces Runner/Filesystem + adaptadores reais (RealRunner, RealFilesystem)
  config/         → Configuração global (~/.yby/config.yaml), carregamento com precedência flags > env > config > defaults
  ai/             → Providers de IA (Ollama > Gemini > OpenAI > Bedrock), factory com auto-detect, vector store
  cloud/          → Suporte multi-cloud (AWS EKS, Azure AKS, GCP GKE): detecção, token generators, auto-refresh
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
- Códigos: `ERR_IO`, `ERR_NETWORK_TIMEOUT`, `ERR_CLUSTER_OFFLINE`, `ERR_PLUGIN`, `ERR_VALIDATION`, `ERR_CONFIG`, `ERR_SCAFFOLD_FAILED`, `ERR_CLOUD_TOKEN_EXPIRED`, `ERR_CLOUD_CLI_MISSING`, `ERR_CLOUD_MODEL_DISABLED`, etc.

### Padrão de Serviços

Todos os serviços usam **injeção de dependência via construtor** com as interfaces `shared.Runner` e `shared.Filesystem`. Para testes, usar `testutil.MockRunner` e `testutil.MockFilesystem`.

### IA

Factory (`pkg/ai/factory.go`) auto-detecta providers na ordem configurável via `ai.priority` em `~/.yby/config.yaml`. Ordem padrão: Ollama (local) → Claude Code CLI → Gemini CLI → Gemini API → OpenAI API → Bedrock (cloud). Idioma padrão via `YBY_AI_LANGUAGE` (default: `pt-BR`). Modelo selecionável via `YBY_AI_MODEL` (aplica-se a qualquer provider).

**Providers CLI:** `ClaudeCLIProvider` (`pkg/ai/claude_cli.go`) e `GeminiCLIProvider` (`pkg/ai/gemini_cli.go`) usam os CLIs `claude -p` e `gemini -p` como providers de IA. Suportam `Completion` e `StreamCompletion`, não suportam embeddings. Valores aceitos em `ai.provider`: `ollama`, `gemini`, `openai`, `claude-cli`, `gemini-cli`, `bedrock`.

**Amazon Bedrock** (`pkg/ai/bedrock.go`, `//go:build aws`): provider de IA usando AWS Bedrock Converse API. Requer build tag `aws` e credenciais AWS configuradas. `Completion` e `StreamCompletion` via Converse/ConverseStream API. `EmbedDocuments` via InvokeModel sequencial (Titan `amazon.titan-embed-text-v2:0`). Modelo padrão: `anthropic.claude-3-5-sonnet-20241022-v2:0`. Registrado via `init()` em `bedrock_factory.go` usando sistema de `registeredProviders` para providers com build tags condicionais.

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

### Suporte Multi-Cloud (EKS/AKS/GKE)

Pacote `pkg/cloud/` fornece abstração para clusters K8s gerenciados em clouds públicas:

- **Interface `CloudProvider`** (`provider.go`): `Name`, `IsAvailable`, `CLIVersion`, `ListClusters`, `ConfigureKubeconfig`, `ValidateCredentials`, `RefreshToken`. Registry com auto-registro via `init()`.
- **Detecção automática** (`detect.go`): parseia `exec.command` do kubeconfig ativo (padrões: `aws`→AWS, `kubelogin`/`az`→Azure, `gke-gcloud-auth-plugin`/`gcloud`→GCP). Sem I/O de rede, < 100ms.
- **Providers CLI** (`aws.go`, `azure.go`, `gcp.go`): implementações via CLIs `aws`, `az`, `gcloud` usando `shared.Runner`. Zero dependências externas.
- **Token generators SDK** (`aws_token.go`, `azure_token.go`, `gcp_token.go`): build tags `aws`/`azure`/`gcp`. Stubs (`*_stub.go`) com fallback CLI quando sem tags.
- **Token cache** (`token_cache.go`): thread-safe (`sync.RWMutex`), TTL com margem 60s.
- **AutoRefreshTransport** (`token_refresh.go`): `http.RoundTripper` que intercepta 401, faz refresh serializado via mutex, propaga 403 sem retry.
- **Integração K8s** (`pkg/plugin/sdk/sdk_k8s_cloud.go`): quando `Cloud != nil` e build tag presente, `GetKubeClient()` injeta bearer token e `WrapTransport = AutoRefreshTransport`.

### Comandos `yby cloud`

- `yby cloud connect [--provider P] [--region R] [--cluster C] [--env-name N]` — guided setup interativo ou não-interativo para conexão a cluster cloud
- `yby cloud list [--provider P] [--region R]` — lista clusters disponíveis em tabela
- `yby cloud status` — exibe credenciais, identidade, expiração do token
- `yby cloud refresh [--provider P]` — força refresh do token de autenticação

Implementados em `cmd/cloud.go`, `cmd/cloud_connect.go`, `cmd/cloud_list.go`, `cmd/cloud_status.go`, `cmd/cloud_refresh.go`.

- `yby cloud audit [--since DURATION] [--export json|csv] [--provider P]` — consulta audit log de operações cloud
- `yby cloud dashboard` — TUI interativo multi-cluster com auto-refresh via Bubbletea

Implementados em `cmd/cloud_audit.go`, `cmd/cloud_dashboard.go`.

### Auth Avançada (Nível 3)

Suporte enterprise-grade a múltiplos métodos de autenticação cloud:

- **AWS:** SSO (Identity Center), assume-role em cadeia (cross-account), IRSA (web identity para pods EKS), MFA no assume-role. Implementado em `pkg/cloud/aws_auth.go` (build tag `aws`).
- **Azure:** device code flow, interactive browser login, service principal com certificado X.509, Managed Identity (MSI). Implementado em `pkg/cloud/azure_auth.go` (build tag `azure`).
- **GCP:** Workload Identity Federation (identidades externas), SA impersonation (sem arquivo de chave), GKE Connect Gateway (clusters Fleet). Implementado em `pkg/cloud/gcp_auth.go` (build tag `gcp`).

**AuthConfig** em `CloudConfig` (`pkg/context/context.go`): campos `method`, `sso_start_url`, `sso_region`, `sso_account`, `sso_role_name`, `mfa_serial`. CloudConfig também inclui `service_account`, `credentials_file`, `fleet_membership` para GCP.

### Credential Store

Pacote `pkg/cloud/credential_store.go`: armazenamento seguro de tokens cloud de longa duração (SSO sessions). Usa OS keychain via `go-keyring` (macOS Keychain, Linux secret-service/kwallet, Windows Credential Manager) com fallback para arquivo encriptado AES-256-GCM em `~/.yby/credentials.enc`.

### Audit Log

Pacote `pkg/cloud/audit.go`: log JSONL de todas as operações de autenticação cloud em `~/.yby/audit.log`. Registra quem autenticou, quando, com qual role/method, em qual cluster/provider. Rotação automática em 10MB. Comando `yby cloud audit` para consulta com filtros (`--since`, `--provider`) e export (`--export json|csv`).

### Configuração de Ambientes

Arquivo `.yby/environments.yaml` define ambientes (local/remote/eks/aks/gke) com tipo, valores, kubeconfig e namespace. Contexto ativo via flag `--context` ou env var `YBY_ENV`. Ambientes cloud possuem campo `Cloud *CloudConfig` opcional com metadados do provider (region, cluster, profile, role_arn, etc.).

O comando `yby env create` aceita flags `--cloud-provider`, `--cloud-region`, `--cloud-cluster` para criar ambientes cloud. O `yby env show` exibe metadata cloud quando presente.

### Configuração Global

Arquivo `~/.yby/config.yaml` (`pkg/config/`) persiste preferências do usuário:
- `ai.provider` — provider de IA preferido (ollama, gemini, openai, bedrock, claude-cli, gemini-cli)
- `ai.model` — modelo específico (ex: gpt-4-turbo, gemini-pro, anthropic.claude-3-5-sonnet-20241022-v2:0)
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

- **Agent com intent classifier**: classifica intencao do usuario via IA e executa tools programaticamente (nao depende da IA gerar JSON)
- **Tools integradas**: sentinel scan/investigate, kubectl get/logs/events/describe, atlas blueprint — executadas automaticamente pelo Bard
- **Enriquecimento Synapstor**: busca semantica nos UKIs do projeto para contexto automatico (RAG)
- **Capacidades do provider**: se o provider tem tools proprias (ex: Claude Code com MCP), o Bard aproveita sem restricoes
- **Tools externas**: usuario pode registrar tools customizadas via YAML em `~/.yby/tools/` ou `.yby/tools/`
- **3 modos de uso**: TUI interativo (`yby bard`), one-shot (`yby bard -p "pergunta"`), batch (`echo "pergunta" | yby bard`)
- **TUI Bubbletea**: viewport com markdown rendering (Glamour), input multiline, status bar
- **Sessoes**: historico por sessao, `/sessions` lista, `/session <id>` carrega

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
- **Build tags cloud:** código que depende de SDKs cloud usa `//go:build aws`/`azure`/`gcp`. Cada arquivo com build tag DEVE ter um stub correspondente (`*_stub.go` com `//go:build !tag`) para fallback CLI. `task build` (sem tags) nunca referencia SDKs cloud. `task build:cloud` compila com todas as tags.
- **GoReleaser:** binário padrão `yby` (sem SDKs cloud) + binário `yby-cloud` (com tags `aws,azure,gcp`)
