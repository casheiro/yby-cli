# Yby CLI — Constitution
# Version: 1.0.0

Princípios fundamentais que governam o desenvolvimento do Yby CLI via forgeAI.
Toda especificação, plano e implementação DEVE estar em conformidade com estes
princípios. Violações detectadas pela fase analyze têm severidade CRITICAL.

---

## Principle 1: Dependency Injection Over Concrete Coupling

Todos os serviços usam injeção de dependência via construtor com as interfaces
`shared.Runner` e `shared.Filesystem`. Nunca instanciar dependências concretas
dentro de serviços — recebê-las como parâmetro. Para testes, usar
`testutil.MockRunner` e `testutil.MockFilesystem`. CloudProviders e
TokenGenerators seguem o mesmo padrão.

## Principle 2: Structured Errors With Hints

Usar `pkg/errors.YbyError` com códigos padronizados (`ERR_IO`, `ERR_NETWORK_TIMEOUT`,
`ERR_CLUSTER_OFFLINE`, `ERR_PLUGIN`, `ERR_VALIDATION`, `ERR_CONFIG`, etc.).
Sempre usar `.WithHint()` para sugestões de correção exibidas ao usuário.
Registry de hints automáticos em `pkg/errors/hints.go`. Nunca retornar erros
genéricos `fmt.Errorf` em código de serviço — wrapping obrigatório com
`errors.Wrap(cause, code, message)`.

## Principle 3: Build Tags for Optional Dependencies

Dependências pesadas (SDKs cloud, K8s client-go) devem usar build tags
condicionais (`//go:build k8s`, `//go:build aws`, `//go:build azure`,
`//go:build gcp`). O build padrão (`task build`) produz binário sem SDKs
opcionais. Arquivos com build tag DEVEM ter um stub correspondente
(`_stub.go` com `//go:build !tag`) para graceful degradation. Padrão
já estabelecido: sentinel com `//go:build k8s`.

## Principle 4: Plugins Are Separate Processes

Plugins (bard, atlas, sentinel, synapstor, viz) são binários independentes
que se comunicam via JSON (STDIN/ENV). Plugins geram artefatos em arquivo,
nunca JSON no stdout para o usuário. Contexto passa via `YBY_PLUGIN_REQUEST`
(env var) ou STDIN (non-interactive). `KubeConfig` path nunca é exposto a
plugins por segurança — apenas `KubeContext` e `Namespace`.

## Principle 5: Configuration Precedence

Flags > variáveis de ambiente (`YBY_*`) > config file (`~/.yby/config.yaml`)
> defaults. Usar `pkg/config` com Viper. Nunca ler env vars diretamente em
serviços — acessar via Config struct. Credenciais cloud vêm de mecanismos
nativos do provider (profiles, env vars, instance metadata) — NUNCA
armazenar tokens, passwords ou API keys em `environments.yaml`,
`config.yaml` ou qualquer arquivo plaintext.

## Principle 6: English in Code, PT-BR in Terminal

Todo código, comentários, variáveis, funções e nomes de arquivo em inglês.
Mensagens de terminal (status, erros, prompts, relatórios) em português do
Brasil (PT-BR). System prompts enviados a LLMs em inglês para performance
ótima do modelo. Logs estruturados (`slog`) em inglês.

## Principle 7: Test With Real Commands

Nunca declarar "testes passam" sem rodar o comando real. Testes unitários
via `go test ./...`, testes com race detector via `-race`, E2E via
`task test:e2e`. Mocks via `testutil/` para isolamento. Build tags separam
testes: `e2e` para E2E, `integration` para chamadas a APIs reais (não
rodam em CI). Testes de concorrência obrigatórios para código com mutex
ou goroutines.

## Principle 8: Backward Compatibility in Data Structures

Alterações em structs serializadas (YAML, JSON) devem ser backward-compatible.
Campos novos usam `omitempty`. Arquivos existentes (`environments.yaml`,
`config.yaml`, `project.yaml`) devem carregar sem erro após mudanças.
Nunca remover campos sem deprecation cycle. Ponteiros (`*Type`) para campos
opcionais permitem teste `nil` e omissão na serialização.

## Principle 9: Observable Operations

Usar `log/slog` estruturado — nunca `fmt.Println` para output de diagnóstico.
Telemetria via `pkg/telemetry`. Usage tracking de IA via `SetUsage()` e
`CostTrackingProvider`. Erros em comandos Cobra retornam `error` — nunca
chamar `os.Exit` diretamente nos `RunE`. Operações cloud devem registrar
identidade, provider e resultado para auditoria.

## Principle 10: Minimal Scope, No Speculative Abstractions

Implementar apenas o que foi pedido. Não adicionar features, refactors ou
"melhorias" além do escopo. Não criar helpers, utilities ou abstrações
para operações únicas. Três linhas similares de código são melhor que uma
abstração prematura. Não adicionar docstrings, comentários ou type
annotations a código que não foi alterado. Bug fix não precisa de cleanup
do código ao redor.

---

## Governance

- **Amendment process**: Mudanças na constitution requerem aprovação explícita
  do usuário. forgeAI pode propor emendas mas nunca auto-aplica.
- **Versioning**: Semantic versioning (MAJOR.MINOR.PATCH). Mudanças de
  princípio são MAJOR.
- **Propagation**: Constitution é injetada nos prompts de prospect e analyze.
  Todas as especificações são checadas contra estes princípios.
