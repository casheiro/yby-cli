# Estado da sessão

> Ponto de partida de toda sessão. O hook `SessionStart` e a skill `/retomar` leem este arquivo; a skill `/encerrar` reescreve a seção "Estado atual" e registra a sessão em [diario.md](diario.md).
> Os campos entre crases são lidos por `scripts/sessao.sh`: mantenha o formato `- **Nome:** \`valor\``.

## Estado atual

- **Projeto ativo:** `Carta do terminal (redesign das TUIs e saídas)`
- **Plano:** `agile/planning/tui-redesign/tasks.md`
- **Branch:** `feat/notas-release-merges`
- **Próxima ação:** v5.8.0 publicada (26/10/2026, 43 binários, notas limpas à mão). Escolher a próxima frente entre as pendências menores (lista em Atenção) ou a saída do beta do cloud (`ROADMAP.md`).
- **Aguardando o usuário:** aprovação e merge do #212 (release v5.8.0 → main) e merge do #213 (notas da release sem merges, hashes curtos e link do CHANGELOG, para as próximas versões).
- **Atenção:** plano completo (39/39), tudo com push; a wiki foi publicada (`b8dc3f7`) antes da versão com o Viz novo sair. O cloud está em beta (anotação `yby/beta`); o que falta para sair do beta está no `ROADMAP.md`. Desde o #206 a release publica sozinha no merge do PR develop → `release/*`; branches de PR para a develop precisam ser `feat/**` (o check obrigatório só roda nelas). Pendências menores: cabeçalho do Bard sem o modelo vindo do config; providers CLI sem a cadeia de decorators; `sentinel --fix` com erros crus; gofmt acusa `plugins/sentinel/cli/checks/rbac_allowlist.go`; TestHook* do Atlas podem sair do `skip_base`; `yby version` em texto simples; a 72×20 o detalhe por cima do painel cloud fica sem a auditoria. `~/.yby/plugins` do usuário tem plugins falsos de testes antigos, não apagados. Contêiner de teste `yby-viz-k3s` removido em 25/10.

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

- `feat/multicloud-support`: 5 commits locais com correções para Windows; o PR #200 foi fechado e as correções entraram pela `feat/tui-bard` (v5.7.0). A branch pode ser apagada.
- Cloud sem cluster real (25/10/2026): decidido não criar contas pagas e deixar o `yby cloud` em beta (`6486e3f`). GCP testada com o gcloud real só no que não exige faturamento (doctor, cloud status/refresh/audit, list com a API desligada, gcloud sem login); contratos de AWS, Azure e GCP conferidos contra a documentação oficial. Nunca testados de verdade: `cloud connect` e `get-credentials` reais, o plugin de autenticação do kubeconfig (gke-gcloud-auth-plugin, kubelogin, aws eks get-token), o token vencendo com o cluster em uso, o `yby-cloud` (SDK) contra cluster. Decisões abertas: `cloud refresh`/`cloud list` saem com 0 mesmo quando tudo falha (o doctor também), e o TTL fixo de 1 h dos tokens GCP pelo CLI (o `config config-helper`, que dá a validade real, não está mais na doc oficial).
- Testes que falham só no Windows e já falhavam antes (permissões `0600`, caminhos estilo Unix, scripts `#!/bin/sh` como plugins falsos, override de `HOME`). Fora do plano da carta.
