---
name: cli-patterns
description: Padrões de estrutura de comandos, flags, UX e formatação de output no Yby CLI
---

# Padrões CLI — Yby CLI

## Estrutura de Comandos

Cada arquivo em `cmd/` representa um comando ou subcomando. A descoberta de plugins é automática via `root.go` — não registrar plugins manualmente.

```
cmd/
  root.go       → flags globais, telemetria, plugin discovery
  up.go         → yby up
  seal.go       → yby seal
  secrets.go    → yby secrets
```

## Registro de Subcomandos

```go
func init() {
    rootCmd.AddCommand(newMyCmd())
}

func newMyCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycommand [flags]",
        Short: "Descrição curta em PT-BR",
        RunE:  runMyCommand,
    }
    cmd.Flags().StringVar(&myFlag, "flag", "default", "descrição da flag")
    return cmd
}
```

## Flags

- Flags locais: `cmd.Flags()` — visíveis só no comando
- Flags globais: `rootCmd.PersistentFlags()` — herdadas por subcomandos
- Flags já globais: `--log-format`, `--context`, `--env`
- Nomear flags em kebab-case (`--dry-run`, `--log-format`)

## Verificação de Dependências

Antes de executar lógica que exige ferramentas externas, verificar com `runner.Run()`:

```go
if err := runner.Run("which", "kubectl"); err != nil {
    return errors.New(errors.ERR_VALIDATION, "kubectl não encontrado no PATH")
}
```

## Output para o Usuário

- Usar `lipgloss` para formatação visual (cores, bordas, tabelas)
- Mensagens de sucesso/erro devem ser claras e em PT-BR
- Para output estruturado (JSON), verificar `--log-format=json` e adaptar

## Plugins

- Plugins comunicam via JSON em STDIN/STDOUT
- Contexto do plugin passa via env var `YBY_PLUGIN_REQUEST`
- Hooks suportados: `manifest`, `context`, `command`, `assets`
- Nunca hardcodar caminhos de plugins — usar o Manager para descoberta
