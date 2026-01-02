# Plano de Correção – Yby CLI (Dev Local, Multi‑Ambiente, Integração em Subdiretórios)

> **STATUS: CONCLUÍDO ✅**
> Todas as fases foram implementadas e validadas via testes BDD.
> Veja o relatório final em [.trae/reports/bdd_validation.md](file:///home/neto/projects/casheiro-org/yby-cli/.trae/reports/bdd_validation.md).

Este documento consolida uma análise técnica e um plano de correção para os problemas observados ao executar `yby dev` no repositório anhumas-nexus com infraestrutura em um subdiretório (`infra/`). O foco é coerência funcional em ambiente local, robustez multi‑ambiente, melhor suporte a caminhos personalizados, e consistência com a documentação.

---

## Visão Geral dos Problemas

- Exigência de GITHUB_REPO no `yby dev` local, apesar de rodar cluster e GitOps local.
- Geração da pasta `.github` dentro de `infra/` quando o usuário escolhe inicializar “dentro da pasta infra”, quebrando pipelines do GitHub Actions.
- Manifesto `.yby/environments.yaml` incoerente: `current` referindo um ambiente não listado e apontando para arquivos de values inexistentes.
- Persistência de referências a `.env` em mensagens e no fluxo de `bootstrap vps`, em conflito com a migração para `.yby/environments.yaml`.
- Integração em subdiretório `infra/` incompleta: comandos dependem do CWD e não do “infra root” detectado; o Mirror Local não é integrado ao ArgoCD root‑app.

---

## Mapa Mental dos Problemas e Impactos

- Dev Local exige upstream (GITHUB_REPO)
  - Impacta: bootstrap do cluster, aplicação root do ArgoCD.
  - Arquivos: [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go), [dev.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go), [root-app.yaml.tmpl](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/manifests/argocd/root-app.yaml.tmpl), [mirror.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/mirror/mirror.go).
  - Efeito: bloqueio do fluxo local sem token/URL; fricção DX; contradição com expectativa “offline/híbrido”.
- `.github` dentro de `infra/`
  - Impacta: execução de GitHub Actions.
  - Arquivos: [engine.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/scaffold/engine.go), [Getting-Started.md](file:///home/neto/projects/casheiro-org/yby-cli/docs/wiki/Getting-Started.md).
  - Efeito: pipelines não disparando; documentação contradiz comportamento.
- `environments.yaml` incoerente
  - Impacta: comandos `yby env`, `yby dev`; validações; arquivos `config/values-*.yaml`.
  - Arquivos: [environments.yaml.tmpl](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/.yby/environments.yaml.tmpl), [context.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/context/context.go), [init.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/init.go), [dev.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go).
  - Efeito: “current” pode apontar para env inexistente; valores faltando; erro ao usar `dev`.
- `.env` ainda referenciado
  - Impacta: `bootstrap vps` e mensagens do `bootstrap cluster`.
  - Arquivos: [bootstrap_vps.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_vps.go), [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go).
  - Efeito: ambiguidade de fonte de configuração; fricção DX; divergência das docs.
- Subdiretório `infra/` parcialmente suportado
  - Impacta: execução dos comandos a partir da raiz; paths do ArgoCD; sincronização do Mirror.
  - Arquivos: [infra_helpers.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/infra_helpers.go), [dev.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go), [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go), [mirror.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/mirror/mirror.go).
  - Efeito: necessidade de “cd infra”; risco de paths quebrados; inconsistências multi‑ambiente.

---

## Narrativa DX (Experiência do Desenvolvedor)

- Ao rodar `yby dev` para subir um cluster local, o desenvolvedor espera que tudo seja auto‑contido e não dependa de GitHub/Tokens. Em vez disso, há exigência de `GITHUB_REPO`.
- O wizard permite escolher instalar “dentro de `infra/`”, mas isto coloca `.github` dentro de `infra/`, quebrando a execução de workflows, gerando surpresa e retrabalho.
- O arquivo `.yby/environments.yaml` vem com `current: dev`, porém a lista contém apenas `prod`. O próximo comando de `dev` falha por inconsistência.
- Mensagens pedem para “Definir no `.env`”, apesar da migração para manifesto `.yby`. O desenvolvedor não sabe qual fonte é “a correta” para a ferramenta.
- Em monorepo, o dev precisa adivinhar onde rodar os comandos (raiz ou `infra/`), pois o gerenciador de contexto usa CWD. Isso aumenta fricção e causa erros de caminho.

---

## Problema 1 — Exigência de GITHUB_REPO no Dev Local

**Ponto Crítico**
- Validação rígida de `GITHUB_REPO` no bootstrap do cluster, mesmo no fluxo “dev local”.
  - Código: [bootstrap_cluster.go:L160-L170](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go#L160-L170).
- `yby dev` inicializa Mirror Local, mas o ArgoCD root‑app continua apontando para o upstream:
  - Código: [dev.go:L113-L131](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go#L113-L131) chamando bootstrap, e [root-app.yaml.tmpl:L15-L23](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/manifests/argocd/root-app.yaml.tmpl#L15-L23).

**Relacionamentos**
- `dev` → `bootstrap_cluster` → `checkEnvVars` (exige repo) → `ensureTemplateAssets` (regrava root‑app com repoURL).
- Mirror Local ([mirror.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/mirror/mirror.go)) não é usado como source do ArgoCD; apenas sincroniza snapshot em pod.

### Refinamento (BDD)

Visão de Negócios:
- Em ambiente local de desenvolvimento, o produto deve funcionar sem dependências externas (GitHub), usando um mirror interno ou caminho local.

Visão Técnica:
- Quando `YBY_ENV=local`, configurar `repoURL` do ArgoCD para o serviço de Git interno (ex.: `git-server` Kubernetes), ou para um path local suportado pelo ArgoCD.
- Tornar opcional `GITHUB_REPO` e `GITHUB_TOKEN` em dev local; usar “fallback” para mirror.

História de Usuário:
- Como desenvolvedor, quero rodar `yby dev` sem definir `GITHUB_REPO/TOKEN`, para testar a infra localmente com ArgoCD apontando para um repositório interno/mirror.

Critérios de Aceite:
- `yby dev` não falha sem `GITHUB_REPO` quando `YBY_ENV=local`.
- O `root-app` aponta para o mirror interno.
- Se o mirror não estiver pronto, o comando informa e tenta reconectar, sem travar o dev.

Cenários (Gherkin) com variações:

```gherkin
Feature: Dev local sem dependência de GitHub
  Background:
    Given o ambiente ativo é "local"

  # Sucesso
  Scenario: ArgoCD usa o mirror interno em dev
    Given o git-server interno está pronto
    When eu executo "yby dev"
    Then a aplicação "root-app" deve usar o repoURL do git-server interno
    And o bootstrap do cluster conclui sem exigir GITHUB_REPO
    And o status mostra ArgoCD sincronizando com sucesso

  # Falha
  Scenario: Mirror interno indisponível
    Given o git-server interno não está pronto
    When eu executo "yby dev"
    Then devo ver uma mensagem de aviso sobre mirror indisponível
    And o comando não deve encerrar com erro fatal
    And deve reintentar o provisionamento do mirror até um timeout configurável

  # Borda
  Scenario: Fallback para repositório público quando explicitamente fornecido
    Given o ambiente é "local" e o usuário fornece GITHUB_REPO público
    When eu executo "yby dev"
    Then a aplicação "root-app" pode usar o GITHUB_REPO fornecido
    And não é exigido GITHUB_TOKEN
```

Exemplos reais:
- Sucesso: `yby dev` em máquina offline; cria `git-server` e root-app aponta para `git://git-server.yby-system.svc/repo.git`.
- Falha: `git-server` demora a subir; CLI avisa e segue tentando por 60s, depois continua sem abortar.
- Borda: Usuário define `GITHUB_REPO=https://github.com/org/repo` público; ArgoCD sincroniza sem token.

---

## Problema 2 — `.github` dentro de `infra/`

**Ponto Crítico**
- Engine escreve todos os assets no `targetDir`, incluindo `.github`, que deve estar sempre na raiz do repositório.
  - Código: [engine.go:L54-L63](file:///home/neto/projects/casheiro-org/yby-cli/pkg/scaffold/engine.go#L54-L63).
- Docs sugerem `.github/` sempre na raiz.
  - Código: [Getting-Started.md:L57-L66](file:///home/neto/projects/casheiro-org/yby-cli/docs/wiki/Getting-Started.md#L57-L66).

**Relacionamentos**
- Wizard (`init`) define `TargetDir`; engine aplica tudo relativo a este dir.
- Workflows do GitHub só são detectados na raiz; pipelines não disparam se `.github` estiver em `infra/`.

### Refinamento (BDD)

Visão de Negócios:
- Pipelines CI/CD devem funcionar “out‑of‑the‑box” em qualquer topologia; `.github/` sempre na raiz.

Visão Técnica:
- Engine deve rotear assets “de raiz” (.github, .devcontainer, LICENSE, etc.) para o diretório raiz independentemente do `TargetDir`.

História de Usuário:
- Como desenvolvedor, ao inicializar infra em `infra/`, quero que os workflows do GitHub sejam gerados na raiz do repositório.

Critérios de Aceite:
- Após `yby init` com `TargetDir=infra/`, `.github/workflows/*` existe na raiz.
- Documentação e mensagens são consistentes com este comportamento.

Cenários (Gherkin) com variações:

```gherkin
Feature: Geração de workflows na raiz do repositório
  Background:
    Given estou em um repositório git existente

  # Sucesso
  Scenario: Inicialização em subdiretório com CI na raiz
    When eu executo "yby init" com target-dir "infra"
    Then deve existir ".github/workflows/feature-pipeline.yaml" na raiz
    And a pasta "infra/" contém apenas infra (charts, manifests, .yby)

  # Falha
  Scenario: Permissões de escrita na raiz negadas
    Given o diretório raiz não permite escrita
    When executo "yby init" com target-dir "infra"
    Then devo ver erro claro indicando falha ao criar ".github"
    And nenhuma estrutura parcial é deixada corrompida

  # Borda
  Scenario: Monorepo com múltiplos projetos
    Given um monorepo com subprojeto infra em "platform/infra"
    When executo "yby init" dentro de "platform"
    Then ".github" deve ser criado na raiz do monorepo (topo)
    And os caminhos internos do ArgoCD devem ser ajustados ao prefixo git
```

Exemplos reais:
- Sucesso: monorepo com `apps/` e `infra/`; `.github/workflows` na raiz; `infra/` com charts/manifests.
- Falha: raiz em filesystem somente‑leitura; erro e rollback; sem resíduos.
- Borda: projeto em `platform/` como submódulo; engine resolve raiz via `git rev-parse --show-toplevel`.

---

## Problema 3 — `environments.yaml` incoerente

**Ponto Crítico**
- Template gera `current: {{ .Environment }}` sem garantir que `.Environment` esteja na lista derivada da topologia.
  - Código: [environments.yaml.tmpl:L1-L8](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/.yby/environments.yaml.tmpl#L1-L8).
- Manager exige consistência e falha se `current` não existir:
  - Código: [context.go:L66-L87](file:///home/neto/projects/casheiro-org/yby-cli/pkg/context/context.go#L66-L87).
- Caso observado: `current: dev`, ambientes listados só `prod`, e `values-dev.yaml` ausente.
  - Arquivo: [anhumas-nexus/infra/.yby/environments.yaml](file:///home/neto/projects/casheiro-org/anhumas-nexus/infra/.yby/environments.yaml).

**Relacionamentos**
- `init` calcula lista via topologia: [init.go:L273-L283](file:///home/neto/projects/casheiro-org/yby-cli/cmd/init.go#L273-L283).
- `dev` força `local` e espera `local` existir: [dev.go:L36-L64](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go#L36-L64).

### Refinamento (BDD)

Visão de Negócios:
- O manifesto de ambientes deve ser sempre coerente, evitando erros no primeiro uso.

Visão Técnica:
- Validação e ajuste no scaffold: se `--env` não pertencer à topologia, ou incluir automaticamente ou alterar `current` para um ambiente válido.
- Garantir geração dos `config/values-*.yaml` para todos ambientes listados e para o `current`.

História de Usuário:
- Como desenvolvedor, quero que o arquivo `.yby/environments.yaml` seja consistente e funcional sem edições manuais pós‑init.

Critérios de Aceite:
- `current` sempre pertence a `environments`.
- Todos `values-<env>.yaml` existem após o init.
- `yby dev` não falha por falta de `local` quando topologia inclui dev/staging/prod.

Cenários (Gherkin) com variações:

```gherkin
Feature: Coerência do manifesto de ambientes
  # Sucesso
  Scenario: Env inicial pertence à topologia
    Given topologia "complete" inclui "local, dev, staging, prod"
    When executo "yby init" com "--env dev"
    Then ".yby/environments.yaml" deve ter "current: dev"
    And "dev:" listado em "environments"
    And "config/values-dev.yaml" gerado

  # Falha
  Scenario: Env inicial fora da topologia
    Given topologia "single" (apenas "prod")
    When executo "yby init" com "--env dev"
    Then o CLI deve ajustar "current" para "prod" com aviso explícito
    And gerar "config/values-prod.yaml"

  # Borda
  Scenario: Topologia ausente (default)
    Given não informo "--topology"
    When executo "yby init" interativo
    Then o CLI deve garantir que "current" está na lista escolhida
    And não gerar estados incoerentes
```

Exemplos reais:
- Sucesso: `init --topology complete --env dev`; manifesto com 4 ambientes e values de cada.
- Falha: `init --topology single --env dev`; ajuste automático para `prod` com alerta.
- Borda: wizard sem flags; valida coerência antes de escrever arquivo.

---

## Problema 4 — Referências a `.env` persistem

**Ponto Crítico**
- `bootstrap vps` carrega `../.env`:
  - Código: [bootstrap_vps.go:L46-L53](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_vps.go#L46-L53).
- Mensagens sugerem `.env` no cluster bootstrap:
  - Código: [bootstrap_cluster.go:L167-L170](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go#L167-L170).

**Relacionamentos**
- Pacote `context` migrou para manifesto `.yby/environments.yaml`: [context.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/context/context.go).
- Docs de migração apontam para o novo modelo: [Migration-Guide-v2.md](file:///home/neto/projects/casheiro-org/yby-cli/docs/wiki/Migration-Guide-v2.md).

### Refinamento (BDD)

Visão de Negócios:
- Uma única fonte de verdade para ambientes, eliminando ambiguidade e reduzindo erros de configuração.

Visão Técnica:
- Remover dependências obrigatórias de `.env` nos comandos; substituir por manifesto `.yby` e/ou flags.
- Mensagens e docs devem refletir o modelo atual.

História de Usuário:
- Como operador, quero provisionar VPS/cluster sem depender de arquivos `.env`, usando manifesto e flags declarativas.

Critérios de Aceite:
- `bootstrap vps/cluster` não requer `.env`.
- Mensagens não mencionam `.env` como requisito.
- Backward‑compat opcional: se `.yby/environments.yaml` ausente e `.env` presente, aceitar com aviso de deprecação.

Cenários (Gherkin) com variações:

```gherkin
Feature: Unificação da gestão de ambientes (sem .env)
  # Sucesso
  Scenario: Bootstrap sem .env usando manifesto
    Given existe ".yby/environments.yaml" com "prod"
    When executo "yby bootstrap vps --local"
    Then não é lido nenhum ".env"
    And parâmetros são obtidos do manifesto/flags

  # Falha
  Scenario: Manifesto ausente sem flags suficientes
    Given não existe ".yby/environments.yaml"
    And não forneço flags obrigatórias
    When executo "yby bootstrap vps"
    Then devo ver erro claro listando quais flags faltam

  # Borda
  Scenario: Backward-compat encontrado .env
    Given não existe ".yby/environments.yaml"
    And existe ".env"
    When executo "yby bootstrap vps"
    Then o CLI pode usar .env com aviso de deprecação
```

Exemplos reais:
- Sucesso: `bootstrap vps --local` com parâmetros default; sem .env.
- Falha: ambiente sem manifesto e sem flags; CLI lista `--host`, `--user`, etc.
- Borda: projeto legado; CLI usa `.env` mas imprime alerta para migração.

---

## Problema 5 — Integração em Subdiretório `infra/` incompleta

**Ponto Crítico**
- `dev` usa `context.NewManager(wd)` (CWD) em vez do infra root detectado.
  - Código: [dev.go:L46-L55](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go#L46-L55).
- `FindInfraRoot` detecta `.yby` “para cima”; alguns comandos usam `JoinInfra(root, ...)` corretamente, outros não.
  - Código: [infra_helpers.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/infra_helpers.go), [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go).
- Mirror sincroniza apenas a pasta `infra/` por tar pipeline, assumindo estrutura específica.
  - Código: [mirror.go:L168-L186](file:///home/neto/projects/casheiro-org/yby-cli/pkg/mirror/mirror.go#L168-L186).

**Relacionamentos**
- Root‑app ajusta `path` com `git prefix`, mas não resolve `.github` ou contexto de env.
  - Código: [bootstrap_cluster.go:L295-L304](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go#L295-L304) e [bootstrap_cluster.go:L316-L322](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go#L316-L322).

### Refinamento (BDD)

Visão de Negócios:
- O CLI deve funcionar do “topo do repo” em monorepos, sem obrigar o dev a navegar para `infra/`.

Visão Técnica:
- Passar `infra root` detectado para o `Manager` e demais comandos; padronizar uso de `FindInfraRoot/JoinInfra`.
- Mirror deve suportar sincronizar apenas `infra/` ou o projeto inteiro conforme configuração.

História de Usuário:
- Como desenvolvedor em monorepo, quero rodar `yby dev/env` da raiz e a CLI encontrar a infra automaticamente.

Critérios de Aceite:
- `yby dev/env` funciona da raiz do repo com `infra/.yby` detectado.
- Root‑app usa paths ajustados pelo `git prefix`.
- Mirror sincroniza corretamente sem assumir caminhos rígidos.

Cenários (Gherkin) com variações:

```gherkin
Feature: Execução a partir da raiz em monorepos
  # Sucesso
  Scenario: Manager usa infra root detectado
    Given ".yby" está em "infra/.yby"
    When executo "yby env list" da raiz
    Then os ambientes são listados corretamente
    And "yby dev" detecta e usa "infra/" para config/values

  # Falha
  Scenario: Nenhum ".yby" encontrado
    Given estou na raiz sem "infra/.yby" nem ".yby"
    When executo "yby dev"
    Then devo ver erro claro indicando ausência do manifesto

  # Borda
  Scenario: Múltiplas pastas infra em um monorepo
    Given existem "platform/infra/.yby" e "services/infra/.yby"
    When executo "yby dev" na raiz
    Then o CLI solicita qual infra usar ou aceita flag "--infra-dir"
```

Exemplos reais:
- Sucesso: monorepo com infra em `infra/`; `yby env show` na raiz funciona.
- Falha: repo sem infra; CLI aponta a ausência do arquivo `.yby/environments.yaml`.
- Borda: duas infra; CLI pede escolha ou usa flag para desambiguar.

---

## Relacionamentos Entre Trechos/Arquivos

- `dev.go` chama `bootstrap_cluster.go` e usa `pkg/context.Manager`:
  - [dev.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go)
  - [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go)
  - [context.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/context/context.go)
- `engine.go` controla roteamento de assets; precisa separar “raiz” vs “infra”:
  - [engine.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/scaffold/engine.go)
- `root-app.yaml.tmpl` define repoURL/path; ajusta com prefixo em bootstrap:
  - [root-app.yaml.tmpl](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/manifests/argocd/root-app.yaml.tmpl)
  - [bootstrap_cluster.go:L316-L322](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go#L316-L322)
- `mirror.go` gerencia git-server interno e sincronização:
  - [mirror.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/mirror/mirror.go)
- `.env` legado em `bootstrap_vps.go` e mensagens em `bootstrap_cluster.go`:
  - [bootstrap_vps.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_vps.go)
  - [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go)
- `environments.yaml.tmpl` e `init.go` geram manifestos e values:
  - [environments.yaml.tmpl](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/.yby/environments.yaml.tmpl)
  - [init.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/init.go)

---

## Plano de Correção (Prioridades)

- P1: Dev Local sem exigência de `GITHUB_REPO`
  - Integrar Mirror Local como repoURL do ArgoCD quando `YBY_ENV=local`.
  - Tornar checkEnvVars permissivo em `local`.
- P2: `.github` sempre na raiz
  - Roteamento no engine para assets “de raiz”.
  - Atualizar docs e mensagens do wizard.
- P3: Coerência de `environments.yaml`
  - Validar/ajustar `current` em relação à topologia.
  - Garantir geração de `config/values-*.yaml` para todos ambientes.
- P4: Remover dependência de `.env`
  - Migrar `bootstrap vps` para manifesto/flags.
  - Atualizar mensagens no cluster bootstrap.
- P5: Suporte pleno a subdiretórios
  - Usar `FindInfraRoot` para o `Manager`.
  - Ajustar Mirror e root‑app para prefixos git e paths robustos.

---

## Apêndice — Referências de Código

- Comandos:
  - [dev.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/dev.go)
  - [bootstrap_cluster.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_cluster.go)
  - [bootstrap_vps.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/bootstrap_vps.go)
  - [env.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/env.go)
  - [infra_helpers.go](file:///home/neto/projects/casheiro-org/yby-cli/cmd/infra_helpers.go)
- Scaffold/Assets:
  - [engine.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/scaffold/engine.go)
  - [environments.yaml.tmpl](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/.yby/environments.yaml.tmpl)
  - [root-app.yaml.tmpl](file:///home/neto/projects/casheiro-org/yby-cli/pkg/templates/assets/manifests/argocd/root-app.yaml.tmpl)
- Contexto:
  - [context.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/context/context.go)
  - [env.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/context/env.go)
- Mirror:
  - [mirror.go](file:///home/neto/projects/casheiro-org/yby-cli/pkg/mirror/mirror.go)

---

## Governança de Documentação

### Atualização Coordenada da Documentação
- README.md: atualizar visão geral, pré‑requisitos e “Getting Started” para refletir dev local com Mirror interno, remoção de exigência de `GITHUB_REPO/TOKEN` em local e posicionamento fixo de `.github/` na raiz.
- docs/wiki/Getting-Started.md: alinhar passo a passo, deixando claro que `.github` é gerado na raiz, que monorepos são suportados com `FindInfraRoot` e que dev local usa mirror.
- docs/wiki/CLI-Reference.md: sincronizar descrição e flags de `yby dev`, `yby bootstrap cluster`, `yby bootstrap vps`, `yby env *`; indicar comportamento em `local` vs `remote`, e requisitos de tokens.
- docs/wiki/Core-Concepts.md: reforçar Infra como Dados, manifesto `.yby/environments.yaml` como fonte única; remover dependência obrigatória de `.env`.
- docs/wiki/Migration-Guide-v2.md: orientar a migração completa para `.yby/environments.yaml`, descrever fallback opcional de `.env` com aviso de deprecação.
- pkg/templates/assets/README.md: atualizar seção de “Uso com Yby CLI”, “Gerenciamento de Contexto” e “Estrutura do Repositório” para refletir os ajustes de paths e Mirror.
- Inserir uma “Matriz Código ↔ Documentação” no README, mapeando:
  - cmd/*.go → docs/wiki/CLI-Reference.md
  - pkg/context/* → Core-Concepts.md e Migration-Guide-v2.md
  - pkg/scaffold/* → Getting-Started.md e README.md
  - pkg/templates/assets/* → assets/README.md

### Processo para Sincronizar Documentação com o Código
- Template de PR com checklist obrigatório:
  - “Impacto em documentação” com seções marcadas: README, CLI Reference, Getting‑Started, Core Concepts, Migration.
  - Link para commits que alteram docs.
- Label automática “needs-docs” quando arquivos em `cmd/`, `pkg/context/`, `pkg/scaffold/` ou `pkg/templates/assets/` mudarem sem alterações em `README.md` ou `docs/wiki/*`.
- Job CI “docs:generate” opcional com `cobra/doc` para gerar referência de comandos (quando aplicável) e comparar com `docs/wiki/CLI-Reference.md`.
- Job CI “docs:verify” obrigatório:
  - Verificar que o PR removeu menções de `.env` quando fluxos migram para `.yby`.
  - Validar que `.github` é mencionado como raiz no Getting‑Started e README.
  - Conferir que comandos presentes em `cmd/*` constam em CLI‑Reference, com descrição e flags.
  - Link check em todos os documentos (internos e externos).
- Política de versionamento de docs:
  - Atualizar docs no mesmo PR do código que altera comportamento.
  - Em releases, gerar “Notas de Versão” destacando mudanças em comportamento de CLI e docs afetados.

### Verificações de Consistência no Fluxo de Desenvolvimento
- Pre‑commit:
  - “docs:lint” (ortografia, estilo, links).
  - “docs:cli-diff” (lista comandos via introspecção do código e compara com CLI‑Reference).
- CI obrigatória:
  - “docs:verify” falha se:
    - Comandos alterados sem atualização em CLI‑Reference.
    - Mudanças em contexto/env sem atualização em Core‑Concepts ou Migration‑Guide.
    - Qualquer link quebrado.
  - “docs:examples-smoke” executa trechos exemplificados (onde viável) para garantir que comandos/doc exemplos não quebram.

### Critérios para Validar Documentação Completa e Atualizada
- Cobertura mínima:
  - Toda mudança em `cmd/*` exige alteração correspondente em CLI‑Reference com flags e exemplos.
  - Mudanças de semântica de ambiente exigem atualização de Core‑Concepts e, se for migração, Migration‑Guide.
  - Mudanças de scaffold/paths exigem atualização em Getting‑Started e README.
- Qualidade:
  - Exemplos executáveis verificados por “docs:examples-smoke”.
  - Ausência de marcadores “TODO” ou “TBD”.
  - Links internos para trechos de código usando referências clicáveis como padrão.
- Aprovação:
  - Ao menos um revisor “Docs Owner” deve aprovar PRs com impacto documental.

### Mecanismos para Prevenir Obsolescência
- Verificação agendada:
  - Job semanal “docs:drift” compara comandos/flags com CLI‑Reference e sinaliza divergências.
- Governança:
  - Aprovação do responsável pelas mudanças em `docs/wiki` e `README.md`.
  - Backlog “Docs Debt” com SLA: itens bloqueadores do próximo release.
- Automação:
  - “needs-docs” obrigatório para merges que tocam áreas sensíveis (cmd/context/scaffold/templates).
  - Geração semiautomática de CLI‑Reference com `cobra/doc` para reduzir esforço manual.

### BDD — Documentação Sincronizada com Código

```gherkin
Feature: Documentação sempre sincronizada com o código
  Background:
    Given existe governança de documentação com CI e revisão do responsável

  # Sucesso
  Scenario: PR altera comando e atualiza CLI-Reference
    Given um PR altera "cmd/dev.go" adicionando uma flag
    And o PR altera "docs/wiki/CLI-Reference.md" com a nova flag e exemplo
    When o CI executa "docs:verify"
    Then o job passa
    And o PR é aprovado pelo responsável

  # Falha
  Scenario: Mudança em contexto sem atualização de docs
    Given um PR altera "pkg/context/context.go" (semântica de environments)
    And o PR não altera "docs/wiki/Core-Concepts.md" nem "Migration-Guide-v2.md"
    When o CI executa "docs:verify"
    Then o job falha com relatório de inconsistência
    And o PR recebe label "needs-docs"

  # Borda
  Scenario: Monorepo com docs em submódulo
    Given a wiki é submódulo Git
    And o PR muda "cmd/bootstrap_cluster.go"
    When o CI executa "docs:verify"
    Then o job valida que o submódulo contém a atualização correspondente
    And se não, falha pedindo sync do submódulo
```

---

## Ajustes para Submódulo `docs/wiki`

### 1) Impacto na Estrutura Atual
- O submódulo em `docs/wiki` significa que os arquivos da wiki têm versionamento separado do repositório principal. Confirmado em [.gitmodules](file:///home/neto/projects/casheiro-org/yby-cli/.gitmodules).
- Hierarquia proposta preserva:
  - Documentos de alto nível no repositório principal (README, plano de correção).
  - Conteúdos aprofundados no submódulo `docs/wiki` (CLI‑Reference, Getting‑Started, Core‑Concepts, Migration‑Guide).
- Ajuste de caminhos e referências:
  - Links entre README (principal) e páginas da wiki devem usar links relativos ao repositório remoto (GitHub) ou rotas canônicas da wiki, não `file:///`.
  - No CI “docs:verify”, clonar o submódulo e resolver caminhos relativos entre repositórios.

### 2) Adequação dos Fluxos de Trabalho
- Atualização e sincronização:
  - PRs que alteram código com impacto documental devem incluir:
    - Commit no submódulo (docs/wiki) com atualização do conteúdo.
    - Atualização do ponteiro do submódulo no repositório principal (git add docs/wiki, commit do novo SHA).
  - Checklist de PR ampliada: “docs submódulo atualizado e ponteiro sincronizado”.
- Colaboração:
- Responsável definido para `docs/wiki` (submódulo) e documentação de topo (README).
  - CI “docs:verify” deve executar `git submodule update --init --recursive` para validar conteúdo mais recente.
- Pontos de conflito/complexidade:
  - Dois repositórios envolvidos por PR: exigir mensagem clara e automação de verificação do ponteiro.
  - Evitar merges do principal sem atualizar o submódulo: CI deve falhar se deteção de mudança em áreas sensíveis não vier acompanhada de diffs no submódulo.

### 3) Proposta de Ajustes
- Estrutura:
  - Manter conteúdo aprofundado em `docs/wiki` e referências de topo no principal.
  - Adicionar um “Índice de Documentação” no README principal, com links para wiki (sem criar novo arquivo extra).
- Mecanismos:
  - CI “docs:verify”:
    - Executar `git submodule update --init --recursive`.
    - Validar que o submódulo contém alterações correlacionadas quando `cmd/*`, `pkg/context/*`, `pkg/scaffold/*` ou `pkg/templates/assets/*` mudarem.
    - Checar links cruzados (principal → wiki, wiki → principal) via verificador de links remotos.
  - Label “needs-docs” ampliada para verificar se houve atualização do ponteiro do submódulo.
- Preservar essência:
  - Continuidade da governança, critérios e BDD já definidos.
  - Apenas adaptar os mecanismos para tratar “duas origens” (principal + submódulo).

### 4) Validação em Ambiente Controlado
- Testes de fluxo:
  - Cenário 1 (sucesso): Alterar `cmd/dev.go` e atualizar `docs/wiki/CLI-Reference.md`; atualizar ponteiro do submódulo; CI executa `docs:verify` e passa.
  - Cenário 2 (falha): Alterar `pkg/context/context.go` sem atualizar wiki; CI detecta falta de alteração no submódulo e falha com instrução de correção.
  - Cenário 3 (borda): Atualização apenas documental na wiki; validar que o PR principal inclui o avanço do ponteiro do submódulo, CI passa.
- Desempenho e usabilidade:
  - Cache de submódulo no CI para reduzir tempo de clone.
  - Guia de contribuição explicitando “como atualizar submódulo” e “como sincronizar ponteiro” para reduzir atrito.

### BDD — Fluxos com Submódulo

```gherkin
Feature: Sincronização de documentação com submódulo
  Background:
    Given o repositório principal possui submódulo "docs/wiki"
    And o CI roda "git submodule update --init --recursive"

  # Sucesso
  Scenario: Código alterado com docs atualizados no submódulo
    Given um PR altera "cmd/bootstrap_cluster.go"
    And o PR atualiza "docs/wiki/Getting-Started.md" no submódulo
    And o PR atualiza o ponteiro do submódulo no principal
    When o CI executa "docs:verify"
    Then o job passa
    And o PR pode ser aprovado

  # Falha
  Scenario: Código alterado sem atualizar wiki
    Given um PR altera "pkg/scaffold/engine.go"
    And o PR não altera conteúdo em "docs/wiki"
    When o CI executa "docs:verify"
    Then o job falha indicando falta de atualização no submódulo
    And o PR recebe label "needs-docs"

  # Borda
  Scenario: Atualização apenas de documentação
    Given um PR altera "docs/wiki/CLI-Reference.md" no submódulo
    And o PR atualiza o ponteiro do submódulo no principal
    When o CI executa "docs:verify"
    Then o job passa
    And nenhuma alteração de código é exigida
```

---

## Estratégia de Execução Incremental (Qualidade Premium)

### Fases e Priorização
- Fase 0 — Governança & Qualidade
- Implementar CI “docs:verify”, checklist de PR e revisão do responsável.
  - Critérios: CI ativo; labels “needs-docs” funcionando; links sem erros.
  - Marcos: pipeline verde; guia de contribuição atualizado.
- Fase 1 — Dev Local sem GITHUB_REPO (P1)
  - Integrar Mirror interno no `root-app` e afrouxar `checkEnvVars` em `local`.
  - Critérios: `yby dev` funciona offline; ArgoCD sincroniza com mirror; sem erro fatal por ausência de `GITHUB_REPO/TOKEN`.
  - Marcos: E2E de dev local passando; docs atualizadas (README, Getting‑Started, CLI‑Reference).
- Fase 2 — `.github` na raiz (P2)
  - Roteamento de assets de raiz no `engine`.
  - Critérios: `.github/workflows/*` na raiz; CI do GitHub aciona; paths de `infra` intactos.
  - Marcos: teste de scaffold em monorepo; docs refletindo comportamento.
- Fase 3 — `environments.yaml` coerente (P3)
  - Validação do `current` vs topologia; geração de `values-*` consistente.
  - Critérios: `init` nunca gera estados incoerentes; `env use/show/dev` funcionam em todas topologias.
  - Marcos: cenários Gherkin (sucesso/falha/borda) passando; docs atualizadas (Core‑Concepts).
- Fase 4 — Remover dependência de `.env` (P4)
  - `bootstrap vps/cluster` com manifesto/flags; mensagens sem `.env`.
  - Critérios: execução sem `.env`; fallback legível com aviso de deprecação quando necessário.
  - Marcos: CLI‑Reference e Migration‑Guide revisadas.
- Fase 5 — Suporte pleno a subdiretórios (P5)
  - `Manager` usando `FindInfraRoot`; Mirror/Root‑App com prefixos git robustos.
  - Critérios: comandos rodam da raiz em monorepo; múltiplas infra suportadas com flag ou prompt.
  - Marcos: E2E de monorepo; docs wiki e principal com índices e links corretos.

### Critérios de Aceitação por Incremento
- Todos os testes unitários e E2E relevantes passam.
- CI “docs:verify” sem falhas (cobertura de comandos/flags, links, submódulo sincronizado).
- Documentação atualizada no mesmo PR, aprovada pelo responsável.
- Nenhum regressão em cenários existentes (sucesso/falha/borda).

### Marcos de Verificação de Qualidade
- Gate 1: Revisão de código (arquitetura, segurança, performance).
- Gate 2: Testes (unit/E2E) e cobertura mínima definida por componente.
- Gate 3: Verificação de documentação (docs:verify).
- Gate 4: Smoke test manual em ambiente controlado (dev local, monorepo, submódulo).

### Sistema de Controle de Contexto Dinâmico
- Contexto Essencial Ativo
  - Ambiente ativo (YBY_ENV), infra root detectado, conjunto de arquivos afetados por incremento, tarefas correntes.
  - Manter apenas o mínimo necessário carregado; buscar detalhes sob demanda.
- Recuperação de Contexto sob Demanda
  - Estratégia de consultas múltiplas (sinônimos e broad/high-level) para rehidratar contexto com alta confiança.
  - “Context Recovery Spec” por incremento, documentando buscas e pontos críticos.
- Atualização Progressiva do Contexto
  - Após cada incremento, atualizar o “mapa de impacto” (arquivos, comandos, docs) e próximos passos.
  - Integrar com o checklist de PR para garantir que o contexto evolui junto com o código e docs.

### Fluxo de Trabalho Otimizado para Desenvolvimento com IA
- Orquestração Modular
  - Descoberta → Análise → Planejamento (todos) → Implementação (patches) → Verificação (tests/CI) → Documentação (wiki/README).
  - Evitar dependência de contexto único ao modularizar tarefas e registrar o estado por incremento.
- Automação de Contexto entre Sessões
  - Persistir metadata mínima (infra root, tarefas, arquivos chave) em artefatos de PR e usar no próximo ciclo.
  - Reidratar contexto via buscas programáticas e referências clicáveis.
- Checks de Consistência entre Incrementos
  - Validação de critérios de aceite, cobertura de docs e ausência de regressões a cada merge.
  - Alertas quando incrementos alteram áreas sensíveis sem ajuste documental correlato.

### Monitoramento Contínuo
- Métricas
  - Taxa de sucesso do E2E por fase; tempo médio de ciclo; falhas por tipo (docs, testes, paths).
  - “Doc drift ratio”: divergências entre CLI e documentação detectadas por “docs:verify”.
  - Latência do ArgoCD no dev local; tempo de bootstrap; erros por dependências externas.
- Ajuste Dinâmico
  - Triage semanal com priorização de itens que aumentam atrito DX.
  - Refinar consultas de contexto e cobertura de testes conforme métricas.
- Documentação do Estado
  - Changelog incremental no PR; Notas de Versão ao concluir fases.
  - Índice de documentação mantido (principal → wiki) e validado em CI.
