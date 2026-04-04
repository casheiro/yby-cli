---
name: forgeai-fast
description: FAST MODE — direct task generation from topic, skipping research and planning phases
---

# Fast Mode

You are in FAST MODE of the forgeAI pipeline. No prospect, no smelt, no separate temper.
You go from user topic directly to executable tasks in ONE invocation.

## Your Mission

1. Read the topic provided
2. Read the codebase to understand current structure, patterns, and conventions
3. Create tasks.md with the MINIMUM tasks needed

## How to Work

You are combining what normally takes 3 phases (prospect + smelt + temper) into one.
But you are NOT doing all three — you are doing a focused, efficient version:

- Do NOT write context.md, requirements.md, spec.md, or plan.md
- Do NOT over-research — read only the files directly relevant to the topic
- Do NOT create more than 5 tasks — if you think you need more, the topic is too complex for fast mode
- DO read the constitution at agile/constitution.md
- DO read existing patterns in the codebase before deciding what to build

## Task Format

Create tasks.md in the spec directory with:

```
### Phase 1: Implementation
- [ ] T001 [US-01] Description including what files to modify and what to verify -- primary/file.go
- [ ] T002 [US-01] Description -- primary/file.go

### Phase 2: Tests
- [ ] T003 [US-01] Description -- primary/file_test.go
```

## Rules of Thumb

- 1-line change (register command, add config field) = merge into parent task
- i18n keys (en.go + pt_br.go) = part of the task that creates the feature
- Tests = same task as implementation unless test file would be >100 lines
- A simple CLI command = 2 tasks (implementation + tests)
- A config change = 1 task
- A new package = 3 tasks max (package + integration + tests)

## Anti-patterns

- Do NOT create separate tasks for "add to en.go" and "add to pt_br.go"
- Do NOT create a task for "register in root.go" (1 line — merge it)
- Do NOT create 9 tasks for a simple feature
- Do NOT write spec.md or plan.md
- Do NOT spend 100+ tool calls researching — 20-30 is enough for fast mode
