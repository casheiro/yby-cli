# Tasks: Suporte Multi-Cloud (EKS/AKS/GKE) + Amazon Bedrock

> **Escopo:** P1 (US-01, US-02, US-03) + P2 (US-04 a US-07, US-10, US-11).
> US-08, US-09, US-12, US-13 (FR-017 a FR-021) são P3 — fora deste milestone.
> Verificação geral: `go test ./pkg/cloud/... ./pkg/ai/... ./cmd/... -race` + `task build` (binário padrão sem SDKs cloud).

---

### Phase 0: Setup

- [x] T001 [US-07] Adicionar `Cloud *CloudConfig` à struct `Environment` em `pkg/context/context.go` com todos os campos opcionais (`provider`, `region`, `cluster`, `profile`, `role_arn`, `resource_group`, `subscription`, `tenant_id`, `login_mode`, `project_id`, `zone`, todos `omitempty`); adicionar tipos de ambiente `eks`, `aks`, `gke` à validação existente em `pkg/services/environment/service.go`; adicionar hints cloud (`ERR_CLOUD_TOKEN_EXPIRED`, `ERR_CLOUD_CLI_MISSING`, `ERR_CLOUD_MODEL_DISABLED`) em `pkg/errors/hints.go`. Verificar: `environments.yaml` existente (sem campo `cloud`) carrega sem erro -- `pkg/context/context.go`

---

### Phase 1: Foundation

- [x] T002 [US-01] Criar `pkg/cloud/provider.go` com interface `CloudProvider` (Name, IsAvailable, CLIVersion, ListClusters, ConfigureKubeconfig, ValidateCredentials, RefreshToken) e structs `ClusterInfo`, `CredentialStatus`, `ListOptions`; criar `pkg/cloud/detect.go` com função `Detect(ctx, runner) []CloudProvider` que parseia `exec.command` do kubeconfig ativo (padrões: `aws`/`aws-iam-authenticator` → AWS, `kubelogin`/`az` → Azure, `gke-gcloud-auth-plugin`/`gcloud` → GCP) e usa `exec.LookPath` para verificar CLIs instalados. Não fazer chamadas de rede. Verificar: `go build ./pkg/cloud/...` compila; detecção completa em < 100ms -- `pkg/cloud/provider.go`

- [x] T003 [US-06] Criar `pkg/cloud/token.go` com interface `TokenGenerator` (GenerateToken) e struct `Token{Value string, ExpiresAt time.Time}`; criar `pkg/cloud/token_cache.go` com `TokenCache` thread-safe usando `sync.RWMutex`, TTL com margem de 60s antes da expiração, métodos `Get`, `Set`, `Invalidate`; adicionar `pkg/cloud/token_cache_test.go` com testes de TTL (token expirado retorna nil,false) e concorrência (100 goroutines, `-race`). Verificar: `go test -race ./pkg/cloud/...` passa sem race conditions -- `pkg/cloud/token_cache.go`

- [x] T004 [P] [US-03] Criar `pkg/ai/bedrock.go` (//go:build aws) implementando a interface `Provider` completa: `Completion` via Converse API (`bedrockruntime.Converse`), `StreamCompletion` via ConverseStream, `EmbedDocuments` via InvokeModel sequencial (Titan `amazon.titan-embed-text-v2:0`), `GenerateGovernance` via Completion, `Name`/`IsAvailable`; extrair `InputTokens`/`OutputTokens`/`TotalTokens` do `ConverseOutput.Usage` e expor via `SetUsage`; adicionar `pkg/ai/bedrock_test.go` com mock do cliente bedrockruntime; adicionar preços Bedrock (Claude Sonnet, Haiku, Titan Embed) em `pkg/ai/cost_provider.go` e context windows em `pkg/ai/tokencount.go`; registrar `"bedrock"` em `defaultPriority` (após `"openai"`), `createProvider` switch, `embeddingCapableProviders` em `pkg/ai/factory.go` e em `validProviders` em `pkg/config/config.go`. Verificar: `go build -tags aws ./pkg/ai/...` compila; `go test -tags aws ./pkg/ai/...` passa -- `pkg/ai/bedrock.go`

---

### Phase 2: Implementation

- [x] T005 [P] [US-02] Criar `pkg/cloud/aws.go` implementando `CloudProvider` para AWS EKS usando `shared.Runner`: `IsAvailable` via `exec.LookPath("aws")`, `CLIVersion` via `aws --version`, `ValidateCredentials` via `aws sts get-caller-identity --output json` (parse `UserId`, `Account`, `Arn`), `ListClusters` via `aws eks list-clusters --region R` + `describe-cluster` para cada, `ConfigureKubeconfig` via `aws eks update-kubeconfig --name N --region R`, `RefreshToken` via `aws eks get-token`; adicionar `pkg/cloud/aws_test.go` usando `testutil.MockRunner` sem acesso real a CLIs. Verificar: `go test ./pkg/cloud/... -run TestAWS` passa com MockRunner -- `pkg/cloud/aws.go`

- [x] T006 [P] [US-02] Criar `pkg/cloud/azure.go` implementando `CloudProvider` para AKS via CLI `az` (`account show` para credenciais, `aks list -g RG` para clusters, `aks get-credentials --name N --resource-group RG`, `account get-access-token`); criar `pkg/cloud/gcp.go` implementando `CloudProvider` para GKE via CLI `gcloud` (`auth list`, `container clusters list --project P --region R`, `container clusters get-credentials N`, `auth print-access-token`); adicionar `pkg/cloud/azure_test.go` e `pkg/cloud/gcp_test.go` com MockRunner. Verificar: `go test ./pkg/cloud/... -run TestAzure|TestGCP` passa -- `pkg/cloud/azure.go`

- [x] T007 [US-11] Criar `pkg/cloud/aws_token.go` (//go:build aws) usando `sigs.k8s.io/aws-iam-authenticator/pkg/token` + `aws-sdk-go-v2/config`; criar `pkg/cloud/azure_token.go` (//go:build azure) usando `azidentity.NewDefaultAzureCredential`; criar `pkg/cloud/gcp_token.go` (//go:build gcp) usando `golang.org/x/oauth2/google`; criar `pkg/cloud/aws_token_stub.go` (//go:build !aws), `pkg/cloud/azure_token_stub.go` (//go:build !azure), `pkg/cloud/gcp_token_stub.go` (//go:build !gcp) com fallback para CLI via `shared.Runner`; criar `pkg/cloud/token_refresh.go` com `AutoRefreshTransport` implementando `http.RoundTripper`: injeta Bearer token no header, intercepta 401 (invalida cache → gera novo token via `TokenGenerator` → repete request), serializa refresh concorrente via `sync.Mutex`, propaga 403 sem retry; adicionar `pkg/cloud/token_refresh_test.go` com mock RoundTripper e teste de exatamente 1 refresh sob 100 goroutines simultâneas; modificar `pkg/plugin/sdk/sdk_k8s.go` para, quando `env.Cloud != nil` e build tag presente, preencher `rest.Config.BearerToken` via TokenCache/TokenGenerator, limpar `BearerTokenFile` e aplicar `WrapTransport = AutoRefreshTransport`. Verificar: `go test -race -tags aws ./pkg/cloud/...` sem race conditions; `task build` (sem tags) compila -- `pkg/cloud/token_refresh.go`

---

### Phase 3: Polish

- [x] T008 [P] [US-02] Criar `cmd/cloud.go` com subcomando raiz `yby cloud` e registrar em `cmd/root.go`; criar `cmd/cloud_connect.go` com modo interativo (survey: detecta CLIs disponíveis → seleciona provider → valida credenciais → lista clusters → configura kubeconfig → oferece criar ambiente em `environments.yaml`) e modo não-interativo (flags `--provider`, `--region`, `--cluster`, `--env-name` sem prompts); criar `cmd/cloud_list.go` com tabela (nome, provider, região, versão K8s, status), flags `--provider`/`--region`, mensagem informativa quando sem CLIs; criar `cmd/cloud_status.go` mostrando provider, identidade, expiração e status do token ativo; criar `cmd/cloud_refresh.go` chamando `CloudProvider.RefreshToken` no ambiente ativo. Verificar: `yby cloud --help` exibe subcomandos; modo não-interativo funciona com flags completas -- `cmd/cloud.go`

- [x] T009 [P] [US-01] Adicionar check "Cloud Providers" ao `pkg/services/doctor/service.go`: chamar `cloud.Detect()` para listar CLIs instalados com versões, chamar `ValidateCredentials` para ambiente ativo (timeout 10s), reportar token expirado com hint de refresh, exibir "Nenhum provider cloud detectado" quando sem CLIs; atualizar `cmd/doctor.go` para renderizar a nova seção. Verificar: `yby doctor` sem CLIs cloud exibe "Nenhum provider cloud detectado" sem erro -- `pkg/services/doctor/service.go`

- [x] T010 [P] [US-07] Estender `cmd/env.go`: adicionar flag `--cloud-provider` (valores: `aws`, `azure`, `gcp`) no subcomando `env create` que, quando especificada, chama `CloudProvider.ConfigureKubeconfig` e persiste `Cloud` preenchido em `environments.yaml`; adicionar bloco de metadados cloud em `env show` (provider, região, cluster, status do token, tempo restante) quando `env.Cloud != nil`; sem flag `--cloud-provider`, comportamento atual é preservado. Verificar: `yby env create nome --type eks --cloud-provider aws` persiste Cloud; `yby env show` exibe seção cloud -- `cmd/env.go`

- [x] T011 [US-11] Adicionar task `build:cloud` ao `Taskfile.yml` (`go build -tags cloud -o dist/yby-cloud ./cmd/yby`); adicionar build variant cloud no `.goreleaser.yaml` (id `yby-cloud`, `-tags cloud`, binários com sufixo `-cloud`); adicionar `test/e2e/cloud_kubeconfig_test.go` (//go:build e2e) com validação de exec plugin kubeconfig. Verificar: `task build` (sem tags) produz binário dentro de ±1% do tamanho anterior; `task build:cloud` compila com todos os SDKs -- `Taskfile.yml`

---

## Dependencies

```
T001 → T002 → T005 (paralelo com T006) → T007 → T008
      → T003 → T007                          → T009
T004 (independente de T001-T003)              → T010
T002 → T009                                   → T011
T001 → T010
```

## Completion Criteria

- Todos os tasks marcados [x]
- `go test -race ./pkg/cloud/... ./pkg/ai/...` passa sem race conditions
- `task build` compila sem tags cloud, binário dentro de ±1% do tamanho atual (SC-007)
- `environments.yaml` existente carrega sem erro após T001 (SC-008)
- FR-001 a FR-016, FR-022 a FR-025 cobertos
- FR-017 a FR-021 (P3) documentados como fora do milestone
