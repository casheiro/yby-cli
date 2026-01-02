# Plano de Documentação: Manual yby-cli

Este documento define o plano incremental para a construção do Manual Oficial do `yby-cli`. O objetivo é criar uma documentação de qualidade industrial, 100% completa, cobrindo todos os comandos e fluxos de trabalho.

## Estratégia: Wiki-First

A documentação será centralizada no **GitHub Wiki** (já presente em `docs/wiki`), garantindo fácil edição e acesso. O plano foca em **Refatorar, Padronizar e Expandir** o conteúdo existente para atingir 100% de cobertura.

A estrutura seguirá o framework **Diátaxis** (Concept, Tutorial, How-to, Reference), adaptada para a navegação plana/hierárquica de Wikis.

## Estratégia: Híbrida (Automação + Contexto)

Para garantir **100% de completude**, abordaremos a documentação em duas frentes integradas:

1.  **Referência Automática (Source-Driven)**: Garantia de que *nenhum* comando ou flag seja esquecido. Gerado via `cobra/doc` diretamente do binário.
2.  **Guias de Contexto (Concept-Driven)**: Documentação manual daquilo que não está no código Go (Arquitetura, Pipelines CI/CD gerados, Estrutura de Pastas, K8s Manifests).

### Mecanismo de Garantia
*   **Automação**: O comando `yby gen-docs` será parte do CI, falhando o build se a doc estiver desatualizada.
*   **Auditoria de Escopo**: Checklist manual para garantir que *workflows* (não apenas comandos) estão cobertos.

## Fases do Projeto

### Fase 1: Automação e Infraestrutura <!-- id: phase-1 -->
**Objetivo**: Criar a "fábrica" de documentação.
- [x] **Gerador de Docs**: Implementar comando oculto `yby gen-docs` usando `spf13/cobra/doc`.
- [x] **Integração na Wiki**: Script para injetar os MDs gerados na estrutura da Wiki.
- [x] **Navbar Dinâmica**: Script para atualizar `_Sidebar.md` com os novos comandos gerados.
- [x] **Baseline de Governança**: Definir `_Footer.md` e Style Guide.

### Fase 2: Onboarding Completo (Tutorial & Concepts) <!-- id: phase-2 -->
**Objetivo**: Guiar o usuário do zero ao "Hello World" funcional.
- [x] **Refatorar `Home.md`**: Transformar em Landing Page orientada a ação.
- [x] **Refatorar `Getting-Started.md`**: Atualizar instalação e setup inicial.
- [x] **Expandir `Core-Concepts.md`**: Explicar profundamente Ambientes, Infra-as-Code e GitOps no Yby.
- [x] **Criar `Architecture.md` (Update)**: Diagramas atualizados do fluxo local vs remoto.

### Fase 3: Referência Automatizada (API Reference) <!-- id: phase-3 -->
**Objetivo**: Gerar a documentação técnica exaustiva.
- [x] **Executar `yby gen-docs`**: Gerar markdown para todos os comandos.
- [x] **Review de Descrições**: Auditar o código Go (`cmd/*.go`) para garantir que as strings de `Use`, `Short` e `Long` description são ricas e explicativas (pois elas viram a doc).
- [x] **Enriquecimento de Exemplos**: Adicionar campos `Example:` nas structs do Cobra onde faltar.

### Fase 4: Cobertura do Ecossistema (Beyond Code) <!-- id: phase-4 -->
**Objetivo**: Documentar os "Efeitos colaterais" e arquitetura, que o código Go não mostra.
- [x] **Spec de Arquivos Gerados**: Documentar linha-a-linha o `environments.yaml`, `blueprint.yaml` e `values-*.yaml`.
- [x] **Arquitetura Gerada**: Explicar a estrutura da pasta `infra/` (Charts, ArgoCD Apps) que o CLI cria.
- [x] **Pipelines e GitOps**: Explicar os workflows do GitHub Actions gerados pelo `yby init`.
- [x] **Guias Operacionais**: Monorepo, Troubleshooting, Secrets (Sealed Secrets).

## Controle de Execução

| ID | Atividade | Status |
| :--- | :--- | :--- |
| **F1.1** | Implementar `yby gen-docs` (Cobra Doc) | ✅ Concluído |
| **F1.2** | Script de Sync Wiki + Sidebar Automática | ✅ Concluído |
| | | |
| **F2.1** | Landing Page (`Home.md` refatorada) | ✅ Concluído |
| **F2.2** | Guia de Instalação & Dependências | ✅ Concluído |
| **F2.3** | Deep Dive: Core Concepts & Arch | ✅ Concluído |
| | | |
| **F3.1** | Code Audit: Enriquecer Help Texts no Go | ✅ Concluído |
| **F3.2** | Geração e Publicação da Referência | ✅ Concluído |
| | | |
| **F4.1** | Deep Dive: Arquivos de Config (.yby) | ✅ Concluído |
| **F4.2** | Deep Dive: Infraestrutura Gerada | ✅ Concluído |
| **F4.3** | Guia: Monorepo & Secrets | ✅ Concluído |

---
**Legenda**: ⏳ Pendente | 🚧 Em Andamento | ✅ Concluído

# ✅ PROJETO CONCLUÍDO (100%)
