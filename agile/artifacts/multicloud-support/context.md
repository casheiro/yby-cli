# Context: Suporte Multi-Cloud (EKS/AKS/GKE) + Amazon Bedrock

## Problem Statement

O Yby CLI atualmente suporta apenas dois tipos de ambiente: `local` (k3d) e `remote` (VPS com SSH). Embora clusters Kubernetes gerenciados em clouds públicas (AWS EKS, Azure AKS, GCP GKE) funcionem tecnicamente quando o kubeconfig já está configurado externamente, o Yby não oferece nenhuma assistência para configuração, validação ou refresh de credenciais cloud. Isso força o usuário a executar manualmente comandos específicos de cada cloud provider (`aws eks update-kubeconfig`, `az aks get-credentials`, `gcloud container clusters get-credentials`) antes de poder usar o Yby, quebrando a experiência fluida que a ferramenta propõe.

Adicionalmente, o subsistema de IA do Yby não inclui Amazon Bedrock como provider, deixando de fora organizações que usam AWS como plataforma padrão e preferem manter chamadas de IA dentro do ecossistema AWS — frequentemente por exigências de compliance, localidade de dados e governança corporativa.

Esses gaps afetam diretamente equipes enterprise e DevOps que operam em ambientes multi-cloud, representando uma barreira significativa para adoção do Yby em cenários profissionais.

## Scope

### In Scope

**Nível 0 — Validação e Doctor aprimorado:**
- Detecção inteligente do cloud provider a partir do kubeconfig ativo (parseando `exec.command`)
- Verificação de CLIs de cloud instalados (aws, az, gcloud) e suas versões
- Detecção de tokens expirados com sugestão de refresh
- Exibição de metadata cloud em `yby env show` (provider, região, cluster name)

**Nível 1 — Guided Setup (`yby cloud connect`):**
- Novo pacote `pkg/cloud` com interface `CloudProvider` e implementações para AWS, Azure e GCP
- Novo comando `yby cloud` com subcomandos: `connect`, `list`, `status`, `refresh`
- Fluxo interativo e não-interativo para configuração de kubeconfig
- Integração com `yby env create` (flag `--cloud-provider`)
- Novos tipos de ambiente: `eks`, `aks`, `gke`

**Nível 2 — SDK Nativo + Bedrock Provider:**
- Extensão da struct `Environment` com campo `Cloud *CloudConfig`
- Token generators programáticos via AWS SDK v2, Azure SDK e GCP oauth2
- Cache de tokens com TTL e invalidação
- Integração com `GetKubeClient()` para injeção automática de tokens
- Amazon Bedrock como AI provider completo (Completion, StreamCompletion, EmbedDocuments)
- Build tags condicionais (`//go:build aws`, `azure`, `gcp`, `cloud`) para evitar inflar o binário

**Nível 3 — Multi-Cloud Enterprise:**
- Autenticação avançada: AWS SSO/IRSA/MFA, Azure AD device code/interactive/MSI, GCP Workload Identity Federation
- Token refresh automático via `AutoRefreshTransport` (middleware HTTP interceptando 401/403)
- Credential store seguro via OS keychain (go-keyring)
- Multi-cluster dashboard TUI
- Auditoria de operações de autenticação cloud

### Out of Scope

- **Provisionamento de clusters** — o Yby não cria/destrói clusters cloud (Terraform, Pulumi, eksctl fazem isso)
- **Gerenciamento de custos cloud** — sem integração com billing APIs
- **Suporte a clouds privadas** (OpenStack, VMware Tanzu) — foco em AWS, Azure e GCP apenas
- **Migração de workloads entre clouds** — o Yby gerencia conexão, não orquestração cross-cloud
- **AWS SDK v1** — toda integração usa aws-sdk-go-v2 (modular, menor footprint)

## Impact Analysis

### Arquivos Modificados

| Arquivo | Tipo de Mudança | Risco |
|---------|----------------|-------|
| `pkg/context/context.go` | Extensão da struct `Environment` com `Cloud *CloudConfig` | Médio — campo `omitempty` garante backward-compat |
| `pkg/config/config.go` | Adição de `"bedrock"` na whitelist de `validProviders` | Baixo |
| `pkg/ai/factory.go` | Novo case `"bedrock"` em `createProvider`, `defaultPriority`, `embeddingCapableProviders` | Baixo |
| `pkg/ai/cost_provider.go` | Tabela de preços Bedrock | Baixo |
| `pkg/ai/tokencount.go` | Context windows dos modelos Bedrock | Baixo |
| `pkg/ai/ratelimit_provider.go` | Rate limit default para Bedrock | Baixo |
| `pkg/plugin/sdk/sdk_k8s.go` | Injeção de token cloud em `GetKubeClient()` | Alto — ponto central de acesso K8s para plugins |
| `pkg/services/doctor/service.go` | Novos checks para CLIs cloud e status de credenciais | Baixo |
| `pkg/services/environment/service.go` | Suporte a tipos de ambiente cloud no `Up()` | Médio |
| `cmd/env.go` | Flag `--cloud-provider` em `env create`, metadata cloud em `env show` | Baixo |
| `cmd/doctor.go` | Seção "Cloud Providers" no relatório | Baixo |

### Arquivos Novos

| Arquivo | Propósito |
|---------|----------|
| `pkg/cloud/provider.go` | Interface `CloudProvider`, structs `ClusterInfo`, `CredentialStatus`, `ListOptions` |
| `pkg/cloud/detect.go` | Detecção automática de CLIs cloud instalados e parsing de kubeconfig |
| `pkg/cloud/aws.go` | Implementação AWS (EKS) via CLI `aws` |
| `pkg/cloud/azure.go` | Implementação Azure (AKS) via CLI `az` |
| `pkg/cloud/gcp.go` | Implementação GCP (GKE) via CLI `gcloud` |
| `pkg/cloud/token.go` | Interface `TokenGenerator` |
| `pkg/cloud/aws_token.go` | Geração de tokens EKS via AWS SDK v2 |
| `pkg/cloud/azure_token.go` | Geração de tokens AKS via Azure SDK |
| `pkg/cloud/gcp_token.go` | Geração de tokens GKE via oauth2/google |
| `pkg/cloud/token_cache.go` | Cache de tokens com TTL e invalidação |
| `pkg/cloud/token_refresh.go` | `AutoRefreshTransport` para refresh automático |
| `pkg/cloud/credential_store.go` | Integração com OS keychain |
| `pkg/cloud/audit.go` | Log de auditoria de operações cloud |
| `pkg/ai/bedrock.go` | BedrockProvider (Converse API + Titan Embeddings) |
| `cmd/cloud.go` | Subcomando raiz `yby cloud` |
| `cmd/cloud_connect.go` | `yby cloud connect` (guided setup) |
| `cmd/cloud_list.go` | `yby cloud list` |
| `cmd/cloud_status.go` | `yby cloud status` |
| `cmd/cloud_refresh.go` | `yby cloud refresh` |

### Dependências Novas (go.mod)

**Nível 0-1:** Nenhuma — tudo via `os/exec` de CLIs existentes.

**Nível 2:**
- `github.com/aws/aws-sdk-go-v2` (core + config + credentials + stscreds + sts + eks + bedrockruntime) — ~5-10MB no binário
- `github.com/Azure/azure-sdk-for-go/sdk/azidentity` + `azcore`
- `golang.org/x/oauth2/google`
- `sigs.k8s.io/aws-iam-authenticator/pkg/token`

**Nível 3:**
- `github.com/zalando/go-keyring` (credential store seguro)
- `github.com/aws/aws-sdk-go-v2/service/ssooidc` (AWS SSO)

**Impacto no binário:** Build tags condicionais (`//go:build aws`, `//go:build azure`, `//go:build gcp`, `//go:build cloud`) são essenciais para que o binário padrão não carregue SDKs desnecessários. O build padrão (`task build`) produz o binário sem SDKs cloud; compilação com `go build -tags cloud` inclui todos.

## Technical Decisions

### TD-01: CLI-first, SDK-second (abordagem em camadas)

**Opções consideradas:**
1. **Apenas CLI** — executar sempre `aws`, `az`, `gcloud` como subprocessos
2. **Apenas SDK** — usar SDKs Go nativos para tudo
3. **CLI-first, SDK-second** — Nível 1 via CLIs, Nível 2 adiciona SDKs opcionais

**Decisão:** Opção 3 — CLI-first, SDK-second.

**Rationale:** A abordagem em camadas permite valor imediato sem dependências (Nível 1), enquanto o Nível 2 adiciona independência de CLIs para cenários CI/CD e automação. Usuários que já têm os CLIs cloud instalados se beneficiam imediatamente; ambientes headless ou containers se beneficiam do SDK nativo.

**Trade-offs:** Duplicação parcial de lógica (CLI wrapper + SDK), mas a interface `CloudProvider` unifica a API. Build tags isolam o peso do SDK.

### TD-02: Build tags condicionais para SDKs cloud

**Opções consideradas:**
1. **Sempre incluir todos os SDKs** — binário ~30MB maior
2. **Build tags por provider** — `//go:build aws`, `//go:build azure`, `//go:build gcp`
3. **Plugin system** — cloud providers como plugins binários separados

**Decisão:** Opção 2 — Build tags por provider.

**Rationale:** Alinha com o padrão existente do projeto (build tag `k8s` para sentinel). Permite que o usuário compile apenas o que precisa. O GoReleaser pode produzir builds variantes.

**Trade-offs:** Complexidade de build, mas o `Taskfile` e GoReleaser abstraem isso. Testes precisam rodar com tags apropriadas.

### TD-03: Bedrock via Converse API (não InvokeModel)

**Opções consideradas:**
1. **InvokeModel** — API genérica que requer formato body específico por modelo
2. **Converse API** — API unificada que abstrai diferenças entre modelos

**Decisão:** Opção 2 — Converse API (`Converse` / `ConverseStream`).

**Rationale:** A Converse API é a abordagem recomendada pela AWS para novos desenvolvimentos. Funciona com Claude, Titan, Llama, Mistral sem mudanças no código. `InvokeModel` é usado apenas para embeddings (Titan Embed não suporta Converse).

**Trade-offs:** Menor controle sobre parâmetros model-specific, mas a uniformidade justifica.

### TD-04: Token cache in-memory com TTL

**Opções consideradas:**
1. **Sem cache** — gerar token a cada request
2. **Cache in-memory com TTL** — `sync.RWMutex` + map com expiração
3. **Cache persistente em disco** — salvar tokens encriptados

**Decisão:** Opção 2 — Cache in-memory com margem de 60s antes da expiração.

**Rationale:** Tokens K8s têm vida curta (15min EKS, 1h Azure/GCP). Cache in-memory é suficiente para uma sessão CLI. Cache em disco introduziria riscos de segurança desnecessários.

**Trade-offs:** Token perdido ao reiniciar o CLI, mas o overhead de regeneração é < 1s.

### TD-05: CloudConfig como campo opcional em Environment

**Opções consideradas:**
1. **Herança** — novos tipos `EKSEnvironment`, `AKSEnvironment` embeddando `Environment`
2. **Composição** — campo `Cloud *CloudConfig` em `Environment`
3. **Map genérico** — `map[string]interface{}` para extensibilidade

**Decisão:** Opção 2 — Composição com `Cloud *CloudConfig`.

**Rationale:** Backward-compatible (`omitempty` no YAML). Tipo concreto ao invés de map garante validação em compile-time. Alinha com o estilo do codebase (structs tipadas com tags YAML).

**Trade-offs:** Campos provider-specific (ARN, ResourceGroup, ProjectID) ficam todos na mesma struct, mas são claramente documentados com comentários.

## Alternatives Investigated

### Crossplane / Cluster API como abstração
Rejeitado — complexidade desproporcional para o caso de uso (conexão, não provisionamento). O Yby precisa apenas autenticar e conectar, não gerenciar lifecycle de clusters.

### kubelogin como dependência externa
Parcialmente adotado — `kubelogin` é detectado pelo doctor para clusters Azure AD, mas não é dependência obrigatória. O token generator SDK substitui a necessidade.

### Vault / External Secrets para credenciais
Rejeitado para esta feature — seria overengineering para tokens temporários de sessão CLI. O credential store via OS keychain é suficiente para tokens SSO de longa duração.

### Provider de IA genérico OpenAI-compatible para Bedrock
Rejeitado — a Bedrock Converse API tem formato próprio que não é OpenAI-compatible. Um wrapper adicionaria latência e perderia features como usage tracking nativo.

## Risk Assessment

| Risco | Probabilidade | Impacto | Mitigação |
|-------|--------------|---------|-----------|
| Peso excessivo do go.mod com AWS/Azure/GCP SDKs | Alta | Médio | Build tags condicionais; builds separados no GoReleaser |
| Breaking change na struct Environment | Baixa | Alto | Campo `Cloud` é ponteiro `*CloudConfig` com `omitempty` — ambientes existentes não são afetados |
| Diversidade de auth flows Azure AD | Média | Médio | Começar com `azurecli` e `spn`; adicionar device code e MSI incrementalmente |
| Token refresh race conditions | Média | Alto | `sync.Mutex` no `AutoRefreshTransport`; testes de concorrência dedicados |
| Bedrock model access requer habilitação manual | Alta | Baixo | Documentar no `yby doctor` e em mensagens de erro com hint |
| Dependência de CLIs cloud na máquina do usuário (Nível 1) | Média | Baixo | Doctor verifica instalação; mensagens de erro com links de instalação |
| Secrets acidentalmente persistidos em environments.yaml | Baixa | Crítico | Nunca armazenar tokens/passwords; apenas referências (profile, role_arn); validação no save |

## Constitution Compliance Notes

- **Princípio 1 (Programmatic Control):** Todas as verificações de credenciais, token expiry e disponibilidade de CLIs são determinísticas via Go code. Nenhuma decisão delegada a LLM.
- **Princípio 8 (Zero External Dependencies Beyond Cobra):** **TENSÃO IDENTIFICADA** — Esta feature adiciona AWS SDK, Azure SDK e GCP oauth2 como dependências. A mitigação via build tags garante que o binário padrão continua sem dependências externas além de Cobra. SDKs são opt-in via build tags.
- **Princípio 9 (English in Code, Localized in Terminal):** Todo código novo em inglês. Mensagens no terminal via i18n onde aplicável. System prompts do Bedrock em inglês.
- **Princípio 4 (Strict Boundaries):** O pacote `pkg/cloud` é isolado com responsabilidade clara. `pkg/ai/bedrock.go` segue o mesmo padrão dos providers existentes.
