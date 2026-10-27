# Relatório de Segurança Sentinel — payments

**Contexto:** kind-yby
**Gerado em:** 2026-10-25 09:30:00
**Resumo:** 2 críticos, 1 alto, 1 médio, 2 baixos
**Perfil:** pci-dss · 2 achados fora do perfil
**Backends:** polaris, opa
**Não rodou:** opa · sem permissão para listar ClusterRoles

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
