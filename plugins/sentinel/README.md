# Sentinel Plugin

Sentinel e o plugin de auditoria de seguranca e conformidade K8s do Yby CLI. Usa **Polaris** e **OPA** como backends de seguranca reais, com IA para recomendacoes e investigacao de pods.

## Funcionalidades

- **Security Scan**: auditoria via Polaris (pod security) + OPA (RBAC, network) com deduplicacao e agrupamento
- **Investigacao IA**: diagnostico inteligente de pods — so aciona IA quando detecta problemas reais
- **Remediacao**: geracao e aplicacao de patches (dry-run ou aplicacao direta)
- **Relatorios**: resumo no terminal + relatorio completo em `~/.yby/reports/`
- **Cache**: resultados de investigacao em `~/.yby/sentinel/cache/` (TTL 1h)

## Arquitetura

```
plugins/sentinel/
└── cli/
    ├── main.go              # Entry point, roteamento de hooks, investigate
    ├── scan.go              # Orquestracao do scan (backends → dedup → IA → relatorio)
    ├── kubeclient.go        # Wrapper do cliente K8s via SDK
    ├── cache.go             # Cache de investigacoes (~/.yby/sentinel/cache/)
    ├── report.go            # Geracao de relatorios (JSON/Markdown)
    ├── prompts.go           # Prompts de IA para scan e investigacao
    ├── backends/            # Backends de seguranca reais
    │   ├── types.go         # Interface SecurityBackend + Finding
    │   ├── polaris.go       # Polaris SDK — pod security, best practices
    │   └── opa.go           # OPA SDK — politicas Rego embarcadas (RBAC, network)
    ├── checks/              # Checks artesanais (fallback se backends falham)
    │   ├── types.go         # Interface SecurityCheck + SecurityFinding
    │   ├── registry.go      # Registro global de checks
    │   ├── rbac_allowlist.go # Allowlists de recursos do sistema K8s
    │   └── *.go             # 20 checks individuais
    ├── profiles/            # Perfis de compliance
    │   ├── cis.go           # CIS Benchmark Level 1 e 2
    │   ├── pci.go           # PCI-DSS
    │   └── soc2.go          # SOC2
    └── remediation/         # Remediacao automatizada
        ├── generator.go     # Gera patches a partir de findings
        └── applier.go       # Aplica patches no cluster
```

## Backends de Seguranca

O Sentinel usa ferramentas de seguranca reais como backends em vez de checks artesanais:

| Backend | SDK | O que escaneia | Allowlists |
|---------|-----|----------------|------------|
| **Polaris** | `fairwindsops/polaris` | Pod security, best practices, resource limits, probes, topology | Built-in do Polaris |
| **OPA** | `open-policy-agent/opa` | RBAC (cluster-admin, wildcard, secrets), NetworkPolicy | Rego embarcado com exclusoes de system:* e controllers conhecidos |

**Fallback**: se nenhum backend funcionar, cai nos checks artesanais internos.

**Deduplicacao**: findings do mesmo check sao agrupados — mostra "Deployment/api (+5)" em vez de repetir pra cada workload.

## Uso

```bash
# Scan de seguranca (usa Polaris + OPA automaticamente)
yby sentinel scan -n default
yby sentinel scan -n nexus-core

# Exportar relatorio
yby sentinel scan -n default -o json -f scan.json
yby sentinel scan -n default -o markdown -f relatorio.md

# Remediacao
yby sentinel scan -n default --fix-dry-run    # ver patches sem aplicar
yby sentinel scan -n default --fix            # aplicar patches

# Investigacao de pod com IA (so aciona IA se detectar problemas)
yby sentinel investigate meu-pod -n default
yby sentinel investigate meu-pod -n default --no-cache

# Ajuda
yby sentinel --help
```

## Investigacao Inteligente

O `investigate` verifica a saude do pod **antes** de chamar a IA:

- **Pod saudavel** (Running, Ready, sem restarts, sem Warning events) → retorna direto sem chamar IA
- **Pod com problemas** (CrashLoopBackOff, eventos Warning, restarts, not Ready) → coleta logs, eventos e metricas, envia pra IA diagnosticar

Isso evita desperdicar chamadas de IA e gerar "recomendacoes" para pods que nao precisam de correcao.

## Relatorios

O scan gera automaticamente:
- **Terminal**: resumo por severidade e categoria + path do relatorio
- **Arquivo**: `~/.yby/reports/sentinel-scan-{namespace}-{data}.md` com findings detalhados + recomendacoes IA

## Providers de IA

Ordem de prioridade configuravel via `~/.yby/config.yaml` (`ai.priority`):

1. Ollama (local)
2. Claude Code CLI
3. Gemini CLI
4. Gemini API
5. OpenAI API

## Build Tag

O Sentinel requer a build tag `k8s`:

```bash
go build -tags k8s ./plugins/sentinel/cli/...
go test -tags k8s ./plugins/sentinel/cli/...
```
