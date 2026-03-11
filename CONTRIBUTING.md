# Contribuindo para o Yby CLI

Obrigado pelo interesse em contribuir para o Yby! Este documento define o processo de desenvolvimento e release para garantir qualidade e automação.

## 🚀 Fluxo de Desenvolvimento (Git-Flow)

Adotamos um modelo **Git-Flow** adaptado.

- **`main`**: 🛡️ Produção. Contém apenas versões estáveis e "taggeadas". **Não abra PRs diretos para cá**, exceto hotfixes críticos.
- **`develop`**: 🧪 Integração. **Esta é a branch base para seus Pull Requests.** Todas as novas features e preparações para release acontecem aqui.

## 🤖 Trabalhando com Agentes & IA

Este repositório utiliza uma governança "AI-First". Isso significa que Agentes de IA são cidadãos de primeira classe no time.

### Personas Ativas
1.  **Governance Steward**: Guardião das regras e do `.synapstor`.
2.  **DevEx Guardian**: Focado na experiência do usuário final.
3.  **Platform Engineer**: Focado na robustez e implementação técnica.

### Workflows Recomendados
Ao abrir uma Issue ou dialogar com os Agentes, use os comandos padrão:

*   **Ideia Nova?** Use `/work-discovery` para ajudar a IA a entender o escopo.
*   **Nova Regra ou Padrão?** Use `/uki-capture` para formalizar uma decisão.
*   **Dúvida de Governança?** Pergunte "O que diz a UKI sobre X?".

## 📝 Como Contribuir

1.  **Fork** o projeto.
2.  Clone seu fork e configure o original como remote `upstream`.
3.  Crie uma **Branch** a partir de `develop`:
    ```bash
    git checkout develop
    git pull upstream develop
    git checkout -b feature/minha-nova-feature
    ```
4.  Implemente suas mudanças.
5.  **Commit** suas mudanças usando **Conventional Commits** (Veja abaixo).
6.  Abra um **Pull Request** apontando para a branch **`develop`** do repositório original.

## 🤖 Padrões de Commit e Automação

Utilizamos **automação total de releases** baseada no [Conventional Commits](https://www.conventionalcommits.org/).

> [!IMPORTANT]
> O título do seu PR e suas mensagens de commit determinam a versão do software automaticamente.
>
> - `feat: ...` -> Gera versão **Minor** (v1.1.0 -> v1.2.0)
> - `fix: ...` -> Gera versão **Patch** (v1.1.0 -> v1.1.1)
> - `BREAKING CHANGE: ...` -> Gera versão **Major** (v1.0.0 -> v2.0.0)

### Tipos Aceitos
- `feat`: Nova funcionalidade.
- `fix`: Correção de bug.
- `docs`: Documentação.
- `style`: Formatação, linting.
- `refactor`: Refatoração de código.
- `perf`: Melhoria de performance.
- `test`: Adição ou correção de testes.
- `chore`: Atualização de build, dependências, ferramentas.

> [!NOTE]
> **Política de Release Inteligente**: Mudanças que afetam apenas **documentação** (`docs/`, `*.md`) ou **governança** (`.synapstor/`) **NÃO** disparam uma nova versão da CLI.
> O release só será gerado se houver alteração em arquivos de código (`.go`, `go.mod`, templates, etc).

## 🧪 Validando Localmente

Pré-requisitos: [Go 1.24+](https://go.dev/doc/install).

```bash
# Clone o repositório
git clone https://github.com/casheiro/yby-cli.git
cd yby-cli

# Instale dependências
go mod tidy

# Rodar testes unitários
task test

# Rodar testes E2E (requer Docker)
task test:e2e

# Rodar linter
golangci-lint run

# Validar modo Server (Self-Provisioning)
go run main.go setup --profile=server
```
