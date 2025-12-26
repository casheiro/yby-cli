# 🌱 Yby CLI

> **Bootstrap facilitado para o ecossistema Yby.**
> Gerencie infraestrutura Kubernetes, contextos e ambientes de desenvolvimento com "Zero Touch".

![Governance](https://img.shields.io/badge/Governance-Matrix_Protocol-blueviolet)

O **Yby CLI** abstrai a complexidade de ferramentas como Helm, Argo CD e Kubernetes, oferecendo uma experiência de desenvolvedor (DX) fluida para iniciar projetos e gerenciar clusters.

> 📚 **Documentação Completa:** Para guias detalhados, arquitetura e referência avançada, visite nossa [Wiki Oficial](https://github.com/casheiro/yby-cli/wiki).

---

## 🧠 Governança AI-Native (Matrix Protocol)

Este projeto opera sob o **[Matrix Protocol](https://matrix-protocol.org)**, onde o Product Owner (Humano) é amplificado por Agentes de IA especializados que atuam como Tech Leads, QA e Arquitetos.

*   **Verdade Semântica:** Todo conhecimento perene (Regras, Decisões) reside em [`.synapstor`](./.synapstor/).
*   **Backlog & Roadmap:** Gerenciado em [`.synapstor/02_BACKLOG_AND_DEBT.md`](./.synapstor/02_BACKLOG_AND_DEBT.md).
*   **Agentes Ativos:**
    *   *Governance Steward* (Guardião do Contexto)
    *   *DevEx Guardian* (Advogado do Usuário)
    *   *Platform Engineer* (Garantia de Rocha)

---

## ⚡ Quick Start

Comece um novo projeto em segundos:

```bash
# 1. Instale a CLI
curl -sfL https://raw.githubusercontent.com/casheiro/yby-cli/main/install.sh | sh -

# 2. Inicialize o projeto
mkdir meu-projeto && cd meu-projeto
yby init

# 3. Suba o ambiente local
yby dev
```

---

## 📋 Pré-requisitos e Segurança

Para utilizar todas as funcionalidades (especialmente o ambiente local `dev`), certifique-se de ter instalado:

| Ferramenta | Necessário Para |
| :--- | :--- |
| **[Go](https://go.dev/dl/)** (v1.22+) | Instalação via Go (opcional) |
| **[Docker](https://docs.docker.com/get-docker/)** | Rodar o cluster local (k3d) |
| **[k3d](https://k3d.io/)** | Criar o cluster Kubernetes |
| **[kubectl](https://kubernetes.io/docs/tasks/tools/)** | Interagir com o Kubernetes |
| **[Helm](https://helm.sh/docs/intro/install/)** | Gerenciar pacotes (charts) |

### 🔐 Token de Acesso (Crítico)
O Yby utiliza o padrão **GitOps**, onde o cluster lê a configuração do seu repositório GitHub. Para isso, ele precisa de autenticação.
**Você deve exportar um PAT (Personal Access Token) válido:**

```bash
export GITHUB_TOKEN="ghp_..."
```
> **Nota:** O token deve ter permissão de `repo` (leitura total).
> **Local Dev:** Para ambiente local (`yby dev`), o token é opcional (modo Local Mirror), mas recomendado para evitar rate-limits.

---

## 🔄 Ciclo de Vida de Desenvolvimento (Atenção!)

Como o Yby segue o **GitOps**, o código que roda no cluster vem do **GitHub**, não da sua pasta local (temporariamente).

1. **Init:** `yby init` (Gera arquivos na pasta `infra/`)
2. **Commit & Push:** `git add . && git commit -m "init" && git push`
   > ⚠️ **IMPORTANTE:** Se você não der push, o cluster não verá a infraestrutura criada!
3. **Run:** `yby dev` (Sobe o cluster e sincroniza com o GitHub)
4. **Iterate:** Edite arquivos -> Commit -> Push -> O cluster atualiza sozinho.

---

## 🚀 Instalação e Atualização

Existem duas formas principais de instalar ou atualizar a Yby CLI.

### Opção 1: Instalador Automático (Recomendado)
Instala o binário em `/usr/local/bin`, acessível para todos os usuários. Não requer configuração de PATH.

**Instalar / Atualizar:**
```bash
curl -sfL https://raw.githubusercontent.com/casheiro/yby-cli/main/install.sh | sh -
```

### Opção 2: Via Go Install (Desenvolvedores)
Instala no seu diretório de usuário (`$HOME/go/bin`). Ideal se você quer compilar da fonte.

**Instalar / Atualizar:**
```bash
go install github.com/casheiro/yby-cli/cmd/yby@latest
```
> **Nota:** Certifique-se de adicionar `export PATH=$PATH:$(go env GOPATH)/bin` ao seu `.zshrc` ou `.bashrc`.

---

## 📖 Referência de Comandos

| Comando | Descrição | Exemplo de Uso |
| :--- | :--- | :--- |
| **`init`** | Inicializa um novo projeto Yby. Configura o blueprint e segredos iniciais. | `yby init` |
| **`dev`** | Sobe o ambiente de desenvolvimento local completo. **(Apenas Contexto Local)** | `yby dev` |
| **`bootstrap cluster`** | Instala a infraestrutura base (ArgoCD, Events, Workflows) em um cluster existente. | `yby bootstrap cluster` |
| **`context set <env>`** | Alterna entre contextos (local, staging, prod). | `yby context set prod` |
| **`context show`** | Exibe o contexto atual. | `yby context show` |
| **`doctor`** | Verifica a saúde das ferramentas e dependências. | `yby doctor` |
| **`status`** | Exibe métricas de operação (KEDA, Kepler, Pods). | `yby status` |
| **`validate`** | Valida os arquivos de configuração do projeto. | `yby validate` |
| **`uninstall`** | Remove a CLI do sistema. | `yby uninstall` |
| **`version`** | Exibe a versão instalada. | `yby version` |

---

## ✨ Funcionalidades Inteligentes

### 🛡️ Auto-Repair (Auto-Reparo)
O `yby dev` é resiliente. Se você (ou o git) apagar acidentalmente arquivos críticos como `infra/manifests` ou diretórios do sistema:
1. A CLI detecta a ausência.
2. Baixa os originais do repositório de template (`casheiro/yby-template`).
3. Restaura a estrutura de pastas automaticamente.

### 🧠 Smart Templating
Ao restaurar arquivos, a CLI não apenas copia — ela **configura**.
- O `root-app.yaml` é injetado com a URL do **seu** repositório GitHub.
- Isso garante que o GitOps funcione imediatamente, sem edição manual de arquivos YAML.

### 🔌 Integration Mode (Modo de Integração)
Você tem um monorepo ou quer organizar a infraestrutura em uma pasta separada (ex: `infra/`)?
- O `yby init` detecta a estrutura do projeto.
- Se você escolher instalar em um subdiretório, a CLI **reescreve automaticamente** os caminhos:
    - No ArgoCD (`root-app.yaml`).
    - No Argo Events (`sensor.yaml`).
    - Nos Workflows do GitHub (`.github/workflows`).
- Resultado: GitOps funcionando perfeitamente, não importa a estrutura de pastas.

### 🔒 Segurança Nativa (Hardened Standard)
O Yby eleva o nível de segurança sem configuração extra:
1. **Segredos em RAM:** Credenciais são montadas em memória (`/run/secrets`) por padrão, eliminando vazamentos via variáveis de ambiente.
2. **Isolamento de Rede:** Policies padrão protegem a infraestrutura crítica (`infra`) contra acesso não autorizado, permitindo apenas tráfego legítimo das apps.

---

## 🩺 Troubleshooting

**Erro: `command not found: yby`**
- Se instalou via Go: Verifique seu PATH.
- Se instalou via script: Verifique se `/usr/local/bin` está no PATH.

**"Missing charts/system"**
- Apenas rode `yby dev` novamente. O sistema de Auto-Repair irá baixar e restaurar a pasta `charts/system` automaticamente.

---

## 🤝 Contribuindo

1. Faça um Fork do projeto
2. Crie sua Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a Branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📄 Licença

Distribuído sob a licença MIT. Veja `LICENSE` para mais informações.
