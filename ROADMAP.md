# Roadmap do Projeto Yby CLI

> **Nota:** Este documento delineia a visão de futuro e as prioridades estratégicas para o `yby-cli`. As datas e prazos são estimativas baseadas no entendimento atual e estão sujeitas a ajustes conforme a evolução do projeto e das necessidades da Casheiro Org.

## 🎯 Visão do Produto

O **Yby CLI** é nosso Assistente de Engenharia de Plataforma e Infraestrutura. Seu propósito é atuar como um **Assistente Inteligente** para bootstrap, governança e operação de clusters Kubernetes, organizando e orquestrando ferramentas padrão da indústria (como Helm, ArgoCD, Kubectl, k3d) em vez de ocultá-las.

**Status Atual:** 🟢 **Active (Governed)** - O projeto está ativo, com processos ágeis estabelecidos, mas exigindo aderência estrita às UKIs para mudanças semânticas e arquiteturais.

---

## 🗺️ Roadmap Estratégico (2025-2027)

### Curto Prazo (Q1/Q2 2026) - Foco: Robustez & Governança

Nesta fase, o foco é solidificar a base do CLI, garantindo que ele seja confiável, observável e siga rigorosamente os padrões de governança.

-   **Observabilidade & Logs:**
    -   [ ] Implementação de sistema de logs estruturados (JSON/Text) com suporte a flag `--log-level` para facilitar debugging e auditoria.
-   **Resiliência:**
    -   [ ] Melhoria substancial no tratamento de erros para subcomandos críticos (`k3d`, `helm`), fornecendo feedbacks mais claros e acionáveis ao usuário.
-   **Automação de Qualidade:**
    -   [ ] Automatização da verificação de **Conventional Commits** no pré-commit para garantir histórico limpo e versionamento semântico correto.
-   **Governança AI-Native:**
    -   [ ] Criação e consolidação das primeiras UKIs focadas em padrões de CLI (Inputs, Outputs, Colors) em `.synapstor/.uki/`.

### Médio Prazo (Q3/Q4 2026) - Foco: Expansão de Capacidades & IA

Com a base sólida, expandiremos as capacidades do Yby para torná-lo um verdadeiro "co-piloto" de infraestrutura.

-   **Multi-Cloud (EKS / AKS / GKE):** ✅ Entregue
    -   [x] Comandos `yby cloud connect|list|status|refresh` para gerenciar clusters em clouds públicas
    -   [x] Tipos de ambiente `eks`, `aks`, `gke` com token generators SDK e fallback CLI
    -   [x] AutoRefreshTransport para refresh automático de tokens K8s expirados
    -   [x] Build variants: `yby` (padrão) e `yby-cloud` (com SDKs nativos)
    -   [x] `yby doctor` com seção "Cloud Providers" (detecção de CLIs e validação de credenciais)
-   **Amazon Bedrock (IA):** ✅ Entregue
    -   [x] 6º provider de IA via Converse API + Titan Embeddings (build tag `aws`)
-   **Inteligência Artificial:**
    -   [ ] Integração de modelos de IA para diagnósticos avançados de falhas em clusters e sugestões de correção proativa.
-   **Ecossistema de Plugins:**
    -   [ ] Expansão da arquitetura de plugins para permitir maior modularidade e facilidade de extensão por outras squads.
-   **Interface Opcional:**
    -   [ ] Desenvolvimento de uma interface Web local (dashboard) opcional para visualização de estado e métricas rápidas.
-   **Cloud — Auth Avançada & Auditoria:** ✅ Entregue
    -   [x] Credential store seguro via OS keychain (go-keyring) com fallback encriptado AES-256-GCM
    -   [x] Auth avançada: SSO/MFA (AWS), device code/interactive/cert/MSI (Azure), WIF/SA impersonation/Connect Gateway (GCP)
    -   [x] Audit log JSONL de operações cloud com rotação 10MB e export json/csv
    -   [x] Dashboard TUI multi-cluster interativo (`yby cloud dashboard`)

### Longo Prazo (2027+) - Foco: Ecossistema & Autonomia

A visão de longo prazo é tornar o Yby um orquestrador autônomo e o centro de um ecossistema vibrante.

-   **Marketplace:**
    -   [ ] Criação de um marketplace de plugins comunitários internos e externos.
-   **Autonomia:**
    -   [ ] Capacidades de orquestração multi-cluster autônoma, onde o Yby pode gerenciar ciclo de vida e drifiting de configuração com supervisão mínima.

---

## 🤝 Como Contribuir

Interessado em ajudar a construir o futuro da Engenharia de Plataforma? Confira nosso [Guia de Contribuição](./CONTRIBUTING.md) para começar.

## 📝 Acompanhamento

Para acompanhar o progresso detalhado de tarefas específicas, consulte as **Issues** do GitHub e o backlog estratégico em [.synapstor/02_BACKLOG_AND_DEBT.md](./.synapstor/02_BACKLOG_AND_DEBT.md).
