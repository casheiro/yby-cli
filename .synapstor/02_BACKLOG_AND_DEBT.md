# Backlog & Technical Debt

> **Nota:** Este documento não substitui Issues do GitHub para tarefas granulares, mas serve como mapa estratégico de médio/longo prazo para entendimento dos agentes.

## 📌 Épicos & Roadmap

### 1. Robustez da CLI
- [ ] Implementar sistema de logs estruturados (JSON/Text) com flag `--log-level`.
- [ ] Melhorar tratamento de erros nos subcomandos `k3d`.

### 2. Governança AI-Native
- [ ] Criar UKIs iniciais para padrões de CLI (Inputs, Outputs, Colors).
- [ ] Automatizar verificação de Conventional Commits no pré-commit.

### 3. Multi-Cloud & Bedrock (Roadmap Futuro — P3)
- [ ] Auth avançada AWS: SSO, IRSA, MFA.
- [ ] Auth avançada Azure: Service Principal, Managed Identity.
- [ ] Credential store integrado (keychain nativo do OS).
- [ ] Auditoria de operações cloud (log de ações por provider/cluster).

---

## 💸 Dívida Técnica (Debt)

### Crítico
- *Nada identificado ainda.*

### Médio
- Falta de testes unitários em `pkg/cmd`.
- Documentação na Wiki pode estar defasada em relação ao código atual.
- Testes de integração real com EKS/AKS/GKE (requer infraestrutura cloud provisionada).

---

## 🧠 Dívida de Negócio/Processo

- **Falta de UKIs**: O conhecimento está todo no código ou na cabeça do PO. Prioridade máxima extrair regras de prompts e validações para `.synapstor/.uki/`.
