# forgeAI Constitution
# Version: 1.0.0

Foundational principles that govern all forgeAI development. Every specification,
plan, and implementation MUST comply with these principles. Violations flagged
by the analyze phase are CRITICAL severity.

---

## Principle 1: Programmatic Control Over LLM Trust

forgeAI controls the LLM, not the reverse. Every LLM output is validated by
deterministic Go code (gates, formulas, regex). The LLM extracts facts; Go
decides pass/fail. Never trust LLM self-assessment — always verify programmatically.

## Principle 2: Iterate Until Satisfied

Never accept an LLM response that doesn't meet the metric. If the response is
wrong, retry with feedback explaining what failed and what's expected. The cost
of re-invoking is negligible compared to the cost of propagating a bad artifact
through the pipeline. Fail-open only after exhausting retries, and only when a
programmatic gate has already validated the minimum.

## Principle 3: Clean Context Per Invocation

Each `claude -p` call is a fresh session with no memory of previous calls.
Context is provided explicitly via prompts (system + user + learnings + feedback).
This ensures reproducibility and eliminates hidden state drift. The only exception
is `--resume` for retries within a single phase.

## Principle 4: Phases Have Strict Boundaries

Each phase has restricted tools (`--allowedTools`), a specific model, and a
defined output format. Prospect never writes code. Hammer never creates specs.
Hone never edits files. These boundaries prevent scope creep and ensure each
phase does one thing well.

## Principle 5: Specification Quality Drives Implementation Quality

Garbage in, garbage out. Invest disproportionately in prospect/smelt/temper
(the FOUNDRY cycle). A well-specified task with clear acceptance criteria,
test scenarios, and scope boundaries requires fewer hammer retries and produces
higher quality code. Ambiguity in specs becomes bugs in code.

## Principle 6: Observable Pipeline

Every invocation is audited (JSONL). Every quality check records extracted facts.
Every self-review records the verdict and reasoning. State is persisted after
every phase. The pipeline must be debuggable post-mortem from audit trail and
iteration logs alone, without re-running.

## Principle 7: Resilience Over Correctness of Execution Path

Rate limits, timeouts, empty responses, and transient errors are expected, not
exceptional. The pipeline must survive all of these without dying. Rate limits
trigger backoff and retry. Timeouts consume an attempt. Empty responses trigger
re-invocation with reinforced prompts. Only budget exhaustion and SIGINT are
legitimate reasons to stop.

## Principle 8: Zero External Dependencies Beyond Cobra

The entire codebase depends only on `github.com/spf13/cobra` and Go stdlib.
No test frameworks, no LLM libraries, no i18n frameworks. This constraint
ensures the binary is self-contained, fast to compile, and has no supply chain risk.

## Principle 9: English in Code, Localized in Terminal

All code, comments, variable names, function names, and documentation are in
English. User-facing terminal output goes through `i18n.T()`/`Tf()` and is
localized. System prompts sent to Claude are always in English for optimal
model performance. This separation is non-negotiable.

## Principle 10: Self-Hosting Development Model

forgeAI builds itself. New features are implemented via `forgeai craft`. The
pipeline that builds the tool is the same pipeline the tool provides. This
creates a tight feedback loop: bugs in the pipeline are felt immediately during
development, and fixes improve both the tool and its own development process.

---

## Governance

- **Amendment process**: Constitution changes require explicit user approval.
  forgeAI may propose amendments based on learnings but never auto-applies them.
- **Versioning**: Semantic versioning (MAJOR.MINOR.PATCH). Principle changes are MAJOR.
- **Propagation**: Constitution is injected into prospect and analyze prompts.
  All specifications are checked against these principles.
