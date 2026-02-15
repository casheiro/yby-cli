# YBY-CLI Feedback — Anhumas Nexus

Registro completo de problemas, inconsistencias, desafios e sugestoes encontrados durante o uso do yby-cli neste projeto.

**Projeto**: Anhumas Nexus (monorepo Python — FastAPI, PostgreSQL, Redis, Kestra, MinIO)
**yby-cli versao**: v4.7.0+
**Data**: 2026-02-14
**Contexto**: Sprint-035 (EPICO-45) — primeira tentativa de IaC completo com yby
**Ciclos de teste**: 2 (tentativa inicial + reteste limpo do zero)
**Branch de teste**: `test/yby-dev-infra`

---

## Resumo Executivo

### A Promessa do yby

O yby-cli promete um fluxo automatizado de infraestrutura GitOps:

```
yby init → gera scaffold completo
yby dev  → sobe cluster local com ArgoCD + stack
           commit → sync automatico → deploy automatico
           Tempo de feedback: < 5 segundos
```

A expectativa e que, apos `yby dev`, o desenvolvedor apenas:
1. Escreva codigo (charts, configs, aplicacoes)
2. Faca commit
3. O yby sincronize automaticamente com o git-server interno
4. O ArgoCD detecte mudancas e faca deploy

Nenhuma intervencao manual com `helm`, `kubectl apply` ou `kubectl port-forward` deveria ser necessaria, exceto para customizacoes especificas do usuario.

### A Realidade

Apos 2 ciclos de teste completos, identificamos:

| Categoria | Quantidade |
|-----------|-----------|
| Bugs | 20 |
| Incongruencias (docs vs realidade) | 4 |
| Sugestoes de melhoria | 12 |
| Achados positivos | 7 |

**Para chegar a um ambiente funcional** (infra + workloads acessiveis), foram necessarias **13+ intervencoes manuais** em 2 fases:

- **Fase 1 (Stack infra)**: 10 workarounds para ArgoCD + stack funcionar
- **Fase 2 (Workloads)**: 3+ workarounds adicionais para nexus-core, Kestra e PostgreSQL ficarem acessiveis

**O pipeline automatico de sync (commit → deploy) NAO FUNCIONA** — o sync scheduler do `yby dev` falha com `exit status 128` porque tenta resolver DNS interno do cluster a partir do host. Todo push para o git-server precisou ser feito manualmente via port-forward.

**Tempo total**: ~45 minutos de intervencoes manuais para um ambiente que deveria levar ~5 minutos no "zero config" prometido.

---

## Jornada Completa: Tudo que Foi Necessario

Esta secao documenta cronologicamente TODAS as intervencoes manuais necessarias desde `yby init` ate ter todos os servicos acessiveis.

### Fase 0: Scaffold (`yby init`)

```bash
yby init \
  --project-name anhumas-nexus \
  --env dev \
  --topology standard \
  --target-dir infra \
  --enable-minio \
  --git-repo https://github.com/casheiro/anhumas-nexus.git \
  --description "..."
```

**Problemas imediatos (pre-cluster)**:
1. `cluster-values.yaml` gerado com `environment: local` (schema so aceita dev/staging/prod) → **BUG-001**
2. `discovery.organization:` gerado como null (schema exige string) → **BUG-002**
3. `root-app.yaml` aponta para GitHub em vez de git-server interno → **BUG-009**
4. `cluster-config-app.yaml` com path sem prefixo `infra/` → **BUG-011**
5. `external-apps.yaml` sem guard condicional (nil pointer) → **BUG-014**
6. `values-local.yaml` referenciado em `environments.yaml` mas NAO gerado → **BUG-004**

**Intervencoes manuais** (5 edicoes de arquivo + commit):
```bash
# Corrigir values, paths, guards, URLs em 5 arquivos
# Commitar antes de rodar yby dev
```

### Fase 1: Bootstrap do Cluster (`yby dev`)

```bash
yby dev
# Output mostra sync scheduler falhando imediatamente:
# ❌ Falha no Sync inicial: exit status 128
# ⚠️ Erro de Sincronizacao: exit status 128 (repete a cada 5s)
```

**Pipeline de sync QUEBRADO** — BUG-007 (CRITICO):
- O sync scheduler tenta acessar `git://git-server.yby-system.svc:80/repo.git` do HOST
- DNS interno do cluster (`*.svc`) nao resolve no host
- Resultado: git-server permanece VAZIO, ArgoCD nao tem codigo para sincronizar

**Intervencoes manuais pos-cluster** (5 operacoes):
```bash
# 1. Patchar porta do git-server service (80→9418) — BUG-008
kubectl patch svc git-server -n yby-system --type merge \
  -p '{"spec":{"ports":[{"port":9418,"targetPort":9418,"protocol":"TCP"}]}}'

# 2. Corrigir HEAD do bare repo (master→main) — BUG-010
kubectl exec -n yby-system deployment/git-server -- \
  git -C /git/repo.git symbolic-ref HEAD refs/heads/main

# 3. Aplicar AppProject manualmente — BUG-006
kubectl apply -f infra/manifests/projects/yby-project.yaml

# 4. Push manual para git-server via port-forward — BUG-007
kubectl port-forward -n yby-system svc/git-server 19418:9418 &
git push git://localhost:19418/repo.git HEAD:main

# 5. Criar SA + RBAC para webhook-info — BUG-012 + BUG-015
kubectl create serviceaccount argo-workflow-executor -n argo-events
kubectl apply -f - <<'EOF'
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: webhook-info-reader
rules:
- apiGroups: [""]
  resources: ["nodes", "services", "secrets"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: webhook-info-reader-binding
subjects:
- kind: ServiceAccount
  name: argo-workflow-executor
  namespace: argo-events
roleRef:
  kind: ClusterRole
  name: webhook-info-reader
  apiGroup: rbac.authorization.k8s.io
EOF

# 6. Criar secret MinIO (SealedSecrets nao descriptaveis) — BUG-016
kubectl create secret generic minio-creds -n default \
  --from-literal=rootUser=admin --from-literal=rootPassword=minioadmin123

# 7. Forcar refresh do ArgoCD
kubectl annotate application root-app -n argocd \
  argocd.argoproj.io/refresh=hard --overwrite
```

**Resultado da Fase 1**: 7 Applications ArgoCD sincronizadas (6 Healthy, 1 Degraded). Stack de infra funcional (ArgoCD, Argo Workflows, Argo Events, Prometheus, KEDA, Sealed Secrets, MinIO, cert-manager).

### Fase 2: Deploy de Workloads (nexus-core, Kestra, PostgreSQL)

O ArgoCD mostra Applications `nexus-core`, `data-operator`, `databases` e `workflow` como Synced/Healthy, porem ao investigar:

**Todos os diretórios de charts customizados estavam VAZIOS** — apenas arquivos `.gitkeep`. O ArgoCD reportava Healthy porque um diretorio vazio e tecnicamente valido.

Para validar o pipeline completo (commit → sync → deploy → acessivel), criamos charts minimos:

#### 2a. nexus-core (nginx placeholder)

```bash
# Criar Chart.yaml, values.yaml, templates/deployment.yaml manualmente
# Fazer helm dependency build (nao necessario aqui, chart simples)
# Commitar, fazer push manual via port-forward — BUG-007 (novamente)
kubectl port-forward -n yby-system svc/git-server 19418:9418 &
git push git://localhost:19418/repo.git test/yby-dev-infra:main
# Aguardar ArgoCD detectar e fazer deploy
```

**Resultado**: nexus-core acessivel via `kubectl port-forward svc/nexus-core 8000:80` → HTTP 200

#### 2b. CloudNativePG Operator (data-operator)

```bash
# Criar Chart.yaml com dependencia do helm chart cloudnative-pg
helm dependency build infra/charts/data/cloudnativepg-operator/
# Commitar (incluindo .tgz do charts/), push manual — BUG-007
```

**Problema critico** — BUG-018: Os CRDs do CNPG excedem 262KB (limite de anotacoes do kubectl client-side apply). ArgoCD falha com:
```
metadata.annotations: Too long: may not be more than 262144 bytes
```

**Intervencao manual** (operacao complexa):
```bash
# Extrair CRDs do chart e aplicar com server-side apply
helm template cnpg infra/charts/data/cloudnativepg-operator/charts/cloudnative-pg-*.tgz \
  | kubectl apply --server-side --force-conflicts -f -
```

Tambem editamos `data-operator.yaml` no bootstrap para incluir `ServerSideApply=true` nos syncOptions. Porem este fix NAO propagou porque o root-app esta preso em revisao antiga (BUG-019).

#### 2c. PostgreSQL (databases via CNPG)

```bash
# Criar Chart.yaml, values.yaml, templates/cluster.yaml (CRD Cluster do CNPG)
# Commitar, push manual — BUG-007
```

**Resultado**: PostgreSQL Cluster criado pelo operador CNPG, acessivel via:
```bash
kubectl port-forward svc/anhumas-db-rw 5432:5432
psql -h localhost -U app -d anhumas  # Conexao aceita
```

#### 2d. Kestra (workflow engine)

```bash
# Criar Chart.yaml, values.yaml, templates/deployment.yaml
# PRIMEIRA TENTATIVA: datasource "postgres" com URL H2 → FALHA
#   Erro: SQL migration com sintaxe PostgreSQL (DO $ blocks) executada no H2
# SEGUNDA TENTATIVA: sem config → FALHA
#   Erro: "missing required properties"
# TERCEIRA TENTATIVA: config minima (h2 queue, h2 repo, local storage) → SUCESSO
# Commitar cada tentativa, push manual — BUG-007 (3 vezes)
```

**Resultado**: Kestra acessivel via `kubectl port-forward svc/kestra 8081:8080` → HTTP 200

#### Resultado da Fase 2

| Servico | Port | Status |
|---------|------|--------|
| nexus-core | localhost:8000 | HTTP 200 |
| Kestra | localhost:8081 | HTTP 200 |
| ArgoCD | localhost:8085 | HTTP 200 |
| MinIO | localhost:9001 | HTTP 200 |
| PostgreSQL | localhost:5432 | Conexao aceita |
| Prometheus | localhost:9090 | HTTP 200 |

**Total de pushes manuais na Fase 2**: 6 (cada commit exigiu port-forward + push manual)

### Resumo de Intervencoes

| Fase | Intervencoes Manuais | Tipo |
|------|---------------------|------|
| Fase 0 (scaffold) | 5 edicoes de arquivo | Pre-cluster |
| Fase 1 (stack infra) | 7 operacoes kubectl | Pos-cluster |
| Fase 2 (workloads) | 6 pushes manuais + 1 server-side apply + 3 iteracoes Kestra | Pos-cluster |
| **Total** | **~22 intervencoes** | |

Cada push manual seguiu o mesmo ritual:
```bash
# Recriar port-forward (cai periodicamente)
kubectl port-forward -n yby-system svc/git-server 19418:9418 &
sleep 2
# Push para o git-server
git push git://localhost:19418/repo.git test/yby-dev-infra:main
# Aguardar ArgoCD detectar (~30s) ou forcar refresh
```

---

## Problemas Encontrados (Bugs)

### BUG-001: `--env dev` gera `global.environment: local` incompativel com schema

**Severidade**: Media
**Comando**: `yby init --env dev --topology standard ...`
**Comportamento**:
- yby emite warning: `Ambiente inicial 'dev' nao existe na topologia 'standard'. Ajustando para 'local'.`
- Gera `global.environment: local` em `cluster-values.yaml`
- Porem o schema `values.schema.json` do chart `cluster-config` so aceita: `["dev", "staging", "prod"]`
- Resultado: `yby validate` / `helm lint` FALHA com erro de schema

**Esperado**: Se yby ajusta para `local`, o schema deveria aceitar `local`. Ou yby deveria manter `dev` em vez de ajustar.

**Workaround**: Editar manualmente `cluster-values.yaml` — trocar `environment: local` para `environment: dev`.

**Status**: Confirmado como BUG REAL

---

### BUG-002: `discovery.organization` gerado como null — schema exige string

**Severidade**: Baixa
**Comando**: `yby init` (sem habilitar GitHub Discovery)
**Comportamento**:
- `cluster-values.yaml` gera `discovery.organization:` (valor vazio/null)
- Schema `values.schema.json` exige tipo `string`, rejeita `null`
- `helm lint` falha: `discovery.organization: Invalid type. Expected: string, given: null`

**Esperado**: Gerar `discovery.organization: ""` (string vazia) quando discovery nao habilitado.

**Workaround**: Editar manualmente `cluster-values.yaml` — trocar `organization:` para `organization: ""`.

**Status**: Confirmado como BUG REAL

---

### BUG-003: Template `external-apps.yaml` causa nil pointer no helm lint

**Severidade**: Media
**Arquivo**: `infra/charts/cluster-config/templates/applicationset/external-apps.yaml`
**Comportamento**:
- Template usa ApplicationSet com go template vars (`{{ .path.basename }}`)
- `helm lint` falha: `nil pointer evaluating interface {}.basename`
- Mesmo com `applicationset.enabled: false`, o template e renderizado

**Nota**: Subsumido pelo BUG-014 (que e a causa raiz).

**Status**: Confirmado como BUG REAL

---

### BUG-004: `values-local.yaml` referenciado mas NAO gerado

**Severidade**: Alta
**Comando**: `yby init --env dev ...`
**Comportamento**:
- `environments.yaml` referencia `config/values-local.yaml` para o ambiente `local`
- Porem `yby init` NAO gera esse arquivo — apenas gera `config/cluster-values.yaml`
- Resultado: `yby dev` ou qualquer operacao que tente ler `values-local.yaml` pode falhar

**Esperado**: `yby init` deveria gerar `values-local.yaml` com valores default para dev local.

**Workaround**: Criar `values-local.yaml` manualmente.

**Status**: Confirmado como BUG REAL

---

### BUG-005: Comando `yby destroy` nao existe

**Severidade**: Media
**Documentacao**: Varios exemplos mencionam `yby destroy --all` para limpar clusters.
**Comportamento**: `yby destroy --all` retorna: `Error: unknown command "destroy" for "yby"`

**Esperado**: Comando deveria existir conforme documentado, ou documentacao deveria indicar o correto.

**Workaround**: Usar `k3d cluster delete <nome>` diretamente.

**Status**: Confirmado como BUG REAL

---

### BUG-006: `yby dev` nao aplica AppProject antes do root-app — loop de sync infinito

**Severidade**: CRITICA
**Comando**: `yby dev`
**Comportamento**:
- `yby dev` aplica `root-app.yaml` que referencia `spec.project: anhumas-nexus`
- O AppProject `anhumas-nexus` NAO e aplicado pelo `yby dev`
- ArgoCD rejeita: `application references project anhumas-nexus which does not exist`
- ArgoCD entra em loop de retry — **NENHUMA Application e sincronizada**

**Impacto**: **Bloqueia o pipeline GitOps inteiro.**

**Esperado**: `yby dev` deveria aplicar `infra/manifests/projects/yby-project.yaml` ANTES de `root-app.yaml`.

**Workaround**: `kubectl apply -f infra/manifests/projects/yby-project.yaml`

**Status**: CONFIRMADO como BUG REAL (reproduzido no reteste limpo)

---

### BUG-007: `yby dev` sync scheduler falha — git-server fica vazio

**Severidade**: CRITICA
**Comando**: `yby dev`
**Comportamento**:
- `yby dev` cria git-server interno (bare repo com `git daemon`) no namespace `yby-system`
- Inicia sync scheduler (loop a cada 5s) que tenta push do codigo local para o git-server
- **FALHA IMEDIATAMENTE** com `exit status 128` porque usa URL DNS interna ao cluster (`git://git-server.yby-system.svc:80/repo.git`) a partir do HOST
- O HOST nao resolve DNS do cluster → falha em loop
- git-server permanece VAZIO (zero commits)

**Evidencia** (output do `yby dev`):
```
📡 Mirror: git://git-server.yby-system.svc:80/repo.git
❌ Falha no Sync inicial: exit status 128
⚠️ Erro de Sincronizacao: exit status 128
⚠️ Erro de Sincronizacao: exit status 128
(repete a cada 5 segundos indefinidamente)
```

**Impacto fundamental**: Este e o bug mais impactante de TODOS. A proposta central do yby e o pipeline automatico commit→sync→deploy. Com o sync scheduler quebrado, TODA mudanca exige:
1. Criar port-forward manual para o git-server
2. Fazer push manual com protocolo git://
3. O port-forward cai periodicamente e precisa ser recriado

Na Fase 2 (deploy de workloads), foram necessarios **6 pushes manuais**, cada um exigindo recriacao de port-forward.

**Causa raiz**: O sync scheduler usa URL DNS interna (`git-server.yby-system.svc`) que so resolve dentro do cluster. Do host, essa URL nao resolve. Alem disso, usa porta 80 mas o git daemon escuta em 9418.

**Esperado**: O sync deveria funcionar via port-forward, `kubectl exec`, ou outro mecanismo que permita ao host acessar o git-server.

**Workaround** (por push):
```bash
kubectl port-forward -n yby-system svc/git-server 19418:9418 &
sleep 2
git push git://localhost:19418/repo.git HEAD:main
```

**Status**: CONFIRMADO como BUG REAL (reproduzido em ambos ciclos de teste)

---

### BUG-008: Service git-server expoe porta errada (80 em vez de 9418)

**Severidade**: Alta
**Recurso**: `Service/git-server` no namespace `yby-system`
**Comportamento**:
- git-server roda `git daemon` na porta 9418 (protocolo git://)
- Service criado com `port: 80 → targetPort: 9418`
- ArgoCD tenta `git://git-server.yby-system.svc/repo.git` (porta 9418 padrao)
- Service so escuta na 80 → timeout

**Esperado**: Service deveria mapear `port: 9418 → targetPort: 9418`.

**Workaround**: `kubectl patch svc git-server -n yby-system --type merge -p '{"spec":{"ports":[{"port":9418,"targetPort":9418,"protocol":"TCP"}]}}'`

**Status**: CONFIRMADO como BUG REAL

---

### BUG-009: root-app.yaml aponta para GitHub em vez do git-server interno

**Severidade**: CRITICA
**Arquivo**: `infra/manifests/argocd/root-app.yaml`
**Comportamento**:
- `yby init` gera `root-app.yaml` com `repoURL: https://github.com/casheiro/anhumas-nexus.git`
- `yby dev` aplica esse root-app ao cluster
- Em dev local, ArgoCD deveria ler do git-server interno, nao do GitHub
- Se repo GitHub e privado → `authentication required`

**Esperado**: Para `yby dev`, root-app deveria apontar para git-server interno.

**Workaround**: Editar `root-app.yaml` ou patchar pos-cluster.

**Status**: CONFIRMADO como BUG REAL

---

### BUG-010: git-server bare repo inicializa com HEAD em `master` mas push vai para `main`

**Severidade**: Baixa
**Comportamento**:
- git-server inicializa com `git init --bare` → HEAD aponta para `refs/heads/master`
- Projetos modernos usam `main`
- ArgoCD com `targetRevision: HEAD` falha: `Unable to resolve 'HEAD' to a commit SHA`

**Esperado**: `git init --bare` deveria usar `--initial-branch=main`.

**Workaround**: `kubectl exec -n yby-system deployment/git-server -- git -C /git/repo.git symbolic-ref HEAD refs/heads/main`

**Status**: CONFIRMADO como BUG REAL

---

### BUG-011: Template `cluster-config-app.yaml` com path incorreto para monorepo

**Severidade**: CRITICA
**Arquivo**: `infra/charts/bootstrap/templates/cluster-config-app.yaml`
**Comportamento**:
- Template gera `path: charts/cluster-config`
- Em monorepo com `--target-dir infra`, deveria ser `path: infra/charts/cluster-config`
- ArgoCD nao encontra o chart

**Nota**: `root-app.yaml` gera path correto (`infra/charts/bootstrap`) mas os templates Helm internos nao consideram `--target-dir`.

**Workaround**: Editar template manualmente.

**Status**: CONFIRMADO como BUG REAL

---

### BUG-012: Job `webhook-info` referencia ServiceAccount em namespace errado

**Severidade**: Alta (bloqueia sync do root-app inteiro)
**Arquivo**: `infra/charts/bootstrap/templates/jobs/webhook-info.yaml`
**Comportamento**:
- Job criado em namespace `argo-events`
- Referencia `serviceAccountName: argo-workflow-executor`
- SA so existe em `argocd` (criada pelo bootstrap), nao em `argo-events`
- Resultado: **bloqueia sync completo do root-app** (Job nunca fica Healthy)

**Workaround**: Criar SA manualmente no namespace correto.

**Status**: CONFIRMADO como BUG REAL

---

### BUG-013: `yby dev --help` menciona `yby destroy` mas comando nao existe

**Severidade**: Baixa
**Status**: Confirmado como BUG REAL (consistente com BUG-005)

---

### BUG-014: Template `external-apps.yaml` renderizado sem guard condicional

**Severidade**: CRITICA (bloqueia sync do cluster-config inteiro)
**Arquivo**: `infra/charts/cluster-config/templates/applicationset/external-apps.yaml`
**Comportamento**:
- Template cria ApplicationSet com `goTemplate: true` + generator git
- NAO tem guard `{{- if .Values.applicationset.enabled }}`
- Renderiza mesmo com `applicationset.enabled: false`
- Generator referencia GitHub hardcoded (nao acessivel do cluster dev)
- Path sem prefixo `infra/` (nao existe)
- `.path` fica nil → nil pointer crash
- **TODO o cluster-config falha o sync**

**Multiplos problemas**:
1. Sem guard condicional (o mais critico)
2. URL GitHub hardcoded em vez de `{{ .Values.repository.url }}`
3. Branch hardcoded em vez de `{{ .Values.repository.branch }}`
4. Path sem prefixo `infra/` (mesmo padrao do BUG-011)

**Workaround**: Adicionar guard `{{- if .Values.applicationset.enabled }}` / `{{- end }}`.

**Status**: CONFIRMADO como BUG REAL

---

### BUG-015: Job `webhook-info` nao tem RBAC para listar nodes

**Severidade**: Media
**Comportamento**:
- Mesmo apos criar SA (fix BUG-012), Job falha: `nodes is forbidden`
- Script executa `kubectl get nodes` sem permissao
- 6+ pods em Error

**Esperado**: Chart deveria incluir ClusterRole/ClusterRoleBinding.

**Workaround**: Criar RBAC manualmente (ClusterRole + ClusterRoleBinding).

**Status**: CONFIRMADO como BUG REAL

---

### BUG-016: SealedSecrets nao descriptaveis em cluster recriado

**Severidade**: Media
**Comportamento**:
- Templates geram SealedSecret resources (minio-creds, github-webhook-secret)
- Ao recriar cluster, chaves sao regeneradas
- SealedSecrets antigos nao descriptam → pods presos em ContainerCreating

**Nota**: Comportamento by design de SealedSecrets, porem para dev local onde clusters sao frequentemente recriados, e fonte constante de friction.

**Workaround**: Criar secrets manualmente (`kubectl create secret generic ...`).

**Status**: CONFIRMADO como BUG REAL

---

### BUG-017: Template `cluster-issuer.yaml` sem guard condicional

**Severidade**: Baixa
**Comportamento**:
- Cria ClusterIssuer Let's Encrypt SEMPRE, sem guard
- Em dev local, `admin@yby.local` nao e valido para ACME
- ClusterIssuer fica Degraded (nao bloqueante)

**Status**: Confirmado como BUG REAL

---

### BUG-018: CRDs do CNPG operator excedem limite de client-side apply (262KB)

**Severidade**: CRITICA (bloqueia deploy do operador de banco de dados)
**Componente**: `data-operator` (ArgoCD Application para CloudNativePG)
**Comportamento**:
- O chart `cloudnative-pg` (operador CNPG) inclui CRDs grandes (ex: `poolers.postgresql.cnpg.io`)
- O CRD `poolers` excede 262144 bytes (limite de `metadata.annotations` do kubectl client-side apply)
- ArgoCD por padrao usa client-side apply, resultando em:
  ```
  metadata.annotations: Too long: may not be more than 262144 bytes
  ```
- A Application `data-operator` fica em Sync Failed permanentemente
- Sem o operador CNPG, nao e possivel criar clusters PostgreSQL via CRD

**Esperado**: O template da Application `data-operator` no bootstrap deveria incluir `ServerSideApply=true` nos syncOptions por padrao, ja que e um padrao conhecido de operadores Kubernetes com CRDs grandes.

**Intervencao manual** (2 etapas):
```bash
# 1. Aplicar CRDs manualmente com server-side apply
helm template cnpg infra/charts/data/cloudnativepg-operator/charts/cloudnative-pg-*.tgz \
  | kubectl apply --server-side --force-conflicts -f -

# 2. Adicionar ServerSideApply no template do bootstrap (para futuros syncs)
# Editado infra/charts/bootstrap/templates/data-operator.yaml:
#   syncOptions:
#     - CreateNamespace=true
#     - ServerSideApply=true
```

**Nota**: O fix no template do bootstrap foi commitado e pushado, mas NAO propagou para o ArgoCD porque o root-app esta preso em revisao antiga (BUG-019).

**Status**: CONFIRMADO como BUG REAL

---

### BUG-019: root-app preso em revisao antiga — nao detecta novos commits no git-server

**Severidade**: Alta
**Comportamento**:
- Apos multiplos pushes para o git-server, o root-app (ArgoCD) permanece apontando para uma revisao antiga
- Mesmo com hard refresh (`argocd.argoproj.io/refresh=hard`), o root-app nao detecta commits mais recentes
- Resultado: mudancas nos templates do bootstrap (como adicionar ServerSideApply no data-operator) NAO propagam para as child Applications
- O ArgoCD viu a revisao inicial e nao reavalia o git-server para detectar novos commits

**Evidencia**:
```bash
# root-app sincronizado na revisao 5e00550 (antiga)
# git-server tem commits ate 1632739 (6 commits a frente)
# Child applications nao refletem mudancas dos commits posteriores
```

**Impacto**: Invalida parcialmente o pipeline GitOps — mudancas posteriores ao bootstrap inicial podem nao ser detectadas.

**Esperado**: ArgoCD deveria detectar novos commits no git-server e re-sincronizar o root-app, propagando mudancas para child Applications.

**Status**: CONFIRMADO como BUG REAL (possivelmente relacionado ao BUG-008 — porta errada do git-server impedindo polling)

---

### BUG-020: Sync scheduler do `yby dev` continua executando apos Ctrl+C parcial

**Severidade**: Baixa
**Comportamento**:
- `yby dev` precisa ser interrompido com Ctrl+C (o sync scheduler falha em loop)
- Em algumas situacoes, o processo filho do sync scheduler continua rodando em background
- Output do watcher continua aparecendo apos Ctrl+C

**Esperado**: Ctrl+C deveria encerrar todos os processos filhos graciosamente.

**Status**: Observado durante testes

---

## Incongruencias: Documentacao vs Realidade

### INC-001: "Sync instantaneo" prometido mas sync falha

**Documentacao** (`.claude/knowledge/yby-cli/02-architecture.md`):
> "Em vez de esperar voce fazer git push para o GitHub, o Yby sincroniza as mudancas locais instantaneamente para o container Git Mirror interno."
> "Tempo de feedback: < 5 segundos"

**Realidade**:
- Sync scheduler EXISTE mas usa URL DNS interna (`git-server.yby-system.svc:80`) a partir do HOST
- HOST nao resolve DNS do cluster → `exit status 128` em loop
- **ZERO syncs automaticos em toda a sessao de teste**
- Cada mudanca exigiu push manual (6 vezes na Fase 2 sozinha)

### INC-002: Documentacao menciona Gitea — realidade e `git daemon` simples

**Documentacao**: Menciona "Gitea" como Git Mirror interno com interface web.

**Realidade**: Container `bitnami/git` rodando `git daemon --bare` (sem interface web, sem autenticacao). NAO e Gitea.

### INC-003: "Zero config" prometido — realidade exige 22+ intervencoes manuais

**Documentacao**: Sugere que `yby dev` sobe tudo pronto para usar.

**Realidade**: Para chegar a um ambiente completo e funcional (infra + workloads acessiveis) apos `yby dev`, foram necessarias **22+ intervencoes manuais** divididas em:

| Fase | Intervencoes | Natureza |
|------|-------------|----------|
| Pre-cluster (scaffold) | 5 edicoes de arquivo | Correcao de valores, paths, guards |
| Pos-cluster (infra) | 7 operacoes kubectl | SA, RBAC, patch service, push manual |
| Workloads | 6 pushes manuais + 1 server-side apply | Cada commit exigiu port-forward + push |
| Debug Kestra | 3 iteracoes de config | Trial-and-error na configuracao |

**Nota**: Alguns destes workarounds sao de escopo do yby (BUG-001 a BUG-015 — scaffold e bootstrap). Outros sao inerentes ao uso de charts customizados (Kestra config). Porem o **push manual** e a **necessidade de port-forward** para cada commit sao universais e quebram completamente a experiencia prometida.

### INC-004: `yby dev` nao gerencia workloads customizados

**Documentacao**: A arquitetura de "App of Apps" do ArgoCD deveria detectar e deployar automaticamente qualquer chart adicionado em `infra/charts/`.

**Realidade**: O pipeline funciona (ArgoCD detecta e faz deploy), MAS o ciclo de desenvolvimento exige:
1. Criar charts manualmente (o yby nao oferece `yby chart create` ou scaffold para novos charts)
2. Se o chart tem dependencias externas, rodar `helm dependency build` manualmente
3. Commitar as mudancas
4. Push manual para o git-server (porque o sync automatico nao funciona — BUG-007)
5. Se o chart usa CRDs grandes, aplicar com ServerSideApply manualmente (BUG-018)
6. Aguardar ArgoCD detectar (ou forcar refresh)

O yby deveria abstrair passos 2, 4, 5 e 6 automaticamente.

---

## Achados Positivos

### OK-001: root-app.yaml com path correto para monorepo

`root-app.yaml` gera `spec.source.path: infra/charts/bootstrap` corretamente com `--target-dir infra`.

### OK-002: Git URL correta

`repoURL` em `root-app.yaml` e `cluster-values.yaml` vieram corretos conforme flag `--git-repo`.

### OK-003: MinIO habilitado corretamente

`storage.minio.enabled: true` em `cluster-values.yaml` conforme flag `--enable-minio`.

### OK-004: AI-assisted scaffold funcional

Flag `--description` ativou motor Gemini que gerou documentacao contextualizada em `.synapstor/`.

### OK-005: Topologia standard funcional

Topologia `standard` gerou charts para ArgoCD, Argo Workflows, Argo Events, Sealed Secrets conforme esperado. Estrutura `bootstrap → cluster-config → system` correta.

### OK-006: `yby access` funciona perfeitamente

Comando `yby access` detecta contexto, cria port-forwards (ArgoCD, MinIO, Prometheus), inicia Grafana via Docker, fornece credenciais. Funcionou no primeiro uso sem problemas.

### OK-007: Arquitetura App of Apps solida

Apos todos os workarounds, o sistema convergiu para estado funcional. Isso confirma que a **ARQUITETURA** do yby e solida — os problemas sao de **IMPLEMENTACAO** (portas, paths, guards, RBAC, sync).

---

## Sugestoes de Melhoria

### SUG-001: Gerar values por ambiente no init

`yby init --env dev` deveria gerar automaticamente `values-local.yaml` com valores sensatos para dev local (replicas=1, sem TLS, recursos minimos).

### SUG-002: Adicionar comando destroy/cleanup

Adicionar `yby destroy` ou `yby cleanup` conforme documentado. Hoje o usuario precisa saber usar `k3d cluster delete`.

### SUG-003: Guard condicional em templates opcionais

Templates de features opcionais (ApplicationSet, ClusterIssuer, KEDA, Kepler) deveriam ter guards `{{- if .Values.<feature>.enabled }}` para nao renderizar quando desabilitados.

### SUG-004: Validar schema antes de gerar (yby init)

`yby init` poderia validar que os valores gerados sao compativeis com os schemas dos proprios charts. Evitaria cenario onde `yby init` gera valores que `yby validate` rejeita.

### SUG-005: Pipeline de sync FUNCIONAL (prioridade maxima)

Este e o problema mais impactante. O sync scheduler precisa funcionar para a proposta do yby se concretizar. Opcoes:

1. **Port-forward automatico**: `yby dev` cria port-forward persistente para o git-server e faz push via `localhost`
2. **kubectl exec**: `yby dev` faz `kubectl exec` no pod do git-server para receber o push
3. **Sidecar**: Rodar um sidecar no git-server que faz pull do host via volume mount compartilhado
4. **k3d volume mount**: Usar `k3d cluster create ... -v /path/to/repo:/git:latest@server:0` para montar o repo local diretamente no cluster

Sem sync funcional, o yby e apenas um scaffold — nao um ambiente de dev.

### SUG-006: Considerar `--target-dir` nos templates internos do bootstrap

Paths nos templates do bootstrap (`cluster-config-app.yaml`, `external-apps.yaml`) deveriam incluir o prefixo do `--target-dir`. `root-app.yaml` ja faz isso corretamente — os templates Helm deveriam ser consistentes.

### SUG-007: Modo dev para SealedSecrets

Para dev local, oferecer opcao de gerar secrets simples (nao sealed) ou re-seal com novas chaves. Exemplo: `yby dev --plain-secrets` ou fallback automatico quando SealedSecrets nao descriptam.

### SUG-008: Sync scheduler deveria usar port-forward

O sync scheduler deveria usar mecanismo que funcione do host (port-forward, kubectl exec, volume mount) em vez de DNS interno do cluster.

### SUG-009: ServerSideApply como default para operadores CRD

Qualquer Application que deploya operadores Kubernetes (CNPG, cert-manager, etc.) deveria ter `ServerSideApply=true` nos syncOptions por padrao. CRDs grandes sao comuns e o limite de 262KB e frequentemente ultrapassado.

### SUG-010: Comando `yby chart create` para scaffold de novos charts

Ao criar novos charts customizados (nexus-core, Kestra, databases), o usuario precisa criar manualmente `Chart.yaml`, `values.yaml` e templates. O yby poderia oferecer:
```bash
yby chart create workflow --type deployment --port 8080 --image kestra/kestra
```

Isso geraria o scaffold basico do chart com Deployment + Service, ja integrado na estrutura do bootstrap.

### SUG-011: `yby dev` deveria executar sequencia completa de bootstrap

`yby dev` deveria executar automaticamente toda a sequencia para ambiente funcional:
1. Criar cluster k3d
2. Instalar ArgoCD
3. **Aplicar AppProject** (BUG-006)
4. **Configurar git-server** (porta correta, HEAD correto — BUG-008, BUG-010)
5. **Fazer push** do codigo local (BUG-007)
6. **Ajustar root-app** para git-server (BUG-009)
7. **Criar RBAC** para jobs (BUG-015)
8. Aplicar root-app
9. Aguardar sync healthy

Hoje passos 3-7 sao manuais.

### SUG-012: `yby dev` deveria aplicar CRDs com ServerSideApply automaticamente

Ao detectar que uma Application falhou com `annotations too long`, o `yby dev` (ou o sync scheduler) poderia automaticamente reaplicar com ServerSideApply. Alternativamente, qualquer Application com charts de operadores poderia ter ServerSideApply habilitado automaticamente.

---

## Analise de Gap: Promessa vs Realidade

### Fluxo Prometido

```
yby init → yby dev → commit → auto-sync → auto-deploy → acessivel
    5 min      5 min     instant    <5s        ~30s        port-forward
```

**Tempo total esperado**: ~10 minutos para ambiente completo funcionando.

### Fluxo Real

```
yby init → FIX scaffold (5 edits) → yby dev → FIX cluster (7 ops) → push manual → aguardar → FIX CRDs → push manual → push manual → ...
    5 min        10 min                5 min       15 min             2 min         30s        5 min        2 min         2 min
```

**Tempo total real**: ~45 minutos de intervencoes manuais + debugging.

### Gap por Area

| Area | Prometido | Real | Gap |
|------|-----------|------|-----|
| **Scaffold** | Gera tudo pronto | Gera com 5 erros de valores/paths | 5 fixes manuais |
| **Bootstrap** | Aplica tudo automatico | Falta AppProject, RBAC | 2 fixes manuais |
| **Sync** | Automatico em <5s | NAO FUNCIONA | Push manual a cada commit |
| **Git-server** | Pronto para uso | Porta errada, HEAD errado | 2 fixes manuais |
| **SealedSecrets** | Gerenciados | Nao descriptaveis apos recrear cluster | Secret manual |
| **CRDs grandes** | Transparente | Falha com 262KB limit | Server-side apply manual |
| **Workloads** | Deploy via commit | Deploy exige push manual + port-forward | Repetitivo |

### Conclusao

**A arquitetura e solida** (App of Apps, ArgoCD, GitOps). Os problemas sao todos de **implementacao**:
- URLs incorretas (DNS interno vs host)
- Portas erradas (80 vs 9418)
- Paths sem `--target-dir` prefix
- Guards condicionais faltando
- RBAC e SA nao criados
- SyncOptions incompletas

Com os fixes acima, o yby seria uma ferramenta excelente. Sem eles, e um scaffold que exige conhecimento profundo de Kubernetes, ArgoCD e Helm para funcionar.

---

## Status Final

### Bugs por Severidade

| Severidade | Quantidade | IDs |
|-----------|-----------|-----|
| CRITICA | 6 | BUG-006, BUG-007, BUG-009, BUG-011, BUG-014, BUG-018 |
| Alta | 4 | BUG-004, BUG-008, BUG-012, BUG-019 |
| Media | 6 | BUG-001, BUG-003, BUG-005, BUG-015, BUG-016, BUG-017 |
| Baixa | 4 | BUG-002, BUG-010, BUG-013, BUG-020 |

### Tabela Completa

| Bug | Severidade | Confirmado | Workaround | Categoria |
|-----|-----------|------------|------------|-----------|
| BUG-001 | Media | REAL | Pre-cluster | Scaffold |
| BUG-002 | Baixa | REAL | Pre-cluster | Scaffold |
| BUG-003 | Media | REAL | Via BUG-014 | Template |
| BUG-004 | Alta | REAL | Manual | Scaffold |
| BUG-005 | Media | REAL | k3d direto | CLI |
| BUG-006 | CRITICA | REAL | Pos-cluster | Bootstrap |
| BUG-007 | CRITICA | REAL | Pos-cluster | **Sync** |
| BUG-008 | Alta | REAL | Pos-cluster | Git-server |
| BUG-009 | CRITICA | REAL | Pre-cluster | Bootstrap |
| BUG-010 | Baixa | REAL | Pos-cluster | Git-server |
| BUG-011 | CRITICA | REAL | Pre-cluster | Template |
| BUG-012 | Alta | REAL | Pos-cluster | RBAC |
| BUG-013 | Baixa | REAL | N/A | CLI |
| BUG-014 | CRITICA | REAL | Pre-cluster | Template |
| BUG-015 | Media | REAL | Pos-cluster | RBAC |
| BUG-016 | Media | REAL | Pos-cluster | Secrets |
| BUG-017 | Baixa | REAL | Nao bloqueante | Template |
| BUG-018 | CRITICA | REAL | Server-side apply | CRD |
| BUG-019 | Alta | REAL | Hard refresh | Sync |
| BUG-020 | Baixa | REAL | Kill manual | CLI |

### Resultado Final

**Sistema funcional**: SIM (apos 22+ intervencoes manuais)

| Servico | Acessivel | Metodo |
|---------|-----------|--------|
| ArgoCD UI | Sim | localhost:8085 via `yby access` |
| MinIO Console | Sim | localhost:9001 via `yby access` |
| Prometheus | Sim | localhost:9090 via `yby access` |
| nexus-core | Sim | localhost:8000 via port-forward |
| Kestra | Sim | localhost:8081 via port-forward |
| PostgreSQL | Sim | localhost:5432 via port-forward |

**Workarounds necessarios**: 22+ (5 pre-cluster + 7 pos-cluster + 6 pushes manuais + 1 server-side apply + 3 iteracoes config)

---

## Workaround Script Completo

### Pre-cluster (antes de `yby dev`)

```bash
# BUG-001: Corrigir environment
sed -i 's/environment: local/environment: dev/' infra/config/cluster-values.yaml

# BUG-002: Corrigir organization
sed -i 's/organization:$/organization: ""/' infra/config/cluster-values.yaml

# BUG-009: Corrigir repoURL para git-server
sed -i 's|repoURL: https://github.com/.*|repoURL: git://git-server.yby-system.svc.cluster.local/repo.git|' \
  infra/manifests/argocd/root-app.yaml

# BUG-011: Corrigir path do cluster-config
sed -i 's|path: charts/cluster-config|path: infra/charts/cluster-config|' \
  infra/charts/bootstrap/templates/cluster-config-app.yaml

# BUG-014: Adicionar guard no external-apps.yaml
sed -i '1i {{- if .Values.applicationset.enabled }}' \
  infra/charts/cluster-config/templates/applicationset/external-apps.yaml
echo '{{- end }}' >> \
  infra/charts/cluster-config/templates/applicationset/external-apps.yaml

# Commitar antes de yby dev
git add -A && git commit -m "fix: workarounds pre-cluster para yby dev"
```

### Executar yby dev

```bash
yby dev
# Ctrl+C apos "Bootstrap concluido" (sync scheduler falha em loop)
```

### Pos-cluster (apos `yby dev`)

```bash
# BUG-008: Corrigir porta do git-server
kubectl patch svc git-server -n yby-system --type merge \
  -p '{"spec":{"ports":[{"port":9418,"targetPort":9418,"protocol":"TCP"}]}}'

# BUG-010: Corrigir HEAD do bare repo
kubectl exec -n yby-system deployment/git-server -- \
  git -C /git/repo.git symbolic-ref HEAD refs/heads/main

# BUG-006: Aplicar AppProject
kubectl apply -f infra/manifests/projects/yby-project.yaml

# BUG-007: Push codigo para git-server
kubectl port-forward -n yby-system svc/git-server 19418:9418 &
sleep 2
git push git://localhost:19418/repo.git HEAD:main

# BUG-012 + BUG-015: Criar SA e RBAC para webhook-info
kubectl create serviceaccount argo-workflow-executor -n argo-events
cat <<'RBAC' | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: webhook-info-reader
rules:
- apiGroups: [""]
  resources: ["nodes", "services", "secrets"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: webhook-info-reader-binding
subjects:
- kind: ServiceAccount
  name: argo-workflow-executor
  namespace: argo-events
roleRef:
  kind: ClusterRole
  name: webhook-info-reader
  apiGroup: rbac.authorization.k8s.io
RBAC
kubectl delete job webhook-info -n argo-events 2>/dev/null

# BUG-016: Criar secret MinIO
kubectl create secret generic minio-creds -n default \
  --from-literal=rootUser=admin --from-literal=rootPassword=minioadmin123

# Forcar refresh
kubectl annotate application root-app -n argocd argocd.argoproj.io/refresh=hard --overwrite

# Aguardar convergencia
sleep 60
kubectl get applications -n argocd

# Abrir acesso
yby access
```

### Para cada novo chart/commit (repetir conforme necessario)

```bash
# Recriar port-forward (cai periodicamente)
kubectl port-forward -n yby-system svc/git-server 19418:9418 &
sleep 2

# Push da branch atual como main no git-server
git push git://localhost:19418/repo.git HEAD:main

# Forcar refresh se ArgoCD nao detectar
kubectl annotate application root-app -n argocd argocd.argoproj.io/refresh=hard --overwrite
```

---

*Ultima atualizacao: 2026-02-14 (consolidacao completa — 2 ciclos de teste + deploy de workloads)*
