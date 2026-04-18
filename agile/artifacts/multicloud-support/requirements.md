# Specification: Suporte Multi-Cloud (EKS/AKS/GKE) + Amazon Bedrock

## Problem Statement

O Yby CLI atualmente reconhece apenas dois tipos de ambiente (`local` e `remote`) e não possui nenhum suporte assistido para clusters Kubernetes em clouds públicas. Usuários que operam clusters EKS, AKS ou GKE precisam configurar manualmente credenciais e kubeconfig fora do Yby antes de poder utilizá-lo, resultando em uma experiência fragmentada e propensa a erros. A struct `Environment` em `pkg/context/context.go` não tem campos para metadados cloud, o `yby doctor` não detecta problemas de autenticação cloud-specific, e o `yby env show` não exibe informações de provider/região.

Adicionalmente, o subsistema de IA (`pkg/ai/`) suporta 5 providers (Ollama, Claude CLI, Gemini CLI, Gemini API, OpenAI API) mas não inclui Amazon Bedrock — deixando de fora organizações AWS-first que necessitam manter chamadas de IA dentro do ecossistema AWS por compliance e governança.

Este gap impacta diretamente a adoção do Yby em ambientes enterprise e multi-cloud, onde a fluidez na conexão a clusters gerenciados e a flexibilidade de providers de IA são requisitos mínimos.

## Scope

### In Scope
- Detecção de cloud provider a partir de kubeconfig existente
- Verificação de CLIs cloud (aws, az, gcloud) no doctor
- Novo pacote `pkg/cloud` com interface `CloudProvider` e 3 implementações
- Novo comando `yby cloud` com subcomandos `connect`, `list`, `status`, `refresh`
- Extensão da struct `Environment` com campo `Cloud *CloudConfig`
- Novos tipos de ambiente: `eks`, `aks`, `gke`
- Token generators programáticos via SDKs nativos (com build tags)
- Cache de tokens in-memory com TTL
- Amazon Bedrock como AI provider (Converse API + Titan Embeddings)
- Token refresh automático via HTTP transport middleware
- Autenticação avançada (SSO, IRSA, Azure AD, Workload Identity)
- Credential store seguro via OS keychain
- Auditoria de operações de autenticação cloud
- Testes unitários com mocks para cada cloud SDK

### Out of Scope
- **Provisionamento/destruição de clusters** — responsabilidade de Terraform, eksctl, etc.
- **Gerenciamento de custos cloud** — fora do escopo do Yby
- **Clouds privadas** (OpenStack, VMware Tanzu) — apenas AWS, Azure, GCP
- **Migração de workloads entre clouds** — apenas conexão e autenticação
- **AWS SDK v1** — toda integração via aws-sdk-go-v2

## User Scenarios

### US-01: Detecção de cloud provider no doctor (P1)
**As a** operador DevOps, **I want** que o `yby doctor` detecte e reporte o cloud provider do cluster atual, **so that** eu saiba se meu ambiente está corretamente configurado sem executar comandos manuais.

**Acceptance Criteria:**
- Given um kubeconfig ativo com exec plugin `aws`, When o usuário executa `yby doctor`, Then o relatório inclui uma seção "Cloud Providers" mostrando "AWS EKS" com região e cluster name
- Given nenhum kubeconfig cloud configurado, When o usuário executa `yby doctor`, Then a seção "Cloud Providers" mostra "Nenhum provider cloud detectado"
- Given um CLI `aws` instalado mas com token expirado, When o usuário executa `yby doctor`, Then o relatório mostra warning com sugestão de refresh

### US-02: Conectar a cluster cloud via guided setup (P1)
**As a** desenvolvedor, **I want** executar `yby cloud connect` para ser guiado na conexão a um cluster cloud, **so that** eu não precise memorizar os comandos específicos de cada provider.

**Acceptance Criteria:**
- Given CLIs `aws` e `az` instalados, When o usuário executa `yby cloud connect`, Then o Yby detecta ambos e exibe lista de seleção
- Given o usuário selecionou AWS e possui credenciais válidas, When a região é especificada, Then o Yby lista clusters EKS disponíveis na região
- Given um cluster foi selecionado, When o kubeconfig é configurado com sucesso, Then o Yby oferece criar um ambiente automaticamente em `environments.yaml`
- Given flags `--provider aws --region us-east-1 --cluster prod`, When executado em modo não-interativo, Then o kubeconfig é configurado sem prompts

### US-03: Usar Amazon Bedrock como AI provider (P1)
**As a** usuário em organização AWS-first, **I want** configurar o Yby para usar Amazon Bedrock como provider de IA, **so that** minhas chamadas de IA fiquem dentro do ecossistema AWS.

**Acceptance Criteria:**
- Given `ai.provider: bedrock` em `~/.yby/config.yaml` e credenciais AWS válidas, When o Yby precisa de IA (ex: `yby bard`), Then o Bedrock é usado via Converse API
- Given credenciais AWS ausentes, When o Yby tenta usar Bedrock, Then retorna erro com hint "Configure credenciais AWS via 'aws configure' ou variáveis de ambiente"
- Given `ai.provider: auto` e Bedrock disponível (credenciais válidas), When nenhum provider de maior prioridade está disponível, Then Bedrock é usado como fallback na ordem de prioridade
- Given streaming habilitado, When uma completion é solicitada, Then o Bedrock responde via ConverseStream com chunks progressivos

### US-04: Listar clusters cloud disponíveis (P2)
**As a** operador DevOps, **I want** executar `yby cloud list` para ver todos os clusters disponíveis nos meus providers cloud, **so that** eu possa escolher rapidamente a qual conectar.

**Acceptance Criteria:**
- Given CLI `aws` disponível e credenciais válidas, When o usuário executa `yby cloud list --provider aws --region us-east-1`, Then uma tabela mostra clusters com nome, região, versão K8s e status
- Given múltiplos providers detectados, When o usuário executa `yby cloud list` sem `--provider`, Then clusters de todos os providers são listados
- Given nenhum provider disponível, When o usuário executa `yby cloud list`, Then mensagem informa que nenhum CLI cloud foi detectado

### US-05: Verificar status de credenciais cloud (P2)
**As a** operador DevOps, **I want** executar `yby cloud status` para ver o estado das minhas credenciais cloud, **so that** eu saiba se preciso re-autenticar antes de operar o cluster.

**Acceptance Criteria:**
- Given ambiente ativo do tipo `eks` com token válido, When o usuário executa `yby cloud status`, Then mostra identidade, provider, expiração do token e status "Autenticado"
- Given token expirado, When o usuário executa `yby cloud status`, Then mostra status "Token expirado" com comando sugerido para refresh

### US-06: Refresh automático de token cloud (P2)
**As a** operador DevOps, **I want** que o Yby faça refresh automático do token quando expirar durante uma sessão, **so that** operações longas (deploy, monitoring) não falhem silenciosamente.

**Acceptance Criteria:**
- Given um token EKS que expira durante uma operação, When o API server retorna 401, Then o `AutoRefreshTransport` gera um novo token e repete a request
- Given falha no refresh (credenciais inválidas), When o retry falha, Then o erro original (401) é propagado com hint de re-autenticação
- Given operações concorrentes, When múltiplas goroutines tentam refresh simultaneamente, Then apenas um refresh é executado (mutex)

### US-07: Criar ambiente cloud via `yby env create` (P2)
**As a** desenvolvedor, **I want** criar um ambiente com metadados cloud via `yby env create`, **so that** o Yby saiba como conectar ao cluster automaticamente.

**Acceptance Criteria:**
- Given flag `--cloud-provider aws`, When o usuário executa `yby env create prod-eks --type eks --cloud-provider aws`, Then o ambiente é criado com `Cloud.Provider: aws` e tipo `eks`
- Given ambiente tipo `eks` com CloudConfig, When o usuário executa `yby env show`, Then mostra provider, região, cluster name e status do token

### US-08: Autenticação avançada AWS (SSO, IRSA, MFA) (P3)
**As a** operador em organização enterprise, **I want** usar SSO corporativo ou IRSA para autenticar no EKS, **so that** minha organização possa aplicar políticas de segurança centralizadas.

**Acceptance Criteria:**
- Given `cloud.auth.method: sso` configurado, When o token expira, Then o Yby inicia fluxo SSO (abre browser para login)
- Given Yby rodando dentro de um pod EKS com service account, When `AWS_WEB_IDENTITY_TOKEN_FILE` está definido, Then o token é gerado via IRSA sem configuração adicional
- Given `mfa_serial` configurado, When assume-role é necessário, Then o Yby solicita o código MFA

### US-09: Autenticação Azure AD (P3)
**As a** operador em organização Azure, **I want** que o Yby suporte múltiplos modos de autenticação Azure AD, **so that** eu possa usar service principal, managed identity ou login interativo conforme meu cenário.

**Acceptance Criteria:**
- Given `cloud.login_mode: azurecli`, When token é necessário, Then é obtido via Azure CLI credential
- Given `cloud.login_mode: spn`, When client_id e client_secret estão configurados, Then token é obtido via service principal
- Given `cloud.login_mode: msi`, When Yby roda em Azure VM, Then token é obtido via managed identity

### US-10: Embeddings via Amazon Bedrock (P2)
**As a** usuário Bedrock, **I want** que embeddings do Synapstor usem Titan Embeddings, **so that** todo o pipeline de IA fique dentro do ecossistema AWS.

**Acceptance Criteria:**
- Given `ai.embedding.bedrock: amazon.titan-embed-text-v2:0` configurado, When o Synapstor indexa documentos, Then embeddings são gerados via Bedrock InvokeModel
- Given batch de 10 textos, When EmbedDocuments é chamado, Then cada texto é processado sequencialmente via InvokeModel (Titan não suporta batch nativo)

### US-11: Token generators via SDK nativo (P2)
**As a** operador em ambiente CI/CD sem CLIs cloud instalados, **I want** que o Yby gere tokens K8s programaticamente via SDKs Go, **so that** não precise instalar aws-cli/az/gcloud em containers de build.

**Acceptance Criteria:**
- Given build com tag `aws` e credenciais AWS via env vars, When `GetKubeClient()` é chamado para ambiente tipo `eks`, Then o token é gerado via `aws-iam-authenticator` lib sem CLI
- Given build sem tag `aws`, When tentativa de gerar token SDK, Then fallback para CLI `aws eks get-token`

### US-12: Credential store seguro (P3)
**As a** operador DevOps, **I want** que tokens SSO de longa duração sejam armazenados no keychain do OS, **so that** eu não precise re-autenticar a cada sessão.

**Acceptance Criteria:**
- Given login SSO bem-sucedido em macOS, When o token de sessão é obtido, Then é armazenado no macOS Keychain via go-keyring
- Given keychain indisponível (Linux sem secret-service), When armazenamento é tentado, Then fallback silencioso sem erro

### US-13: Auditoria de operações cloud (P3)
**As a** administrador de segurança, **I want** que todas as operações de autenticação cloud sejam registradas, **so that** eu tenha trilha de auditoria para compliance.

**Acceptance Criteria:**
- Given operação de autenticação cloud, When executada com sucesso, Then registro em `~/.yby/audit.log` contém timestamp, provider, identity, cluster, role
- Given flag `--audit-export`, When executado, Then logs são exportados em formato compatível com SIEM

## Functional Requirements

### FR-001: Interface CloudProvider
O sistema deve definir uma interface `CloudProvider` em `pkg/cloud/provider.go` com os métodos: `Name() string`, `IsAvailable(ctx) bool`, `CLIVersion(ctx) (string, error)`, `ListClusters(ctx, ListOptions) ([]ClusterInfo, error)`, `ConfigureKubeconfig(ctx, ClusterInfo) error`, `ValidateCredentials(ctx) (*CredentialStatus, error)`, `RefreshToken(ctx, ClusterInfo) error`. Três implementações concretas devem existir: AWS (EKS), Azure (AKS), GCP (GKE).

### FR-002: Detecção automática de cloud provider
O sistema deve detectar o cloud provider do kubeconfig ativo parseando o campo `exec.command` da entry do usuário atual. Padrões reconhecidos: `aws` ou `aws-iam-authenticator` → AWS, `kubelogin` ou `az` → Azure, `gke-gcloud-auth-plugin` ou `gcloud` → GCP.

### FR-003: Comando `yby cloud connect`
O sistema deve implementar o comando `yby cloud connect` que: (a) detecta CLIs cloud instalados, (b) valida credenciais, (c) lista clusters disponíveis, (d) configura kubeconfig via CLI do cloud, (e) opcionalmente cria ambiente em `environments.yaml`. Deve suportar modo interativo (prompts) e modo não-interativo (todas as flags).

### FR-004: Comando `yby cloud list`
O sistema deve implementar `yby cloud list [--provider P] [--region R]` que lista clusters disponíveis em formato tabular com colunas: nome, provider, região, versão K8s, status.

### FR-005: Comando `yby cloud status`
O sistema deve implementar `yby cloud status` que mostra: provider, identidade autenticada, método de autenticação, expiração do token, e status geral de conectividade.

### FR-006: Comando `yby cloud refresh`
O sistema deve implementar `yby cloud refresh` que detecta automaticamente o provider do ambiente atual e força refresh do token de autenticação.

### FR-007: Extensão da struct Environment
A struct `Environment` em `pkg/context/context.go` deve receber um campo `Cloud *CloudConfig` com `yaml:"cloud,omitempty"`. `CloudConfig` deve conter campos para provider, region, cluster, profile, role_arn, resource_group, subscription, tenant_id, login_mode, project_id, zone. Campos existentes devem permanecer inalterados (backward-compatible).

### FR-008: Novos tipos de ambiente
O sistema deve suportar os tipos de ambiente `eks`, `aks`, `gke` além dos existentes `local` e `remote`. O tipo deve ser usado para determinar o flow de autenticação e os checks do doctor.

### FR-009: Amazon Bedrock AI Provider
O sistema deve implementar `BedrockProvider` em `pkg/ai/bedrock.go` que implementa a interface `Provider` com os 6 métodos: `Name()`, `IsAvailable()`, `Completion()` (via Converse API), `StreamCompletion()` (via ConverseStream API), `EmbedDocuments()` (via InvokeModel com Titan), `GenerateGovernance()` (via Completion com system prompt específico).

### FR-010: Registro do Bedrock na factory
O sistema deve registrar `"bedrock"` em: `defaultPriority` (após `"openai"`), `createProvider` switch, `embeddingCapableProviders` map, e `validProviders` em `config.go`.

### FR-011: Interface TokenGenerator
O sistema deve definir uma interface `TokenGenerator` com método `GenerateToken(ctx, *CloudConfig) (*Token, error)` e implementações para AWS (via aws-iam-authenticator lib), Azure (via azidentity) e GCP (via oauth2/google). Compilação condicional via build tags.

### FR-012: Cache de tokens
O sistema deve implementar `TokenCache` com armazenamento in-memory thread-safe (`sync.RWMutex`), TTL baseado no campo `ExpiresAt` do token com margem de segurança de 60 segundos, e método `Invalidate` para forçar regeneração.

### FR-013: Integração de token cloud em GetKubeClient
Quando o ambiente ativo tem `Cloud` configurado e o build inclui a tag do provider correspondente, `GetKubeClient()` deve usar o `TokenGenerator` para obter um bearer token e injetá-lo em `rest.Config.BearerToken`, limpando `BearerTokenFile` para evitar conflitos.

### FR-014: Doctor checks para cloud
O `yby doctor` deve incluir uma seção "Cloud Providers" que: (a) lista CLIs cloud detectados e suas versões, (b) para o ambiente ativo, valida credenciais e reporta status, (c) detecta tokens expirados com sugestão de refresh.

### FR-015: AutoRefreshTransport
O sistema deve implementar `AutoRefreshTransport` (implementando `http.RoundTripper`) que intercepta respostas 401 do API server K8s, invalida o token cacheado, gera novo token via `TokenGenerator`, e repete a request. Refresh concorrente deve ser serializado via mutex.

### FR-016: Build tags condicionais
Código que depende de SDKs cloud deve usar build tags: `//go:build aws` para AWS SDK, `//go:build azure` para Azure SDK, `//go:build gcp` para GCP SDK, `//go:build cloud` para todos. O build padrão (`task build`) não deve incluir nenhuma tag cloud.

### FR-017: Autenticação avançada AWS
O sistema deve suportar: AWS SSO (Identity Center) via `sso-oidc`, assume-role em cadeia (cross-account), Web Identity (IRSA) via `AWS_WEB_IDENTITY_TOKEN_FILE`, e MFA via `mfa_serial`.

### FR-018: Autenticação avançada Azure
O sistema deve suportar: Azure CLI credential (`azurecli`), Service Principal com secret ou certificado (`spn`), Managed Identity (`msi`), Device Code Flow (`devicecode`), e Default Azure Credential (`default`).

### FR-019: Autenticação avançada GCP
O sistema deve suportar: Application Default Credentials, Workload Identity Federation, GKE Connect Gateway para clusters Fleet, e Service Account impersonation.

### FR-020: Credential store seguro
O sistema deve integrar com OS keychain (macOS Keychain, Linux secret-service, Windows Credential Manager) via `go-keyring` para armazenar tokens SSO de longa duração. Fallback silencioso quando keychain não disponível.

### FR-021: Auditoria de autenticação
O sistema deve registrar todas as operações de autenticação cloud em `~/.yby/audit.log` com: timestamp, provider, identity, cluster, role, método de auth, resultado (sucesso/falha). Formato JSON lines para compatibilidade com SIEM.

### FR-022: Env show com metadata cloud
O comando `yby env show` deve, para ambientes cloud, exibir: provider, região, cluster name, status do token (válido/expirado/desconhecido), e tempo restante do token.

### FR-023: Flag --cloud-provider em env create
O comando `yby env create` deve aceitar flag `--cloud-provider` que, quando especificada, dispara o fluxo de `CloudProvider.ConfigureKubeconfig()` e preenche o campo `Cloud` do ambiente.

### FR-024: Configuração Bedrock via config.yaml
O provider Bedrock deve ser configurável via `~/.yby/config.yaml` (campos `ai.provider`, `ai.models.bedrock`, `ai.embedding.bedrock`) e variáveis de ambiente (`AWS_REGION`, `AWS_PROFILE`, `YBY_AI_PROVIDER=bedrock`, `YBY_AI_MODEL`).

### FR-025: Usage tracking para Bedrock
O provider Bedrock deve extrair e registrar `usage` (InputTokens, OutputTokens, TotalTokens) das respostas Converse/ConverseStream e alimentar o `CostTrackingProvider` com preços atualizados dos modelos Bedrock.

## Non-Functional Requirements

### NFR-001: Backward compatibility
A adição do campo `Cloud` na struct `Environment` não deve quebrar `environments.yaml` existentes. Todos os campos novos devem ter tag `omitempty`. Ambientes sem `Cloud` devem continuar funcionando identicamente.

### NFR-002: Tamanho do binário padrão
O binário compilado sem build tags cloud (`task build`) não deve aumentar de tamanho. O aumento é aceitável apenas quando compilado com tags (`-tags cloud`), limitado a no máximo +15MB sobre o binário base.

### NFR-003: Latência de detecção cloud
A detecção de cloud provider via parsing de kubeconfig deve completar em menos de 100ms (operação local, sem I/O de rede). A validação de credenciais (que faz chamada de rede) deve ter timeout de 10 segundos.

### NFR-004: Latência de geração de token
A geração de token via SDK deve completar em menos de 5 segundos em condições normais de rede. O cache deve servir tokens em menos de 1ms.

### NFR-005: Segurança de credenciais
O sistema nunca deve persistir tokens, passwords ou API keys em `environments.yaml`, `config.yaml` ou qualquer arquivo plaintext. Apenas referências (profile names, role ARNs, resource group names) são permitidas em configuração.

### NFR-006: Concorrência do token refresh
O `AutoRefreshTransport` deve suportar pelo menos 100 goroutines concorrentes sem race conditions. Apenas 1 refresh deve ser executado por vez (serialização via mutex).

### NFR-007: Resiliência a CLIs ausentes
Quando um CLI cloud não está instalado, o sistema deve falhar graciosamente com mensagem informativa e link de instalação, sem panic ou stack trace.

### NFR-008: Bedrock response time
O provider Bedrock deve respeitar os mesmos timeouts configurados para outros providers. StreamCompletion deve entregar o primeiro chunk em menos de 3 segundos para modelos Sonnet.

### NFR-009: Testabilidade
Todas as implementações de `CloudProvider` e `TokenGenerator` devem aceitar `shared.Runner` como dependência para execução de comandos, permitindo mock completo em testes unitários sem acesso real a CLIs ou APIs cloud.

### NFR-010: Internacionalização
Todas as mensagens de terminal (status, erros, prompts, relatórios) devem passar por `i18n.T()`/`Tf()` quando o sistema de i18n estiver disponível. Enquanto não estiver, mensagens em PT-BR diretamente. Código e logs em inglês.

## Success Criteria

### SC-001: Cloud detection functional
Executar `yby doctor` em máquina com kubeconfig apontando para EKS retorna seção "Cloud Providers" com provider, região e cluster identificados corretamente.

### SC-002: Guided connect end-to-end
Executar `yby cloud connect` com AWS CLI configurado lista clusters e configura kubeconfig com sucesso, resultando em `kubectl get nodes` funcional.

### SC-003: Bedrock completion functional
Com `ai.provider: bedrock` e credenciais AWS válidas, `yby bard -p "hello"` retorna resposta via Bedrock Converse API.

### SC-004: Bedrock embedding functional
Com Bedrock configurado, o Synapstor gera embeddings via Titan Embed para pelo menos 1 documento sem erro.

### SC-005: Non-interactive connect
Executar `yby cloud connect --provider aws --region us-east-1 --cluster X --env-name prod` em CI/CD configura kubeconfig e cria ambiente sem input interativo.

### SC-006: Token refresh transparent
Durante operação longa (ex: `yby sentinel scan`) com token EKS que expira, o refresh ocorre transparentemente sem interrupção visível ao usuário.

### SC-007: Binary size unchanged
`task build` (sem tags cloud) produz binário com tamanho dentro de +-1% do tamanho atual.

### SC-008: Backward compatibility verified
`environments.yaml` existente (sem campo `cloud`) é carregado sem erro após as mudanças na struct `Environment`.

### SC-009: Doctor cloud checks
`yby doctor` em máquina sem CLIs cloud exibe "Nenhum provider cloud detectado" sem erro.

### SC-010: Concurrent token refresh safe
Teste de concorrência com 100 goroutines chamando `GetKubeClient()` simultaneamente com token expirado resulta em exatamente 1 chamada de refresh.

## Edge Cases

- **Kubeconfig com múltiplos clusters de providers diferentes:** `yby cloud list` sem flag `--provider` deve listar todos, agrupados por provider.
- **Token expira exatamente durante request:** `AutoRefreshTransport` deve detectar 401, não confundir com 403 (permissão negada real).
- **AWS profile inexistente:** Erro claro "AWS profile 'X' não encontrado. Profiles disponíveis: default, prod" com hint.
- **Região inválida:** Erro claro com lista de regiões válidas para o provider.
- **Cluster deletado entre list e connect:** Erro claro durante `ConfigureKubeconfig` com sugestão de re-listar.
- **Bedrock model não habilitado na conta:** Erro com hint "Habilite o modelo 'X' no console AWS Bedrock antes de usar".
- **Múltiplos kubeconfigs (KUBECONFIG com `:`):** Respeitar a cadeia de precedência do client-go sem conflito.
- **Credenciais AWS via instance metadata (EC2):** `IsAvailable` deve funcionar sem env vars ou profiles explícitos.
- **Azure com tenant múltiplo:** Respeitar `tenant_id` da CloudConfig ao invés de auto-detect.
- **GCP com projetos múltiplos:** Flag `--project` deve filtrar clusters corretamente.
- **Build sem nenhuma tag cloud:** Código que referencia SDKs não deve compilar; stubs devem existir para funcionalidade graceful.
- **Bedrock com modelo que não suporta Converse API:** Fallback para InvokeModel com formato body específico do modelo.
- **Rate limit da AWS API (ThrottlingException):** Retry com backoff exponencial via `RetryProvider` wrapper existente.

## Key Entities

| Entity | Description | Relationships |
|--------|-------------|---------------|
| `CloudProvider` | Interface de abstração para providers cloud (AWS, Azure, GCP) | Usado por `yby cloud` commands e `doctor` |
| `CloudConfig` | Struct com metadados cloud de um ambiente | Campo em `Environment`, usado por `TokenGenerator` |
| `ClusterInfo` | Metadados de um cluster K8s em cloud (name, region, version, status) | Retornado por `CloudProvider.ListClusters()` |
| `CredentialStatus` | Status das credenciais cloud (authenticated, identity, expires_at) | Retornado por `CloudProvider.ValidateCredentials()` |
| `TokenGenerator` | Interface para geração programática de tokens K8s | Usado por `GetKubeClient()` e `AutoRefreshTransport` |
| `Token` | Token bearer com valor e timestamp de expiração | Produzido por `TokenGenerator`, cacheado em `TokenCache` |
| `TokenCache` | Cache thread-safe de tokens com TTL | Usado por `AutoRefreshTransport` e `GetKubeClient()` |
| `AutoRefreshTransport` | HTTP RoundTripper que intercepta 401 e faz refresh | Integrado via `rest.Config.WrapTransport` |
| `BedrockProvider` | Provider de IA usando AWS Bedrock Converse API | Registrado na factory `pkg/ai` |
| `Environment.Cloud` | Campo opcional com CloudConfig em ambientes | Persistido em `.yby/environments.yaml` |

## Assumptions

- Os CLIs dos cloud providers (`aws`, `az`, `gcloud`) seguem o contrato de output JSON estável documentado por cada vendor.
- O exec plugin mechanism do kubeconfig (`user.exec.command`) é o padrão de facto para autenticação cloud em client-go.
- A Converse API do Bedrock suporta todos os modelos que o Yby precisa (Claude, Titan). Modelos futuros seguirão o mesmo contrato.
- Titan Embeddings V2 (`amazon.titan-embed-text-v2:0`) é o modelo de embedding padrão mais amplamente disponível no Bedrock.
- Build tags Go (`//go:build`) são suportadas por todos os toolchains Go 1.17+ e funcionam com GoReleaser.
- O pacote `go-keyring` funciona em Linux quando `gnome-keyring` ou `kwallet` está disponível; em headless Linux, o fallback silencioso é aceitável.
- A margem de 60 segundos antes da expiração do token é suficiente para cobrir latência de rede e tempo de processamento.

## Open Questions

- O Nível 3 (autenticação avançada SSO/IRSA/MFA, keychain, auditoria) é **separado do milestone atual** e será tratado como feature futura, entregue sob demanda conforme necessidade enterprise. O milestone `multicloud-support` cobre apenas Níveis 0, 1 e 2. Os requisitos P3 desta spec documentam o comportamento esperado para quando essa feature futura for iniciada, mas **não fazem parte do escopo de implementação atual**. Evidência: `plans/cloud-multicloud-support.md` classifica L3 explicitamente como "sob demanda, estimativa 20-30 dias" e o recomenda após L0+L1+L2 estarem estáveis.
- O GoReleaser deve produzir **artefatos separados** para builds com tags cloud: o binário padrão de release (`yby-linux-amd64`) **não inclui SDKs cloud**, mantendo tamanho inalterado (NFR-002). Binários cloud-enabled são publicados como artefatos adicionais com sufixo `cloud` (ex: `yby-cloud-linux-amd64`), seguindo o padrão já estabelecido no `.goreleaser.yaml` onde o plugin sentinel usa a flag `-tags=k8s` em build separado. O Taskfile deve adicionar task `build:cloud` com `-tags cloud` para builds locais.
- O `AutoRefreshTransport` deve interceptar **apenas 401**. A spec já define explicitamente nos Edge Cases: "deve detectar 401, não confundir com 403 (permissão negada real)". Interceptar 403 cegamente causaria loops em erros reais de RBAC, representando risco de segurança. Vendors que retornam 403 para tokens expirados receberão erro propagado com hint de re-autenticação via `yby cloud refresh` — comportamento seguro e aceitável. A implementação de referência em `plans/cloud-multicloud-support.go` confirma: `if resp.StatusCode == 401`.
