---
description: Workflow para capturar novas Unidades de Conhecimento (UKIs)
---
# Workflow: UKI Capture

Este workflow guia a criação de uma nova UKI quando um padrão, regra ou decisão é identificado.

## 1. Trigger
- Um agente ou usuário identifica um conhecimento repetível ou uma decisão arquitetural.
- Uso do comando `/uki-capture`.

## 2. Passos

### Passo 1: Identificação
- O agente pergunta: "Qual é o conhecimento a ser capturado?"
- O agente pergunta: "Isso é uma Regra, Decisão, Padrão ou Métrica?"

### Passo 2: Rascunho
- O agente propõe o ID da UKI seguindo `dominio.subdominio.nome`.
- O agente preenche o template definido em `.synapstor/UKI_SPEC.md`.

### Passo 3: Validação
- O agente pergunta: "Este conhecimento conflita com alguma UKI existente?"
- O agente solicita revisão do usuário (ou Governance Steward).

### Passo 4: Persistência
- Salvar o arquivo em `.synapstor/.uki/<dominio>/<id>.md`.
- (Opcional) Adicionar entrada em `.synapstor/01_EXECUTION_LOG.md`.

## 3. Personas Envolvidas
- **Governance Steward**: Valida a estrutura e semântica.
- **Qualquer Agente**: Pode propor a criação.
