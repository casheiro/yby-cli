# Estado da sessão

> Ponto de partida de toda sessão. O hook `SessionStart` e a skill `/retomar` leem este arquivo; a skill `/encerrar` reescreve a seção "Estado atual" e registra a sessão em [diario.md](diario.md).
> Os campos entre crases são lidos por `scripts/sessao.sh`: mantenha o formato `- **Nome:** \`valor\``.

## Estado atual

- **Projeto ativo:** `Carta do terminal (redesign das TUIs e saídas)`
- **Plano:** `agile/planning/tui-redesign/tasks.md`
- **Branch:** `feat/tui-carta`
- **Próxima ação:** checkpoint da fase 0 (golangci-lint) e PR "feat(ui): pacote pkg/ui da carta do terminal"; depois atualizar a branch com a `main` antes da fase 2 (Bard).
- **Aguardando o usuário:** instalar o golangci-lint.
- **Atenção:** a branch partiu de `origin/main` e não tem as correções para Windows que estão em `feat/multicloud-support`. A fase 0 não depende delas; antes da fase 2 (Bard), atualizar esta branch com a `main`.

## Ambiente

- Windows 11 com Git Bash. Go 1.27 em `C:\Program Files\Go\bin`; sessões abertas antes da instalação precisam de `export PATH="$PATH:/c/Program Files/Go/bin"` em cada comando.
- Binários em `bin/*.exe` (`task build`). Plugins instalados em `~/.yby/plugins`; depois de cada build, reinstalar com `./bin/yby.exe plugin install bin/yby-plugin-<nome>.exe --force` (o checksum muda).
- `OLLAMA_HOST=0.0.0.0:11434` está definido na máquina; o Ollama normalmente não está rodando, então o Bard usa o próximo provider.
- Prancha aprovada: https://claude.ai/artifact/MuC7dcxzr8xxMAxqubQJQk

## Outras frentes abertas

- `feat/multicloud-support`: 5 commits sem push com correções para Windows (`.exe` no Taskfile, `.gitattributes` e modo executável dos scripts, descoberta de plugins `.exe`, `OLLAMA_HOST`, download do modelo de embeddings). PR #200 aberto para `develop`.
- Testes que falham só no Windows e já falhavam antes (permissões `0600`, caminhos estilo Unix, scripts `#!/bin/sh` como plugins falsos, override de `HOME`). Fora do plano da carta.
