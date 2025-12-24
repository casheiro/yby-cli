# UKI Specification (Entity: yby-cli)

> **Unit of Knowledge Intelligence (UKI)**
> Menor unidade atômica de conhecimento que descreve uma regra, decisão, padrão ou métrica perene do projeto.

## 1. Estrutura do Arquivo

Todas as UKIs devem residir em `.synapstor/.uki/` e seguir esta estrutura de pastas:
`.synapstor/.uki/<dominio>/<id-da-uki>.md`

### 1.1 Frontmatter Obrigatório

```yaml
---
uki_id: <dominio>.<subdominio>.<nome-kebab-case>
type: [rule | decision | pattern | metric]
status: [active | deprecated | proposal]
created_at: YYYY-MM-DD
updated_at: YYYY-MM-DD
tags: [lista, de, tags]
---
```

## 2. Padrões de Conteúdo

### 2.1 Regra (Rule)
Regras de negócio ou limites técnicos rígidos.
- **Contexto**: Por que essa regra existe?
- **Enunciado**: A regra em si (deve/não deve).
- **Exceções**: Casos conhecidos onde ela não se aplica.

### 2.2 Decisão (Decision) aka ADR light
Registros de escolhas arquiteturais.
- **Problema**: O que precisava ser resolvido.
- **Opções**: O que foi considerado.
- **Decisão**: O que foi escolhido e porquê.
- **Consequências**: Impactos positivos e negativos.

### 2.3 Padrão (Pattern)
Guias de estilo ou implementação.
- **Quando usar**: Gatilho para aplicar o padrão.
- **Template/Exemplo**: Bloco de código ou estrutura de referência.
- **Anti-pattern**: O que não fazer.

## 3. Workflow de Vida

1. Agent identifica um padrão repetido -> Sugere criação de UKI.
2. Cria em `status: proposal`.
3. Validado pelo **Governance Steward** -> Muda para `status: active`.
4. Obsoleto -> Muda para `status: deprecated`.

---
**Exemplo de Path**: `.synapstor/.uki/cli/ux-command-output.md`
**Exemplo de ID**: `cli.ux.json-flag-structure`
