# Estado da sessão

> Ponto de partida de toda sessão. O hook `SessionStart` e a skill `/retomar` leem este arquivo; a skill `/encerrar` reescreve a seção "Estado atual" e registra a sessão em [diario.md](diario.md).
> Os campos entre crases são lidos por `scripts/sessao.sh`: mantenha o formato `- **Nome:** \`valor\``.

## Estado atual

- **Projeto ativo:** `Carta do terminal (redesign das TUIs e saídas)`
- **Plano:** `agile/planning/tui-redesign/tasks.md`
- **Branch:** `feat/tui-bard`
- **Próxima ação:** checkpoint da fase 2 com o usuário (galeria https://claude.ai/artifact/N8Cjoz1A8EKx6tiJHLRoc8); com o aval, push da `feat/tui-bard` e PR "feat(bard): nova TUI da carta do terminal" para a `develop`, aberto à mão. Depois, fase 3 (Viz, T017).
- **Aguardando o usuário:** conferir a galeria da fase 2 e decidir os 4 pontos dela (ctrl+j para nova linha, ctrl+y para copiar, resposta interrompida fora do histórico, /modelo só troca o provider).
- **Atenção:** fases 0 e 1 na `develop` (PR #201 mergeado); PRs #199 e #200 fechados (a logo e as correções para Windows vêm nesta branch). Fase 2 completa (T011–T016) e mais T036 (logo), T037 (logo no Bard) e T038 (ajuda em português), sem push. O secret `GH_PAT` do CI está vencido: o job "Create PR to Develop" falha com 401 e os releases também vão falhar até ele ser renovado. `~/.yby/plugins` do usuário tem plugins falsos de testes antigos (`yby-plugin-dup`, `-fake`, `-http`…), não apagados. Teste real: o usuário aceita as capturas do agente (ConPTY + xterm.js + Playwright) publicadas numa galeria; o Bard é capturado com claude e kubectl falsos (`scratchpad/captura/shims_bard`).

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
