# Changelog

As notas completas de cada versão, geradas a partir dos commits, ficam nas [releases do GitHub](https://github.com/casheiro/yby-cli/releases). Aqui ficam as mudanças que pedem atenção de quem já usa o yby: teclas que mudaram, códigos de saída e comportamentos novos.

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
