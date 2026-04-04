---
name: project-conventions
description: Padrões de código, nomenclatura e decisões arquiteturais do Yby CLI
---

# Convenções do Projeto Yby CLI

## Tratamento de Erros

Sempre usar `pkg/errors.YbyError` — nunca `fmt.Errorf` ou `errors.New` da stdlib diretamente.

```go
// Correto
return errors.New(errors.ERR_VALIDATION, "ambiente inválido")
return errors.Wrap(err, errors.ERR_IO, "falha ao ler config").WithContext("path", configPath)

// Errado
return fmt.Errorf("ambiente inválido")
```

Códigos de erro disponíveis: `ERR_IO`, `ERR_NETWORK_TIMEOUT`, `ERR_CLUSTER_OFFLINE`, `ERR_PLUGIN`, `ERR_VALIDATION`, `ERR_CONFIG`, `ERR_SCAFFOLD_FAILED`.

## Serviços e Injeção de Dependência

Todo serviço em `pkg/services/` recebe `shared.Runner` e `shared.Filesystem` via construtor:

```go
type MyService struct {
    runner shared.Runner
    fs     shared.Filesystem
}

func NewMyService(runner shared.Runner, fs shared.Filesystem) *MyService {
    return &MyService{runner: runner, fs: fs}
}
```

Nunca instanciar `exec.Command` diretamente — usar `runner.Run()` para que os testes possam usar `testutil.MockRunner`.

## Testes

- Mocks ficam em `pkg/testutil/`: `MockRunner`, `MockFilesystem`, `exec_mock`
- Testes extras (casos edge) vão em `*_extra_test.go` no mesmo pacote
- Testes E2E em `test/e2e/` com build tag `e2e` e godog/Cucumber
- Testes de integração com APIs externas usam build tag `integration` — NUNCA rodar no CI

## Comandos Cobra

- Usar `RunE` (nunca `Run`) para suportar retorno de erro
- Nunca chamar `os.Exit` dentro de comandos — retornar `error`
- Flags de escopo global ficam em `cmd/root.go`; flags específicas no próprio comando

## Logs e Output

- Diagnósticos e debug: `slog.Info/Warn/Error` — nunca `fmt.Println`
- Output para o usuário (resultado de comando): pode usar `fmt.Println` ou `lipgloss` para formatação
- Formato configurável via `--log-format` (text/json)

## Anti-padrões

- Não usar `os.Exit` em `RunE`
- Não criar abstrações genéricas para uso único
- Não adicionar tratamento de erro para cenários impossíveis
- Não usar `fmt.Println` para logs de diagnóstico
- Não adicionar flags globais em subcomandos (usar `PersistentFlags` no root)
