# Especificação de UKI (projeto: yby-cli)

> **Unidade de Conhecimento Interligada (UKI, de Unit of Knowledge Interlinked)**
> A menor unidade de conhecimento do projeto: descreve uma regra, decisão, padrão, métrica ou referência que continua valendo.

## 1. Estrutura do arquivo

Os UKIs ficam em `.synapstor/.uki/`, com o nome `UKI-<timestamp>-<slug>.md` (o timestamp é o Unix de quando o UKI foi criado). Subpastas por domínio são aceitas:
`.synapstor/.uki/[<dominio>/]UKI-<timestamp>-<slug>.md`

O quality, o decay, o export e o index leem a pasta e as subpastas; as pastas ocultas (como a `.obsidian`) ficam de fora.

### 1.1 Frontmatter obrigatório

```yaml
---
id: <dominio>.<subdominio>.<nome-em-kebab-case>
tipo: regra | decisao | padrao | metrica | referencia
status: rascunho | ativo | obsoleto
criado_em: AAAA-MM-DD
atualizado_em: AAAA-MM-DD
tags: [lista, de, tags]
---
```

O `capture` e o `study` montam o frontmatter: o ID, o tipo e as tags vêm da IA, o status começa em `rascunho` e as datas são as do dia. A IA escreve só o corpo, que começa pelo título (`# Título`).

### 1.2 Formatos antigos

O Synapstor também lê os metadados dos formatos anteriores a esta especificação:

- o frontmatter em inglês (`uki_id`, `type`, `created_at`, `updated_at`, com `rule`, `decision`, `pattern`, `metric`, `reference`, `active`, `proposal`, `draft` e `deprecated`), lido com os nomes em PT-BR;
- as linhas em negrito no começo do corpo (`**ID:**`, `**Tipo:**` ou `**Type:**`, `**Status:**`, `**Tags:**`).

## 2. Padrões de conteúdo

Cada tipo tem as suas seções (`##`), na ordem abaixo.

### 2.1 Regra (`regra`)
Regras de negócio ou limites técnicos rígidos.
- **Contexto**: por que a regra existe.
- **Enunciado**: a regra em si (deve / não deve).
- **Exceções**: os casos conhecidos em que ela não vale.

### 2.2 Decisão (`decisao`), um ADR leve
Registro de escolhas de arquitetura.
- **Problema**: o que precisava ser resolvido.
- **Opções**: o que foi considerado.
- **Decisão**: o que foi escolhido e por quê.
- **Consequências**: os impactos positivos e negativos.

### 2.3 Padrão (`padrao`)
Guias de estilo ou de implementação.
- **Quando usar**: o gatilho para aplicar o padrão.
- **Exemplo**: um bloco de código ou a estrutura de referência.
- **Antipadrão**: o que não fazer.

### 2.4 Métrica (`metrica`)
Indicadores que o projeto acompanha.
- **Contexto**: o que a métrica mostra e por que importa.
- **Definição**: como se mede (fonte, fórmula, frequência).
- **Meta**: o valor esperado e o que fazer fora dele.

### 2.5 Referência (`referencia`)
Como um componente funciona; é o tipo do `study`.
- **Visão geral**: o que é o componente e por que existe.
- **Arquitetura**: como funciona por dentro.
- **Referências no código**: os arquivos e as funções principais.

## 3. Ciclo de vida

1. Um agente identifica um padrão repetido e sugere um UKI.
2. O UKI nasce em `status: rascunho`.
3. Validado pelo **Governance Steward**, passa a `status: ativo`.
4. Quando deixa de valer, passa a `status: obsoleto`.

A cada mudança no conteúdo, o `atualizado_em` passa para a data do dia.

---
**Exemplo de caminho**: `.synapstor/.uki/cli/UKI-1761480000-saida-json.md`
**Exemplo de ID**: `cli.ux.estrutura-flag-json`
