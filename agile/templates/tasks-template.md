# Tasks: [FEATURE NAME]

## Task Format

Each task follows this format:
```
- [ ] T001 [P?] [US-NN] Description — file/path.go
```
- `T001`: Sequential task ID
- `[P]`: Can be executed in parallel with other [P] tasks in same phase
- `[US-NN]`: Parent user story
- Description: What to implement (one atomic action)
- File path: Primary file affected

## Phases

Tasks MUST be executed in phase order. Within a phase, [P] tasks can be parallelized.

### Phase 0: Setup
[One-time setup: create directories, config files, scaffolding]

- [ ] T001 [US-01] Description — path/file.go

### Phase 1: Foundation (Blocking)
[Core infrastructure that other tasks depend on. No [P] marker — sequential.]

- [ ] T002 [US-01] Description — path/file.go
- [ ] T003 [US-01] Description — path/file.go

### Phase 2: User Stories
[One section per story. Tasks within a story are sequential unless marked [P].]

#### US-01: [Story Name]
- [ ] T004 [US-01] Description — path/file.go
- [ ] T005 [P] [US-01] Description — path/file.go

#### US-02: [Story Name]
- [ ] T006 [US-02] Description — path/file.go

### Phase 3: Polish
[Integration tests, documentation updates, cleanup]

- [ ] T007 [P] Verification task — path/file_test.go

## Dependencies

```
T001 → T002 → T004
              → T005 (parallel with T004)
T003 → T006
```

## Completion Criteria

All tasks marked [X], all tests passing, all acceptance criteria from spec verified.
