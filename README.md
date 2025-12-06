# Yby CLI

O **Yby CLI** é a ferramenta oficial de linha de comando para interagir com o ecossistema Yby. Ele simplifica a criação, configuração e gerenciamento de clusters Kubernetes dentro do padrão "Zero Touch" do projeto.

## Instalação

### Download da Última Versão

Acesse a página de [Releases](https://github.com/casheiro/yby-cli/releases) e baixe o binário compatível com o seu sistema operacional.

### via Go Install

Se você tem o Go instalado:

```bash
go install github.com/casheiro/yby-cli@latest
```

## Uso Básico

```bash
# Inicializa um novo projeto Yby no diretório atual
yby init

# Mostra ajuda
yby --help
```

## Funcionalidades

- **Init**: Configura novos projetos com defaults inteligentes.
- **GitOps**: Integração nativa com arquitetura GitOps.
- **Contextos**: Gerenciamento seguro de múltiplos ambientes (dev, prod).

## Contribuindo

Quer ajudar? Leia nosso [Guia de Contribuição](CONTRIBUTING.md) para começar.

## Licença

Distribuído sob a licença MIT. Veja `LICENSE` para mais informações.
