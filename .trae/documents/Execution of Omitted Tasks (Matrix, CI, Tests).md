Based on the re-analysis of `PLANO-DE-CORRECAO.md`, I have identified **3 pending items** that were omitted from the previous execution cycle:

### 1. Documentation Matrix in README
*   **Description**: Add a specific "Code ↔ Documentation Matrix" section to `README.md` as explicitly requested in the "Governança de Documentação" section of the plan.
*   **Deadline**: Immediate.
*   **Owner**: Trae AI.
*   **Validation**: Matrix present in README linking code components to wiki pages.

### 2. CI/CD Integration (`docs:verify`)
*   **Description**: The plan requires "Job CI 'docs:verify' obrigatório". The script exists, but it's not hooked into the repository's GitHub Actions workflows.
*   **Deadline**: Immediate.
*   **Owner**: Trae AI.
*   **Validation**: `.github/workflows/feature-pipeline.yaml` (or equivalent) includes a step to run `.trae/scripts/docs_verify.sh`.

### 3. Unit Tests Execution
*   **Description**: "Todos os testes unitários e E2E relevantes passam". Only build was verified.
*   **Deadline**: Immediate.
*   **Owner**: Trae AI.
*   **Validation**: `go test ./...` returns `PASS`.

I will update the execution plan to include these tasks and proceed with them.