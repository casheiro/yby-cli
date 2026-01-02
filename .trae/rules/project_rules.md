# Regras do Projeto

## Contexto do Projeto

- Objetivos e Metas
  - Entregar um CLI robusto para orquestração de ambientes Yby com foco em dev local offline, GitOps (ArgoCD) e suporte a monorepos.
  - Garantir DX premium: inicialização coerente, ambientes consistentes, documentação sincronizada e testes verificáveis.
  - Evolução incremental com governança clara, evitando regressões e drift entre código e documentação.
- Público‑Alvo e Stakeholders
  - Desenvolvedores de plataforma e SREs que operam clusters locais, ambientes híbridos e pipelines GitOps.
  - Mantenedor responsável pelo repositório e pela wiki (submódulo).
- Justificativa e Benefícios
  - Eliminar dependências externas em dev local, reduzir fricção de setup, padronizar fluxos em monorepos, e aplicar verificação contínua de qualidade e documentação.

## Comandos

- build: go build ./cmd/yby
- test: go test ./...
- unit: go test ./... -run Test -v
- e2e: go test ./test/e2e/... -v
- fmt: go fmt ./...
- vet: go vet ./...

## Validacoes

- lint: go vet ./...
- typecheck: go build ./cmd/yby
- docs_verify: ./.trae/scripts/docs_verify.sh

## Regras Detalhadas

- Diretrizes de Desenvolvimento
  - Aplicar mudanças verticais e minimais por incremento, com verificação de lint, typecheck, build e testes.
  - Evitar dependência de contexto único: modularizar tarefas, registrar estado por incremento e usar reidratação sob demanda.
  - Manter referências clicáveis para trechos de código nos documentos quando relevante.
- Convenções de Código e Padrões Arquiteturais
  - Linguagem: Go 1.24.x; organização por `cmd/` (comandos) e `pkg/` (módulos).
  - CLI baseado em Cobra: comandos devem ter `Use`, `Short` e exemplos mínimos.
  - Contexto: manifesto `.yby/environments.yaml` como fonte única; evitar `.env` exceto fallback legado com aviso.
  - GitOps: root‑app de ArgoCD deve suportar mirror interno no ambiente `local` e prefixos de monorepo.
- Fluxos de Trabalho e Processos Obrigatórios
  - Antes do merge: passar `fmt`, `vet`, `build`, `test`, `e2e` (quando aplicável) e `docs_verify`.
  - Atualizações de código que alterem comportamento devem incluir atualização documental (README e/ou wiki) no mesmo PR.
  - Em monorepo/submódulo: avançar ponteiro da wiki quando a documentação for alterada.

## Requisitos Técnicos

- Stack Tecnológica
  - Go: 1.24.0 (toolchain 1.24.11) [go.mod](file:///home/neto/projects/casheiro-org/yby-cli/go.mod#L3-L5)
  - Cobra (CLI): v1.10.2 [go.mod](file:///home/neto/projects/casheiro-org/yby-cli/go.mod#L11)
  - Survey (wizard): v2.3.7 [go.mod](file:///home/neto/projects/casheiro-org/yby-cli/go.mod#L8)
  - Lipgloss (UI): v1.1.0 [go.mod](file:///home/neto/projects/casheiro-org/yby-cli/go.mod#L9)
  - YAML v3: v3.0.1 [go.mod](file:///home/neto/projects/casheiro-org/yby-cli/go.mod#L13-L14)
- Dependências e Integrações Externas
  - ArgoCD (GitOps): sincronização via repoURL; root‑app em `pkg/templates/assets/manifests/argocd`.
  - Kubernetes local (kind/k3d): cluster `yby-local` para dev; mirror interno para repositório Git.
- Configurações de Ambiente
  - YBY_ENV: `local` para dev offline; outros ambientes conforme topologia (dev, staging, prod).
  - Infra Root: `.yby` pode estar em subdiretório `infra/`; comandos devem detectar via `FindInfraRoot`.
  - Submódulo de documentação: `docs/wiki` deve ser sincronizado e o ponteiro atualizado no PR quando docs forem alteradas.

## Governança

- Estrutura e Responsabilidades
  - Responsável único pelo repositório e pela wiki coordena merges, valida documentação e garante qualidade.
  - Orquestração por agentes (Trae): orquestrador, contexto, desenvolvimento, documentação, QA, GitOps e release.
- Processos de Aprovação e Revisão
  - PR deve passar por verificação técnica e documental; aprovação do responsável é obrigatória.
  - “docs_verify” é gate de qualidade para assegurar cobertura de comandos/flags e links válidos.
- Ciclo de Vida e Versionamento
  - Atualizações em docs no mesmo PR de mudanças comportamentais.
  - Notas de versão por fase; registro de métricas em `.trae/metrics`.

## Anexos Úteis

- Diagramas Arquiteturais
  - Mapa de Bootstrap GitOps e Mirror Local (referência: [PLANO-DE-CORRECAO.md](file:///home/neto/projects/casheiro-org/yby-cli/PLANO-DE-CORRECAO.md)).
- Referências e Documentação
  - CLI Reference na wiki (submódulo): `docs/wiki/CLI-Reference.md`
  - Getting‑Started e Core‑Concepts na wiki.
- Exemplos de Implementação Correta
  - Scaffold com `.github` na raiz e `infra/` para charts/manifests.
  - Manifesto `.yby/environments.yaml` coerente: `current` presente na lista; `config/values-<env>.yaml` gerados.
