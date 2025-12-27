# Yby CLI

Ferramenta CLI oficial para automação e gerenciamento de clusters na Casheiro Org, seguindo princípios GitOps.

## Instalação

```bash
go install github.com/casheiro/yby-cli@latest
```

## Começando (Getting Started)

A nova versão do Yby CLI utiliza uma **Engine de Scaffold Nativa** e gestão de ambientes explícita.

### 1. Inicializar Projeto

Use o comando `init` para gerar a estrutura base.

```bash
# Modo Interativo (Wizard)
yby init

# Modo Headless (Flags - Recomendado para Scripts)
yby init \
  --topology standard \
  --workflow gitflow \
  --git-repo https://github.com/my-org/my-project.git \
  --env dev
```

**Opções de Topologia:**
- `single`: Apenas ambiente `prod`.
- `standard`: `local` e `prod`.
- `complete`: `local`, `dev`, `staging`, `prod`.

### 2. Gerenciar Ambientes

O Yby CLI agora gerencia o contexto via `.yby/environments.yaml` (substituindo o antigo `.env`).

```bash
# Listar ambientes disponíveis
yby env list

# Trocar para ambiente de produção
yby env use prod

# Ver detalhes do ambiente atual
yby env show
```

### 3. Desenvolvimento Local

Para subir um cluster local (k3d) espelhando a infraestrutura:

```bash
# Inicia cluster e mirror
yby dev
```

## Estrutura do Projeto

```
.
├── .github/workflows/    # Pipelines CI/CD (gerados)
├── .yby/
│   └── environments.yaml # Definição de ambientes
├── config/
│   ├── values-local.yaml
│   └── values-prod.yaml
├── infra/                # Manifestos Kubernetes (Argo CD)
└── README.md
```

## Testing E2E

Para rodar os testes end-to-end da CLI (requer Docker):

```bash
go test -v ./test/e2e/...
```
