# Estado da sessão

> Ponto de partida de toda sessão. O hook `SessionStart` e a skill `/retomar` leem este arquivo; a skill `/encerrar` reescreve a seção "Estado atual" e registra a sessão em [diario.md](diario.md).
> Os campos entre crases são lidos por `scripts/sessao.sh`: mantenha o formato `- **Nome:** \`valor\``.

## Estado atual

- **Projeto ativo:** `Carta do terminal (redesign das TUIs e saídas)`
- **Plano:** `agile/planning/tui-redesign/tasks.md`
- **Branch:** `feat/tui-bard`
- **Próxima ação:** fase 3 (Viz), começando pela T017 (atualizar dependências do Viz, commit isolado), numa branch nova a partir da `feat/tui-bard`.
- **Aguardando o usuário:** revisão e merge do PR da fase 2 (`feat/tui-bard` → `develop`).
- **Atenção:** fases 0 e 1 na `develop` (PR #201 mergeado); PRs #199 e #200 fechados (a logo e as correções para Windows vêm nesta branch). Fase 2 completa (T011–T016) e mais T036 (logo), T037 (logo no Bard) e T038 (ajuda em português), no PR da fase 2. O yby fica só em português (decisão de 24/10/2026, sem i18n). O secret `GH_PAT` foi apagado pelo usuário: o job "Create PR to Develop" passa com o `GITHUB_TOKEN`, mas no release a tag empurrada pelo `GITHUB_TOKEN` não dispara o `release.yaml` (GoReleaser) nem o PR da `release/**` dispara o `release-pipeline.yaml`; o release vai precisar de um PAT novo ou de disparo manual. `~/.yby/plugins` do usuário tem plugins falsos de testes antigos (`yby-plugin-dup`, `-fake`, `-http`…), não apagados. Teste real: o usuário aceita as capturas do agente (ConPTY + xterm.js + Playwright) publicadas numa galeria; o Bard é capturado com claude e kubectl falsos (`scratchpad/captura/shims_bard`).

## Ambiente

- Windows 11 com Git Bash. Go 1.27 em `C:\Program Files\Go\bin`; sessões abertas antes da instalação precisam de `export PATH="$PATH:/c/Program Files/Go/bin"` em cada comando.
- Binários em `bin/*.exe` (`task build`). Plugins instalados em `~/.yby/plugins`; depois de cada build, reinstalar com `./bin/yby.exe plugin install bin/yby-plugin-<nome>.exe --force` (o checksum muda).
- `OLLAMA_HOST=0.0.0.0:11434` está definido na máquina; o Ollama normalmente não está rodando, então o Bard usa o próximo provider.
- golangci-lint 2.15 instalado via winget; o `.golangci.yml` do repo é v1 (CI usa 1.64.8). Para rodar local: `golangci-lint migrate` numa cópia temporária da config e `golangci-lint run -c <cópia>`.
- No PowerShell do usuário o Go também não está no PATH: `$env:Path += ";C:\Program Files\Go\bin"`.
- Especificação local de `pkg/ui`: `docs/design/tui/especificacao-pkg-ui.md`; catálogo: `go run ./tools/ui-catalogo [--tema claro|escuro] [--largura N] [--ascii] [--sem-cor]`.
- Prancha aprovada: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk
- gcloud 590 instalado (winget) em `%LOCALAPPDATA%\Google\Cloud SDK\google-cloud-sdk\bin`, logado como o usuário, projeto padrão `yby-teste-2610` (sem faturamento: a conta de faturamento está fechada e reabrir pede R$ 200 adiantados). No Git Bash, chamar `gcloud.cmd` por esse caminho. Um gcloud sem login para teste: `CLOUDSDK_CONFIG=<pasta vazia>`. Sem uso, o projeto pode ser apagado com `gcloud projects delete yby-teste-2610`.

## Outras frentes abertas

- `feat/multicloud-support`: 5 commits sem push com correções para Windows (`.exe` no Taskfile, `.gitattributes` e modo executável dos scripts, descoberta de plugins `.exe`, `OLLAMA_HOST`, download do modelo de embeddings). PR #200 aberto para `develop`.
- Cloud sem cluster real (25/10/2026): decidido não criar contas pagas. GCP testada com o gcloud real só no que não exige faturamento (doctor, cloud status/refresh/audit, list com a API desligada, gcloud sem login); contratos de AWS, Azure e GCP conferidos contra a documentação oficial. Nunca testados de verdade: `cloud connect` e `get-credentials` reais, o plugin de autenticação do kubeconfig (gke-gcloud-auth-plugin, kubelogin, aws eks get-token), o token vencendo com o cluster em uso, o `yby-cloud` (SDK) contra cluster. Decisões abertas: `cloud refresh`/`cloud list` saem com 0 mesmo quando tudo falha (o doctor também), e o TTL fixo de 1 h dos tokens GCP pelo CLI (o `config config-helper`, que dá a validade real, não está mais na doc oficial).
- Testes que falham só no Windows e já falhavam antes (permissões `0600`, caminhos estilo Unix, scripts `#!/bin/sh` como plugins falsos, override de `HOME`). Fora do plano da carta.
