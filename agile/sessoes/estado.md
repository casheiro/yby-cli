# Estado da sessão

> Ponto de partida de toda sessão. O hook `SessionStart` e a skill `/retomar` leem este arquivo; a skill `/encerrar` reescreve a seção "Estado atual" e registra a sessão em [diario.md](diario.md).
> Os campos entre crases são lidos por `scripts/sessao.sh`: mantenha o formato `- **Nome:** \`valor\``.

## Estado atual

- **Projeto ativo:** `Correções dos plugins (Bard, Sentinel, Synapstor, Atlas, Viz)`
- **Plano:** `agile/planning/correcoes-plugins/tasks.md`
- **Branch:** `feat/correcoes-plugins`
- **Próxima ação:** onda 0 na `feat/correcoes-plugins` (C02, C01, R01, K01), por workflow: implementador e revisor adversarial por tarefa, cada um no seu worktree, e integração por cherry-pick. Depois do PR da onda 0, abrir as cinco trilhas da onda 1 (tabela "Ondas" do plano).
- **Aguardando o usuário:** release v5.8.1 (PR #219, aberto pelo pipeline depois do merge do #216, do #217 e do #218); atualizar o yby e os plugins juntos no WSL.
- **Atenção:**
  - **Escopo autorizado (26/10/2026):** corrigir tudo o que é erro ou está errado em relação à proposta. Features e perfumaria ficam fora (lista no fim do plano).
  - **Push e PR:** a cada trilha, direto, porque são correções.
  - **Execução (decisão de 26/10/2026):** ondas com trilhas paralelas, uma branch `feat/correcoes-<trilha>` e um worktree por trilha, e um PR por trilha. As decisões do usuário sobre K01, V02, Y06 e Y07 estão no topo do plano. O mapa da conferência (arquivos, dependências e modelo por tarefa) ficou no scratchpad da sessão de 26/10; o que muda o plano foi copiado para as notas "Conferência (26/10)".
  - **Plano anterior:** o da carta do terminal está completo (39/39) e foi para "Outras frentes".
  - **Testes locais:** use `scripts/falhas.sh <dir> <saida>` na branch e num worktree da `origin/develop` e compare. Só podem falhar os testes que já falham no Windows: TestAuditLogger_Permissions, TestConfigureDirenv_EnvrcNaoExiste, TestCopyFile_Permissao0600, TestEncryptedFileStore_Permissions, TestEncryptWithSOPS_Success, TestFindInfraRoot_NotFound_MensagemErro, TestGenerateMermaid_PathsAbsolutos, TestGetAssets_PluginComAssetsAbsoluto, TestNewVectorStore_InvalidPath, TestResolveOverridePaths, TestValidateRenderedYAML_SubdirRecursivo, TestWriteEnvFile_FilePermissions. Dois deles entram em pânico e escondem o resto do pacote: rodando o `pkg/plugin` sem o TestGetAssets_PluginComAssetsAbsoluto, falham também TestGetAssets_PluginComAssetsRelativo, TestExecuteContextHook_ComPatchDeDados, TestExecuteContextHook_DataNilInicializado e TestScanDirectory_WithValidPlugin; o `cmd` sem o TestFindInfraRoot_NotFound_MensagemErro também mostra falhas só do Windows. Para conferir um pacote assim, compare `go test -v -skip <o que entra em pânico>` teste a teste com a develop.
  - **Lint local:** o `.golangci.yml` é v1. Crie uma config v2 no scratchpad com `version: "2"`, os linters govet, ineffassign e revive, e o formatter gofmt. Rode com `--new-from-rev HEAD` (ou o commit base), porque sem o revive `exported` desligado aparecem uns 50 avisos antigos.
  - **Wiki:** a linha do `sentinel scan --no-ai` só entra depois da release (P02).
  - **Home do usuário:** `~/.yby/plugins` tem plugins falsos de testes antigos, que não devem ser apagados.

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

- Carta do terminal (`agile/planning/tui-redesign/tasks.md`): completa (39/39), v5.8.0 publicada em 26/10/2026. Pendências menores que sobraram foram para o plano de correções (B13, B14, P01) ou para "Fora do escopo" (`yby version`).
- `feat/multicloud-support`: 5 commits locais com correções para Windows; o PR #200 foi fechado e as correções entraram pela `feat/tui-bard` (v5.7.0). A branch pode ser apagada.
- Cloud sem cluster real (25/10/2026): decidido não criar contas pagas e deixar o `yby cloud` em beta (`6486e3f`). GCP testada com o gcloud real só no que não exige faturamento (doctor, cloud status/refresh/audit, list com a API desligada, gcloud sem login); contratos de AWS, Azure e GCP conferidos contra a documentação oficial. Nunca testados de verdade: `cloud connect` e `get-credentials` reais, o plugin de autenticação do kubeconfig (gke-gcloud-auth-plugin, kubelogin, aws eks get-token), o token vencendo com o cluster em uso, o `yby-cloud` (SDK) contra cluster. Decisões abertas: `cloud refresh`/`cloud list` saem com 0 mesmo quando tudo falha (o doctor também), e o TTL fixo de 1 h dos tokens GCP pelo CLI (o `config config-helper`, que dá a validade real, não está mais na doc oficial).
- Testes que falham só no Windows e já falhavam antes (permissões `0600`, caminhos estilo Unix, scripts `#!/bin/sh` como plugins falsos, override de `HOME`). Fora do plano da carta.
