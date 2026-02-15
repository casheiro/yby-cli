# 🚀 Yby CLI - GitOps Radical

<div align="center">

<img src="https://i.imgur.com/2ZOMsy3.jpeg" alt="Yby Logo" width="160">

### [🌐 Website Oficial](https://yby.dev.br)

</div>

> **Yby (Tupi: Terra)** - O solo fértil para suas aplicações. CLI oficial para provisionamento de clusters Kubernetes **Ecofuturistas**: GitOps Radical, Eficiência Energética e Zero-Touch Discovery.

---

## 📋 Visão Geral

A **Yby CLI** é a interface unificada para gerenciar todo o ciclo de vida da infraestrutura da Casheiro Org, abstaindo a complexidade de Kubernetes, Helm e Argo CD.

- **🌱 Ecofuturista**: Padrões nativos para eficiência energética (Kepler) e scale-to-zero (KEDA).
- **🔒 GitOps Puro**: Tudo é gerenciado via Argo CD. Sem comandos imperativos.
- **🛠️ Self-Provisioning**: Configure VPS e clusters diretamente (`yby bootstrap vps`).
- **🏠 Offline-First**: O modo `yby dev` roda 100% local com Mirror Git interno.

---

## 🚀 Instalação Rápida

```bash
# Via Script (Linux/Mac)
curl -sfL https://raw.githubusercontent.com/casheiro/yby-cli/main/install.sh | sh -

# Via Go
go install github.com/casheiro/yby-cli@latest
```

> **Verificação:** Rode `yby doctor` para checar dependências (Docker, Helm, Kubectl).

---

## 📚 Documentação

A documentação completa foi movida para a nossa **Wiki**.

### 🎓 Guia Principal
- **[Getting Started](docs/wiki/Getting-Started.md)**: Passos iniciais.
- **[Core Concepts](docs/wiki/Core-Concepts.md)**: Estrutura, Monorepo e Arquivos Gerados.
- **[Architecture](docs/wiki/Architecture.md)**: Diagramas, Componentes e Segurança.

### 📖 Referência & Operação
- **[CLI Reference](docs/wiki/CLI-Reference.md)**: Todos os comandos.
- **[Plugins](docs/wiki/Plugins.md)**: Guia completo de extensão e plugins oficiais.
- **[Operations](docs/wiki/Operations.md)**: Manual do dia-a-dia e Troubleshooting.
- **[Governance](docs/wiki/Governance.md)**: IA, Agentes e DevGovOps.

---

## 🛠️ Exemplo de Uso

Inicie um novo projeto GitOps pronto para produção em segundos:

```bash
# 1. Crie o scaffold interativo
yby init

# 2. Suba o ambiente (Local = Sync, Remoto = Check)
yby up

# 3. Crie Workloads Padronizados
yby chart create meu-app

# 4. Limpeza (Ambiente Local)
yby destroy
```

> [!IMPORTANT]
> **Discovery Topic**: O Yby usa tópicos do GitHub para descobrir quais repositórios deployar.
> Por padrão, ele busca repositórios com a tag `yby-cluster`. Se você alterar isso no `values.yaml`, lembre-se de adicionar a nova tag nos seus repositórios!

---

## 📂 Estrutura do Projeto

Ao iniciar um projeto (`yby init`), você obtém:

```text
.
├── .github/workflows/    # Pipelines CI/CD e Release Automation
├── .yby/                 # Definições do Blueprint e Ambientes
├── infra/                # Manifestos Kubernetes (Helm/Kustomize)
│   ├── charts/           # Charts locais
│   └── manifests/        # ArgoCD Apps
└── README.md
```

---

<div align="center">
  <sub>Construído com 💚 pela Casheiro Org</sub>
</div>
