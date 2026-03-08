# PLAN-robustness-sprint-2.md

**Sprint 2: Hardening Core e Documentação do SDK**

---

## 🎯 Visão Geral
Esta sprint visa consolidar a arquitetura de **Injeção de Dependência (DI)** em todo o CLI, focando nos comandos de ciclo de vida do ambiente (`up` e `access`). Além disso, iniciaremos a formalização do **Plugin SDK** através de documentação técnica robusta e realizaremos uma auditoria completa para eliminar incongruências na documentação (Wiki/CLI-Reference).

---

## 🚀 Especificações Técnicas

### 🏗️ Padrão de Arquitetura (DI)
Seguiremos o padrão estabelecido na Sprint 1:
- **Interfaces**: Definição de contratos em `pkg/services/<service>/interfaces.go`.
- **Adapters**: Implementações reais de I/O em `pkg/services/<service>/adapters.go`.
- **Service**: Lógica de negócio pura em `pkg/services/<service>/service.go`.
- **Unit Tests**: Mocks completos para as interfaces.

### 🧪 Estratégia de Testes
1. **Unitários**: Mock de `exec.Command`, `docker`, `kind` e `kubectl`.
2. **Integração (Local)**: Suite de testes que executa comandos reais contra o Docker host (marcados com `build tags` para rodar apenas localmente).

---

## 📋 Task Breakdown

### Ação 1: Hardening de `yby up` (Priority: P0)
- **T1.1**: Criar `pkg/services/environment/up_service.go` e interfaces.
- **T1.2**: Abstrair chamadas de `kind` e `docker` em adapters.
- **T1.3**: Escrever testes unitários com mocks (75% cobertura alvo).
- **T1.4**: Criar suite de integração `up_integration_test.go` (local-only).
- **T1.5**: Refatorar `cmd/up.go` para usar o serviço.

### Ação 2: Hardening de `yby access` (Priority: P1)
- **T2.1**: Criar `pkg/services/network/access_service.go` e interfaces.
- **T2.2**: Abstrair chamadas de `kubectl port-forward` e gerenciamento de processos.
- **T2.3**: Escrever testes unitários.
- **T2.4**: Refatorar `cmd/access.go`.

### Ação 3: Documentação do SDK de Plugins (Priority: P1)
- **T3.1**: Criar guia `docs/wiki/Plugins.md` detalhando o ciclo de vida (`Init`, `Exec`, `Status`).
- **T3.2**: Documentar o protocolo de troca de mensagens Plugin <-> Core.

### Ação 4: Faxina de Documentação (Priority: P2)
- **T4.1**: Auditar Wiki e CLI-Reference para remover referências a `yby dev`.
- **T4.2**: Corrigir exemplos de comandos desalinhados com a versão atual.
- **T4.3**: Documentar o **Deep Context Protocol** (uso de governança via UKIs).

---

## 🛠️ Atribuições de Agentes
- **Backend Specialist**: Ações 1 e 2 (Refatoração DI e Mocks).
- **Technical Writer (Antigravity)**: Ações 3 e 4 (Documentação e Wiki).
- **Test Engineer**: Suite de integração Docker.

---

## ✅ PHASE X: Verificação de Pronto
- [x] Build concluído sem erros (`go build ./cmd/yby/...`).
- [x] Testes unitários de `UpService` e `AccessService` passando.
- [x] Wiki atualizada e sem referências a `yby dev`.
- [x] Documento de SDK revisado.
- [ ] Métrica: Cobertura global do projeto aumentada em pelo menos 5%.

---

[OK] Plan created: docs/PLAN-robustness-sprint-2.md
