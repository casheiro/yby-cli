# Tasks — Testes Unitários do Plugin Sentinel CLI

**Branch:** feat/sentinel-unit-tests
**Escopo:** `plugins/sentinel/cli/`

## Contexto

`cache.go`, `report.go` e `scan.go` não possuem arquivos de teste individuais.
`integration_test.go` já contém alguns testes misturados para `cache` e `report`,
mas `scan.go` não tem nenhum teste. As funções de detecção de segurança em `scan.go`
estão embutidas em `scanNamespace` (que depende do k8s client), tornando-as
não testáveis unitariamente sem refatoração.

---

### Phase 1: Refatoração para Testabilidade

- [x] T001 [US-01] Extrair lógica de detecção de segurança de `scanNamespace` em funções puras: `checkRootContainer`, `checkResourceLimits`, `checkImagePullPolicy`, `checkExposedSecrets` — cada uma recebe `pod corev1.Pod, container corev1.Container, namespace string` e retorna `[]SecurityFinding`; atualizar `scanNamespace` para compor essas funções -- plugins/sentinel/cli/scan.go
  * Status: Done

### Phase 2: Testes Unitários

- [x] T002 [US-01] Criar `plugins/sentinel/cli/cache_test.go` com: `TestCacheKey_Deterministico`, `TestCacheKey_InputsDiferentes`, `TestCacheKey_TruncaLogsAcima500Chars`, `TestSaveAndLoadCache_RoundTrip`, `TestLoadCache_TTLExpirado` (manipula `entry.Timestamp` via arquivo JSON no tmpDir), `TestLoadCache_ArquivoInexistente` — build tag `k8s` -- plugins/sentinel/cli/cache_test.go
  * Status: Done

- [x] T003 [US-01] Criar `plugins/sentinel/cli/report_test.go` com: `TestExportJSON_FormatoValido`, `TestExportJSON_MetadadosCorretos`, `TestExportMarkdown_SecoesObrigatorias`, `TestExportMarkdown_KubectlPatchNil`, `TestExportMarkdown_KubectlPatchNone` (não deve gerar seção "Comando Sugerido"), `TestWriteReport_ParaArquivo`, `TestWriteReport_ParaStdout` (captura `os.Stdout`) — build tag `k8s` -- plugins/sentinel/cli/report_test.go
  * Status: Done

- [x] T004 [US-01] Criar `plugins/sentinel/cli/scan_test.go` com: `TestCheckRootContainer_RunAsRoot`, `TestCheckRootContainer_SemSecurityContext`, `TestCheckRootContainer_RunAsNonRootTrue`, `TestCheckResourceLimits_SemLimites`, `TestCheckResourceLimits_ComLimites`, `TestCheckImagePullPolicy_NaoAlways`, `TestCheckImagePullPolicy_Always`, `TestCheckExposedSecrets_EnvHardcoded`, `TestCheckExposedSecrets_EnvViaSecretKeyRef`, `TestExportScanMarkdown_SemFindings`, `TestExportScanMarkdown_ContaCriticaisEAvisos`, `TestExportScanMarkdown_ComRecomendacoes` — build tag `k8s` -- plugins/sentinel/cli/scan_test.go
  * Status: Done
