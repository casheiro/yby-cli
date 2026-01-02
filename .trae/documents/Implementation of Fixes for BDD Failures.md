I will implement the fixes to address the BDD test failures:

1.  **Implement `--offline` Flag**:
    *   Add `Offline bool` to `InitOptions` struct in `cmd/init.go`.
    *   Register the flag `--offline` in `init()` function.
    *   In `buildContext`, logic to skip git repository prompts or validations if `Offline` is true.

2.  **Fix Monorepo Detection**:
    *   Modify `cmd/infra_helpers.go` (specifically `FindInfraRoot`) to properly traverse directories upwards looking for `.yby/environments.yaml` or `infra/.yby/environments.yaml` from the root.
    *   Ensure `yby dev` uses this logic correctly.

3.  **Adjust Test Scenarios**:
    *   Update `test/e2e/steps_test.go` to remove the expectation of `--no-env` flag if it was just a placeholder, or implement it if intended. Since the plan said "No .env", and we deprecated it, simply not passing it (and not having the file) should be enough. I will adjust the test to NOT pass `--no-env` but verify behavior when `.env` is absent.

I will start by modifying `cmd/init.go` to add the `--offline` flag.