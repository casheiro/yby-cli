# Implementation Plan: Suporte Multi-Cloud (EKS/AKS/GKE) + Amazon Bedrock

## Technical Context

- **Language:** Go 1.26
- **Dependencies existentes relevantes:** `client-go v0.35.3` (já no projeto), `k8s.io/client-go/tools/clientcmd`, `github.com/spf13/cobra`, `pkg/services/shared` (Runner/Filesystem interfaces), `pkg/errors` (YbyError), `pkg/ai` (interface Provider e cadeia de decorators)
- **Dependências novas — Nível 1:** Nenhuma (tudo via `os/exec`)
- **Dependências novas — Nível 2 (build tag `aws`):** `github.com/aws/aws-sdk-go-v2` (core, config, credentials, stscreds, sts, eks, bedrockruntime), `sigs.k8s.io/aws-iam-authenticator/pkg/token`
- **Dependências novas — Nível 2 (build tag `azure`):** `github.com/Azure/azure-sdk-for-go/sdk/azidentity`, `azcore`
- **Dependências novas — Nível 2 (build tag `gcp`):** `golang.org/x/oauth2/google`
- **Dependências novas — Nível 3 (P3, fora do milestone):** `github.com/zalando/go-keyring`, `aws-sdk-go-v2/service/ssooidc`
- **Testing:** `go test ./...`, `go test -race`, MockRunner/MockFilesystem de `testutil/`, build tag `integration` para testes contra APIs reais (não rodam em CI)
- **Platform:** Linux (primário), macOS, Windows (credential store via go-keyring no keychain do OS)
- **Constraints:** Build padrão (`task build`) sem SDKs cloud (NFR-002); backward-compat em `environments.yaml` (NFR-001); nunca persistir tokens em arquivo plaintext (NFR-005); mensagens de terminal em PT-BR via i18n (NFR-010)

---

## Constitution Check

| Princípio | Status | Observação |
|-----------|--------|------------|
| **P1 — Programmatic Control Over LLM Trust** | ✅ Conforme | Toda verificação de credenciais, token expiry e disponibilidade de CLIs é determinística via Go code. Nenhuma decisão delegada a LLM. Bedrock é apenas canal de inferência — as decisões de pass/fail permanecem em Go. |
| **P2 — Iterate Until Satisfied** | ✅ Conforme | Não afetado por esta feature (controla o pipeline forgeAI, não a feature em si). |
| **P3 — Clean Context Per Invocation** | ✅ Conforme | BedrockProvider usa `claude -p` equivalente via Converse API com fresh context por chamada. TokenCache é in-memory por sessão, não compartilhado entre invocações CLI. |
| **P4 — Phases Have Strict Boundaries** | ✅ Conforme | `pkg/cloud` tem responsabilidade isolada. `pkg/ai/bedrock.go` segue padrão dos providers existentes. `cmd/cloud*.go` são apenas orquestradores de CLI. |
| **P5 — Specification Quality Drives Implementation Quality** | ✅ Conforme | Esta spec cobre 25 FRs, 13 USs, 10 NFRs com critérios de aceite precisos. |
| **P6 — Observable Pipeline** | ✅ Conforme | Auditoria em `~/.yby/audit.log` (FR-021, P3). Usage tracking Bedrock via slog `ai.usage` (FR-025). Doctor checks observáveis no terminal. |
| **P7 — Resilience Over Correctness of Execution Path** | ✅ Conforme | AutoRefreshTransport com mutex e retry (FR-015). Fallback CLI quando SDK ausente (FR-016). Fallback silencioso para keychain indisponível (FR-020). |
| **P8 — Zero External Dependencies Beyond Cobra** | ⚠️ TENSÃO | Esta feature adiciona AWS SDK, Azure SDK e GCP oauth2. **Mitigação:** build tags condicionais (`//go:build aws/azure/gcp/cloud`) garantem que o binário padrão permanece zero-dependency além de cobra. SDKs são opt-in. `task build` continua sem SDKs cloud. |
| **P9 — English in Code, Localized in Terminal** | ✅ Conforme | Todo código novo em inglês. Mensagens de terminal em PT-BR (via i18n quando disponível, diretamente enquanto não). System prompts Bedrock em inglês. |
| **P10 — Self-Hosting Development Model** | ✅ Conforme | Não afetado. |

**Tensão P8 mitigada:** A build tag `//go:build aws` (e análogas) é o mesmo padrão já usado no projeto para o plugin sentinel (`//go:build k8s`). O GoReleaser produz dois artefatos: `yby-linux-amd64` (sem SDKs) e `yby-cloud-linux-amd64` (com SDKs). A Taskfile adiciona `build:cloud` com `-tags cloud`.

---

## Architecture

### Components Affected

#### Arquivos Modificados

| Componente | Arquivo | Tipo de Mudança | Risco |
|-----------|---------|----------------|-------|
| Environment struct | `pkg/context/context.go` | Extensão com campo `Cloud *CloudConfig` (`omitempty`) | Médio — ponto central; `omitempty` garante backward-compat |
| AI Config validation | `pkg/config/config.go` | Adicionar `"bedrock"` na whitelist de `validProviders` | Baixo |
| AI Factory | `pkg/ai/factory.go` | Novo case `"bedrock"` em `createProvider`, `defaultPriority`, `embeddingCapableProviders` | Baixo |
| AI Cost tracking | `pkg/ai/cost_provider.go` | Tabela de preços Bedrock (Claude Sonnet/Haiku/Opus, Titan) | Baixo |
| AI Token count | `pkg/ai/tokencount.go` | Context windows dos modelos Bedrock | Baixo |
| AI Rate limit | `pkg/ai/ratelimit_provider.go` | Rate limit default para Bedrock (1 req/s conservador) | Baixo |
| K8s SDK | `pkg/plugin/sdk/sdk_k8s.go` | Injeção de bearer token via TokenGenerator quando `Cloud != nil` | Alto — ponto central de acesso K8s para todos os plugins |
| Doctor service | `pkg/services/doctor/service.go` | Novos checks: CLIs cloud instalados, versões, status de credenciais | Baixo |
| Environment service | `pkg/services/environment/service.go` | Suporte a tipos `eks`, `aks`, `gke` no `Up()` | Médio |
| Env command | `cmd/env.go` | Flag `--cloud-provider` em `env create`; metadata cloud em `env show` | Baixo |
| Doctor command | `cmd/doctor.go` | Seção "Cloud Providers" no relatório | Baixo |

#### Arquivos Novos

| Componente | Arquivo | Propósito |
|-----------|---------|----------|
| Cloud interface | `pkg/cloud/provider.go` | Interface `CloudProvider`, structs `ClusterInfo`, `CredentialStatus`, `ListOptions` |
| Cloud detection | `pkg/cloud/detect.go` | Parsing de kubeconfig (`exec.command`) e detecção de CLIs via `exec.LookPath` |
| AWS provider | `pkg/cloud/aws.go` | Implementação AWS EKS via CLI `aws` (IsAvailable, ValidateCredentials, ListClusters, ConfigureKubeconfig, RefreshToken) |
| Azure provider | `pkg/cloud/azure.go` | Implementação Azure AKS via CLI `az` |
| GCP provider | `pkg/cloud/gcp.go` | Implementação GCP GKE via CLI `gcloud` |
| Token interface | `pkg/cloud/token.go` | Interface `TokenGenerator`, struct `Token` |
| AWS token SDK | `pkg/cloud/aws_token.go` | `//go:build aws` — EKS token via aws-iam-authenticator lib |
| Azure token SDK | `pkg/cloud/azure_token.go` | `//go:build azure` — AKS token via azidentity |
| GCP token SDK | `pkg/cloud/gcp_token.go` | `//go:build gcp` — GKE token via oauth2/google |
| Token cache | `pkg/cloud/token_cache.go` | Cache in-memory thread-safe com TTL (sync.RWMutex, margem 60s) |
| Auto refresh | `pkg/cloud/token_refresh.go` | `AutoRefreshTransport` — http.RoundTripper com mutex, intercepta 401 |
| Credential store | `pkg/cloud/credential_store.go` | `//go:build cloud` — OS keychain via go-keyring (P3) |
| Audit log | `pkg/cloud/audit.go` | Log de operações de autenticação em `~/.yby/audit.log` JSONL (P3) |
| Bedrock provider | `pkg/ai/bedrock.go` | `//go:build aws` — BedrockProvider: Converse API + ConverseStream + InvokeModel (Titan Embeddings) |
| Bedrock tests | `pkg/ai/bedrock_test.go` | Testes unitários com mock do bedrockruntime client |
| Cloud command root | `cmd/cloud.go` | Subcomando raiz `yby cloud` registrado em root.go |
| Cloud connect | `cmd/cloud_connect.go` | `yby cloud connect` — guided setup interativo e não-interativo |
| Cloud list | `cmd/cloud_list.go` | `yby cloud list [--provider P] [--region R]` |
| Cloud status | `cmd/cloud_status.go` | `yby cloud status` |
| Cloud refresh | `cmd/cloud_refresh.go` | `yby cloud refresh` |
| AWS cloud tests | `pkg/cloud/aws_test.go` | Testes via MockRunner |
| Azure cloud tests | `pkg/cloud/azure_test.go` | Testes via MockRunner |
| GCP cloud tests | `pkg/cloud/gcp_test.go` | Testes via MockRunner |
| Token cache tests | `pkg/cloud/token_cache_test.go` | Testes de cache, TTL, concorrência |
| Token refresh tests | `pkg/cloud/token_refresh_test.go` | Testes do AutoRefreshTransport (mock RoundTripper) |
| E2E cloud tests | `test/e2e/cloud_kubeconfig_test.go` | `//go:build e2e` — Validação com exec plugin kubeconfig |

---

### Data Flow

#### Fluxo: `yby cloud connect` (modo não-interativo)

```
cmd/cloud_connect.go
  → cloud.Detect() [pkg/cloud/detect.go]
      → exec.LookPath("aws"/"az"/"gcloud")
      → retorna []CloudProvider disponíveis
  → provider.ValidateCredentials(ctx) [pkg/cloud/aws.go]
      → shared.Runner.Run("aws sts get-caller-identity")
      → JSON parse → CredentialStatus
  → provider.ListClusters(ctx, ListOptions{Region}) [pkg/cloud/aws.go]
      → shared.Runner.Run("aws eks list-clusters --region X")
      → shared.Runner.Run("aws eks describe-cluster --name X")
      → []ClusterInfo
  → provider.ConfigureKubeconfig(ctx, ClusterInfo) [pkg/cloud/aws.go]
      → shared.Runner.Run("aws eks update-kubeconfig --name X --region Y")
  → context.AddEnvironment(env) [pkg/context/context.go]
      → persiste em .yby/environments.yaml com Cloud: {provider, region, cluster}
```

#### Fluxo: `GetKubeClient()` com Cloud config

```
pkg/plugin/sdk/sdk_k8s.go
  → clientcmd.NewDefaultClientConfigLoadingRules().Load()
  → clientConfig.ClientConfig() → rest.Config
  → if env.Cloud != nil && build tag presente:
      → cloud.GetToken(ctx, env.Cloud) [pkg/cloud/token.go]
          → TokenCache.Get(key) → cache hit: retorna Token (< 1ms)
          → cache miss: TokenGenerator.GenerateToken(ctx, config)
              [//go:build aws] pkg/cloud/aws_token.go
                → awsconfig.LoadDefaultConfig()
                → aws-iam-authenticator token.NewGenerator()
                → Token{Value, ExpiresAt}
          → TokenCache.Set(key, token)
      → rest.Config.BearerToken = token.Value
      → rest.Config.BearerTokenFile = "" (limpar conflito)
      → rest.Config.WrapTransport = AutoRefreshTransport{...}
  → kubernetes.NewForConfig(config)
```

#### Fluxo: `AutoRefreshTransport` (401 detection)

```
http.RoundTripper.RoundTrip(req)
  → TokenCache.Get(key) → token válido
  → req.Header.Set("Authorization", "Bearer "+token)
  → Base.RoundTrip(req) → resp
  → if resp.StatusCode == 401:
      → t.mu.Lock() (serializa refresh concorrente)
      → TokenCache.Invalidate(key)
      → TokenGenerator.GenerateToken(ctx, cloudConfig) → novo Token
      → TokenCache.Set(key, token)
      → t.mu.Unlock()
      → req.Header.Set("Authorization", "Bearer "+newToken)
      → Base.RoundTrip(req) → resp final
  → if resp.StatusCode == 403: propaga sem retry
  → return resp
```

#### Fluxo: `yby bard -p "..."` com Bedrock

```
plugins/bard/... → pkg/ai/factory.go
  → createProvider("bedrock") → pkg/ai/bedrock.go
      → NewBedrockProvider()
          → awsconfig.LoadDefaultConfig(region)
          → bedrockruntime.NewFromConfig(cfg)
  → wrapProvider(bedrock, model) → cadeia:
      CachedEmbeddingProvider → TokenAwareProvider → CostTrackingProvider
      → RateLimitProvider → RetryProvider → BedrockProvider
  → BedrockProvider.Completion(ctx, systemPrompt, userPrompt)
      → bedrockruntime.Client.Converse(ctx, ConverseInput)
      → extrai texto de ConverseOutput
      → SetUsage(ctx, UsageMetadata{InputTokens, OutputTokens, ...})
  → CostTrackingProvider registra custo via tabela bedrock em cost_provider.go
```

---

## Technical Decisions

### TD-01: CLI-first, SDK-second (abordagem em camadas)

- **Opções:** (A) Apenas CLI via `os/exec`; (B) Apenas SDK Go nativo; (C) CLI-first no Nível 1, SDK no Nível 2
- **Escolhida:** C — CLI-first, SDK-second
- **Rationale:** Nível 1 entrega valor imediato sem novas dependências. Usuários com CLIs cloud instalados se beneficiam imediatamente. Nível 2 adiciona independência de CLIs para CI/CD headless. A interface `CloudProvider` unifica a API entre os dois níveis.
- **Trade-offs:** Duplicação parcial de lógica (CLI wrapper + SDK implementam a mesma interface). Mitigado pela interface comum e pelo isolamento via build tags.

---

### TD-02: Build tags condicionais para SDKs cloud

- **Opções:** (A) Sempre incluir todos os SDKs — binário +30MB; (B) Build tags por provider (`//go:build aws`, `//go:build azure`, `//go:build gcp`); (C) Cloud providers como plugins binários separados
- **Escolhida:** B — Build tags por provider, com `//go:build cloud` como alias para todos
- **Rationale:** Alinha com o padrão existente do projeto (build tag `k8s` para sentinel). Permite que o usuário compile apenas o que precisa. GoReleaser produz builds variantes. O build padrão (`task build`) continua sem SDKs.
- **Trade-offs:** Complexidade de build e manutenção de stubs (arquivos `_stub.go` com `//go:build !aws` para graceful degradation). Testes precisam rodar com tags apropriadas. Taskfile abstrai isso com tasks `build:cloud`.

---

### TD-03: Bedrock via Converse API (não InvokeModel)

- **Opções:** (A) `InvokeModel` — API genérica, body específico por modelo; (B) Converse API (`Converse` / `ConverseStream`) — abstração unificada
- **Escolhida:** B — Converse API para Completion e StreamCompletion; InvokeModel apenas para Embeddings (Titan Embed não suporta Converse)
- **Rationale:** Converse API é a abordagem recomendada pela AWS para novos desenvolvimentos. Funciona com Claude, Titan, Llama, Mistral sem mudanças de código. Menor acoplamento a formatos de modelo específicos.
- **Trade-offs:** Menor controle sobre parâmetros model-specific. Modelos que não suportam Converse (edge case) precisarão de fallback para InvokeModel com formato específico.

---

### TD-04: Token cache in-memory com TTL e margem de 60s

- **Opções:** (A) Sem cache — token gerado a cada request; (B) Cache in-memory com TTL (`sync.RWMutex` + map); (C) Cache persistente em disco com encriptação
- **Escolhida:** B — Cache in-memory com margem de segurança de 60s antes da expiração
- **Rationale:** Tokens K8s têm vida curta (15min EKS, 1h Azure/GCP). Cache in-memory é suficiente para uma sessão CLI. Cache em disco introduz riscos de segurança (tokens em plaintext/encriptados) sem benefício proporcional — o overhead de regenerar um token é < 1s.
- **Trade-offs:** Token perdido ao reiniciar o CLI (overhead aceitável). Cache em memória não persiste entre processos simultâneos (margem de segurança de 60s mitiga race window).

---

### TD-05: CloudConfig como campo opcional em Environment (composição, não herança)

- **Opções:** (A) Herança — `EKSEnvironment`, `AKSEnvironment` embeddando `Environment`; (B) Composição — campo `Cloud *CloudConfig` em `Environment`; (C) Map genérico `map[string]interface{}`
- **Escolhida:** B — Composição com `Cloud *CloudConfig`
- **Rationale:** Backward-compatible (`omitempty` no YAML — `environments.yaml` existentes não são afetados). Tipo concreto garante validação em compile-time. Alinha com o estilo do codebase (structs tipadas com tags YAML). Ponteiro `*CloudConfig` permite teste `nil` trivial.
- **Trade-offs:** Campos provider-specific (ARN, ResourceGroup, ProjectID) coexistem na mesma struct. Mitigado com comentários e validação no nível do serviço (campos incompatíveis para um provider são ignorados).

---

### TD-06: AutoRefreshTransport intercepta somente 401

- **Opções:** (A) Interceptar 401 e 403; (B) Interceptar apenas 401
- **Escolhida:** B — Apenas 401
- **Rationale:** 403 indica permissão negada real (RBAC), não token expirado. Interceptar 403 causaria loops em erros reais de RBAC — risco de segurança inaceitável. Vendors que retornam 403 para tokens expirados receberão erro propagado com hint de `yby cloud refresh`.
- **Trade-offs:** Comportamento subótimo em providers que abusam de 403 para expiração (documentado nos edge cases). Comportamento seguro e previsível em todos os outros casos.

---

## Contracts

### Interface Changes

#### `pkg/cloud/provider.go` (novo)

```go
type CloudProvider interface {
    Name() string
    IsAvailable(ctx context.Context) bool
    CLIVersion(ctx context.Context) (string, error)
    ListClusters(ctx context.Context, opts ListOptions) ([]ClusterInfo, error)
    ConfigureKubeconfig(ctx context.Context, cluster ClusterInfo) error
    ValidateCredentials(ctx context.Context) (*CredentialStatus, error)
    RefreshToken(ctx context.Context, cluster ClusterInfo) error
}
```

#### `pkg/cloud/token.go` (novo)

```go
type TokenGenerator interface {
    GenerateToken(ctx context.Context, config *CloudConfig) (*Token, error)
}

type Token struct {
    Value     string
    ExpiresAt time.Time
}
```

#### `pkg/context/context.go` (modificado)

```go
// Adicionado a Environment:
Cloud *CloudConfig `yaml:"cloud,omitempty"`

// Novo tipo:
type CloudConfig struct {
    Provider      string `yaml:"provider"`
    Region        string `yaml:"region,omitempty"`
    Cluster       string `yaml:"cluster,omitempty"`
    Profile       string `yaml:"profile,omitempty"`
    RoleARN       string `yaml:"role_arn,omitempty"`
    ResourceGroup string `yaml:"resource_group,omitempty"`
    Subscription  string `yaml:"subscription,omitempty"`
    TenantID      string `yaml:"tenant_id,omitempty"`
    LoginMode     string `yaml:"login_mode,omitempty"`
    ProjectID     string `yaml:"project_id,omitempty"`
    Zone          string `yaml:"zone,omitempty"`
}
```

#### `pkg/ai/provider.go` (sem mudança na interface — BedrockProvider implementa Provider existente)

Cadeia de decorators existente (`wrapProvider`) não muda. BedrockProvider implementa os 6 métodos: `Name()`, `IsAvailable()`, `Completion()`, `StreamCompletion()`, `EmbedDocuments()`, `GenerateGovernance()`.

### Integration Points

| Ponto | Arquivo origem | Arquivo destino | Descrição |
|-------|---------------|-----------------|-----------|
| `GetKubeClient()` + TokenGenerator | `pkg/plugin/sdk/sdk_k8s.go` | `pkg/cloud/token.go` + `pkg/cloud/token_cache.go` | Injeção de BearerToken quando `Cloud != nil` |
| `GetKubeClient()` + AutoRefreshTransport | `pkg/plugin/sdk/sdk_k8s.go` | `pkg/cloud/token_refresh.go` | `rest.Config.WrapTransport` para refresh automático |
| Doctor + CloudProvider | `pkg/services/doctor/service.go` | `pkg/cloud/detect.go` + providers | Seção "Cloud Providers" no relatório |
| `yby env create` + CloudProvider | `cmd/env.go` | `pkg/cloud/provider.go` | Flag `--cloud-provider` dispara `ConfigureKubeconfig()` |
| `yby env show` + CredentialStatus | `cmd/env.go` | `pkg/cloud/provider.go` | Exibição de metadata cloud |
| Bedrock factory registration | `pkg/ai/factory.go` | `pkg/ai/bedrock.go` | `createProvider("bedrock")` |
| Bedrock cost tracking | `pkg/ai/bedrock.go` | `pkg/ai/cost_provider.go` | UsageMetadata → tabela de preços Bedrock |
| Environment service cloud types | `pkg/services/environment/service.go` | `pkg/context/context.go` | Tipos `eks`, `aks`, `gke` no `Up()` |

---

## FR Traceability: Components

| FR | Arquivo(s) Principal(is) de Implementação |
|----|-------------------------------------------|
| FR-001 | `pkg/cloud/provider.go`, `pkg/cloud/aws.go`, `pkg/cloud/azure.go`, `pkg/cloud/gcp.go` |
| FR-002 | `pkg/cloud/detect.go` |
| FR-003 | `cmd/cloud_connect.go`, `pkg/cloud/detect.go`, `pkg/cloud/aws.go`, `pkg/cloud/azure.go`, `pkg/cloud/gcp.go` |
| FR-004 | `cmd/cloud_list.go`, `pkg/cloud/provider.go` |
| FR-005 | `cmd/cloud_status.go`, `pkg/cloud/provider.go` |
| FR-006 | `cmd/cloud_refresh.go`, `pkg/cloud/provider.go` |
| FR-007 | `pkg/context/context.go` |
| FR-008 | `pkg/context/context.go`, `pkg/services/environment/service.go` |
| FR-009 | `pkg/ai/bedrock.go` |
| FR-010 | `pkg/ai/factory.go`, `pkg/config/config.go` |
| FR-011 | `pkg/cloud/token.go`, `pkg/cloud/aws_token.go`, `pkg/cloud/azure_token.go`, `pkg/cloud/gcp_token.go` |
| FR-012 | `pkg/cloud/token_cache.go` |
| FR-013 | `pkg/plugin/sdk/sdk_k8s.go`, `pkg/cloud/token.go` |
| FR-014 | `pkg/services/doctor/service.go`, `pkg/cloud/detect.go`, `cmd/doctor.go` |
| FR-015 | `pkg/cloud/token_refresh.go`, `pkg/plugin/sdk/sdk_k8s.go` |
| FR-016 | Todos os arquivos em `pkg/cloud/aws*.go`, `pkg/cloud/azure*.go`, `pkg/cloud/gcp*.go`, `pkg/ai/bedrock.go` |
| FR-017 | `pkg/cloud/aws_token.go` (P3 — milestone futuro) |
| FR-018 | `pkg/cloud/azure_token.go` (P3 — milestone futuro) |
| FR-019 | `pkg/cloud/gcp_token.go` (P3 — milestone futuro) |
| FR-020 | `pkg/cloud/credential_store.go` (P3 — milestone futuro) |
| FR-021 | `pkg/cloud/audit.go` (P3 — milestone futuro) |
| FR-022 | `cmd/env.go` |
| FR-023 | `cmd/env.go`, `pkg/cloud/provider.go` |
| FR-024 | `pkg/ai/bedrock.go`, `pkg/config/config.go` |
| FR-025 | `pkg/ai/bedrock.go`, `pkg/ai/cost_provider.go` |

---

## Complexity Tracking

| Story | Complexidade Estimada | Áreas de Risco |
|-------|----------------------|----------------|
| US-01 | Low | Parsing de kubeconfig pode ter formatos não antecipados; timeout na validação de credenciais |
| US-02 | High | Fluxo interativo (survey/promptui) complexo; 3 providers com CLIs distintos; modo não-interativo precisa paridade total |
| US-03 | Medium | BedrockProvider segue padrão dos providers existentes; risco em extração de usage do ConverseOutput |
| US-04 | Low | Listagem tabular simples; risco em rate limiting das APIs cloud durante listagem em batch |
| US-05 | Low | Parsing de CredentialStatus; risco em providers que não retornam expiração explícita |
| US-06 | High | AutoRefreshTransport com mutex — race conditions sutis; testes de concorrência com -race obrigatórios; acoplamento com rest.Config.WrapTransport |
| US-07 | Low | Extensão de env create/show existentes; risco em backward-compat do YAML |
| US-08 | High (P3) | SSO/IRSA/MFA são fluxos complexos e provider-specific; fora do milestone atual |
| US-09 | Medium (P3) | Múltiplos modos azidentity; Managed Identity requer ambiente Azure; fora do milestone atual |
| US-10 | Low | EmbedDocuments sequencial (Titan sem batch nativo); apenas loop sobre InvokeModel |
| US-11 | Medium | Build tags com stubs para graceful degradation; acoplamento entre GetKubeClient e TokenGenerator; fallback CLI precisa ser testado |
| US-12 | Medium (P3) | go-keyring requer daemons de keychain no Linux; fallback silencioso não pode mascarar erros reais; fora do milestone |
| US-13 | Low (P3) | Escrita em JSONL é simples; complexidade real está em definir schema do audit log; fora do milestone |

---

## Ordem de Implementação Recomendada

Baseado no plano `plans/cloud-multicloud-support.md` e na análise de dependências:

1. **Nível 0 — Fundação (US-01 parcial):** `pkg/cloud/detect.go`, melhorias em `pkg/services/doctor/service.go` e `cmd/env.go` para metadata cloud. Zero dependências novas. Valida que kubeconfig existente funciona.

2. **Bedrock Provider (US-03, US-10, FR-009-010-024-025):** Isolado do resto cloud. Não depende de `pkg/cloud`. Entrega valor imediato para usuários AWS sem aguardar os comandos cloud.

3. **Nível 1 — Guided Setup (US-02, US-04, US-05, US-06-parcial, US-07):** `pkg/cloud/provider.go` + `aws.go` + `azure.go` + `gcp.go`, `cmd/cloud*.go`, extensão `cmd/env.go`. Zero dependências Go novas.

4. **Nível 2 — SDK Nativo (US-06-completo, US-11):** `pkg/cloud/token*.go`, `pkg/cloud/token_cache.go`, `pkg/cloud/token_refresh.go`, modificação de `pkg/plugin/sdk/sdk_k8s.go`. Adiciona SDKs via build tags.

5. **Nível 3 — Enterprise (US-08, US-09, US-12, US-13) — P3, fora do milestone:** `pkg/cloud/credential_store.go`, `pkg/cloud/audit.go`, auth avançada. Sob demanda.

---

## Risk Assessment

| Risco | Probabilidade | Impacto | Mitigação |
|-------|--------------|---------|-----------|
| Peso excessivo do go.mod com AWS/Azure/GCP SDKs | Alta | Médio | Build tags condicionais; GoReleaser produz binários separados; `task build` não inclui SDKs |
| Breaking change na struct Environment | Baixa | Alto | Campo `Cloud` é ponteiro `*CloudConfig` com `omitempty` — `environments.yaml` existentes não são afetados (SC-008) |
| Diversidade de auth flows Azure AD | Média | Médio | Nível 1 usa apenas `az aks get-credentials` (CLI); SDK Azure com múltiplos modos é Nível 2 e P3 |
| Token refresh race conditions | Média | Alto | `sync.Mutex` no `AutoRefreshTransport`; testes de concorrência com `-race` são critério de aceite do SC-010 |
| Bedrock model access requer habilitação manual | Alta | Baixo | Doctor verifica e informa; mensagem de erro com hint "Habilite o modelo X no console AWS Bedrock" |
| Dependência de CLIs cloud (Nível 1) | Média | Baixo | Doctor verifica instalação; erros com mensagem informativa e link de instalação (NFR-007) |
| Secrets acidentalmente em environments.yaml | Baixa | Crítico | Apenas referências permitidas (profile names, role ARNs); validação no save impede tokens/passwords (NFR-005) |
| Acoplamento em `sdk_k8s.go` | Média | Alto | Ponto central de acesso K8s para todos os plugins; modificação requer testes de integração com mock; build condicional via interface `TokenGenerator` |
| Kubeconfig exec plugin com formatos não documentados | Baixa | Médio | Parsing defensivo em `detect.go`; retornar `nil` (sem provider detectado) em vez de panic para padrões não reconhecidos |
| Converse API Bedrock não suporta modelo configurado | Média | Baixo | Mensagem de erro clara com lista de modelos suportados; edge case documentado |

---

## Research Notes

### Alternativas Investigadas e Descartadas

- **Crossplane / Cluster API como abstração:** Rejeitado — complexidade desproporcional para conexão/autenticação. Yby não gerencia lifecycle de clusters.
- **kubelogin como dependência obrigatória:** Parcialmente adotado — detectado pelo doctor para clusters Azure AD, mas não é dependência obrigatória. Token generator SDK substitui a necessidade.
- **Vault / External Secrets para credenciais:** Rejeitado — overengineering para tokens temporários de sessão CLI.
- **Provider OpenAI-compatible para Bedrock:** Rejeitado — Bedrock Converse API tem formato próprio; wrapper adicionaria latência e perderia usage tracking nativo.

### Padrões de Referência no Codebase

- Build tag `k8s` no plugin sentinel: padrão para compilação condicional (mesmo padrão para `aws`, `azure`, `gcp`)
- `pkg/ai/openai.go`: template para BedrockProvider (HTTP client, SSE, embeddings, usage extraction)
- `pkg/services/shared/runner.go`: padrão de injeção de dependência via interface — CloudProvider deve seguir
- `pkg/errors/hints.go`: registry de hints automáticos — adicionar hints para erros cloud (token expirado, CLI ausente, model não habilitado)
- `testutil/mock_runner.go`: mock para testes de CloudProvider sem CLIs reais

### Notas sobre GoReleaser

O `.goreleaser.yaml` existente já produz um build separado para sentinel com `-tags=k8s`. O mesmo padrão deve ser seguido:
- Build padrão: `go build` (sem tags cloud)
- Build cloud: `go build -tags cloud`
- Artefatos: `yby-linux-amd64` (padrão) e `yby-cloud-linux-amd64` (com SDKs)
- Taskfile: adicionar `build:cloud` com `-tags cloud`
