# Relatório Sentinel — payments/worker-5d8b6-q7tvn

**Data:** <data>

## Causa Raiz

O worker não encontra o banco

## Detalhe Técnico

DB_HOST aponta para localhost e o contêiner reinicia a cada 30 s.

## Confiança

82%

## Sugestão de Correção

Aponte DB_HOST para o Service do Postgres.

## Comando Sugerido

```bash
kubectl -n payments set env deploy/worker DB_HOST=postgres.payments
```
