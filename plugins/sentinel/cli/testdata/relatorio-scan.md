# Relatório de Segurança Sentinel — payments

**Resumo:** 2 críticos, 1 alto, 1 médio, 1 baixo
**Backends:** polaris, opa

## Vulnerabilidades

- ▲ **[pod-security]** `Deployment/web`, `Deployment/worker`, `Deployment/local-path-provisioner-com-nome-bem-comprido`: Memory limits should be set
- ✕ **[rbac]** `ClusterRoleBinding/ci-deployer`: ServiceAccount payments/ci com cluster-admin
- ✕ **[pod-security]** `Deployment/api`: Should not be allowed to run as root
- ▲ **[network]** `Namespace/payments`: Namespace sem NetworkPolicy default-deny
- ○ **[supply-chain]** `StatefulSet/redis`: Image pull policy should be Always

## Recomendações

1. Tire o cluster-admin do ci-deployer.
2. Defina limites de memória.
