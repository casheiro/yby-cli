# Rascunho do CHANGELOG (P03): correções dos plugins

> Colado no `CHANGELOG.md` como `v5.8.2` em 27/10/2026; vale o texto de lá. Este é o rascunho da tarefa P03. O texto abaixo, da linha `## vX.Y.Z` até a linha `---`, é para colar no `CHANGELOG.md`, acima da seção `## v5.8.0`, no commit da release que levar os PRs #220 a #226. O título da versão (número e subtítulo) fica para a release. A tabela de conferência depois da linha `---` não vai para o CHANGELOG.

## vX.Y.Z: correções dos plugins

### O que pode quebrar scripts e rotinas

- **JSON do `yby sentinel scan -o json`:** campos novos `scope` ("namespace" ou "cluster"), `ignored_by_allowlist`, `contexto`, `perfil` e `fora_do_perfil` (os dois últimos só com `--profile`) e `nao_rodaram` (lista de `backend`, `verificacoes` e `motivo`, só quando algo não rodou); cada achado ganha `scope` e `resources`. Mudou o conteúdo de campos que já existiam: `type` passa a ser sempre igual a `severity` (antes podia contradizê-la), `message` e `description` do Polaris saem em português, e `resource` de um achado agrupado traz o primeiro recurso, sem o sufixo "(+N)"; a lista inteira está em `resources`.
- **Relatório do scan:** o arquivo em `~/.yby/reports/` passa de `sentinel-scan-<namespace>-<data>.md` para `sentinel-scan-<namespace>-<contexto>-<data>-<hora>.md` (por exemplo `sentinel-scan-payments-k3d-yby-local-2026-10-27-143005.md`). Dois scans no mesmo dia deixam de se sobrescrever, e quem lia o relatório do dia por nome fixo precisa listar a pasta.
- **`yby sentinel scan --fix`** pede confirmação do plano antes de aplicar. Sem terminal na entrada ele não pergunta e exige `--yes`; num script ou no CI, acrescente `--yes`.
- **`.yby/tools/*.yaml` e `.yby/prompts/*.txt` do projeto só valem depois de `yby plugin trust <arquivo>`** (por exemplo `yby plugin trust .yby/tools/backup.yaml`). O yby guarda o SHA-256 do arquivo em `~/.yby/trusted-files.json`; se o arquivo mudar, a confiança cai e é preciso confirmar de novo. Sem a confiança, o Bard não carrega a ferramenta (o `/ferramentas` mostra o motivo) nem usa o prompt do projeto. As ferramentas de `~/.yby/tools/` seguem sem pedir confiança.
- **O Bard só roda o Sentinel e o Atlas que passam pela verificação de confiança do core** (o mesmo `yby plugin trust <nome>`), inclusive os de `./.yby/plugins`. Sem confiança, o `/ferramentas` diz "não confiável · rode yby plugin trust <nome>".
- **Synapstor: o índice é refeito uma vez.** O índice passa a ser guardado por trechos de seção e registra com que embeddings foi feito, e o manifest mudou de `.synapstor/.index_manifest.json` para `.synapstor/.index/manifest.json`. Depois de atualizar, o `yby synapstor search` e o Bard avisam "rode yby synapstor index --full", e o `yby synapstor index` (com ou sem `--full`) apaga o índice antigo e refaz tudo na primeira vez.
- **Synapstor: o modelo local de embeddings mudou** para o paraphrase-multilingual-MiniLM-L12-v2 em int8, que entende português. Quem usa os embeddings locais (sem Ollama, Gemini nem OpenAI) tem um download de cerca de 120 MB para `~/.yby/models`, numa etapa própria na primeira busca ou indexação. Trocar de modelo ou de provider de embeddings refaz o índice sozinho no `index`.
- **Synapstor: formato de UKI novo.** O UKI nasce com frontmatter em português (`id`, `tipo`, `status`, `criado_em`, `atualizado_em`, `tags`), com os tipos `regra`, `decisao`, `padrao`, `metrica` e `referencia` e os status `rascunho`, `ativo` e `obsoleto`; o `capture` e o `study` preenchem status e datas. Os UKIs antigos (frontmatter em inglês ou linhas em negrito) continuam sendo lidos, e subpastas de domínio em `.synapstor/.uki/` são aceitas. A especificação está em `.synapstor/UKI_SPEC.md`.
- **Bard: o `kubectl_get` lista só em `wide` ou `name`**; `yaml`, `json`, `jsonpath` e `go-template` são recusados em qualquer recurso, para o conteúdo dos Secrets não ir à IA. Valores que a IA monta e começam com "-" (nome, namespace, pod, `--tail`) são recusados em vez de virar flag do kubectl.
- **Atlas: o blueprint sai com caminhos relativos à pasta atual e com `/`** (`Roots` é `["."]`), em vez de caminhos absolutos. Quem lê o hook `context` do Atlas vê a mudança.
- **Viz: excluir um Job leva os pods junto** (propagação em segundo plano), e a exclusão só vale para o objeto que estava selecionado (confere o UID).

### Sentinel

- **`--fix` e `--fix-dry-run`** passam a corrigir o dono do pod (Deployment, StatefulSet, DaemonSet ou CronJob) com um patch por recurso, e não o pod, que o controller refazia. Só entram as correções seguras: `allowPrivilegeEscalation: false` e `capabilities.drop: [ALL]`; as outras (`runAsNonRoot`, `readOnlyRootFilesystem`, `privileged`, limites, `automountServiceAccountToken`) ficam sem correção automática e a tela diz quantas. Pod sem dono, ReplicaSet sem Deployment, Job solto e recursos do Argo CD são recusados com o motivo. O `--fix-dry-run` e o `--fix` passam por um dry-run no servidor antes.
- **Escopo do RBAC:** o scan de namespace só olha o RBAC que vale nele (Roles e RoleBindings do namespace, os ClusterRoles a que eles apontam e os ClusterRoleBindings com ServiceAccount do namespace). `--scope cluster` traz o RBAC do cluster inteiro.
- **Seções novas:** "confiabilidade" e "eficiência" (checks do Polaris que antes não tinham seção), na tela e no relatório, que passa a ter uma seção por categoria. A severidade dos achados do Polaris vem da categoria do próprio check: em Security, danger é crítico e warning é alto (médio nos checks de endurecimento, como `automountServiceAccountToken` e `notReadOnlyRootFilesystem`); em confiabilidade e eficiência, danger é médio e warning é baixo.
- **Allowlist do RBAC por subject** (`system:masters`, controllers conhecidos nos namespaces deles, plano de controle), e não pelo nome do recurso; o que ela ignora aparece contado na tela, no relatório e no JSON.
- **`--profile`** passa a filtrar os achados de todos os motores (Polaris e OPA incluídos), e a tela, o JSON e o relatório dizem o perfil aplicado e quantos achados ficaram de fora. Antes só afetava os checks internos.
- **Os checks internos** só rodam quando nenhum motor varre; `cosign` só verifica com chave ou identidade configuradas, e o que não pôde ser verificado entra em `nao_rodaram`, sem virar achado.
- **`yby sentinel investigate --no-ai`** (flag nova) só coleta: mostra o estado do pod, os eventos e o fim dos logs, já sem segredos, sem chamar a IA nem o cache. O JSON traz `metadata`, `healthy`, `phase`, `containers`, `events` e `logs`.
- **`investigate`** confere a saúde do pod antes do cache: pod saudável não aciona a IA (um reinício só conta se o contêiner terminou nas últimas 2 h), os logs vêm por contêiner, com os da execução anterior nos que reiniciaram, e a chave do cache inclui o contexto, o UID e os reinícios. Logs, eventos e achados passam pela redação de segredos antes de ir à IA.

### Bard

- **O Bard lembra da conversa:** as últimas trocas da sessão (até 10, redigidas e limitadas a 3 KB) vão junto de cada pergunta, para "e os logs dele?" funcionar. O `/limpar` e o `/sessões` recomeçam a conversa; no modo batch, as perguntas da mesma entrada formam uma sessão.
- **Tetos no que vai à IA:** saída de ferramenta até 8 KB, notas do Synapstor até 8 KB somadas, cluster até 3 KB (20 pods, os com problema primeiro). O que passa do teto é cortado no meio com "… N linhas omitidas". O `kubectl_logs` aceita `--tail` até 200 (padrão 50), o `kubectl_events` devolve os 50 eventos mais recentes, e o scan do Sentinel chega resumido (contagem por severidade e os 10 mais graves).
- **Notas do Synapstor:** o corte de relevância é 0,35 (era 0,6), o mesmo da busca do Synapstor; um `relevance_threshold` no `.yby/bard.yaml` continua valendo, inclusive 0. Os trechos de um arquivo entram juntos, com título e caminho.
- **Índice de outro modelo:** em vez de "Synapstor indisponível", a etapa de contexto falha com "índice de outro modelo", e o aviso inteiro (feito com X, embeddings agora de Y, e o comando `yby synapstor index --full` que o refaz) aparece na conversa e, no `-p`, abaixo das etapas.
- **`kubectl_events` filtra pelo pod** quando há pod; o `kubectl_logs` ganha `container` e `previous`; o `sentinel_investigate` roda com `--no-ai` e o Bard analisa com o próprio provider.
- **Ferramentas YAML** (`.yby/tools` e `~/.yby/tools`) passam a ser escolhidas pelos `intents`, a do projeto vence a global de mesmo nome, e o `command` roda sem shell: cada `{{param}}` é um argumento inteiro (e também a env `YBY_PARAM_<NOME>`), com prazo de 2 min. Valor que começa com "-" é recusado, e no Windows um programa que roda pelo cmd.exe (`.bat`, `.cmd`) recusa valor com metacaractere do cmd. Comando que só funciona num shell não carrega. Valor da IA que faria o comando virar `delete namespace`, `delete clusterrole`, `drain`, `cordon`, `apply --force` ou `scale --replicas=0` é recusado ("operação bloqueada"); o que o próprio YAML já escreve no `command` não é barrado.
- **Cabeçalho e providers CLI:** o cabeçalho mostra o modelo do config (por exemplo "claude-cli · opus"), e o `claude-cli` e o `gemini-cli` passam a usar esse modelo (`ai.models`, `ai.model` ou `YBY_AI_MODEL`), que antes ignoravam. Os dois levam o prompt pelo stdin e o esc os interrompe.
- **Erro que não adianta repetir** (chave inválida, modelo inexistente, `ERR_TOKEN_LIMIT`) aparece na primeira tentativa, sem "tentando de novo".

### Synapstor

- `yby synapstor index` passa a indexar um trecho por seção `##` (IDs `caminho#n`), ignora `_Footer.md` e `_Sidebar.md`, e a busca mostra cada UKI uma vez. O `--full` apaga e recria o índice de verdade; arquivo que não abre mantém a entrada antiga com aviso; dois `index` ao mesmo tempo no projeto são barrados por uma trava (`.synapstor/.index/index.lock`) com a mensagem "outro yby synapstor index está rodando neste projeto".
- O resumo do `index` e do `search` diz de onde vêm os embeddings.
- `yby synapstor study` busca por termos do tópico (nome do arquivo pesa mais que caminho, que pesa mais que conteúdo), respeita o `.gitignore`, pula testes e goldens e responde em português.
- `yby synapstor quality` pontua pelas seções do padrão de cada tipo (regra, decisão, padrão, métrica e referência) e o resumo diz qual seção falta.
- `yby synapstor decay` marca como desatualizado o UKI cujo código citado teve commits depois da nota, e não só o que ficou 90 dias sem atividade.
- `yby synapstor export` grava `.synapstor-export.json` no destino e, na exportação seguinte, apaga só o que ele listou e sumiu da fonte; o formato Docusaurus escapa `{` e `<` fora de blocos de código para o MDX.

### Atlas

- `yby atlas diagram --detail full` não passa mais pela IA (a etapa aparece pulada, com o motivo); o diagrama montado a partir dos arquivos fica sempre também em `.yby/atlas-diagram.raw.mmd`.
- O Mermaid que a IA devolve só substitui o diagrama montado se conferir com os arquivos (nós declarados, `subgraph` fechados, só os IDs do rascunho), com prazo de 60 s; senão fica o montado, e a tela diz.
- O overview mantém workloads, charts, aplicações, módulos e serviços do Compose mesmo sem relação, e acima de 25 nós agrupa por tipo; as estatísticas contam o que está desenhado.
- A varredura respeita o `.gitignore` (via `git ls-files`, quando há git) e ignora também `dist`, `build`, `target`, `.venv`, `.next`, `.terraform`, `coverage` e `__pycache__`: recursos desses lugares somem do blueprint e do diagrama.
- Relações mais fiéis: Service seleciona só workloads do mesmo namespace, Kustomize liga os recursos que lista (com patches e namespace herdado), Terraform liga `module` e referências `tipo.nome`, Compose lê ports e volumes na sintaxe longa e mescla `docker-compose.*.yml` e `compose.*.yml` com o arquivo base. Os IDs dos recursos do Terraform fora da raiz ganham o diretório como prefixo.

### Viz

- `yby -c staging viz` (ou `YBY_ENV=staging yby viz`) abre no kubeconfig, no contexto e no namespace do ambiente do `.yby/environments.yaml`. `--context` e `--kubeconfig` trocam o cluster inteiro, `-n` troca só o namespace, e um `kube_context` que o kubeconfig não tem dá `ERR_CONFIG` antes de abrir a TUI. Sem ambiente pedido, nada muda: vale o current-context do kubeconfig.
- O quadro de logs (`l`) avisa quando o texto é da execução anterior do contêiner e lê no máximo 1 MiB; o YAML (`y`) sai sem `managedFields` nem a anotação `last-applied`.
- Aba que o RBAC não deixa listar mostra "sem permissão para listar …" em vez de ficar reconectando; quando o plugin de credencial do kubeconfig não entrega o token, a tela de erro sugere `yby cloud refresh`. As listas têm prazo de 10 s.

### Para autores de plugin

- **Manifesto:** campo novo `consome_contexto` (booleano, `PluginManifest.ConsomeContexto`). Declarar o hook `context` agora é fornecer contexto (o `yby init` usa), não consumir. Só quem declara `"consome_contexto": true` recebe no `command` o contexto caro (overview, backlog e os hooks `context` dos outros plugins); nenhum plugin nativo declara. Antes, quem tinha o hook `context` recebia tudo, o que levava `yby atlas …` e `yby synapstor …` a varrer o repositório a cada comando.
- **Contexto barato para todo plugin:** `project_name`, `environment`, `infra` (com o `kube_config` do ambiente, o `kube_context` e o `namespace`) e `values` vão no pedido a qualquer plugin, mas só quando o ambiente foi pedido: `yby -c staging <plugin>` (a flag vem antes do nome do plugin, em qualquer forma do pflag, inclusive `-cstaging`) ou `YBY_ENV`. Sem isso, nenhum ambiente vai (o `current` do `environments.yaml` não conta). Ambiente pedido que não existe é `ERR_VALIDATION`; flag que não é do core antes do nome do plugin também. Args com `-h` ou `--help` abrem sem contexto.
- O `UKIIndex` deixou de existir no contexto do core, e o Synapstor recebe o payload sem o embrulho duplo.
- O hook `context` do Atlas aceita o argumento `infra`, que acrescenta `infra` (o `InfraBlueprint`) ao lado de `blueprint`; só a ferramenta do Bard pede.

---

## Tabela de conferência (não vai para o CHANGELOG)

Cada tarefa `[x]` que muda saída, flag, código de saída, caminho, formato ou tecla, com o ponto do rascunho que a cobre.

| Tarefa | Mudança visível | Onde o rascunho cobre |
| :--- | :--- | :--- |
| K01 | `consome_contexto`, contexto barato com `-c`/`YBY_ENV`, `--help` sem contexto, `UKIIndex` removido | Para autores de plugin (1.º, 2.º e 3.º itens) |
| K02 | hook `context` do Atlas com argumento `infra`; blueprint lido pelo Bard | Para autores de plugin (4.º item) |
| K03, K04 | seções do system prompt removidas e testes de contrato: sem mudança que o usuário veja | interna (ver lista abaixo) |
| B01 | Bard só roda plugin confiável, `.exe` no Windows, aviso no `/ferramentas` | Quebra scripts (5.º item) |
| B02 | memória da conversa, `/limpar` e `/sessões` recomeçam | Bard (1.º item) |
| B03 | classificador com esquema e namespace da sessão | interna: melhora a escolha de ferramenta, sem flag nem formato |
| B04 | confiança para `.yby/tools` e `.yby/prompts`, `yby plugin trust <arquivo>`, projeto vence global, `command` sem shell | Bard (ferramentas YAML) |
| B05 | notas do Synapstor mantidas quando uma ferramenta roda | interna: corrige o prompt, sem mudança de interface |
| B06 | `kubectl_get` só `wide`/`name`, valor com "-" recusado, saída redigida | Quebra scripts (9.º item) |
| B07 | tetos, "… N linhas omitidas", `--tail` até 200, 50 eventos, scan resumido | Bard (2.º item) |
| B08 | erro permanente sem nova tentativa | Bard (último item) |
| B09 | prazo no enriquecimento e esc mata o plugin | interna: não mudou formato nem flag (a TUI abre mais rápido com o cluster fora) |
| B10 | prompt dos CLIs pelo stdin, esc interrompe | Bard (cabeçalho e providers CLI) |
| B11 | system prompt sem JSON de ferramenta, guardas nas ferramentas YAML | Bard (ferramentas YAML) |
| B12 | `kubectl_events` por pod, `container` e `previous`, `sentinel_investigate --no-ai` | Bard (5.º item) |
| B13 | cabeçalho com o modelo do config; CLIs usam `--model` | Bard (cabeçalho e providers CLI) |
| B14 | decorators nos providers CLI: sem mudança visível além da janela de contexto | interna |
| B15 | corte de relevância 0,35, aviso do índice de outro modelo | Bard (3.º e 4.º itens) |
| Y01 | índice versionado por provider/modelo/dimensão, refaz ao trocar de modelo | Quebra scripts (6.º e 7.º itens); Synapstor (1.º item) |
| Y02 | `--full` refaz de verdade, IDs com `/` | Synapstor (1.º item) |
| Y03 | manifest em `.synapstor/.index/manifest.json` | Quebra scripts (6.º item) |
| Y04 | topK limitado, coleção vazia sem erro | interna (sem erro do chromem na busca em índice vazio) |
| Y05 | trechos por seção, IDs `caminho#n`, reindexação | Quebra scripts (6.º item); Synapstor (1.º item) |
| Y06 | modelo multilíngue int8, ~120 MB, resumo com a origem dos embeddings | Quebra scripts (7.º item); Synapstor (2.º item) |
| Y07 | formato de UKI em PT-BR com frontmatter e subpastas | Quebra scripts (8.º item) |
| Y08 | `study` por termos, `.gitignore`, PT-BR | Synapstor (3.º item) |
| Y09 | arquivo ilegível preservado, trava do índice | Synapstor (1.º item) |
| Y10 | `decay` mede o código citado | Synapstor (5.º item) |
| Y11 | `quality` pelas seções do tipo | Synapstor (4.º item) |
| Y12 | export com manifesto e MDX escapado | Synapstor (6.º item) |
| S01 | `--fix` no dono do pod, confirmação, `--yes` | Quebra scripts (3.º item); Sentinel (1.º item) |
| S03 | `resources` no JSON, "(+N)" sai de `resource` | Quebra scripts (1.º item) |
| S04 | `--scope`, campo `scope`, RBAC de namespace | Sentinel (2.º item) |
| S05 | severidade e categorias reais, seções "confiabilidade" e "eficiência" | Sentinel (3.º item) |
| S06 | `type` = `severity`, PT-BR no JSON, `nao_rodaram` | Quebra scripts (1.º item) |
| S07 | `--profile` filtra todos os motores; `perfil` e `fora_do_perfil` | Sentinel (5.º item) |
| S08 | checks internos como plano B, cosign, env sensível | Sentinel (6.º item) |
| S09 | allowlist por subject, `ignored_by_allowlist` | Sentinel (4.º item) |
| S10 | `investigate` com saúde antes do cache, logs por contêiner | Sentinel (último item) |
| S11 | redação de segredos antes da IA | Sentinel (último item) |
| S12 | `investigate --no-ai` | Sentinel (7.º item) |
| S14 | nome e conteúdo do relatório | Quebra scripts (2.º item) |
| A02 | `--detail full` sem IA | Atlas (1.º item) |
| A03 | overview com teto de 25 nós e agrupamento | Atlas (3.º item) |
| A04, A05, A06, A07, A08 | relações e nomes do blueprint/diagrama | Atlas (último item); A07 (nome de template Helm) só nessa linha, sem destaque |
| A09 | `.gitignore` e ignorados | Atlas (4.º item) |
| A10 | Mermaid da IA validado, prazo de 60 s, `.raw.mmd` | Atlas (1.º e 2.º itens) |
| A11 | caminhos relativos e com `/` | Quebra scripts (10.º item) |
| V01 | exclusão com propagação e UID | Quebra scripts (11.º item) |
| V02 | Viz no ambiente do core | Viz (1.º item) |
| V05 | prazo, aba sem permissão, dica `yby cloud refresh` | Viz (3.º item) |
| V06 | log anterior avisado, 1 MiB, YAML sem ruído | Viz (2.º item) |

**Tarefas `[x]` que julguei internas, sem linha própria:** C01, C02 (CI e lint), R01 (pacote de redação, aparece só como "sem segredos" no Bard e no Sentinel), K03, K04, B03, B05, B09, B14, Y04, Y13 (código morto), S02 (Polaris no kubeconfig do yby), S13 (OPA compila uma vez), S15 (log do Polaris), A01 (relações falsas, só desaparecem), A12 (ordem determinística e goldens), A13 (código morto), V03 (métricas de pod: o `fetchPodMetrics` guardava CPU e memória do pod, mas nenhuma tela os mostrava, então não há mudança visível), V04 (memória dos informers), V07 (desempenho do render).
