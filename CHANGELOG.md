# Changelog

As notas completas de cada versão, geradas a partir dos commits, ficam nas [releases do GitHub](https://github.com/casheiro/yby-cli/releases). Aqui ficam as mudanças que pedem atenção de quem já usa o yby: teclas que mudaram, códigos de saída e comportamentos novos.

## Próxima versão

- **`go install` muda de caminho:** passa a ser `go install github.com/casheiro/yby-cli/v5/cmd/yby@latest`. O caminho antigo, sem o `/v5`, instalava a v1.12.2. Quem instalou assim deve apagar `%USERPROFILE%\go\bin\yby.exe` (no Linux e no macOS, `~/go/bin/yby`) e reinstalar: essa pasta costuma vir antes no PATH, e o yby antigo continua sendo o que roda.
- **Plugins de terceiros em Go** importam `github.com/casheiro/yby-cli/v5/pkg/plugin/sdk` (antes, `github.com/casheiro/yby-cli/pkg/plugin/sdk`).
- **Instalação no Windows pelo `install.ps1`:** `irm https://raw.githubusercontent.com/casheiro/yby-cli/main/install.ps1 | iex` instala em `%LOCALAPPDATA%\Programs\yby`, sem administrador, e põe a pasta no PATH do usuário. O `install.sh` passa a conferir o SHA-256 do download.
- **`yby uninstall` funciona no Windows:** o `yby.exe` sai do lugar na hora e some do disco quando o yby fecha. Antes, falhava com "Acesso negado".
- **`yby version` mostra a versão também num yby do `go install`**, em vez de `dev`.
- **`yby upgrade` volta a funcionar.** Até a v5.8.3 ele procurava o pacote `yby_v<versão>_…` (e `.tar.gz` também no Windows), que não existe, e parava em "asset não encontrado". Quem está na v5.8.3 ou antes chega a esta versão pelo `install.sh`, pelo `install.ps1` ou pelo pacote da release, uma vez; daí em diante o `yby upgrade` atualiza. Ele também diz "em dia" quando já está na última versão, aceita `--version 5.8.3` sem o "v" e, no `yby-cloud`, baixa o pacote `yby-cloud_…`, não o do yby padrão.
- **`yby plugin install` de plugin nativo funciona no Windows** (o pacote da release é `.zip`, que antes dava "unsupported archive format"). Na v5.8.3 do Windows, instale os plugins à mão: extraia o `yby-plugin-<nome>_5.8.3_windows_amd64.zip` da release em `%USERPROFILE%\.yby\plugins` e rode `yby plugin trust <nome>`.
- **`yby plugin update` atualiza os plugins nativos** para a versão do yby em execução. Antes, pedia uma tag `vlatest`, que não existe, e falhava. `--version latest` no `yby plugin install` passa a ser recusado com a dica de informar a tag.
- **Projetos do `yby init`:** o devcontainer e o pipeline de feature gerados instalam o yby pelo `install.sh`; antes, baixavam arquivos que não existem nas releases e falhavam.

## v5.8.3: plugins não confiáveis e namespace do Sentinel

- **`yby sentinel scan` e `investigate` sem `-n`** examinam o namespace do ambiente pedido (`yby -c <ambiente> sentinel …`) ou, sem ele, o do contexto do kubeconfig, como o kubectl e o Viz; `default` só quando nenhum dos dois diz. Antes, era sempre `default`. Em scripts, passe `-n`.
- **Plugins não confiáveis** (fora de `yby plugin trust`) não rodam nem para a descoberta: aparecem na ajuda e no `yby plugin list` como "não confiável · rode yby plugin trust <nome>", e o erro ao abrir um deles dá o nome certo para o `trust`.
- `yby sentinel investigate` volta a mostrar as métricas do pod sem ambiente pedido.

## v5.8.2: correções dos plugins

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

- `yby -c staging viz` (ou `YBY_ENV=staging yby viz`) abre no kubeconfig, no contexto e no namespace do ambiente do `.yby/environments.yaml`. `--context` e `--kubeconfig` trocam o cluster inteiro, `-n` troca só o namespace, e um `kube_context` que o kubeconfig não tem dá `ERR_CONFIG_INVALID` antes de abrir a TUI. Sem ambiente pedido, nada muda: vale o current-context do kubeconfig.
- O quadro de logs (`l`) avisa quando o texto é da execução anterior do contêiner e lê no máximo 1 MiB; o YAML (`y`) sai sem `managedFields` nem a anotação `last-applied`.
- Aba que o RBAC não deixa listar mostra "sem permissão para listar …" em vez de ficar reconectando; quando o plugin de credencial do kubeconfig não entrega o token, a tela de erro sugere `yby cloud refresh`. As listas têm prazo de 10 s.

### Para autores de plugin

- **Manifesto:** campo novo `consome_contexto` (booleano, `PluginManifest.ConsomeContexto`). Declarar o hook `context` agora é fornecer contexto (o `yby init` usa), não consumir. Só quem declara `"consome_contexto": true` recebe no `command` o contexto caro (overview, backlog e os hooks `context` dos outros plugins); nenhum plugin nativo declara. Antes, quem tinha o hook `context` recebia tudo, o que levava `yby atlas …` e `yby synapstor …` a varrer o repositório a cada comando.
- **Contexto barato para todo plugin:** `project_name`, `environment`, `infra` (com o `kube_config` do ambiente, o `kube_context` e o `namespace`) e `values` vão no pedido a qualquer plugin, mas só quando o ambiente foi pedido: `yby -c staging <plugin>` (a flag vem antes do nome do plugin, em qualquer forma do pflag, inclusive `-cstaging`) ou `YBY_ENV`. Sem isso, nenhum ambiente vai (o `current` do `environments.yaml` não conta). Ambiente pedido que não existe é `ERR_CONFIG_INVALID`; flag que não é do core antes do nome do plugin é `ERR_VALIDATION`. Args com `-h` ou `--help` abrem sem contexto.
- O `UKIIndex` deixou de existir no contexto do core, e o Synapstor recebe o payload sem o embrulho duplo.
- O hook `context` do Atlas aceita o argumento `infra`, que acrescenta `infra` (o `InfraBlueprint`) ao lado de `blueprint`; só a ferramenta do Bard pede.

## v5.8.0: carta do terminal (Viz, Cloud e plugins)

### Viz (`yby viz`)

![Viz com o detalhe do pod ao lado](docs/assets/telas/viz-pods.png)

Teclas que mudaram:

| Antes | Agora | Ação |
| :--- | :--- | :--- |
| `/` | `n` | trocar o namespace (vazio: todos) |
| `f` | `/` | buscar por nome, enquanto digita |
| `L` e `S` | `f` | filtrar por status (só pods) ou por label (texto com `=`, `!`, `,` ou parênteses) |
| `d` | `x` → excluir | excluir, agora com confirmação pelo nome |
| `s` | `x` → escalar | escalar deployments e statefulsets |
| `r` | `x` → reiniciar | reiniciar deployments e statefulsets |
| — | `r` | recarregar a aba |
| `Esc` limpa filtros | `Esc` fecha a camada de cima; sem camada, limpa os filtros | |
| `q` e `ctrl+c` saem | `q` sai só da lista; `ctrl+c` sai de qualquer lugar | |

As teclas antigas (`d`, `s`, `L`, `S`) avisam na linha de estado onde a ação foi parar.

Novidades:

- 9 abas (Pods, Deployments, StatefulSets, Services, Nodes, Jobs, Ingresses, ConfigMaps, Eventos), trocadas por `1`–`9` ou `⇥`/`⇧⇥`, com a contagem ao vivo pelo watch do cluster.
- Problemas primeiro na lista e o mais grave na linha de estado.
- Detalhe ao lado a partir de 120 colunas e por cima abaixo disso (`↵`); quadros de logs (`l`), YAML (`y`) e eventos (`e`).
- `b` abre o Bard com uma pergunta sobre o recurso selecionado, escrita no campo e sem enviar; o `esc` do Bard volta ao Viz.
- `x` abre o menu de ações; `?` mostra a ajuda; o mouse rola e seleciona linha e aba.
- `/` e `?` digitados com AltGr (teclado ABNT2 no Windows) funcionam.
- Janela abaixo de 60×16 mostra "aumente a janela" e cancela o que estava aberto.

### Cloud (`yby cloud`), em beta

![Painel dos clusters cloud](docs/assets/telas/cloud-painel.png)

Os comandos `yby cloud` estão em **beta**: testados com os CLIs `aws`, `az` e `gcloud` e conferidos contra a documentação oficial, mas ainda não contra clusters reais. O que falta para sair do beta está no [ROADMAP](./ROADMAP.md).

Teclas do painel (`yby cloud dashboard`) que mudaram:

| Antes | Agora | Ação |
| :--- | :--- | :--- |
| `r` | `t` | renovar o token do cluster selecionado |
| `R` | `r` | recarregar o painel |
| `Enter` (detalhes) | `↵` | abrir o cluster no Viz; abaixo de 120 colunas, abre o detalhe |
| `Esc` sai | `Esc` fecha a camada de cima ou limpa a busca; `q` sai | |
| — | `/` | buscar pelo nome do cluster |
| — | `?` | ajuda |

O `R` antigo avisa que recarregar agora é `r`.

Outras mudanças no cloud:

- Painel novo: resumo no topo, colunas por largura, barra de validade do token, detalhe com identidade e a auditoria recente, aviso quando uma leitura volta parcial.
- `yby cloud list`, `status`, `refresh` e `audit` no formato da carta. Os erros dos CLIs mostram a linha de erro do CLI, com os segredos ocultos.
- Região do EKS vem do ARN do cluster; sem ela, de `AWS_REGION`, `AWS_DEFAULT_REGION` e `aws configure get region`.
- `cloud refresh` e `cloud connect` gravam no audit log.

### Bard

- `/modelo` troca o provider, o modelo ou os dois, só para a sessão: `/modelo <provider>`, `/modelo <provider>/<modelo>` ou `/modelo <modelo>`. Sem argumento, lista os providers e os modelos de cada um.
- `yby bard --pergunta "…" --voltar <instrumento>` abre a TUI com a pergunta escrita no campo; o `esc` volta a quem abriu.

### Sentinel

- `yby sentinel scan` com o cluster fora do ar sai com erro `ERR_CLUSTER_OFFLINE` e código 1 (antes dizia "nenhum problema" e saía com 0).
- Namespace que não existe dá erro de validação, sem gravar relatório.
- Achados só do namespace pedido, em ordem fixa, em colunas; resumo das recomendações da IA na tela.

### Synapstor e Atlas

- Saídas no formato da carta: etapas, colunas e caminhos com `~`.
- O Atlas diz quando a IA respondeu sem diagrama e fica com o diagrama montado a partir dos arquivos.
- Os modelos de embeddings do Ollama não aparecem mais como opção de conversa.

### Para quem desenvolve

- A guarda `pkg/ui/guarda` roda no `task test` e no pipeline e falha com emoji de status em texto ou cor do lipgloss montada à mão fora do `pkg/ui`.

## v5.7.0: carta do terminal (core e Bard)

- Ajuda de todos os comandos em português, no formato da carta (`yby <comando> --help`).
- Modo visual: `YBY_THEME` (`auto`, `dark`, `light`), `YBY_ASCII=1` e `NO_COLOR`, ou `ui.theme` e `ui.ascii` no `~/.yby/config.yaml`.
- `yby init` abre um assistente; sem terminal, exige `--non-interactive`.
- Bard com a TUI nova: comandos `/` em português (`/sessões`, `/limpar`, `/contexto`, `/ferramentas`, `/modelo`, `/copiar`, `/exportar`, `/ajuda`). Os apelidos antigos (`/clear`, `/session`, `/sessions`, `/help`) valem digitados por inteiro.
- Teclas do Bard: `ctrl+j` ou `alt+enter` faz nova linha; só `pgup`/`pgdn` e o mouse rolam; `↑` no campo vazio edita a última pergunta; `↵` no campo vazio abre a saída da ferramenta; `ctrl+y` copia o último comando; `esc` interrompe a resposta; `ctrl+c` sai.
- `yby bard -p` e o modo batch mandam só a resposta para o stdout e saem com 1 em falha e 130 no `ctrl+c`.
