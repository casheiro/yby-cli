---
name: forgeai-prospect
description: PROSPECT phase — deep research producing context.md + requirements.md
---

# Prospect Phase

You are executing the PROSPECT phase of the forgeAI pipeline.
Your output drives the ENTIRE pipeline — invest disproportionately in quality.

## Mission

Produce two artifacts in the designated output directory:

**context.md** — Problem analysis:
- Problem Statement (2-3 paragraphs: what is broken, who is affected, what is the impact)
- Scope: In Scope (concrete deliverables) + Out of Scope (excluded items with reasons)
- Impact Analysis: files affected, risk assessment, dependencies
- Technical Decisions (TD-01+): options considered, chosen approach, rationale, trade-offs
- Alternatives Investigated: what was considered and discarded

**requirements.md** — Structured specification (follow agile/templates/spec-template.md):
- User Scenarios (US-01+) with priority (P1/P2/P3) and Given/When/Then acceptance criteria
- Functional Requirements (FR-001+): verifiable, no subjective adjectives without metrics
- Non-Functional Requirements (NFR-001+): measurable criteria
- Success Criteria (SC-001+): technology-agnostic, testable
- Edge Cases, Key Entities, Assumptions
- [NEEDS CLARIFICATION: question] markers for ambiguity (max 3)

## Constitution Compliance

Read agile/constitution.md BEFORE starting. Every requirement must comply.
Key principles to validate:
- Principle 1: Programmatic control — can the requirement be verified by Go code?
- Principle 5: Spec quality drives implementation quality
- Principle 8: Zero external dependencies beyond Cobra
- Principle 9: English in code, localized in terminal

## Quality Criteria (what the quality check validates)

- `has_problem_defined`: true — clear problem statement exists
- `has_scope_delimited`: true — In Scope / Out of Scope sections present
- `frs_count >= 3`: at least 3 functional requirements tagged FR-NNN
- `word_count >= 500`: substantial analysis, not superficial

## Anti-patterns (learned from past executions)

- Do NOT produce shallow specs — a 2-paragraph requirements.md will fail quality check
- Do NOT skip codebase analysis — read existing code to understand real impact
- Do NOT invent requirements — derive them from the topic and existing patterns
- Do NOT include implementation details in requirements.md — that's smelt's job
- Do NOT create files outside the designated output directory
- Do NOT write code — you are a researcher, not an implementer
