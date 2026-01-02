# Plano de Documentação: Manual yby-cli

Este documento define o plano incremental para a construção do Manual Oficial do `yby-cli`. O objetivo é criar uma documentação de qualidade industrial, 100% completa, cobrindo todos os comandos e fluxos de trabalho.

## Estratégia

A documentação será construída em **Fases**, permitindo entrega contínua e validação progressiva. A estrutura seguirá o framework **Diátaxis** (Concept, Tutorial, How-to, Reference), padrão ouro na indústria de software.

## Fases do Projeto

### Fase 1: Fundação e Estrutura Arquitetural <!-- id: phase-1 -->
**Objetivo**: Definir a taxonomia, o estilo e criar a estrutura de arquivos da documentação.
- [ ] **Definição de Information Architecture (IA)**: Mapear a árvore de navegação.
- [ ] **Style Guide**: Definir tom de voz (técnico porém acessível), formatação de código e convenções de avisos (Note/Warning).
- [ ] **Setup Inicial**: Criar o diretório `docs/` e arquivos de índice (`README` da doc).
- [ ] **Overview do Produto**: Escrever "O que é o Yby CLI", "Arquitetura GitOps" e "Vocabulário (Domain Ubiquitous Language)".

### Fase 2: Onboarding e Core Loop <!-- id: phase-2 -->
**Objetivo**: Garantir que um usuário novo consiga instalar e rodar o "Hello World" do Yby.
- [ ] **Instalação**: Linux, Setup de Dependências (Docker, k3d, Go).
- [ ] **Quickstart**: Do zero a aplicação rodando (`yby init` -> `yby dev`).
- [ ] **Conceito de Ambientes**: Explicar a gestão de contextos (`yby env`).
- [ ] **Arquitetura de Diretórios**: Documentar o que é gerado na estrutura de pastas (`infra/`, `.yby/`).

### Fase 3: Referência de Comandos (API Reference) <!-- id: phase-3 -->
**Objetivo**: Documentação exaustiva de cada comando, flags e comportamento.
- [ ] **Grupo Lifecycle**: `init`, `dev`, `status`, `uninstall`.
- [ ] **Grupo Environment**: `env list`, `env use`, `env show`, `env create`.
- [ ] **Grupo Bootstrap & Setup**: `setup`, `bootstrap vps`, `bootstrap cluster`.
- [ ] **Grupo Utilities**: `access`, `doctor`, `validate`, `version`.
- [ ] **Grupo Secrets & Generators**: `secrets`, `seal`, `generate keda`.

### Fase 4: Guias Avançados e Receitas (Cookbook) <!-- id: phase-4 -->
**Objetivo**: Cobrir casos de uso complexos e "Day 2 operations".
- [ ] **Guia de Monorepo**: Como trabalhar com `yby` em repositórios complexos (baseado nas correções recentes).
- [ ] **Segurança e Secrets**: Fluxo completo de Sealed Secrets.
- [ ] **Troubleshooting**: Guia de solução de problemas comuns (`doctor`, logs).
- [ ] **Contribuição**: Como desenvolver no próprio CLI.

## Controle de Execução

| ID | Atividade | Status |
| :--- | :--- | :--- |
| **F1.1** | Information Architecture & Style Guide | ⏳ Pendente |
| **F1.2** | Setup de `docs/` e Index | ⏳ Pendente |
| **F1.3** | Introdução e Arquitetura | ⏳ Pendente |
| | | |
| **F2.1** | Guia de Instalação | ⏳ Pendente |
| **F2.2** | Tutorial Quickstart | ⏳ Pendente |
| **F2.3** | Deep Dive: Ambientes e Arquivos | ⏳ Pendente |
| | | |
| **F3.1** | Ref: Lifecycle Commands | ⏳ Pendente |
| **F3.2** | Ref: Env Commands | ⏳ Pendente |
| **F3.3** | Ref: Bootstrap/Setup | ⏳ Pendente |
| **F3.4** | Ref: Utils/Secrets | ⏳ Pendente |
| | | |
| **F4.1** | Guia: Monorepos | ⏳ Pendente |
| **F4.2** | Guia: Secrets Management | ⏳ Pendente |
| **F4.3** | Troubleshooting & FAQ | ⏳ Pendente |

---
**Legenda**: ⏳ Pendente | 🚧 Em Andamento | ✅ Concluído
