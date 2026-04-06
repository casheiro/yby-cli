# 🤖 Yby CLI - Infrastructure Intelligence Assistant

<div align="center">

<img src="https://i.imgur.com/2ZOMsy3.jpeg" alt="Yby Logo" width="160">

### [🌐 Website Oficial](https://yby.dev.br)

</div>

> **Yby (Tupi: Terra)** - O solo fértil para suas aplicações. A **Plataforma de Engenharia** que atua como seu assistente inteligente para bootstrap, governança e operação de clusters Kubernetes.

---

## 🧠 O Que é o Yby?

O **Yby CLI** não é apenas um wrapper de comandos. Ele é um **Assistente Estratégico** que orbita o ciclo de vida da sua infraestrutura, fornecendo:

1.  🚀 **Bootstrap Platform**: Entrega um cluster de produção "batteries-included" em minutos (VPS -> K3s -> ArgoCD -> Monitoring).
2.  🛡️ **Guardian (Governança)**: Garante padrões de arquitetura e segurança desde o Day 0.
3.  🤖 **Intelligence Layer**: Usa IA para diagnosticar problemas (`sentinel`), documentar arquitetura (`atlas`) e gerir conhecimento (`synapstor`).
4.  💻 **Developer Experience**: Abstrai a complexidade do ambiente local sem esconder a realidade do Kubernetes.

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
go install github.com/casheiro/yby-cli@latest
```

> **Verificação:** Rode `yby doctor` para checar se você tem as ferramentas necessárias (Docker, Helm, Kubectl).

---

## 🛠️ Exemplo de Uso

### 1. Bootstrap (Day 0)
Crie um novo projeto GitOps pronto para produção:
```bash
yby init        # Cria a estrutura do projeto
yby bootstrap   # Provisiona o cluster (Local ou VPS)
```

### 2. Operação Assistida (Day 1+)
Seu pod falhou? Pergunte ao Sentinel:
```bash
yby sentinel investigate pod-xyz -n production
# 🤖 Sentinel: "Detectei OOMKilled. Seu limite de memória é 128Mi, mas o pico foi 256Mi."
```

### 3. Evolução (Day N)
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
