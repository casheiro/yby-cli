# Yby CLI

O **Yby CLI** é a ferramenta oficial de linha de comando para interagir com o ecossistema Yby. Ele simplifica a criação, configuração e gerenciamento de clusters Kubernetes dentro do padrão "Zero Touch" do projeto.

## Instalação

### Instalação Rápida (Linux/macOS)
```bash
curl -sfL https://raw.githubusercontent.com/casheiro/yby-cli/main/install.sh | sh -
```

### via Go Install
Se você tem o Go instalado:
```bash
go install github.com/casheiro/yby-cli/cmd/yby@latest
```

## Desinstalação

Para remover a CLI do sistema:
```bash
yby uninstall
```

## Uso Básico

```bash
# Inicializa um novo projeto Yby no diretório atual
yby init

# Mostra ajuda
yby --help
```

## Funcionalidades

- **Smart Init (Blueprint Engine)**: Configura projetos lendo dinamicamente `.yby/blueprint.yaml`. Zero hardcoding.
- **Ecofuturismo Tangível**: `yby status` exibe métricas de autoscale (KEDA) e status de sensores de energia (Kepler).
- **Diagnóstico Profundo**: `yby doctor` verifica a integridade da plataforma (CRDs) além de binários locais.
- **GitOps**: Integração nativa com arquitetura GitOps.
- **Contextos**: Gerenciamento seguro de múltiplos ambientes (dev, prod).

## Contribuindo

Quer ajudar? Leia nosso [Guia de Contribuição](CONTRIBUTING.md) para começar.

## Licença

Distribuído sob a licença MIT. Veja `LICENSE` para mais informações.  
