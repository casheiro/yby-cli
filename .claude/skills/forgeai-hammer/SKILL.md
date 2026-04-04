---
name: forgeai-hammer
description: HAMMER phase — executes one task from tasks.md with TDD
---

# Hammer Phase

You are executing the HAMMER phase of the forgeAI pipeline.
You implement ONE task at a time. Quality over speed.

## Mission

Read the task description provided in the prompt. Implement EXACTLY what it describes.
Follow TDD when possible: write test first, then implementation.

When done, mark the task as [x] in tasks.md by changing `- [ ] TNNN` to `- [x] TNNN`.

## Scope Rule

IMPLEMENT ONLY what this specific task describes.
- If the task says "modify fsutil/paths.go", modify ONLY fsutil/paths.go
- If the task says "categories 1-5 only", do NOT implement categories 6-18
- If the task says "no tests in this task", do NOT add tests
- Over-delivery will be REJECTED by self-review

Read the task description carefully for "Scope" and "Out of Scope" sections.

## Before Marking Done

1. All new items (i18n keys, config fields, enum values) covered by tests
2. No slice/map access without length/nil check (panic risk)
3. `go vet` and `go test` pass for ALL affected packages
4. No hardcoded user-facing strings — use i18n.T()/Tf()
5. Error wrapping with context: `fmt.Errorf("context: %w", err)`

## Code Quality Rules

These are checked by self-review. Violations = rejection:
- Never access slice/array elements without checking length first
- Never assume a string is non-empty before splitting or indexing
- When adding items to a collection, update ALL existing tests that enumerate it
- When modifying a function signature, verify all callers are updated
- Run `go vet` and `go test` for affected packages before marking Done

## Self-Review Expectations

After your implementation, a separate critic (self-review) will:
1. Read the task description
2. Read every file you modified
3. Evaluate: correctness, error handling, test coverage, scope adherence
4. Output APPROVE or REJECT with specific reasons

Common rejection reasons:
- Scope violation (implemented beyond task boundaries)
- Missing `defer` cleanup in tests that modify global state
- Nil/empty checks missing on map/slice access
- Tests that test format instead of behavior

## Anti-patterns

- Do NOT implement work from other tasks
- Do NOT refactor code unrelated to the task
- Do NOT skip tests "because they'll be added later"
- Do NOT use testify, gomock, or any external test framework
- Do NOT add comments to code you didn't change
