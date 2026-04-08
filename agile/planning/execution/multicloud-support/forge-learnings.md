
## T001 — Done
Todos os testes passam. A correção foi aplicar `gofmt -w` em `pkg/errors/hints.go`, que tinha espaços de alinhamento inconsistentes no map `defaultHints` — os novos campos cloud (`ErrCodeCloudTok...

## T002 — Done
Todos os 18 testes passam. T002 já estava marcado `[x]` no tasks.md — a única ação necessária era corrigir o teste flaky.

**Correção aplicada:** `TestDetectFromKubeconfig_NoFile` agora seta ...

## T003 — Done
T003 concluído. Resumo do que foi criado:

- **`pkg/cloud/token.go`** — interface `TokenGenerator` com `GenerateToken(ctx) (*Token, error)` e struct `Token{Value string, ExpiresAt time.Time}`
- **`...

## T004 — Incomplete


## T005 — Done


## T006 — Done

