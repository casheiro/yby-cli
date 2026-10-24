# Estado da sessão

> Ponto de partida de toda sessão. O hook `SessionStart` e a skill `/retomar` leem este arquivo; a skill `/encerrar` reescreve a seção "Estado atual" e registra a sessão em [diario.md](diario.md).
> Os campos entre crases são lidos por `scripts/sessao.sh`: mantenha o formato `- **Nome:** \`valor\``.

## Estado atual

- **Projeto ativo:** `Carta do terminal (redesign das TUIs e saídas)`
- **Plano:** `agile/planning/tui-redesign/tasks.md`
- **Branch:** `feat/tui-bard`
- **Próxima ação:** fase 2 (Bard), começando pela T011 (fluxo de resposta em eventos), na `feat/tui-bard`, que nasceu da `feat/tui-carta`.
- **Aguardando o usuário:** revisão e merge do PR #201 (fases 0 e 1, `feat/tui-carta` → `develop`).
- **Atenção:** fases 0 e 1 completas (T001–T010, T023–T028) no PR #201. O job "Create PR to Develop" do CI falha com HTTP 401 (token do workflow); a validação passa e os PRs são abertos à mão com `gh pr create`. A `feat/tui-bard` leva por cherry-pick 4 correções para Windows da `feat/multicloud-support` (local, sem push; PR #200). `~/.yby/plugins` do usuário tem plugins falsos que testes antigos deixaram (`yby-plugin-dup`, `-fake`, `-http`…); não foram apagados. Teste real: o usuário aceita as capturas do agente (ConPTY + xterm.js + Playwright) publicadas numa galeria.

## Ambiente

- Windows 11 com Git Bash. Go 1.27 em `C:\Program Files\Go\bin`; sessões abertas antes da instalação precisam de `export PATH="$PATH:/c/Program Files/Go/bin"` em cada comando.
- Binários em `bin/*.exe` (`task build`). Plugins instalados em `~/.yby/plugins`; depois de cada build, reinstalar com `./bin/yby.exe plugin install bin/yby-plugin-<nome>.exe --force` (o checksum muda).
- `OLLAMA_HOST=0.0.0.0:11434` está definido na máquina; o Ollama normalmente não está rodando, então o Bard usa o próximo provider.
- golangci-lint 2.15 instalado via winget; o `.golangci.yml` do repo é v1 (CI usa 1.64.8). Para rodar local: `golangci-lint migrate` numa cópia temporária da config e `golangci-lint run -c <cópia>`.
- No PowerShell do usuário o Go também não está no PATH: `$env:Path += ";C:\Program Files\Go\bin"`.
- Especificação local de `pkg/ui`: `docs/design/tui/especificacao-pkg-ui.md`; catálogo: `go run ./tools/ui-catalogo [--tema claro|escuro] [--largura N] [--ascii] [--sem-cor]`.
- Prancha aprovada: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk

## Outras frentes abertas

- `feat/multicloud-support`: 5 commits sem push com correções para Windows (`.exe` no Taskfile, `.gitattributes` e modo executável dos scripts, descoberta de plugins `.exe`, `OLLAMA_HOST`, download do modelo de embeddings). PR #200 aberto para `develop`.
- Testes que falham só no Windows e já falhavam antes (permissões `0600`, caminhos estilo Unix, scripts `#!/bin/sh` como plugins falsos, override de `HOME`). Fora do plano da carta.
