# Tasks: Correções dos plugins (Bard, Sentinel, Synapstor, Atlas, Viz)

> **Fonte da verdade do progresso.** Toda sessão de trabalho começa e termina neste arquivo.
> **Origem:** revisão de 26/10/2026, com um agente por plugin. Os relatórios citavam arquivo e linha; as linhas abaixo são aproximadas e devem ser conferidas antes de editar. ✓ = conferido no código pela sessão principal; ▶ = reproduzido pelo agente rodando.
> **Escopo (decisão do usuário, 26/10/2026):** corrigir **tudo o que é erro ou está errado em relação à proposta do plugin**. Ficam de fora **features novas e perfumaria**, que estão na seção "Fora do escopo" no fim deste arquivo. Na dúvida entre "conserto" e "feature", vale a pergunta: o plugin promete isso (ajuda, README, CLAUDE.md, nome da flag) e não entrega? Se sim, é conserto.
> **Verificação geral:** `go test ./pkg/... ./plugins/bard/... ./plugins/atlas/... ./cmd/...` + `go test -tags k8s ./plugins/sentinel/...` + `(cd plugins/viz && go test ./...)` + `(cd plugins/synapstor && go test ./...)` + lint. Compare com a develop usando `falhas.sh`: só podem falhar os 11 testes que já falham no Windows (lista em `agile/sessoes/estado.md`).

## Decisões do usuário (26/10/2026, 2ª sessão)

- **Contexto dos plugins (K01):** todo plugin recebe o contexto barato, montado sem rodar nada: `environment`, `infra` (kube_context, namespace e o caminho do kube_config do ambiente) e `values`, lidos do `.yby/environments.yaml`. Os hooks dos outros plugins, o overview, o backlog e o UKIIndex vão só para quem declarar `consome_contexto`.
- **Ambiente sem `-c` (K01/V02):** o `infra` do ambiente só vale quando o ambiente foi pedido (`-c`/`--context` ou `YBY_ENV`). Sem isso, Viz e Sentinel continuam no current-context do kubeconfig, como a ajuda promete. O `current` do environments.yaml não muda o `yby viz` nem o `yby sentinel` puros.
- **Embeddings locais (Y06):** modelo multilíngue quantizado (paraphrase-multilingual-MiniLM-L12-v2 ou multilingual-e5-small em int8, ~120 MB), se o hugot em Go puro carregar o formato. Se não carregar, a tarefa fica `[!]` e o usuário é avisado.
- **Formato de UKI (Y07):** a especificação é traduzida para PT-BR (tipos `decisao`, `regra`, `padrao`, `metrica`…; status `ativo`, `rascunho`…), com frontmatter e datas preenchidas pelo código. O nome continua `UKI-<timestamp>-slug.md` em `.synapstor/.uki/`, com subpastas por domínio aceitas. O parser lê também o formato antigo em negrito.
- **Execução em ondas e trilhas paralelas**, cada trilha numa branch `feat/correcoes-<trilha>` e num worktree próprio, com um PR por trilha. Detalhe na seção "Ondas" abaixo.

## Ondas

A conferência de 26/10/2026 (um agente por fase, só leitura) mapeou os arquivos de cada tarefa. As trilhas não disputam arquivos entre si; o que é compartilhado foi para a onda 0.

| Onda | Trilha (branch) | Tarefas, nesta ordem | Depende de |
|---|---|---|---|
| 0 | base (`feat/correcoes-plugins`) | C02, C01, R01, K01 | — |
| 1 | sentinel (`feat/correcoes-sentinel`) | S10, S11, S12, S02, S13, S09, S03, S05, S06, S08, S07, S04, S01, S14, S15 | onda 0 |
| 1 | viz (`feat/correcoes-viz`) | V01, V06, V03, V05, V07 (benchmarks), V04, V07 (o resto), V02 | onda 0 |
| 1 | atlas (`feat/correcoes-atlas`) | A12, A11, A05, A09, A01, A06, A07, A04, A08, A03, A02, A10, A13 | onda 0; A10 depois do B10 |
| 1 | synapstor (`feat/correcoes-synapstor`) | Y01, Y02, Y03, Y09, Y04, Y05, Y06, Y07, Y08, Y10, Y11, Y12, Y13 | onda 0 |
| 1 | bard (`feat/correcoes-bard`) | B05, B08, B10, B03, B06, B04, B01, B09, B11, B14, B13 | onda 0 |
| 2 | contratos (`feat/correcoes-contratos`) | K02, K04, K03, B07, B02, B12, B15 (o lado do Bard no Y01, Y04 e Y05) | integração local das trilhas da onda 1 |
| 3 | fecho | P01, P03, P02 | onda 2 e a release |

**Arquivos quentes entre trilhas** (merge com cuidado): `pkg/ai/prompts/defaults.go` (B03, B11, K03, Y07, Y08, Y13, A02, A10, cada um no seu prompt), `pkg/ai/factory.go` (B13 e B14 no `GetProvider`; Y01 e Y06 no `GetEmbeddingProvider`), `CLAUDE.md`.

## Como retomar

Use `/retomar` ao abrir a sessão e `/encerrar` ao parar.

1. A próxima tarefa de cada trilha é a primeira `[~]` dela. Se não houver nenhuma, é a primeira `[ ]` na ordem da tabela "Ondas".
2. **Cada tarefa:**
   - escreva primeiro um teste que falhe sem a correção (quando der) e confira que ele falha;
   - corrija;
   - rode o "Verificar" da tarefa;
   - faça **um commit** `fix(<plugin>): …` com o código, a caixa `[x]` e uma linha na tabela da fase no Histórico (uma tabela por fase, para as trilhas não conflitarem).
3. **Ao fechar uma trilha:** rode a verificação geral, faça push e abra o PR para a `develop`. É correção, então push e PR vão direto, sem perguntar.
4. **Antes de começar uma onda,** a anterior tem de estar mergeada; as trilhas saem da `origin/develop`.

**Legenda:** `[ ]` pendente · `[~]` em andamento · `[x]` concluída · `[!]` bloqueada (motivo no Histórico) · `[-]` descartada (motivo no Histórico)

**Branches:** sempre `feat/**`, porque só nessas branches roda o check obrigatório "🧪 Validate Go Code". Um PR por trilha.

**Regras deste repo que valem aqui:**
- Tudo em PT-BR.
- Saídas pelo `pkg/ui`.
- Erros com `pkg/errors`.
- Contexto nos subprocessos.
- Capturas de tela do Bard e dos plugins só com IA falsa.
- Nada contra o `~/.kube` nem o `~/.yby` do usuário.
- Heredoc no Bash engole `\\`: escreva arquivos com barra invertida pelo Write/Edit.

---

### Fase 0 · CI e base

- [x] **C01 · CI roda os testes do Sentinel, do Viz e do Synapstor** ✓
  - **Problema:** `.github/workflows/feature-pipeline.yaml:35` faz `go list ./... | grep -v 'plugins/sentinel/cli'`. Viz e Synapstor são módulos próprios, que o `go list ./...` da raiz não alcança. Resultado: os testes desses três plugins nunca rodam no PR.
  - **Fazer:**
    - incluir no job os passos `go test -race -tags k8s ./plugins/sentinel/...`, `cd plugins/viz && go test -race ./...` e `cd plugins/synapstor && go test -race ./...`;
    - fazer o mesmo no `release-pipeline.yaml`, se ele também rodar testes;
    - conferir se o runner Linux passa nos testes, que até hoje só rodaram no Windows.
  - **Verificar:** o check do PR mostra os três pacotes rodando.
- [x] **C02 · Lint do Sentinel de verdade e gofmt** ✓
  - **Problema:** `.golangci.yml` tem `exclude-dirs: plugins/sentinel/cli`, o que anula o passo "Lint K8s" do pipeline. O gofmt acusa `plugins/sentinel/cli/checks/rbac_allowlist.go` e `plugins/synapstor/internal/scanner/scanner_test.go`.
  - **Fazer:** tirar o exclude, corrigir o que o lint apontar no Sentinel e rodar `gofmt -w` nos dois arquivos.
  - **Verificar:** `golangci-lint run --build-tags k8s plugins/sentinel/cli/...` limpo, com a config v1 do CI. Localmente, migrar uma cópia da config: v2, com o revive `exported` desligado, `exclude-use-default` e os mesmos linters.
  - **Conferência (26/10):** o v1 não pula o diretório passado como argumento, então o exclude anula só os subpacotes do Sentinel. Viz e Synapstor nunca são lintados no CI: acrescentar `golangci-lint run` dentro de cada módulo. A regex `test` do exclude-dirs também tira o `pkg/testutil`: trocar por `^test/`, corrigindo o que aparecer.
- [x] **R01 · Pacote de redação compartilhado** (novo, saiu do B06 e do S11)
  - **Problema:** `sanitizeContent` (`plugins/bard/history.go:~18-51`) está no `package main` do Bard, e nem o `plugins/bard/tools` nem o Sentinel conseguem importar. Faltam padrões para JWT, credencial em URL, `Authorization:` genérico, AWS secret key e bloco PEM.
  - **Fazer:** pacote-folha `pkg/redacao` (só `regexp` e `strings`), com `Redigir(string) string`; o Bard passa a usá-lo no histórico e no `/exportar`. Os usos novos ficam com o B06 e o S11.
  - **Verificar:** testes de tabela com cada padrão e com texto que não pode ser redigido (ex.: `KEYCLOAK_URL=https://kc`).

### Fase 1 · Contratos entre os plugins e contexto

- [x] **K01 · Separar "fornece contexto" de "consome contexto"** ✓
  - **Conferência (26/10), que muda o Fazer:** o Sentinel consome contexto (`plugins/sentinel/cli/main.go:~120-131` lê `Infra.Namespace`/`KubeContext`; `pkg/plugin/sdk/sdk_k8s.go:~23-26` monta o client com `Infra`), e o V02 quer o mesmo para o Viz. Os dois declaram só `command` e hoje não recebem nada: `yby -c staging sentinel scan` ignora o ambiente. Pela decisão do usuário: (1) todo plugin recebe o contexto barato (`environment`, `infra` com o caminho do kube_config, `values`), só quando o ambiente foi pedido por `-c` ou `YBY_ENV`; (2) o caro só com `consome_contexto`; (3) o `yby init` (`cmd/init.go:~229`) continua usando o hook `context` como fornecedor; (4) `ExecuteContextHook` com contexto do chamador, e não `context.Background()`. Testes do manager com o processo auxiliar `GO_WANT_HELPER_PROCESS` (`pkg/plugin/executor_test.go`), que roda no Windows.
  - **Problema:** em `pkg/plugin/manager.go:~275-316`, quem declara o hook `context` recebe o contexto inteiro (`GetCoreContext` + os hooks de todos os plugins). Só o Atlas e o Synapstor declaram, e nenhum dos dois usa o `req.Context` (`plugins/atlas/main.go:~82`, `plugins/synapstor/main.go:~38-45`). Por isso todo `yby atlas …` e `yby synapstor …`, até o `--help`, varre o repo e manda mais de 128 KB a quem descarta. Foi a origem do E2BIG do #217.
  - **Fazer:**
    - campo novo no manifesto (ex.: `consome_contexto bool`, com JSON estável); `needsContext` passa a olhar esse campo e não o hook;
    - nenhum plugin atual consome;
    - não montar contexto quando os args forem `--help`/`-h`;
    - manter o `UKIIndex` do core (`pkg/context/core.go:~92-110`) só se alguém ler; hoje ninguém lê;
    - o Synapstor embrulha o payload duas vezes (`main.go:~130,177-179`): desembrulhar.
  - **Verificar:** teste do manager mostrando que um plugin com hook `context` e sem `consome_contexto` abre sem rodar os hooks; `yby synapstor --help` instantâneo num repo grande.
- [x] **K02 · `atlas_blueprint` do Bard lê o que o Atlas manda** ✓
  - **Problema:** o Atlas responde `{"data":{"blueprint":{…}}}` (`plugins/atlas/main.go:~68-76`). `formatBlueprintResults` procura `components`/`relations`/`languages` no topo (`plugins/bard/tools/atlas_blueprint.go:~55-95`), e `languages` nem existe. A IA recebe só "## Blueprint do Projeto". O teste `plugins/bard/tools/atlas_test.go:~64-76` passa porque usa um formato inventado.
  - **Fazer:**
    - desembrulhar `data.blueprint`;
    - resumir com nomes, não só contagens: componentes, charts, workloads e relações;
    - se o `InfraBlueprint` estiver disponível no hook, usá-lo;
    - limite de tamanho (~8 KB).
  - **Verificar:** teste de contrato que gera a saída com o código real do Atlas (chamar `executar()`/`respond` num buffer, ou um golden gerado pelo Atlas) e a passa pelo `formatBlueprintResults` do Bard.
- [x] **K03 · Bard sem a seção de blueprint que nunca chega** ✓
  - **Problema:** o Bard declara só `command`, então `ctxData` chega nil (`plugins/bard/main.go:~120-128`) e a seção "Technical Blueprint (Atlas)" do system prompt (`main.go:~290-296`) sai sempre "Nenhum blueprint disponível."
  - **Fazer:** tirar a seção morta; o blueprint chega pela ferramenta (K02). Se o resumo for pequeno e barato, dá para pedir ao Atlas um resumo compacto e em cache na abertura. Decida pelo custo medido.
  - **Verificar:** o system prompt não promete o que não tem; teste do prompt.
  - **Conferência (26/10):** as seções "Project Overview" e "Backlog & Debt" do `buildSystemPrompt` também estão mortas (leem chaves no topo; os dados viriam em `ctxData["data"]`). Manter o placeholder `{{blueprint_json_summary}}` vazio, por compatibilidade com overrides. Não pedir resumo ao Atlas na abertura.
- [x] **K04 · Testes de contrato entre os plugins**
  - **Problema:** os bugs do Sentinel (PR #218), do K02 e do B05 passaram com a suíte verde porque os fixtures inventam formatos.
  - **Fazer:** goldens em `testdata` gerados por quem escreve (o Sentinel grava o JSON do scan e do investigate; o Atlas grava o blueprint) e lidos pelos testes de quem consome (as tools do Bard). Um teste de cada lado: o escritor confere que o golden está atual (com `-update`) e o leitor o consome.
  - **Verificar:** mudar um campo no escritor sem atualizar o golden quebra o teste.

### Fase 2 · Bard

- [x] **B01 · Descoberta dos plugins com `.exe` e com confiança** ▶
  - **Problema:** `plugins/bard/tools/plugin_helper.go:~16-34` procura `yby-plugin-<nome>` e `<nome>` sem `.exe`, então no Windows o Sentinel e o Atlas nunca rodam pelo Bard. Também roda `./.yby/plugins/<nome>` do diretório atual sem a verificação de confiança do core: um repo clonado pode trazer um binário.
  - **Fazer:** de preferência, o core passa ao Bard os caminhos já resolvidos e confiáveis dos plugins (no pedido ou numa env, ex.: `YBY_PLUGINS` com JSON nome→caminho), e o Bard usa só esses. Sem o core (binário direto), procurar com `.exe` no Windows e só em `~/.yby/plugins`.
  - **Verificar:** teste por GOOS; teste de que `./.yby/plugins` sem confiança não roda.
- [x] **B02 · Memória da conversa atual** ✓
  - **Problema:** `tui/model.go:~670` chama `m.fluxo.Iniciar(ctx, m.rodada, input)` só com a pergunta, e o `Fluxo` (`tui/fluxo.go`) monta o prompt só com `entrada` e um `SystemPrompt` fixo. O histórico de sessões anteriores entra (`main.go:~231-234`), mas a sessão atual não.
  - **Fazer:** o `Fluxo` recebe as últimas N trocas da sessão, com orçamento de tokens (B07), e as usa na resposta e no classificador. Vale também para o batch.
  - **Verificar:** teste do `Fluxo` com duas rodadas: a segunda pergunta ("e os logs dele?") chega ao provider falso com a primeira troca.
- [x] **B03 · Classificador: namespace da sessão, esquema de parâmetros e validação**
  - **Problema:**
    - o prompt manda usar "default" quando não há namespace (`pkg/ai/prompts/defaults.go:~169`), e `namespaceDe` prefere o parâmetro da IA ao da sessão (`tools/kube.go:~21-26`);
    - o classificador recebe só `intent: descrição` (`tui/model.go:~968-1003`), sem os nomes dos parâmetros nem o cluster/namespace da sessão;
    - `params map[string]string` quebra com `"tail": 100` numérico;
    - texto antes do JSON quebra o parse (não usa `extractJSON`);
    - erro vira `Direct:true` sem log;
    - só o primeiro intent de cada tool é listado;
    - a ordem do registry é aleatória.
  - **Fazer:**
    - tirar a regra do "default";
    - mandar o namespace e o contexto da sessão;
    - mandar o esquema de parâmetros de cada tool;
    - parse tolerante (número→string, JSON no meio do texto);
    - `slog.Debug` nas falhas;
    - todos os intents;
    - ordem estável;
    - validar os obrigatórios antes de executar e devolver erro claro.
  - **Verificar:** testes de tabela do parse e do prompt montado.
- [x] **B04 · Ferramentas externas (YAML) com intents, precedência certa e confiança** ▶
  - **Problema:** `registerExternalTool` não copia `def.Intents` (`tools/loader.go:~109-117`), então nenhuma tool YAML é escolhida. A global carrega antes e o `continue` (`loader.go:~72`) descarta a do projeto, ao contrário do comentário. Ao consertar, uma tool de `.yby/tools` de um repo clonado roda `sh -c` com valores da IA, sem confiança, sem ctx e sem aspas (`loader.go:~147`).
  - **Fazer:**
    - copiar os intents;
    - o projeto vence a global;
    - tools do projeto exigem confiança, como os plugins (reusar o mecanismo de trust do core, ou ignorar com aviso enquanto não houver confiança);
    - `exec.CommandContext` com timeout;
    - parâmetros passados por env ou escapados, nunca interpolados crus no `sh -c`.
  - **Verificar:** `loader_test.go` confere `Intents` e a precedência; teste de que o parâmetro `"; rm -rf ~"` não executa nada.
  - **Conferência (26/10):** entra também o override de prompt do projeto: `prompts.Get` lê `.yby/prompts/bard.*.txt` do diretório atual (`pkg/ai/prompts/prompts.go:~28`), e um repo clonado reescreve o system prompt sem confiança. No Windows não há `sh` sem o Git Bash: rodar sem shell (argv com os parâmetros inteiros) ou recusar com mensagem clara, nunca interpolar.
- [x] **B05 · Notas do Synapstor somem quando uma tool roda** ✓
  - **Problema:** `tui/fluxo.go:~232` refaz `entradaFinal` a partir de `entrada` e descarta o bloco de notas montado em `~183-186`.
  - **Fazer:** compor as notas e o resultado da ferramenta.
  - **Verificar:** teste do `Fluxo` com tool + Synapstor falsos: o prompt tem os dois.
- [x] **B06 · Secrets e dados sensíveis não vão à IA**
  - **Problema:** `kubectl_get` aceita `resource` e `output_format` livres (`tools/kubectl_get.go`), então `get secrets -o yaml` vai inteiro à IA. O `sanitizeContent` só roda no histórico e no `/exportar` (`history.go:~182`, `comandos_acoes.go:~367`). Logs também vão crus.
  - **Fazer:**
    - bloquear `secret(s)` com `-o json|yaml|jsonpath|go-template` (ou redigir `data`/`stringData`);
    - restringir `output_format` a vazio, `wide` e `name`;
    - aplicar `sanitizeContent` à saída de toda tool antes do prompt.
  - **Verificar:** testes.
  - **Conferência (26/10):** usa o `pkg/redacao` do R01. Entra também a injeção de flag: valores da IA que começam com `-` viram flags do kubectl em todas as `kubectl_*` (`--as=system:admin`, `--raw=…`); validar nomes e recursos. O bloqueio cobre `secret`, `secrets` e `sec`.
- [x] **B07 · Orçamento de tamanho do que vai à IA**
  - **Problema:** nada limita o tamanho:
    - blueprint com `MarshalIndent`;
    - 20 mensagens inteiras de histórico;
    - todos os pods do namespace (`context_enricher.go:~88-130`);
    - notas inteiras do Synapstor (`fluxo.go:~296`);
    - saída das tools sem corte (o scan indentado em `sentinel_scan.go`, `kubectl_events` sem limite, `tail` livre).
    
    O `TokenAwareProvider` assume 8192 de janela para modelos desconhecidos e para o `llama3` (`pkg/ai/tokencount.go:~49-55`). Com o Ollama padrão, perguntas com tool falham com ERR_TOKEN_LIMIT.
  - **Fazer:**
    - orçamento por bloco (tool ≤ ~8 KB com "… N linhas omitidas", notas ≤ ~8 KB no total, histórico pelo orçamento);
    - o scan do Sentinel resumido (contagem por severidade + os N mais graves com recursos);
    - JSON compacto em vez de indentado.
  - **Verificar:** testes de corte.
- [x] **B08 · Retry só do que é retentável**
  - **Problema:** `tui/fluxo.go:~239-261` tenta de novo qualquer erro por 30 s. Por dentro, o `RetryProvider` (`pkg/ai/retry_provider.go`) já tenta 429/502/503 por até 2 min. Chave inválida, modelo inexistente e ERR_TOKEN_LIMIT mostram "tentando de novo" quatro vezes.
  - **Fazer:** o `Fluxo` reusa a classificação de erro do `pkg/ai` e marca como permanente o resto, incluindo YbyError de config e de token.
  - **Verificar:** teste com erro permanente: uma tentativa só.
  - **Conferência (26/10):** o `RetryProvider` (`pkg/ai/retry_provider.go:~105-130`) repete qualquer erro que não seja `APIError`, inclusive o ERR_TOKEN_LIMIT do `TokenAwareProvider`: 2 min de espera antes do erro. Corrigir a classificação no `pkg/ai` primeiro. Os testes que usam `errors.New` esperando nova tentativa (`direto_test.go`, `fluxo_test.go`) passam a usar `APIError`.
- [x] **B09 · Subprocessos com contexto e prazo**
  - **Problema:** `invokePlugin` usa `exec.Command` sem ctx (`tools/plugin_helper.go`), e o esc durante um scan deixa o Sentinel rodando. O `EnrichContext` usa `context.Background()` (`main.go:~207,225`) e chama o kubectl em sequência sem `--request-timeout` (`context_enricher.go:~159`), então com o cluster fora do ar a TUI demora a abrir.
  - **Fazer:** `exec.CommandContext` nas tools; `--request-timeout=5s` e prazo total de ~5 s no enriquecimento.
  - **Verificar:** testes com comando falso lento.
- [x] **B10 · Providers CLI: prompt pelo stdin e cancelamento**
  - **Problema:** `pkg/ai/claude_cli.go` e `pkg/ai/gemini_cli.go` passam system prompt, histórico, blueprint e saída da tool num único argumento de `-p`, com `exec.Command` sem ctx. Isso estoura no Windows (~32 K) e no Linux (128 KiB por argumento), que é a mesma classe do #217, e o esc não mata o `claude`.
  - **Fazer:** prompt pelo stdin (confirmar a forma suportada pelo `claude -p` e pelo `gemini -p` atuais) e `exec.CommandContext`.
  - **Verificar:** teste com comando falso conferindo stdin e cancelamento; teste real local com o `claude` instalado, se houver.
- [x] **B11 · System prompt coerente; parser e guardrails mortos**
  - **Problema:**
    - o `BardSystem` diz "Voce NAO precisa chamar nenhuma ferramenta manualmente" (`pkg/ai/prompts/defaults.go:~47-48`) e injeta `FormatToolsPrompt`, que manda responder com bloco JSON (`tools/registry.go:~50-52`);
    - `ParseToolCalls`/`ValidateToolCall` só aparecem nos testes;
    - o prompt ainda incentiva usar "kubectl, helm, MCP" do runtime (`defaults.go:~54-55`), o que, com o claude-cli, empurra o `claude` a rodar comandos fora das guardas do Bard.
  - **Fazer:**
    - tirar a instrução de JSON e a de usar as capacidades do runtime para ações de escrita;
    - apagar o parser morto, ou aplicar os guardrails no ponto único de execução das tools, o que for coerente com o código atual;
    - os guardrails que fizerem sentido passam a valer na execução.
  - **Verificar:** testes do prompt e, se ficarem, testes dos guardrails no caminho de execução.
- [x] **B12 · Ferramentas que entregam errado**
  - **Problema:**
    - `kubectl_events` não filtra pelo objeto apesar do intent `pod_events`, e devolve o namespace inteiro;
    - `kubectl_logs` sem `container` falha em pod com vários contêineres e não tem `previous` (no CrashLoop, o log atual vem vazio);
    - `sentinel_investigate` faz o Sentinel chamar a própria IA, o que dá três chamadas de LLM por pergunta, com o provider do Sentinel e não o do `/modelo`, e cache de 1 h impossível de pular.
  - **Fazer:**
    - `field-selector involvedObject.name=` quando houver pod;
    - parâmetros `container` e `previous` (e mostrar os contêineres no erro);
    - no investigate, usar o modo de dados brutos do Sentinel (S12) em vez da análise por IA dele.
  - **Verificar:** testes dos args montados.
- [x] **B13 · Cabeçalho do Bard com o modelo do config** (pendência antiga)
  - **Problema:** o cabeçalho não mostra o modelo vindo de `ai.model` do config.
  - **Fazer:** mostrar o modelo do config.
  - **Verificar:** golden da TUI.
- [x] **B14 · Providers CLI sem a cadeia de decorators** (pendência antiga)
  - **Problema:** claude-cli e gemini-cli não passam pelo `wrapProvider` (`pkg/ai/factory.go`), então ficam sem retry, token-aware e custo.
  - **Fazer:** passar os dois pela cadeia, se ela fizer sentido para eles.
  - **Verificar:** teste da factory.
- [x] **B15 · O Bard usa o índice do Synapstor como ele é** (lado do Bard do Y01, Y04 e Y05)
  - **Problema:** `tui/fluxo.go:~306-310` mostra "Synapstor indisponível" para qualquer erro, inclusive o índice feito com outros embeddings (`ai.ErrIndiceDeOutroModelo`, do Y01). O corte de relevância é 0,6 no Bard (`config.go:~21`) e 0,35 no Synapstor (`search.go:~21`). Desde o Y05 o índice guarda trechos de seção, mas o Bard injeta o trecho só com o título e pode repetir o mesmo arquivo.
  - **Fazer:**
    - erro claro na etapa de contexto: "o índice foi feito com X; rode yby synapstor index --full";
    - o corte de relevância num lugar só (`pkg/ai`), usado pelo Synapstor e pelo Bard; o `relevance_threshold` do config do Bard continua valendo quando definido;
    - injetar o trecho com o caminho e a seção, dentro do teto de notas do B07.
  - **Verificar:** teste do `Fluxo` com buscador falso para o erro, o corte e a injeção.

### Fase 3 · Synapstor

- [x] **Y01 · Índice versionado por provider/modelo/dimensão** ✓▶
  - **Problema:** `GetEmbeddingProvider` (`pkg/ai/factory.go:~197-226`) escolhe conforme o momento (Ollama se no ar, senão config, senão MiniLM local com 384 dimensões). A coleção tem nome fixo `synapstor-knowledge` (`pkg/ai/vector_store.go:~41`) e o manifest só guarda sha256 e data (`plugins/synapstor/internal/indexer/indexer.go:~19-28`). Trocando de provider, a busca falha com `vectors must have the same length` (reproduzido neste repo). No Bard isso vira só "Synapstor indisponível" (`plugins/bard/tui/fluxo.go:~287-290`).
  - **Fazer:**
    - gravar provider, modelo e dimensão no manifest (ou no nome da coleção, ex.: `synapstor-<modelo>-<dim>`);
    - o `index` refaz tudo sozinho quando mudar;
    - `search` e Bard com erro claro: "o índice foi feito com X; rode yby synapstor index --full".
  - **Verificar:** teste com provider falso de dimensão configurável.
  - **Conferência (26/10):** a identidade do índice (provider, modelo e dimensão) mora no `VectorStore` de `pkg/ai`, e não só no manifest do Synapstor, porque o Bard precisa dela para o erro claro. O Ollama sem config usa o primeiro modelo do `/api/tags`, e o `Name()` não traz o modelo: expor o modelo efetivo pelos decorators. O lado do Bard (mensagem em `tui/fluxo.go`) fica na onda 2.
- [x] **Y02 · `--full` refaz de verdade; órfãos; IDs com `/`**
  - **Problema:**
    - o `--full` só regrava pelo mesmo ID (`indexer.go:~175`) e nunca limpa, e só sai do índice o que está no manifest (`indexer.go:~217-224`);
    - neste repo há 183 trechos antigos de docs que não existem mais;
    - o ID e o `source` saem de `filepath.Rel`, com `\` no Windows.
  - **Fazer:**
    - `--full` apaga e recria a coleção;
    - conciliar órfãos (tudo na coleção que não está no manifest sai);
    - `filepath.ToSlash` no ID, no `source` e nas chaves do manifest;
    - a dica "rode index --full" (`search.go:~108`) passa a ser verdade.
  - **Verificar:** testes.
- [x] **Y03 · Manifest sem índice**
  - **Problema:** o `.gitignore` ignora `.synapstor/.index`, mas não `.synapstor/.index_manifest.json`. Num clone com o manifest, o `index` pula tudo por hash (`indexer.go:~175-180`) e o `search` diz "índice vazio".
  - **Fazer:** com a pasta do índice ausente ou `Count()==0`, tratar o manifest como vazio. De preferência, guardar o manifest dentro de `.synapstor/.index/`.
  - **Verificar:** teste.
- [x] **Y04 · TopK e corte de relevância**
  - **Problema:** o chromem recusa `nResults > len(documents)`. O Synapstor limita (`search.go:~117`), mas o Bard passa `TopK=5` direto (`plugins/bard/tui/fluxo.go:~287`, `plugins/bard/config.go:~20`), então com menos de 5 UKIs o RAG some. O corte é 0,35 na busca (`search.go:~20`) e 0,6 no Bard (`plugins/bard/config.go:~21`).
  - **Fazer:** limitar `min(topK, Count())` dentro de `VectorStore.Search` (`pkg/ai/vector_store.go`), com vazio quando não houver documento, e definir o corte num lugar só.
  - **Verificar:** teste com 2 documentos e TopK 5.
- [x] **Y05 · Divisão em trechos**
  - **Problema:** o arquivo inteiro vira um vetor (`indexer.go:~188-207`), e o `splitMarkdown` (`~309-351`) é código morto. O MiniLM corta em 2000 bytes, podendo partir um caractere UTF-8 (`pkg/ai/local_embedding.go:~211-216`). O Bard injeta o documento inteiro (`fluxo.go:~293-296`): até 5 docs por pergunta.
  - **Fazer:**
    - dividir por H2, com ~1500 caracteres e sobreposição; ID `caminho#n`; título e seção nos metadados e no começo do trecho;
    - corte do MiniLM respeitando runas;
    - o Bard injeta só o trecho e o caminho, com teto (B07);
    - ignorar `_Footer.md`/`_Sidebar.md`.
  - **Verificar:** testes do split e do corte.
  - **Conferência (26/10):** apagar os trechos do arquivo (por `source`) antes de readicionar, senão um UKI que encolhe deixa trechos órfãos. O resumo do `index` passa a contar trechos e arquivos. A injeção no Bard (`tui/fluxo.go`) fica na onda 2.
- [x] **Y06 · Embeddings: dizer qual é, avisar o download e usar modelo que entende PT-BR**
  - **Problema:** `GetEmbeddingProvider` nunca devolve nil, então `semEmbeddings` e as checagens de nil (`comandos.go:~46-50,152-155`, `search.go:~87-90`) nunca rodam. Sem Ollama, tudo "funciona" com o all-MiniLM-L6-v2, que só entende inglês, contra UKIs em PT-BR. O primeiro uso baixa ~90 MB só com `slog.Info`.
  - **Fazer:**
    - campo "embeddings: <provider/modelo>" no resumo do `index` e do `search`;
    - etapa "baixar o modelo (~90 MB)";
    - modelo local multilíngue como padrão (multilingual-e5-small ou paraphrase-multilingual-MiniLM-L12-v2: conferir o formato que o `local_embedding.go` carrega e o tamanho). A troca de modelo depende do Y01;
    - remover o erro morto.
  - **Verificar:** teste do resumo; teste manual de busca em PT-BR.
- [x] **Y07 · Um formato só de UKI**
  - **Problema:**
    - `.synapstor/UKI_SPEC.md` exige frontmatter (`uki_id`, `type`, `status`, `created_at`, `updated_at`, `tags`) e subpastas `.uki/<dominio>/`;
    - os prompts geram `**ID:**`/`**Tipo:**` sem frontmatter (`pkg/ai/prompts/defaults.go:~94-104`);
    - o quality procura `**ID:**` (`quality.go:~95-97`);
    - quality, decay, export e o `indexUKIs` do core leem só o primeiro nível (`quality.go:~108`, `decay.go:~56`, `exporter.go:~41`, `pkg/context/core.go:~94`), enquanto o indexador desce nas subpastas;
    - o export acrescenta um segundo frontmatter (`exporter.go:~90-99`).
  - **Fazer:**
    - uma função `listarUKIs` e um parser de metadados (aceita frontmatter e o formato antigo em negrito), usados por todos;
    - prompts de capture e study gerando o formato da especificação;
    - quality aceitando as seções da especificação ("## Contexto" sem depender da caixa).
  - **Verificar:** testes com UKI nos dois formatos e em subpasta.
- [x] **Y08 · `study`: busca, prioridade, idioma e prompt de correção**
  - **Problema:**
    - `scanner.go:~25,84,100` procura a frase inteira como substring;
    - o limite de 50 conta os achados por conteúdo antes dos melhores (simulado: `pkg/services/bootstrap` ficou fora para "bootstrap");
    - não respeita `.gitignore` e inclui `_test.go` e testdata;
    - `SynapsotorStudy` está em inglês (`defaults.go:~115-147`), e a mensagem do usuário também (`agent.go:~110`);
    - a correção usa o prompt de capture (`agent.go:~273`).
  - **Fazer:**
    - termos sem palavras vazias;
    - pontuar nome > caminho > conteúdo, ordenar e depois cortar;
    - `git ls-files` quando houver git;
    - pular testes e goldens;
    - prompt em PT-BR no formato da especificação;
    - correção com o prompt do próprio comando.
  - **Verificar:** testes do scanner com fixture de mais de 50 achados.
- [x] **Y09 · Arquivo ilegível não é apagado; trava do índice**
  - **Problema:** erro de leitura faz `continue` (`indexer.go:~169-186`), e o arquivo some do manifest e cai em `removedFiles`. O arquivo é lido duas vezes, e não há trava contra dois `index` ao mesmo tempo.
  - **Fazer:** manter a entrada antiga e avisar; hash do conteúdo já lido; `.synapstor/.index.lock`.
  - **Verificar:** testes.
- [x] **Y10 · `decay` mede o código, não só a nota**
  - **Problema:** `decay.go:~116-117` roda `git log` só no próprio UKI. Sem histórico, usa o timestamp do nome (`decay.go:~88-93`), que a IA escolhe (`agent.go:~177-179` aceita qualquer `UKI-…`). Um `UKI-20261026-x` pode ser lido como unix de 1970. UKI não commitado dá 0 dias.
  - **Fazer:** extrair os caminhos citados no UKI e contar os commits neles depois do último commit do UKI; usar o `updated_at` do frontmatter (Y07); validar o timestamp do nome.
  - **Verificar:** testes com repo git temporário.
- [x] **Y11 · `quality` pelos critérios da especificação**
  - **Problema:** um bloco de código vale 20 pontos (`quality.go:~53-56`), o que pune Decisões, e o tamanho vale até 20.
  - **Fazer:** critérios por tipo, vindos do `UKI_SPEC.md` (ex.: Decisão: Problema, Opções, Decisão, Consequências).
  - **Verificar:** testes por tipo.
- [x] **Y12 · Export**
  - **Problema:** `Tags` fica sempre vazio (`exporter.go:~17,93-98`), UKIs apagados continuam no destino e o H1 duplica o título.
  - **Fazer:**
    - tags do frontmatter (ou do tipo);
    - limpar só o que o export gerou (marcar com um manifesto no destino);
    - tirar o H1 duplicado;
    - conferir a hipótese do MDX no Docusaurus 3 (`{{ }}`, `<pod>`) e escapar ou documentar.
  - **Verificar:** testes.
- [x] **Y13 · Código morto**
  - **Problema:** código sem uso:
    - `internal/graph`;
    - `internal/tagger`, a menos que vire as tags do Y12;
    - `splitMarkdown`, que vira o Y05;
    - `quality.FormatScore`, `decay.FindStale`;
    - o prompt `synapstor.tagger`;
    - o campo `Summary` descartado (`agent.go:~26`): usar como metadado ou tirar do prompt;
    - `semEmbeddings` (Y06).
  - **Fazer:** apagar ou ligar, conforme indicado em cada item.
  - **Decisão (26/10, execução):** `Summary` sai do prompt e do struct (nada o lê; o índice usa o título e o trecho); `internal/tagger` sai junto com o prompt `synapstor.tagger`, porque as tags já vêm da resposta da IA no capture (Y07); `internal/graph` sai sem consumidor e sem promessa na ajuda ou no README.
  - **Verificar:** `go vet`, testes.

### Fase 4 · Sentinel

- [x] **S01 · `--fix` corrige o dono do pod** ✓
  - **Problema:**
    - todos os patches são `ResourceKind: "Pod"` (`remediation/generator.go:~62-69`) aplicados no Pod (`remediation/applier.go:~33-35`), e o spec do Pod é imutável nesses campos (422). Nunca corrigiu nada num cluster real; daí os "erros crus";
    - `generatePatch` só conhece os IDs dos checks internos (`generator.go:~27-47`), não os `polaris/*`;
    - não há confirmação nem dry-run no servidor;
    - a dica aponta deployments (`scan.go:~300`);
    - limits fixos (500m/256Mi) e `readOnlyRootFilesystem` podem derrubar workloads.
  - **Fazer:**
    - subir pelos `ownerReferences` (Pod→ReplicaSet→Deployment; Pod→Job→CronJob; StatefulSet; DaemonSet) e montar o patch no caminho do template;
    - Job solto não é patcheável: avisar;
    - mapear os IDs `polaris/*`;
    - `--fix-dry-run` com `DryRun: ["All"]` no servidor e o diff;
    - `--fix` pede confirmação pelo Prompter (sem terminal, exige `--yes`);
    - recurso gerido pelo Argo CD (label/annotation de tracking): recusar com explicação;
    - não aplicar `readOnlyRootFilesystem` nem limits inventados sem `--yes` explícito; de preferência, só patches seguros (`allowPrivilegeEscalation:false`, `capabilities.drop`, `runAsNonRoot` quando a imagem permitir);
    - erros pelo `pkg/errors`.
    - Se isso ficar grande demais, o mínimo honesto é desligar o `--fix` com mensagem clara e registrar.
  - **Verificar:** testes com fake clientset e owner chain; dry-run no servidor com o reactor do fake.
  - **Conferência (26/10):** os achados do Polaris já vêm no controlador (Deployment/x), e o `validator.ResultMessage` tem `Mutations`, que podem gerar os patches `polaris/*`; o achado precisa guardar o contêiner. A cadeia de donos serve aos checks internos. Com o Polaris achando algo, hoje o `--fix` não gera patch nenhum. O `Prompter` está em `cmd/`: usar a confirmação do `pkg/ui`.
- [x] **S02 · Polaris no contexto e kubeconfig do yby**
  - **Problema:** `backends/polaris.go:~58,81-92` monta o cliente dinâmico com as regras padrão do kubeconfig, ignorando `Infra.KubeContext`/`KubeConfig` e o token cloud que o `sdk.GetKubeClient` respeita (`pkg/plugin/sdk/sdk_k8s.go`). Um mesmo scan mistura dois clusters.
  - **Fazer:** expor `sdk.GetRestConfig()` e montar os dois clientes a partir dele.
  - **Verificar:** teste do SDK; teste de que o backend recebe o config.
- [x] **S03 · "(+N)" não apaga os recursos**
  - **Problema:**
    - o grupo é por `check|severidade`, com `Resource = "X (+N)"` (`backends/polaris.go:~114,181-184`);
    - mensagem e recomendação vêm do primeiro recurso;
    - a tela corta em 32 colunas (`saida.go:~172-187`);
    - o relatório em `~/.yby/reports` grava só `Resource` (`scan.go:~376-379`).
  - **Fazer:**
    - campo `Resources []string` no achado, com `Resource` como o primeiro, por compatibilidade;
    - na tela, a linha do check e embaixo os recursos (até N, depois "e mais K"), sem reticências no meio do nome;
    - JSON e markdown com a lista inteira.
  - **Verificar:** golden da tela; teste do JSON.
- [x] **S04 · RBAC de escopo de cluster fora do scan de namespace**
  - **Problema:** `scanClusterAdmin`, `scanWildcard` e `scanSecretsAccess` olham o cluster todo (`backends/opa.go:~268-393`), e o Polaris carimba `Namespace` em todo achado (`polaris.go:~192`). Os mesmos achados se repetem em todo namespace e inflam as contagens.
  - **Fazer:**
    - no scan de namespace, só entram:
      - Roles e RoleBindings do namespace;
      - ClusterRoles referenciados por RoleBindings do namespace;
      - ClusterRoleBindings cujos subjects são ServiceAccounts do namespace;
    - o resto do cluster vai para `--scope cluster`, o que conserta a promessa "achados só do namespace pedido" da wiki;
    - campo `scope` no achado.
  - **Verificar:** testes com fake.
  - **Conferência (26/10):** os checks internos `rbac_cluster_admin`, `rbac_wildcard` e `rbac_secrets_access` também olham o cluster todo; os checks RBAC do Polaris (`clusterrolebinding*`) passam o filtro de `polaris.go:~117`. O OPA não avalia RoleBindings (cluster-admin dado por RoleBinding passa), o wildcard ignora Roles e a mensagem diz sempre "ClusterRole".
- [x] **S05 · Severidade e categoria reais do Polaris**
  - **Problema:** a categoria é fixa em "pod-security" (`polaris.go:~190`) e danger/warning viram crítico/alto (`~203-211`), descartando `ResultMessage.Category` (Security, Efficiency, Reliability). "Defina a priority class" sai como alto em segurança; "tag da imagem", como crítico.
  - **Fazer:**
    - Security danger→crítico;
    - Security warning→alto ou médio por tabela de checks;
    - Reliability/Efficiency→baixo ou médio;
    - seções "confiabilidade" e "eficiência" na tela e no relatório.
  - **Verificar:** testes do mapeamento; golden.
- [x] **S06 · JSON coerente e em PT-BR**
  - **Problema:**
    - campos legados se contradizem (`checks/image_signature.go:~80,88`: `Severity: high` com `Type: "critical"`; `checks/pod_root.go:~59-60`);
    - as verificações que não rodaram (`naoRodaram`) só aparecem na tela;
    - as mensagens do Polaris saem em inglês no JSON (`saida.go:~105-107` traduz só na tela).
  - **Fazer:** `type` coerente com `severity` (ou fora do JSON, se ninguém ler); campo `nao_rodaram` (backend, verificações, motivo); mensagens traduzidas também no JSON e no markdown.
  - **Verificar:** testes do JSON.
- [x] **S07 · `--profile` faz o que promete**
  - **Problema:** o perfil só entra nos checks internos (`scan.go:~61-71,234`), que só rodam quando os backends não acham nada. `--profile pci-dss` devolve o mesmo Polaris.
  - **Fazer:** filtrar os achados de todos os motores pelo perfil, com uma tabela check→perfis que inclua `polaris/*` e `opa/*`, e dizer na tela qual perfil foi aplicado. Se a tabela não for defensável, tirar a flag e registrar.
  - **Verificar:** testes por perfil.
- [x] **S08 · Checks internos: papel definido e falsos positivos**
  - **Problema:**
    - os internos rodam quando os backends rodam **limpos**, não quando falham (`scan.go:~210-229`);
    - `cosign verify` sem chave nem identidade (`checks/image_signature.go:~69`) provavelmente falha sempre;
    - a substring "key" marca `KEYCLOAK_URL` como crítico (`checks/pod_secrets.go:~38-41`);
    - o `pod_root` ignora o `RunAsNonRoot` do pod (`checks/pod_root.go:~57`);
    - cada check lista os pods de novo.
  - **Fazer:**
    - internos só como fallback quando o backend **falha** (ou só no que o Polaris não cobre, mas decida e documente);
    - cosign só com chave ou identidade configurada (senão "não verificado", não achado);
    - casar nomes de env por palavra (`_KEY`, `KEY_`, `PASSWORD`, `TOKEN`, `SECRET`) e não por substring;
    - herdar o `securityContext` do pod;
    - listar os pods uma vez só.
  - **Verificar:** testes de tabela.
- [x] **S09 · Allowlist única e por subject**
  - **Problema:** são quatro cópias divergentes (`checks/rbac_allowlist.go:~44-76` e três no Rego em `backends/opa.go:~25-127`), e o casamento pelo nome deixa um `helm-qualquer` dar cluster-admin calado (`opa.go:~43-45`).
  - **Fazer:** uma fonte só, passada ao Rego como `data`; casar por subject (namespace/SA) e labels `managed-by`, não pelo nome do binding; "N ignorados pela allowlist" na tela e no JSON.
  - **Verificar:** testes do Rego.
- [x] **S10 · Investigate: saúde, logs e cache**
  - **Problema:**
    - um restart antigo deixa o pod "não saudável" para sempre (`investigate.go:~288-293`);
    - pod `Succeeded` conta como problema (`~279`);
    - `GetLogs` sem `Container` falha em pod com vários contêineres e não lê `Previous` (`~64-66`);
    - o cache é lido antes da checagem de saúde (`~77-84`), e a chave ignora o cluster e o UID (`cache.go:~33-40`);
    - os eventos vão inteiros, com `managedFields` (`~96`).
  - **Fazer:**
    - `FinishedAt` < 2 h;
    - `Succeeded` saudável;
    - logs por contêiner, com `Previous` nos que reiniciaram;
    - checar a saúde antes do cache;
    - chave com contexto, UID e `restartCount`;
    - eventos só com reason, message, count e lastTimestamp.
  - **Verificar:** testes.
- [x] **S11 · Redigir segredos antes da IA**
  - **Problema:** `investigate.go:~133` manda logs e eventos crus, e o provider pode ser remoto.
  - **Fazer:** redigir tokens, JWT, chaves AWS, senhas em URL e `Authorization:` antes de enviar (função compartilhada com o B06, se couber em `pkg/`).
  - **Verificar:** testes.
- [x] **S12 · `investigate --no-ai`**
  - **Problema:** coerência com o `scan --no-ai`; é o que o Bard precisa no B12.
  - **Fazer:** parar na coleta e devolver os dados brutos (status, eventos resumidos, fim dos logs) no JSON.
  - **Verificar:** testes.
- [x] **S13 · OPA compila uma vez**
  - **Problema:** `rego.New`+`Eval` por recurso (`backends/opa.go:~469-476`) e ClusterRoles listados duas vezes.
  - **Fazer:** `PrepareForEval` por política e listar uma vez só.
  - **Verificar:** testes existentes; benchmark simples.
- [x] **S14 · O relatório não se sobrescreve e diz o cluster**
  - **Problema:** `sentinel-scan-<ns>-<dia>.md` (`scan.go:~352-353`) faz o segundo scan do dia apagar o primeiro; o markdown não traz contexto, hora, perfil nem o que não rodou.
  - **Fazer:** contexto e hora no nome e no conteúdo, além do perfil e de "não rodaram"; atualizar a menção ao caminho na ajuda, no README e no CLAUDE.md.
  - **Verificar:** golden.
- [x] **S15 · Dívidas pequenas**
  - **Problema:** `backends/polaris.go:~45-46` silencia o `log` padrão e o logrus do processo inteiro; a dica do `--fix` aponta deployments (some com o S01).
  - **Fazer:** silenciar só durante a chamada do Polaris, ou redirecionar para o slog em debug.
  - **Verificar:** teste.

### Fase 5 · Atlas

> **Conferência (26/10):** os analyzers ficam em `plugins/atlas/discovery/analyzers/*.go`, e não em `plugins/atlas/analysis/` (lá estão só `diagram.go`, `cycles.go`, `metrics.go` e `diff.go`). O A12 (ordem determinística e goldens) e o A11 (`ToSlash`, caminhos relativos) vêm primeiro, porque os outros verificam por golden e mexem nos mesmos IDs. O A10 precisa reescrever o prompt `AtlasRefine` para manter os IDs do rascunho, senão quase toda resposta válida seria recusada.

- [x] **A01 · Relações falsas e duplicadas no blueprint**
  - **Problema:**
    - `detectDockerfileRelations` faz `strings.Contains(line, relPath)` (`plugins/atlas/discovery/scanner.go:~235-247`): o `.` casa com qualquer COPY e "api" casa com "apigateway";
    - `detectGoImportRelations` cria uma relação por pacote, sem deduplicar, e casa por `HasPrefix` sem fronteira (`~370-387`), lendo só a pasta raiz e só blocos `import (`.
  - **Fazer:**
    - deduplicar por (from, to, type);
    - casar o caminho do COPY por segmento normalizado com `ToSlash`;
    - nunca casar `.`;
    - o módulo mais longo com fronteira `/`;
    - aceitar `import "x"` de linha única.
  - **Verificar:** testes com fixture.
- [x] **A02 · `--detail full` não é desfeito pela IA**
  - **Problema:** o refinamento roda em qualquer nível (`plugins/atlas/main.go:~305-310`) com o prompt "no maximo 15-25 nos" (`pkg/ai/prompts/defaults.go:~181,190`). As estatísticas impressas são do diagrama programático, não do gravado.
  - **Fazer:** pular a IA no `full` (ou usar um prompt que preserve todos os nós); estatísticas do que foi gravado.
  - **Verificar:** teste.
- [x] **A03 · Overview sem sumir com o que não tem relação; teto programático**
  - **Problema:** `diagram.go:~326-333` tira todo recurso sem aresta, e Terraform e Compose sem `depends_on` somem. `GetInfraStats` (`~394-403`) não aplica o mesmo filtro. O "max 25 nós" do CLAUDE.md só existe no prompt da IA, e com `--no-ai` não há teto.
  - **Fazer:** manter workloads, charts, módulos e serviços isolados; teto programático (agrupar por kind acima de N); estatísticas dos nós desenhados.
  - **Verificar:** goldens (A12).
- [x] **A04 · Kustomize**
  - **Problema:**
    - o recurso se chama pelo diretório relativo inteiro (`analysis/kustomize.go:~81-92`), mas a referência usa só o último segmento (`~135-160`): os IDs nunca batem;
    - `resources: [deploy.yaml]` vira `Kustomization/deploy.yaml`;
    - o `patch.yaml` do overlay vira um segundo `Deployment/api` (`scanner.go:~633-651`);
    - `patches`, `helmCharts`, `images` e `namespace` são ignorados.
  - **Fazer:**
    - referência resolvida para o caminho relativo normalizado e usada como ID;
    - ligar a Kustomization aos recursos dos arquivos listados;
    - excluir do analyzer K8s os arquivos de `patches`/`patchesStrategicMerge`;
    - aplicar o `namespace` herdado.
  - **Verificar:** fixture base+overlay e golden.
- [x] **A05 · Service seleciona workload**
  - **Problema:** `analysis/k8s.go:~204-219` compara o selector com `metadata.labels` de qualquer recurso. O Service "seleciona" ConfigMap e não o Deployment.
  - **Fazer:** só workloads (Deployment, StatefulSet, DaemonSet, Pod, Job), via `spec.template.metadata.labels`, no mesmo namespace.
  - **Verificar:** teste.
- [x] **A06 · Compose com sintaxe longa e overrides**
  - **Problema:** `Ports []string`/`Volumes []string` (`analysis/compose.go:~24,26`): a sintaxe longa derruba o arquivo inteiro. Só nomes exatos contam (`scanner.go:~519-527`).
  - **Fazer:** `UnmarshalYAML` tolerante; aceitar `*compose*.y*ml` e `docker-compose.*.yml`.
  - **Verificar:** teste.
- [x] **A07 · Helm: nome dos templates**
  - **Problema:** `nameRe` com `(\S+)` (`analysis/helm.go:~189`) captura `{{` e tudo vira `Deployment/{{`, que o diagrama descarta. `templates/` sem recursão (`~195`). `HelmRepository` duplicado entre charts. O check de `_helpers.tpl`/`NOTES.txt` (`~211`) nunca é alcançado.
  - **Fazer:**
    - nome templado → `<chart>-<kind>` (ou `<chart>/<arquivo>`);
    - descer em subpastas de `templates/`;
    - deduplicar repositórios;
    - limpar o check inalcançável.
  - **Verificar:** fixture de chart real e golden.
- [x] **A08 · Terraform**
  - **Problema:**
    - `module "net" { source = "../../modules/net" }` gera um self-loop (`analysis/terraform.go:~135-141`);
    - o módulo não liga aos recursos do filho;
    - `.terraform` é varrido (`main.go:~155`);
    - nomes iguais em `envs/dev` e `envs/prod` colidem no ID;
    - não há relações entre recursos.
  - **Fazer:**
    - ID do módulo-alvo = diretório resolvido, ligado aos recursos desse diretório;
    - ignorar `.terraform`;
    - prefixar o ID com o diretório;
    - extrair referências `tipo.nome.`/`module.x.` por regex.
  - **Verificar:** fixture e golden.
- [x] **A09 · Varredura: ignorados, `.gitignore` e caminho relativo**
  - **Problema:**
    - só `node_modules`, `vendor`, `.git`, `.idea` e `.vscode` são ignorados (`main.go:~155`);
    - `ShouldIgnore` testa os segmentos do caminho **absoluto** (`discovery/scanner.go:~18-28`): um projeto dentro de `.../vendor/proj` não acha nada;
    - cada YAML é lido duas vezes;
    - `isSopsEncrypted` procura `"sops:\n"` e falha com CRLF;
    - o cache por hash (`discovery/cache.go`) não é usado.
  - **Fazer:**
    - testar o caminho relativo à raiz;
    - acrescentar `dist`, `build`, `target`, `.venv`, `.next`, `.terraform`, `coverage` e `__pycache__`;
    - respeitar o `.gitignore` (`git ls-files` quando houver git);
    - ler uma vez;
    - aceitar CRLF;
    - usar ou apagar o cache (A13).
  - **Verificar:** testes.
- [x] **A10 · Mermaid da IA validado e com prazo**
  - **Problema:**
    - basta `strings.Contains(refined, "flowchart")` (`main.go:~441`), e `extractMermaidFromResponse` devolve a resposta inteira como fallback (`~477-478`);
    - nada confere os nós contra o inventário;
    - o refinado sobrescreve o programático;
    - `context.Background()` sem prazo (`~310`);
    - o inventário inteiro vai no prompt.
  - **Fazer:**
    - validar: começa com `flowchart`/`graph`, `subgraph`/`end` balanceados, IDs das arestas declarados, IDs dentro do inventário; senão, cair no programático com aviso;
    - gravar também o programático (`atlas-diagram.raw.mmd`);
    - prazo de ~60 s;
    - inventário resumido quando grande.
  - **Verificar:** testes com respostas falsas.
- [x] **A11 · Caminhos no Windows**
  - **Problema:**
    - `filepath.Rel` sem `ToSlash` (`scanner.go`, `kustomize.go:~81`, `helm.go:~65`);
    - `infraNodeID` apaga o `\` (`diagram.go:~199`), e o ID muda com o SO;
    - o blueprint leva caminhos absolutos (`Component.Path`, `Roots`).
  - **Fazer:** `ToSlash` na borda de cada analyzer e no blueprint; caminhos relativos.
  - **Verificar:** testes que rodam igual no Windows e no Linux.
- [x] **A12 · Saída ordenada, goldens e testes rápidos**
  - **Problema:** a ordem de mapas (`helm.go:~94`, `terraform.go:~54`, `range cf.Services`) muda o `.mmd` a cada execução. `helperBuildAtlas` faz `go build` em cada teste (`main_test.go:~19-31`), e o pacote leva ~68 s.
  - **Fazer:** ordenar recursos e relações no fim do `ScanInfra`; goldens de `GenerateInfraMermaid` com as fixtures dos itens A04 a A08; compilar uma vez (`TestMain`) ou chamar `executar()` no processo.
  - **Verificar:** dois `diagram` seguidos dão o mesmo arquivo; o pacote abaixo de ~15 s.
- [x] **A13 · Código morto**
  - **Problema:** sem uso fora dos testes:
    - `analysis.GenerateMermaid`, `ValidateDiagram` e `GenerateC4` (`diagram.go:~13-166`);
    - `cycles.go`, `metrics.go` e `diff.go`;
    - `discovery/cache.go` (se o A09 não usar);
    - `discovery.Scan`, `Match`.
  - **Fazer:** apagar o que não for usado. Some também o `TestGenerateMermaid_PathsAbsolutos`, que falha no Windows.
  - **Verificar:** testes; atualizar a lista de falhas conhecidas no `estado.md`.

### Fase 6 · Viz

- [x] **V01 · Exclusão com propagação e UID** ✓
  - **Problema:** `plugins/viz/internal/monitor/actions.go:~29-41` usa `metav1.DeleteOptions{}` em tudo. Job em `batch/v1` deixa os pods órfãos, e a tela promete "O Job e os pods dele somem" (`ui/actions.go:~571`). O alvo é guardado só pelo nome (`ui/actions.go:~108`), então um pod de StatefulSet recriado durante a confirmação é apagado.
  - **Fazer:** `PropagationPolicy: Background` e `Preconditions{UID}`, guardando o UID no alvo (os tipos do monitor ainda não guardam).
  - **Verificar:** teste com reactor do fake conferindo as opções.
- [x] **V02 · Viz respeita o ambiente do core** ✓
  - **Problema:** `plugins/viz/main.go:~95` usa só `req.Args` e descarta `req.Context`, onde vem `infra.kube_context`/`infra.namespace` do ambiente (`-c`/`YBY_ENV`). `yby -c staging viz` abre no current-context.
  - **Fazer:** sem `--context`/`-n` explícitos, usar os do pedido. Contexto inexistente → `ContextoInexistenteError`.
  - **Verificar:** teste em `args_test.go`.
  - **Conferência (26/10):** o Viz nunca recebe contexto (declara só `command`); com o K01, passa a receber o `infra` quando o ambiente for pedido. O cloud dashboard (`cmd/cloud_painel_viz.go:~141`) passa `--context` explícito, e esse caminho não pode regredir.
- [x] **V03 · O relógio não faz LIST à toa** ✓
  - **Problema:** todo tick lê a aba ativa (`ui/model.go:~496-497`) e descarta o resultado quando o watch cobre (`~527-532`). Na aba Pods, `GetPods` chama `fetchPodMetrics` (`monitor/client.go:~165-213`), que busca as métricas do cluster todo, nunca mostradas, e calculadas só com o primeiro contêiner.
  - **Fazer:** com a aba vigiada, o sinal de conexão vira `Discovery().ServerVersion()` ou o estado do watch; tirar `fetchPodMetrics` do `GetPods` (apagar, já que não aparece na tela).
  - **Verificar:** teste de que o tick não chama List em aba vigiada.
- [x] **V04 · Memória dos informers**
  - **Problema:** o transform só tira o `managedFields` (`monitor/vigia.go:~165-170`). ConfigMap inteiro (a aba só mostra quantas chaves e o tamanho), annotation `last-applied-configuration`, spec cheio dos pods e todos os Events ficam em memória.
  - **Fazer:**
    - transform por tipo: ConfigMap sem valores (guardando contagem e tamanho), sem last-applied, Pod sem env/volumes/args/command;
    - Events limitados (N mais recentes ou última hora);
    - medir com benchmark antes e depois.
  - **Verificar:** benchmark (V07) e testes da aba ConfigMaps.
  - **Conferência (26/10):** a contagem da aba Eventos tem de continuar exata, como o CLAUDE.md promete. Se os benchmarks do V07 não mostrarem problema de memória nos Events, não limitar; o transform não descarta objetos. O tamanho do ConfigMap é calculado no transform, antes de esvaziar `Data`.
- [x] **V05 · Leituras com prazo; retry sem erro permanente; mensagem certa para falta de permissão**
  - **Problema:**
    - os `Get*` usam `context.Background()` (`monitor/client.go`) sem timeout no `rest.Config`, e o tick só se reagenda quando a leitura volta (`ui/model.go:~543-544`);
    - o `RetryClient` repete Forbidden/NotFound 3 vezes (`retry_client.go:~45-52`);
    - Forbidden aparece como "sem acesso ao cluster… confira se está no ar" (`ui/view.go:~19,272-285`) e entra em reconexão (`ui/model.go:~355-367`).
  - **Fazer:**
    - prazo de ~10 s por leitura;
    - não repetir `IsForbidden`, `IsUnauthorized`, `IsNotFound` nem `IsInvalid`;
    - Forbidden → "sem permissão para listar X em todos os namespaces" com a dica "n escolhe um namespace", sem reconectar;
    - erro de credencial exec → dica `yby cloud refresh`.
  - **Verificar:** testes.
- [x] **V06 · Logs e YAML mostram o que dizem** ✓
  - **Problema:** `LogsDoPod` cai calado no log anterior (`monitor/detalhe.go:~86-93`) enquanto o quadro diz "últimas 100 linhas do log" (`ui/quadro.go:~235`). Não há `LimitBytes` (`detalhe.go:~106`). O YAML vem com `managedFields` (`monitor/yaml.go:~58`).
  - **Fazer:** dizer "execução anterior" quando for; `LimitBytes` ~1 MiB; tirar `managedFields` e last-applied do YAML.
  - **Verificar:** testes e goldens.
- [x] **V07 · Render e watch em O(N) por quadro**
  - **Problema:** `View` monta `itensDaAba` para as 9 abas e várias vezes para a ativa (`ui/view.go:~36,108-120`, `model.go:~208`, `teclas.go:~60`). `Tabela` desenha todas as linhas e depois corta (`view.go:~219-226`). O watch reconverte a lista inteira a cada lote (`vigia.go:~249-304`). O efeito é **hipótese**: medir antes.
  - **Fazer:** benchmarks de `View` e `listaDoCache` com 10k pods e 20k eventos. Se confirmar: cache de `itensDaAba` por versão e filtros, só a janela visível desenhada, conversão por chave no handler.
  - **Verificar:** benchmark antes e depois.

### Fase 7 · Pendências antigas

- [x] **P01 · `yby cloud dashboard` a 72×20**
  - **Problema:** o detalhe por cima do painel fica sem a auditoria.
  - **Fazer:** mostrar a auditoria também nesse tamanho.
  - **Verificar:** golden.
- [ ] **P02 · Wiki**
  - **Problema:** a wiki ainda não documenta o `sentinel scan --no-ai` (#218) nem as mudanças de comportamento das fases acima.
  - **Fazer:** depois da release que levar o #218 e as fases, acrescentar uma linha em `docs/wiki/Plugins.md` para o `--no-ai`, mais as mudanças que pedirem. A wiki é submódulo e publica direto no push: só depois da release.
  - **Verificar:** a wiki descreve o que a versão publicada faz.
- [ ] **P03 · CHANGELOG**
  - **Problema:** o CHANGELOG registra o que pede atenção de quem já usa o yby, e as fases mudam comportamento.
  - **Fazer:** registrar, para a release, o que mudou de comportamento: relatório do Sentinel com novo nome, `--fix` que confirma, seções novas, escopo do RBAC, Synapstor refazendo o índice ao trocar de modelo, formato de UKI, Viz no ambiente do core.
  - **Verificar:** o CHANGELOG cobre cada mudança de comportamento.
  - **Rascunho (27/10):** o texto está pronto em `changelog-rascunho.md`, nesta pasta, conferido contra o código, com o que quebra scripts no topo e a tabela de conferência por tarefa. O CHANGELOG é escrito no commit da release: a caixa fecha quando o texto for colado ali.

---

## Fora do escopo (features ou perfumaria; decidir com o usuário)

- **Bard:**
  - laço de agente com tool calling nativo (várias ferramentas por pergunta), o "próximo grande passo";
  - pré-filtro para não chamar o classificador em "oi";
  - `selector`/`-A` no `kubectl_get`.
- **Sentinel:**
  - `schema_version` e `--fail-on` para CI (código de saída 2);
  - config do Polaris do projeto (`.yby/sentinel/polaris.yaml`);
  - supressões com justificativa e validade;
  - histórico com "N novos, M resolvidos";
  - mapeamento completo para controles CIS (5.1.1 …) com cobertura por controle;
  - patch kustomize para o repositório em vez de aplicar ao vivo.
- **Synapstor:**
  - `quality --min N` e `-o json` nos comandos;
  - status do índice no Bard.
- **Atlas:**
  - renderizar com `helm template`/`kubectl kustomize` quando houver o binário;
  - `yby atlas diff`/ciclos;
  - aposentar o `Blueprint` legado em favor do `InfraBlueprint` entregue sob demanda.
- **Viz:**
  - modo somente leitura e contextos protegidos (selo, nome digitado nas ações, dry-run no servidor antes do ↵, auditoria em `~/.yby/audit.log`, escalar para 0 como destrutivo). **Recomendado como a primeira feature**, porque o contexto padrão do usuário é produção;
  - aviso de HPA e de Argo CD com self-heal no escalar e no reiniciar;
  - follow, troca de contêiner e "mais linhas" nos logs;
  - ir do workload aos pods;
  - DaemonSet, CronJob e o modo `:recurso` genérico (CRDs, PVC, HPA);
  - top, port-forward, exec, rollout;
  - achados do Sentinel no recurso;
  - seletor de namespace e de contexto.
- `yby version` em texto simples (perfumaria).

## Histórico

Uma tabela por fase: cada trilha só escreve na tabela da sua fase, e os merges não conflitam.

### Plano

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | — | Plano criado a partir da revisão dos cinco plugins (um agente por plugin, achados graves conferidos no código). | 27d6461 |
| 26/10/2026 | — | Conferência das 71 tarefas no código (7 agentes, só leitura), decisões do usuário, ondas e trilhas paralelas, R01 novo e notas de conferência. | (este commit) |

### Fase 0 · CI e base

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | C02 | O `.golangci.yml` deixa de excluir o Sentinel e exclui só a pasta `test/` da raiz; o pipeline ganha lint do Viz e do Synapstor; gofmt nos dois arquivos. Validado com o golangci-lint v1.64.8 do CI (GOTOOLCHAIN=go1.26.1). | este commit |
| 26/10/2026 | C01 | Testes do Sentinel (`-tags k8s`, GOGC=30, `-p 1`), do Viz e do Synapstor no feature-pipeline, no release-pipeline, no release.yaml e no `task test`. Rodados com `-race` no Linux (golang:1.26) sem falha; a raiz no Linux pede LANG=C.UTF-8 por causa dos glifos. | este commit |
| 26/10/2026 | R01 | Pacote-folha `pkg/redacao` (`Redigir`, marca `[REDIGIDO]`) no lugar do `sanitizeContent` do Bard: tokens com formato próprio, PEM, senha em URL, Authorization, Bearer/Basic e valores de nomes sensíveis (inclusive camelCase, compostos colados e `senha`), redigidos pela união dos trechos achados sobre o mesmo texto. Seis voltas de revisão adversarial com busca diferencial contra o antigo: nenhum vazamento novo realista. | este commit |
| 26/10/2026 | R01 | Ajuste (achado do B06): nome terminado em `name`/`ref` (`secretName`, `SecretName` do describe, `secretKeyRef`, `secret_ref`) deixa de ser sensível; o Bard e o Sentinel precisam dele para dizer qual Secret falta. | este commit |

### Fase 1 · Contratos

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | K01 | Contexto barato (ambiente, infra com kube_config, values) a todo plugin quando o ambiente é pedido (`-c` em qualquer forma do pflag antes do nome, ou `YBY_ENV`); o caro só com `consome_contexto`; `--help` sem contexto; `ExecuteContextHook` com o contexto do chamador; Synapstor sem embrulho duplo; `UKIIndex` removido. | este commit |
| 26/10/2026 | K02 | O hook `context` do Atlas aceita o argumento `infra` (só a ferramenta do Bard pede) e manda o `InfraBlueprint` ao lado do blueprint; `formatBlueprintResults` desembrulha a resposta real e resume com nomes (componentes, charts, workloads, outros recursos por kind e relações), cortado em 8 KB; contrato no `testdata` do Atlas (pedido e golden gravado pelo `executar`) lido pelo teste do Bard. | este commit |
| 26/10/2026 | K04 | O Sentinel grava no `testdata` os pedidos do Bard e os goldens do JSON do scan `--no-ai` (recursos, escopo, `type` normalizado, `nao_rodaram`) e do investigate `--no-ai`, rodando o hook command com cluster falso e o relógio injetável no `timestamp`; o Bard confere o pedido do scan e passa os dois goldens pelos formatadores (`formatInvestigateResults` extraído), no lugar dos fixtures inventados. | este commit |
| 26/10/2026 | K03 | O system prompt do Bard perdeu as seções mortas (Project Overview, Backlog & Debt, Technical Blueprint): o Bard não recebe contexto do core e o blueprint chega pela ferramenta; `{{blueprint_json_summary}}` segue aceito e vira vazio, por compatibilidade com overrides, e o Atlas não é consultado na abertura | este commit |

### Fase 2 · Bard

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | B05 | Com ferramenta, o prompt final mantém o bloco de notas do Synapstor antes do resultado da ferramenta; teste do `Fluxo` com busca e tool falsas. | este commit |
| 26/10/2026 | B08 | `ai.ErroRetentavel` classifica o erro (429/502/503, rede, stream cortado, YbyError de timeout); o `RetryProvider` e o `Fluxo` deixam de repetir o resto (chave inválida, modelo inexistente, ERR_TOKEN_LIMIT, config, falha de CLI). | este commit |
| 26/10/2026 | B10 | Providers claude-cli e gemini-cli levam o prompt pelo stdin (`claude -p` sem texto; `gemini -p` com instrução curta), rodam com `CommandContext` e `WaitDelay`, limitam o stderr no erro e o stream do gemini perde as linhas "Loaded cached credentials". | este commit |
| 26/10/2026 | B03 | O classificador recebe todos os intents com o esquema de parâmetros e o cluster/namespace da sessão (sem a regra do "default"), lê JSON no meio do texto e número como texto, registra falhas em debug, e o Fluxo não executa ferramenta sem os parâmetros obrigatórios; registry em ordem estável. | este commit |
| 26/10/2026 | B06 | A saída e o erro de toda ferramenta passam pelo `pkg/redacao` no `Fluxo` (prompt e "↵ ver"); o `kubectl_get` lista só em wide ou name (sem yaml, json, jsonpath, go-template em nenhum recurso, Secrets inclusive); resource, name, pod, namespace e tail da IA são conferidos antes do kubectl e do Sentinel, e valor com "-" no começo não vira flag. | este commit |
| 26/10/2026 | B04 | Tools YAML com os intents, a do projeto vencendo a global e só com confiança (`yby plugin trust <arquivo>` grava o SHA-256 do arquivo em `pkg/confianca`; sem ela, ou mudado, aparece no `/ferramentas` com o motivo), o mesmo para os prompts de `.yby/prompts`; o `command` roda sem shell, com cada `{{param}}` como argumento inteiro, valor com "-" recusado, `YBY_PARAM_<NOME>` no env, contexto do Bard e prazo de 2 min; no Windows o programa que de fato roda pelo cmd.exe, resolvido pelo PATHEXT mesmo em caminho relativo sem extensão (`cmd`, `.bat`, `.cmd` como o `az.cmd`) recusa valor com metacaractere do cmd (aspas, `%`, `&`, barra vertical, `<`, `>`, `^`, `!`, parênteses ou quebra de linha), e comando que só funciona num shell (ou shell com `{{}}`) não carrega. | este commit |
| 26/10/2026 | B01 | O Bard acha o Sentinel e o Atlas como o core (`~/.yby/plugins` e depois `./.yby/plugins`, `yby-plugin-<nome>` com `.exe` no Windows, sem os nomes legados) e só roda o binário que passa no `plugin.IsTrusted`, o mesmo registro do core; sem confiança, o `/ferramentas` diz "não confiável · rode yby plugin trust". Sem contrato novo: o `$YBY_BIN sentinel` perderia o `--context` da sessão (o Sentinel lê o kube_context só do pedido) e o `YBY_PLUGINS` faria o core calcular o SHA-256 de todo plugin a cada comando. | este commit |
| 26/10/2026 | B09 | `invokePlugin` roda com `exec.CommandContext` (o esc mata o Sentinel/Atlas, com `WaitDelay`); o `EnrichContext` tem prazo total de 5 s e o kubectl leva `--request-timeout=5s`, então cluster fora do ar não segura a abertura da TUI. | este commit |
| 26/10/2026 | B11 | O system prompt do Bard não pede chamada de ferramenta em JSON nem manda usar kubectl/helm do runtime (só consulta; escrita vira comando sugerido); o parser morto saiu, e as guardas (delete de namespace/ClusterRole, drain, cordon, apply --force, scale a 0) valem na execução das tools externas, barrando o que os valores da IA montam e o YAML não tinha. | este commit |
| 26/10/2026 | B14 | claude-cli e gemini-cli passam por `wrapCLIProvider` (TokenAware com a janela do CLI, 200 mil tokens no Claude e 1 milhão no Gemini, também sem modelo escolhido, e Retry, que não repete a falha do CLI); cache, custo e taxa ficam de fora porque não valem para um processo local. | este commit |
| 26/10/2026 | B13 | O cabeçalho do Bard mostra o modelo do config ("claude-cli · opus", `ai.models`/`ai.model`/`YBY_AI_MODEL` via `ai.ConfiguredModel`) e os providers CLI passam a usar esse modelo (`--model`), que antes ignoravam; golden `TestTelaModeloDoConfig`. | este commit |
| 26/10/2026 | B07 | Orçamento do que vai à IA (`plugins/bard/tui/orcamento.go`: `Cortar` por linhas com "… N linhas omitidas", `Recentes` e os tetos por bloco): saída de ferramenta ≤ 8 KB, notas do Synapstor ≤ 8 KB somadas, histórico ≤ 4 KB (1 KB por mensagem), cluster ≤ 3 KB com 20 pods (os com problema primeiro); scan do Sentinel resumido (contagem por severidade, os 10 mais graves com até 10 recursos, o que não rodou), investigate em JSON compacto com os logs no fim, `kubectl_events` com os 50 últimos e `--tail` até 200; `GetModelMetadata` casa pelo nome base sem a tag do Ollama (`llama3.1:8b`, `qwen2.5:7b`). | este commit |
| 26/10/2026 | B02 | O `Fluxo` recebe as trocas anteriores da sessão (`tui.Troca`, até 10, redigidas pelo `pkg/redacao` e no novo `TetoConversa` de 3 KB, com o `TetoHistorico` das sessões anteriores baixado a 2 KB) e as põe antes da pergunta na resposta e no classificador; a TUI guarda as trocas da conversa (o `/limpar` e o `/sessões` recomeçam, porque a retomada já vai pelo system prompt; o `/modelo` mantém) e as perguntas do mesmo batch formam uma sessão. | este commit |
| 26/10/2026 | B12 | `kubectl_events` filtra por `involvedObject.name` quando há pod, `kubectl_logs` ganha `container` e `previous` (e lista os contêineres no erro de contêiner), `sentinel_investigate` pede `--no-ai` e o Bard analisa com o próprio provider; os valores novos passam pela validação do B06. | este commit |
| 26/10/2026 | B15 | O índice feito com outros embeddings vira falha da etapa de contexto, com a nota começando pelo comando ("rode yby synapstor index --full (índice de outro modelo)", que cabe em 80 colunas), e o aviso inteiro ("o índice foi feito com X, e os embeddings agora vêm de Y", com o comando na linha de baixo) fica na conversa da TUI depois da rodada e abaixo das etapas no `-p`, em vez de "Synapstor indisponível"; o corte de relevância é o `ai.RelevanciaMinima` (0,35), usado pela busca do Synapstor e como padrão do Bard (o `relevance_threshold` do bard.yaml, mesmo 0, continua valendo); os trechos entram juntos por arquivo, com título e caminho no cabeçalho e a seção em cada trecho, no teto das notas, e a etapa conta arquivos. | este commit |
| 27/10/2026 | K02, K04, K03, B12, B15 Ajuste | Ajustes da revisão adversarial da onda 2: o chart do blueprint não se repete como `HelmChart` na infra, o golden do Atlas ganha um Compose com imagem, o escopo do scan é conferido no contrato, os contêineres de init e efêmeros entram no erro do `kubectl_logs`, os testes de valor com cara de flag conferem a validação e que o kubectl não rodou, as notas do Synapstor não repetem título e seção que o trecho já traz e o corte compara em float32, como o Synapstor, além dos comentários e do CLAUDE.md. | este commit |
| 27/10/2026 | B08 Ajuste (CI do #225) | O CLI que nem inicia (fora do PATH, sem permissão, exec format error) embrulha um `syscall.Errno`, que também é um `net.Error`, e o `ErroRetentavel` o repetia por ~100 s; `*fs.PathError` e `*exec.Error` agora são permanentes. O CLI falso dos testes ganhou o `#!/bin/sh`, sem o qual o Linux nem o inicia | este commit |
| 27/10/2026 | B15 Ajuste (CI do #226) | A nota da etapa vira "índice de outro modelo": com a busca demorando, a coluna do tempo cortava o comando da nota antiga (no CI, sempre). O comando vai inteiro no aviso; o teste do `-p` ganhou uma busca que demora e falha sem a correção | este commit |

### Fase 3 · Synapstor

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | Y01 | O `VectorStore` grava com que provider, modelo e dimensão o índice foi feito (`identidade.json`); a busca com outros embeddings devolve `IndiceDeOutroModeloError` com "rode yby synapstor index --full", e o `index` apaga e refaz tudo sozinho quando o modelo muda. | este commit |
| 26/10/2026 | Y02 | O `--full` apaga e recria o índice, o `index` refaz tudo quando o número de documentos não bate com o manifest (órfãos) e o ID, o `source` e as chaves do manifest usam `/`. | este commit |
| 26/10/2026 | Y03 | O manifest passou para `.synapstor/.index/manifest.json` (dentro do que o git ignora); o do lugar antigo conta como vazio e é apagado na primeira gravação. | este commit |
| 26/10/2026 | Y09 | Arquivo que não abre mantém a entrada antiga no manifest e no índice, com aviso; cada arquivo é lido uma vez e o hash sai do conteúdo lido; o `index` pega uma trava do sistema (`gofrs/flock`) em `.synapstor/.index/index.lock`, que sai sozinha quando o processo cai, e o segundo `index` para com "outro yby synapstor index está rodando neste projeto". | este commit |
| 26/10/2026 | Y10 | O `decay` conta os commits nos arquivos que o UKI cita (que existem no projeto) depois do último commit da nota e marca como desatualizado se houver algum; usa o `updated_at` do frontmatter, descarta timestamp do nome fora de 2000..amanhã e roda o git com prazo de 15 s. | este commit |
| 26/10/2026 | Y11 | O `quality` pontua pelas seções do padrão do tipo (regra, decisão, padrão, métrica, referência), com bloco de código só exigido de padrão e UKI sem tipo, e o resumo diz qual seção falta. | este commit |
| 26/10/2026 | Y12 | O export grava um manifesto `.synapstor-export.json` no destino e apaga só o que ele listou e sumiu da fonte (com as subpastas vazias), e o Docusaurus escapa `{` e `<` fora do código para o MDX; tags e frontmatter único já vinham do Y07. | este commit |
| 26/10/2026 | Y04 | `VectorStore.Search` limita o topK ao número de documentos e devolve vazio com a coleção vazia, sem o erro do chromem; a unificação do corte de relevância com o Bard fica na onda 2. | este commit |
| 26/10/2026 | Y05 | O índice guarda um trecho por seção H2 (ID `caminho#n`, ~1500 caracteres com sobreposição), apaga os trechos do arquivo antes de readicionar, ignora `_Footer.md`/`_Sidebar.md`, corta o texto do MiniLM em divisa de runa e a busca mostra cada UKI uma vez; a injeção no Bard fica na onda 2. | este commit |
| 26/10/2026 | Y06 | O modelo local passou a ser o paraphrase-multilingual-MiniLM-L12-v2 em int8 (~120 MB, revisão fixada, conferido no hugot em Go puro), baixado numa etapa própria (`ai.PrepararEmbeddings`) antes de gerar embeddings e de buscar; o resumo do `index` e do `search` diz de onde vêm os embeddings, o `semEmbeddings` morto saiu, a ajuda do `index` diz a verdade sobre o modelo local e o texto vai ao modelo cortado em 512 tokens com o tokenizador dele (antes, em 2000 bytes, um UKI com kubeconfig ou certificado derrubava o `index` inteiro). | este commit |
| 26/10/2026 | Y07 | O pacote `internal/uki` lista os UKIs com as subpastas de domínio e lê os metadados do frontmatter em PT-BR, do em inglês e das linhas em negrito; quality, decay, export e index usam os dois, o export sai com um frontmatter só, o capture e o study gravam o frontmatter da especificação (ID, tipo e tags da IA; status e datas do código) e o `UKI_SPEC.md` foi traduzido, com os tipos `regra`, `decisao`, `padrao`, `metrica` e `referencia`. | este commit |
| 26/10/2026 | Y08 | O `study` quebra a pergunta em termos sem palavras vazias, pontua nome > caminho > conteúdo e só então corta em 50, usa `git ls-files` (respeita o `.gitignore`), pula testes e goldens, devolve caminhos com barras, pede e corrige a resposta com o prompt `synapstor.study` e a mensagem ao modelo saiu em PT-BR. | este commit |
| 26/10/2026 | Y13 | Saíram o `internal/graph` e o `internal/tagger` (sem uso fora dos testes), o prompt `synapstor.tagger`, `quality.FormatScore` e `decay.FindStale` (os testes de decay filtram `IsStale` por helper) e o campo `Summary` da resposta, que saiu do prompt e do JSON; `splitMarkdown` e `semEmbeddings` já tinham saído nos Y05 e Y06. | este commit |
| 26/10/2026 | Y10 | Ajuste da revisão: sem commit da nota, o `updated_at` só com a data vale o dia inteiro, e os commits do mesmo dia (feitos antes da nota) deixam de marcá-la como desatualizada. | este commit |

### Fase 4 · Sentinel

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | S10 | Investigate checa a saúde antes do cache (Succeeded saudável, reinício só vale se for de menos de 2 h), lê logs por contêiner com Previous, chave de cache com contexto, UID e reinícios, e eventos resumidos | este commit |
| 26/10/2026 | S11 | Investigate redige logs, eventos e métricas e o scan redige os achados com o pkg/redacao antes da IA; o pkg/redacao passou a redigir o value de env com name sensível e o valor de ConfigMap no kubectl describe | este commit |
| 26/10/2026 | S12 | O `investigate --no-ai` para na coleta e devolve os dados brutos (fase, contêineres, eventos e fim dos logs, já redigidos) em terminal, json e markdown, sem chamar a IA nem o cache | este commit |
| 26/10/2026 | S13 | O OPA compila cada política Rego uma vez (PrepareForEval em cache) e lista os ClusterRoles uma só vez por scan, mantendo as falhas em FalhaParcial | este commit |
| 26/10/2026 | S14 | O relatório do scan leva contexto e hora no nome e no conteúdo (dois scans não se sobrescrevem; contexto sanitizado) e a ajuda, o README e o CLAUDE.md citam o novo caminho | este commit |
| 26/10/2026 | S15 | O Polaris silencia o logrus e o log padrão só durante a chamada e restaura os destinos de antes (a dica do --fix já estava certa desde o S01) | este commit |
| 26/10/2026 | S02 | `sdk.GetRestConfig()` entrega o config do cluster do yby (kubeconfig, contexto, token cloud); o clientset e o cliente dinâmico do Polaris saem dele e o backend recebe o config no construtor | este commit |
| 26/10/2026 | S09 | Allowlist única no pacote `allowlist` (checks internos em Go, OPA como `data.allowlist`), casando pelo subject (system:masters, SAs de controllers no namespace deles, plano de controle só nos `system:*`; `admin`/`edit`/`view`/`cluster-admin` só por RoleBinding ou, por ClusterRoleBinding, ao plano de controle) e não pelo nome; os ignorados contam uma vez cada e aparecem na tela, no relatório e no JSON (`ignored_by_allowlist`) | este commit |
| 26/10/2026 | S03 | O achado agrupado do Polaris leva `Resources` (lista inteira, `Resource` é o primeiro, sem "(+N)"); a tela mostra "N recursos" e a lista embaixo (até 5, "e mais K"), o JSON e o relatório trazem todos | este commit |
| 26/10/2026 | S05 | O Polaris usa a categoria do próprio check: Security danger é crítico e warning é alto ou médio por tabela (endurecimento é médio), os checks de RBAC, rede e secrets vão para as seções deles, e Reliability e Efficiency viram as seções "confiabilidade" e "eficiência" (danger médio, warning baixo), na tela e no relatório, que passa a ter uma seção por categoria | este commit |
| 26/10/2026 | S06 | O JSON do scan tem `type` igual à `severity` (normalizado num ponto só), mensagens do Polaris em PT-BR também no JSON e no markdown e o campo `nao_rodaram` (backend, verificações, motivo), também no relatório markdown | este commit |
| 26/10/2026 | S08 | Checks internos viram plano B (só rodam quando nenhum backend varre; backend limpo vira fonte com 0 achados), pods listados uma vez por scan (cache no contexto), cosign só com chave ou identidade+emissor configurados (senão ErrNaoVerificado, que entra em `nao_rodaram` sem virar achado) e com prazo, env sensível: KEY por palavra (KEYCLOAK_URL não) e PASSWORD/SECRET/TOKEN por trecho (PGPASSWORD sim), `pod_root` e `pod_seccomp` herdam/olham o contêiner | este commit |
| 26/10/2026 | S07 | O `--profile` filtra os achados de todos os motores por uma tabela check→perfis (cada `polaris/*` e `opa/*` pelo check interno equivalente; hostPath/HostProcess nos CIS e TLS do Ingress no PCI-DSS e no SOC2 pelo próprio ID; confiabilidade e checks customizados fora) e a tela, o JSON (`perfil`, `fora_do_perfil`) e o relatório dizem o perfil aplicado e quantos achados ficaram de fora | este commit |
| 26/10/2026 | S04 | Pacote `escopo`: no scan de namespace o OPA, os checks internos e os checks de RBAC do Polaris só olham o RBAC que vale nele (Roles e RoleBindings do namespace, ClusterRoles a que eles apontam, ClusterRoleBindings com SA do namespace, reduzidos aos subjects dele, e a allowlist julga cada ClusterRole por quem o recebe ali); `--scope cluster` traz o RBAC do cluster todo; campo `scope` no achado e no relatório JSON; o wildcard do OPA passou a ver os Roles do namespace | este commit |
| 26/10/2026 | S01 | O `--fix` sobe do pod ao dono pelos `ownerReferences` (Deployment, StatefulSet, DaemonSet, CronJob) e faz um strategic merge patch por recurso no template, só com as correções seguras (`allowPrivilegeEscalation: false` e `capabilities.drop: [ALL]`, dos checks internos e dos `polaris/*`, com o diff por contêiner); runAsNonRoot, readOnlyRootFilesystem, privileged, limites e automount ficam sem correção automática (contados na tela); pod sem dono, ReplicaSet sem Deployment, Job solto, dono desconhecido e recurso do Argo CD (anotação `tracking-id` ou rótulo `argocd.argoproj.io/instance`) são recusados com o motivo; `--fix-dry-run` e o `--fix` passam pelo dry-run no servidor (`DryRun: [All]`), e o `--fix` mostra o plano e pede confirmação pelo `ui.Confirmar` (sem terminal na entrada, exige `--yes`); a dica de permissão usa o tipo do recurso | este commit |

### Fase 5 · Atlas

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | A12 | ScanInfra e o analyzer de compose ordenam recursos e relações (diagrama determinístico), goldens do GenerateInfraMermaid e o atlas compilado uma só vez nos testes (pacote de ~72 s para ~8 s) | este commit |
| 26/10/2026 | A11 | `relSlash` (ToSlash) em todos os analyzers e no scanner, Kustomize/Helm com `path`, e o blueprint sai com caminhos relativos (`Component.Path`, `Roots=["."]`, `RootPath="."`) | este commit |
| 26/10/2026 | A05 | Service passa a selecionar só workloads (Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob) pelos labels do template do pod e no mesmo namespace | este commit |
| 26/10/2026 | A06 | Compose aceita ports e volumes na sintaxe longa, reconhece docker-compose.*.yml e compose.*.yml e mescla base e override em um nó só por serviço | este commit |
| 26/10/2026 | A09 | Varredura testa o caminho relativo à raiz, ignora também dist/build/target/.venv/.next/.terraform/coverage/__pycache__, respeita o .gitignore via `git ls-files` (com fallback sem git), lê cada YAML uma vez e reconhece SOPS com CRLF | este commit |
| 26/10/2026 | A07 | Helm: nome templado vira <chart>-<kind>, templates/ é varrido com subpastas, HelmRepository deduplicado por (repositório, nome) e charts processados em ordem estável | este commit |
| 26/10/2026 | A01 | Relações do blueprint sem falsos positivos: COPY casa por segmento e nunca a raiz, imports Go pelo módulo mais longo com fronteira `/` (linha única e subpastas via go/parser), relações deduplicadas por (from, to, type) | este commit |
| 26/10/2026 | A04 | Kustomize: Kustomization identificada pelo diretório relativo (referências resolvidas pelo caminho), ligada aos recursos dos manifests que lista, patches fora do analyzer K8s e namespace herdado aplicado antes das relações (uma cópia por overlay), com fixture base+overlays e golden | este commit |
| 26/10/2026 | A08 | Terraform: IDs prefixados com o diretório, a chamada `module.x` com source local liga ao nó do diretório resolvido (que liga aos blocos dele, sem o laço de antes) e referências `tipo.nome`/`data.`/`module.` viram relações `references`, sem contar comentários e texto de strings, com fixture envs/dev+prod e golden | este commit |
| 26/10/2026 | A03 | Overview mantém workloads, charts, apps, módulos e serviços do compose isolados (e tudo, se nada mais sobrar), agrupa por kind acima de 25 nós (`MaxNosOverview`, arestas reapontadas e sem repetir) e o `GetInfraStats` conta os nós e arestas desenhados, com testes e golden do teto | este commit |
| 26/10/2026 | A02 | O `--detail full` não passa mais pela IA (etapa pulada com o motivo) e as estatísticas de recursos e relações vêm do diagrama gravado, contando nós e arestas do Mermaid da IA (`ContarMermaid`), com testes e goldens | este commit |
| 26/10/2026 | A10 | O Mermaid da IA só substitui o programático se passar no `analysis.ValidarMermaid` (flowchart/graph, subgraph e end fechados, arestas entre nós declarados, nós só com IDs do rascunho, que o prompt `atlas.refine` agora manda manter), com prazo de 60 s pelo contexto, o programático sempre em `.yby/atlas-diagram.raw.mmd` e o inventário do prompt resumido (10 nomes por kind, sem as relações) | este commit |
| 26/10/2026 | A13 | Código morto apagado: analysis (GenerateMermaid, ValidateDiagram, GenerateC4, cycles, metrics, diff) e discovery (cache, Scan, Match), com os testes deles; o TestGenerateMermaid_PathsAbsolutos sai junto e a lista de falhas do Windows cai para 11 | este commit |

### Fase 6 · Viz

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 26/10/2026 | V01 | Exclusão com propagação em segundo plano (Job não deixa pods órfãos) e precondição de UID guardado no alvo da ação | este commit |
| 26/10/2026 | V06 | `LogsDoPod` avisa quando o log é da execução anterior (quadro e painel dizem), leitura com `LimitBytes` de 1 MiB e YAML sem `managedFields` nem last-applied | este commit |
| 26/10/2026 | V03 | Com a aba vigiada o tique só sonda `/version` (`monitor.Sondador`) em vez de listar; `fetchPodMetrics` e `Pod.CPU`/`Memory` removidos | este commit |
| 26/10/2026 | V05 | Listas com prazo de 10 s por contexto (sem `rest.Config.Timeout`), `RetryClient` sem repetir Forbidden/Unauthorized/NotFound/Invalid/credencial exec/prazo, aba negada com "sem permissão para listar X …" sem reconectar e sem LIST no relógio, e dica `yby cloud refresh` para a credencial exec | este commit |
| 26/10/2026 | V07 | Benchmarks com 10 mil pods e 20 mil eventos confirmaram a hipótese: `itensDaAba` em cache (chave: a lista, os filtros, o segundo e o modo visual), a tabela montada uma vez por lista e só a janela desenhada (`yui.TabelaJanela`) levam a `View` de ~800 ms para ~7 ms nos pods e de ~1,3 s para ~10 ms nos eventos (tecla + quadro de ~1,4 s para ~7 ms, lote + quadro de ~1,1 s para ~55 ms), e `listaDoCache` ordena pela chave calculada uma vez e não copia os eventos (pods ~30→13 ms, eventos ~55→22 ms e 32→4,7 MB por lote); a conversão por chave no handler é hipótese não confirmada (a conversão é ~10% do lote) e ficou de fora | este commit |
| 26/10/2026 | V04 | Transform por tipo no cache do watch (`paraOCache`): sem last-applied em todo tipo, ConfigMap sem os valores (chaves ficam, tamanho calculado antes numa anotação só do cache: 10 mil ConfigMaps de ~98 MB para ~10 MB) e Pod sem env, volumes, args e command; os Events não foram limitados (20 mil ocupam ~15 MB, e a contagem da aba segue exata) | este commit |
| 26/10/2026 | V02 | Sem `--context`, `--kubeconfig` e `-n`, o Viz abre no kube_config, no kube_context e no namespace do ambiente que o core manda no pedido (`yby -c staging viz` ou `YBY_ENV`); `--context`/`--kubeconfig` trocam o cluster inteiro, `-n` só o namespace, e o kube_context que o kubeconfig não tem sai como ERR_CONFIG antes da TUI | este commit |

### Fase 7 · Pendências

| Data | Tarefa | O que foi feito | Commit |
|---|---|---|---|
| 27/10/2026 | P01 | O detalhe por cima ganhou um aperto novo (`semBranco`: AUDITORIA RECENTE logo abaixo da IDENTIDADE, sem a linha em branco, só em coluna única) antes de perder a auditoria; a 72×20 o quadro mostra a identidade, o token e o último registro, com o aviso de rolagem da tabela; só o golden de 72×20 mudou, e a 80×16 (duas colunas, limite de 5 linhas) segue sem a auditoria | este commit |
| 27/10/2026 | P03 (rascunho) | Texto do CHANGELOG para a release dos #220 a #226 em `changelog-rascunho.md`: o que quebra scripts no topo, uma seção por instrumento e para autores de plugin, e a tabela de conferência por tarefa; as afirmações tiradas do Histórico foram conferidas no código (5 corrigidas). A caixa fica aberta até a release | este commit |
