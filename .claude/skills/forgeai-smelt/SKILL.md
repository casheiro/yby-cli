---
name: forgeai-smelt
description: SMELT phase — produces spec.md + plan.md from prospect artifacts
---

# Smelt Phase

You are executing the SMELT phase of the forgeAI pipeline.
You STRUCTURE the prospect's research into actionable documents. You do NOT simplify it.

## Mission

Read context.md + requirements.md from the artifacts directory.
Create two files in the planning directory:

**spec.md** — Refined specification (WHAT):
- Macro objective (clear, measurable, 1-2 sentences)
- ALL user scenarios from requirements.md (US-01+) with Given/When/Then criteria
- ALL functional requirements (FR-001+) with acceptance criteria preserved
- ALL non-functional requirements (NFR-001+) preserved
- Success criteria (SC-001+)
- Edge cases
- FR traceability table

**plan.md** — Technical plan (HOW):
- Technical context: language, dependencies, testing, platform, constraints
- Constitution check: verify each principle from agile/constitution.md
- Architecture: components affected with exact file paths, data flow, integration points
- Technical decisions (TD-01+): options, chosen, rationale, trade-offs
- FR traceability: which component/file implements which FR
- Complexity estimates per user scenario (Low/Medium/High with risk areas)

## CRITICAL RULE: Maintain Fidelity

The prospect phase (opus) produced detailed, thorough research. Your job is to
ORGANIZE and STRUCTURE that research — NOT to simplify, reduce, or reinterpret it.

- If requirements.md has 14 FRs, spec.md MUST have 14 FRs
- If requirements.md has 9 user scenarios, spec.md MUST have 9 user scenarios
- If context.md describes 6 data sources to consume, plan.md MUST address all 6
- Do NOT drop requirements because they seem complex or hard to implement
- Do NOT merge multiple FRs into one to simplify
- Do NOT rephrase user scenarios in a way that loses specificity

The quality check compares spec.md against requirements.md. Missing FRs = failure.

## Output Rules

- Create ONLY spec.md and plan.md — no EPIC.md, no us-NN/ directories, no story.md
- Both files in the planning directory provided in the prompt
- Follow agile/templates/plan-template.md for plan.md structure
- Constitution compliance is mandatory — flag tensions, don't ignore them

## Anti-patterns

- Do NOT simplify the spec to make it easier to decompose into tasks
- Do NOT create subdirectories — flat model only (spec.md + plan.md)
- Do NOT skip the constitution check in plan.md
- Do NOT use vague architecture descriptions — list exact file paths
- Do NOT write code — you are a planner, not an implementer
