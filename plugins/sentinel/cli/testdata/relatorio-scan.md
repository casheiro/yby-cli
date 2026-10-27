# Relatório de Segurança Sentinel — payments

**Resumo:** 2 críticos, 1 alto, 1 médio, 2 baixos
**Backends:** polaris, opa

## Vulnerabilidades

### Segurança dos pods

- ✕ `Deployment/api`: Should not be allowed to run as root
- ▲ `Deployment/web`: HostPath volumes must be forbidden

### RBAC

- ✕ `ClusterRoleBinding/ci-deployer`: ServiceAccount payments/ci com cluster-admin

### Rede

- ▲ `Namespace/payments`: Namespace sem NetworkPolicy default-deny

### Confiabilidade

- ○ `StatefulSet/redis`: Image pull policy should be Always

### Eficiência

- ○ `Deployment/web`, `Deployment/worker`, `Deployment/local-path-provisioner-com-nome-bem-comprido`: Memory limits should be set

## Recomendações

1. Tire o cluster-admin do ci-deployer.
2. Defina limites de memória.
