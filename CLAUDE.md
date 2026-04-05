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

**Yby CLI** é um assistente de infraestrutura Kubernetes escrito em Go 1.24. Combina scaffolding interativo, automação de clusters (k3d local / VPS remoto), IA multi-provider e um sistema de plugins baseado em processos.

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
  telemetry/      → Coleta de métricas por comando
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

Factory (`pkg/ai/factory.go`) auto-detecta providers na ordem: Ollama (local) → Gemini → OpenAI. Idioma padrão via `YBY_AI_LANGUAGE` (default: `pt-BR`). Modelo selecionável via `YBY_AI_MODEL` (aplica-se a qualquer provider). `TokenAwareProvider` é um decorator que valida se o prompt cabe no context window antes de enviar, com contagem de tokens via `pkg/ai/tokencount.go`.

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

## Convenções

- **Versionamento:** ldflags injetam `Version`, `commit`, `date` via GoReleaser
- **Linting:** golangci-lint com gofmt, govet, ineffassign, revive (exclui vendor, testdata, test/)
- **Logs:** `log/slog` estruturado — nunca usar `fmt.Println` para output de diagnóstico
- **Erros em comandos:** retornar `error` (nunca chamar `os.Exit` diretamente nos RunE)
- **Testes:** mocks via `testutil/`, testes extras em `*_extra_test.go`, E2E separados por build tag
