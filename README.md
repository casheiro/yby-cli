# 🤖 Yby CLI - Infrastructure Intelligence Assistant

<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/brand/yby-logo-white.svg">
  <img src="docs/assets/brand/yby-logo.svg" alt="Yby" width="320">
</picture>

### [🌐 Website Oficial](https://yby.dev.br)

</div>

> **Yby (Tupi: Terra)** - O solo fértil para suas aplicações. A **Plataforma de Engenharia** que atua como seu assistente inteligente para bootstrap, governança e operação de clusters Kubernetes.

---

## 🧠 O Que é o Yby?

O **Yby CLI** não é apenas um wrapper de comandos. Ele é um **Assistente Estratégico** que orbita o ciclo de vida da sua infraestrutura, fornecendo:

1.  🚀 **Bootstrap Platform**: Entrega um cluster de produção "batteries-included" em minutos (VPS -> K3s -> ArgoCD -> Monitoring).
2.  ☁️ **Multi-Cloud** (beta): Conecta e gerencia clusters em AWS EKS, Azure AKS e GCP GKE com refresh automático de credenciais.
3.  🛡️ **Guardian (Governança)**: Garante padrões de arquitetura e segurança desde o Day 0.
4.  🤖 **Intelligence Layer**: Usa IA (Ollama, Claude, Gemini, OpenAI, Bedrock) para diagnosticar problemas (`sentinel`), documentar arquitetura (`atlas`) e gerir conhecimento (`synapstor`).
5.  💻 **Developer Experience**: Abstrai a complexidade do ambiente local sem esconder a realidade do Kubernetes.

---

## 🆚 Yby vs Kubectl

O Yby **não substitui** o `kubectl`. Eles trabalham juntos:

| Funcionalidade | Ferramenta Padrão | Abordagem Yby |
| :--- | :--- | :--- |
| **Interagir com Cluster** | `kubectl get pods` | `yby sentinel investigate` (Adiciona IA e Contexto ao diagnóstico) |
| **Gerenciar Releases** | `helm install` | `yby chart create` (Gera Boilerplate Padronizado e Seguro) |
| **Setup Inicial** | Scripts Bash manuais | `yby bootstrap vps` (Declarativo, Idempotente e Seguro) |
| **Documentação** | Wiki desatualizada | `yby synapstor` (Conhecimento vivo extraído do código) |

> **Resumo:** Use o `kubectl` para operar. Use o `yby` para entender, planejar e evoluir.

---

## 🚀 Instalação Rápida

```bash
# Via Script (Linux/Mac)
curl -sfL https://raw.githubusercontent.com/casheiro/yby-cli/main/install.sh | sh -

# Via Go
go install github.com/casheiro/yby-cli/cmd/yby@latest
```

> **Verificação:** Rode `yby doctor` para checar se você tem as ferramentas necessárias (Docker, Helm, Kubectl) e validar credenciais cloud.

---

## 🛠️ Exemplo de Uso

### 1. Bootstrap (Day 0)
Crie um novo projeto GitOps pronto para produção:
```bash
yby init        # Cria a estrutura do projeto
yby bootstrap   # Provisiona o cluster (Local ou VPS)
```

### 2. Operação Assistida (Day 1+)
Acompanhe o cluster ao vivo no Viz, com os problemas primeiro:
```bash
yby viz
```

![yby viz: pods com o detalhe ao lado](docs/assets/telas/viz-pods.png)

Seu pod falhou? Pergunte ao Sentinel:
```bash
yby sentinel investigate worker-748cf7f544-9htmh -n payments
```

![yby sentinel investigate: diagnóstico com a confiança e o comando sugerido](docs/assets/telas/sentinel-investigate.png)

Ou converse com o Bard, que roda as ferramentas por você (no Viz, `b` abre o Bard já com a pergunta sobre o recurso selecionado):
```bash
yby bard
```

![yby bard: resposta com a ferramenta usada e o comando sugerido](docs/assets/telas/bard-conversa.png)

> **Modo visual:** o tema segue o terminal. Force com `YBY_THEME=dark` ou `YBY_THEME=light`, use `YBY_ASCII=1` em terminais sem Unicode e `NO_COLOR=1` para tirar as cores (ou `ui.theme` e `ui.ascii` no `~/.yby/config.yaml`).

### 3. Multi-Cloud (EKS / AKS / GKE) · beta
> **Beta:** testado com os CLIs `aws`, `az` e `gcloud` e conferido contra a documentação oficial, mas ainda não contra clusters reais. O que falta validar está no [ROADMAP](./ROADMAP.md). Relate problemas nas [issues](https://github.com/casheiro/yby-cli/issues).

Conecte clusters de qualquer cloud pública com auth avançada:
```bash
# Conectar a cluster cloud (interativo)
yby cloud connect

# Conectar a cluster EKS (não-interativo)
yby cloud connect --provider aws --region us-east-1 --cluster meu-cluster --env-name prod

# Listar clusters disponíveis
yby cloud list

# Verificar status de credenciais
yby cloud status

# Refresh de token
yby cloud refresh

# Dashboard multi-cluster interativo
yby cloud dashboard

# Auditoria de operações cloud
yby cloud audit --since 24h --export json
```

![yby cloud dashboard: clusters com a validade dos tokens](docs/assets/telas/cloud-painel.png)

**Auth avançada:** SSO/MFA (AWS), device code/interactive/certificado/MSI (Azure), Workload Identity Federation/SA impersonation/Connect Gateway (GCP). Credential store seguro via OS keychain com fallback encriptado. Audit log JSONL para compliance.

> **Build cloud:** Para binário com SDKs nativos (AWS/Azure/GCP), use `task build:cloud`. O binário padrão (`task build`) usa fallback via CLIs instalados.

### 4. Evolução (Day N)
Precisa de um novo serviço?
```bash
yby chart create meu-novo-app  # Cria chart seguindo Golden Path da empresa
yby up                         # Sobe ambiente de dev local espelhando produção
```

---

## 🧩 Extensibilidade (Plugins)

O Yby foi desenhado para ser estendido. Não encontrou o que precisa? Crie seu próprio comando!

*   **Linguagem Agnóstica**: Seu plugin pode ser em Go, Rust, Bash, Python... qualquer coisa que fale JSON.
*   **API Simples**: Receba contexto no `STDIN`, devolva ações no `STDOUT`.
*   **Distribuição Fácil**: Publique no GitHub e instale com `yby plugin install`.

[📖 Guia de Desenvolvimento de Plugins](docs/wiki/Plugins.md)

---

## 🏢 Enterprise Ready

O Yby entrega defaults opinativos para PMEs, mas adapta-se a qualquer ambiente enterprise via um arquivo de overrides:

```yaml
# .yby/overrides.yaml
registry:
  url: 012345678.dkr.ecr.sa-east-1.amazonaws.com
  pullSecret: regcred
ingress:
  className: nginx
namespaces:
  prefix: "fintech"
tls:
  issuer: custom
  caSecretName: corp-ca
```

```bash
yby init --config .yby/overrides.yaml
```

Customiza: registry privado, ingress class, TLS/CA corporativa, storage class, namespaces com prefixo, labels de compliance, versoes de charts, resource profiles e mais. [Guia completo](docs/wiki/Enterprise-Overrides.md).

---

## 📚 Documentação

A documentação completa está na nossa **Wiki**:

- **[Getting Started](docs/wiki/Getting-Started.md)**: Passos iniciais.
- **[Core Concepts](docs/wiki/Core-Concepts.md)**: Estrutura, Monorepo e Arquivos Gerados.
- **[Architecture](docs/wiki/Architecture.md)**: Diagramas, Componentes e Segurança.
- **[Enterprise Overrides](docs/wiki/Enterprise-Overrides.md)**: Customizacao para ambientes corporativos.
- **[Governance](docs/wiki/Governance.md)**: IA, Agentes e DevGovOps.
- **[Plugins](docs/wiki/Plugins.md)**: Bard, Viz, Sentinel, Synapstor e Atlas, com as teclas de cada TUI.
- **[Operations](docs/wiki/Operations.md)**: Operação do dia a dia, clusters cloud e solução de problemas.
- **[CLI Reference](docs/wiki/CLI-Reference.md)**: Todos os comandos e flags.
- **[CHANGELOG](CHANGELOG.md)**: Teclas e comportamentos que mudaram entre versões.

---

## 📂 Estrutura Gerada

```text
.
├── .github/workflows/    # Pipelines CI/CD e Release Automation
├── .yby/                 # Definições do Blueprint e Ambientes
├── infra/                # Manifestos Kubernetes (Helm/Kustomize)
│   ├── charts/           # Charts locais (Golden paths)
│   └── manifests/        # ArgoCD Apps (GitOps state)
└── README.md
```

---

<div align="center">
  <sub>Construído com 💚 pela Casheiro Org</sub>
</div>
