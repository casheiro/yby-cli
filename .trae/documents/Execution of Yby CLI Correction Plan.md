I will orchestrate the execution of the correction plan defined in `PLANO-DE-CORRECAO.md`, following the incremental strategy.

### Phase 0: Governance & Quality
1.  **Create Verification Script**: Implement `.trae/scripts/docs_verify.sh` to validate:
    *   Presence of commands in `CLI-Reference.md`.
    *   Consistency of `environments.yaml`.
    *   Absence of `.env` references (where migrated).
    *   Links integrity.
2.  **Add CI Task**: Register the script as a `make` target or just a script to be run.
3.  **Initial Verification**: Run the script to establish a baseline.

### Phase 1: Local Dev Improvements (Priority 1)
1.  **Modify `bootstrap_cluster.go`**:
    *   Allow execution without `GITHUB_REPO` when environment is `local`.
    *   Use internal mirror URL as fallback for `repoURL`.
2.  **Update `dev.go`**:
    *   Ensure `yby dev` starts the mirror before bootstrapping the cluster if running locally.
3.  **Adjust Templates**:
    *   Update `root-app.yaml.tmpl` to support internal mirror URLs.

### Subsequent Phases (Overview)
*   **Phase 2**: Fix `.github` generation path (ensure it's always at repo root).
*   **Phase 3**: Validate and repair `environments.yaml` consistency.
*   **Phase 4**: Remove legacy `.env` dependencies from bootstrap commands.
*   **Phase 5**: Full support for `infra/` subdirectory in all commands (Monorepo).

I will start by implementing **Phase 0** and **Phase 1** immediately.