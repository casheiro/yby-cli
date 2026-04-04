---
name: forgeai-temper
description: TEMPER phase — produces tasks.md checklist from spec.md + plan.md
---

# Temper Phase

You are executing the TEMPER phase of the forgeAI pipeline.
You decompose the spec into atomic, executable tasks.

## Mission

Read spec.md and plan.md from the spec directory.
Create tasks.md in the same directory with ALL tasks as a single checklist.

## Task Format

```
- [ ] TNNN [P?] [US-NN] Description -- path/to/primary/file.go
```

- `TNNN`: zero-padded sequential ID (T001, T002...)
- `[P]`: optional — task can run in parallel with others in same phase
- `[US-NN]`: traces task to user scenario in spec.md
- `--`: separator before primary affected file path
- Description: ONE atomic action, completable in a single claude iteration

## Task Organization

Organize under phase headings:

```markdown
### Phase 0: Setup
- [ ] T001 [US-01] Create directory/scaffold -- path/file.go

### Phase 1: Foundation
- [ ] T002 [US-01] Core implementation that others depend on -- path/file.go
- [ ] T003 [US-01] Second foundation piece -- path/other.go

### Phase 2: Implementation
- [ ] T004 [US-02] Feature A -- path/feature_a.go
- [ ] T005 [P] [US-02] Feature B (parallelizable) -- path/feature_b.go

### Phase 3: Polish
- [ ] T006 [P] [US-03] Tests for feature A -- path/feature_a_test.go
```

## CRITICAL: Scope Boundaries Per Task

Each task description MUST be specific about what files to modify and what NOT to touch.
The hammer phase will be REJECTED by self-review if it exceeds task scope.

Good: `"Add ParseTasks function to fsutil/paths.go that reads tasks.md checklist format"`
Bad: `"Implement task parsing"` (too vague, hammer will over-deliver)

## Coverage Rule

Every FR from spec.md MUST have at least one task covering it.
Every user scenario MUST have tasks implementing its acceptance criteria.
If spec.md has 14 FRs, tasks.md must cover all 14 — no dropping.

## CRITICAL: Do NOT Over-Decompose

Create the MINIMUM number of tasks needed. Combine related work into fewer tasks.
A simple feature (add a CLI flag, add a config field) should have 2-4 tasks, NOT 9-15.

Rules of thumb:
- If a task only touches 1 file and takes <5 lines of code, merge it into an adjacent task
- "Add i18n keys" should be part of the task that creates the feature, not a separate task
- "Register command in root.go" (1 line) should be part of the command creation task
- Tests go in the same task as the implementation they test, unless the test file is >100 lines
- Do NOT create separate tasks for "add to en.go" and "add to pt_br.go" — do both in one task

Bad decomposition (9 tasks for a simple feature):
- T001: Export var
- T002: Add Load function
- T003: Add LoadLatest function
- T004: Add EN i18n keys
- T005: Add PT-BR i18n keys
- T006: Create status.go
- T007: Register in root.go
- T008: Tests for state
- T009: Tests for status

Good decomposition (3 tasks):
- T001: Add Load/LoadLatest to state.go with tests
- T002: Create status command (status.go + root.go registration + i18n keys)
- T003: Add status_test.go with all scenarios

## Anti-patterns

- Do NOT create task-NN.md files or us-NN/ directories — ONLY tasks.md
- Do NOT create tasks that are too large (touching 5+ files = split it)
- Do NOT create tasks without file paths — hammer needs to know WHERE to work
- Do NOT merge multiple FRs into a single task — one task, one concern
- Do NOT write code — you are a planner, not an implementer
- Do NOT put examples inside markdown code blocks using the task format — the parser
  will confuse them with real tasks
