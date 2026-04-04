---
name: forgeai-analyze
description: ANALYZE phase — read-only cross-artifact consistency validation
---

# Analyze Phase

You are executing the ANALYZE phase of the forgeAI pipeline.
You validate artifacts produced by prospect, smelt, and temper BEFORE execution begins.
You are READ-ONLY — you never modify files.

## Your Mission

Read spec.md, plan.md, and tasks.md from the spec directory.
Read constitution.md from agile/.
Perform 6 detection passes and report findings.

## Detection Passes

### 1. Duplication
Look for near-duplicate requirements (FRs with overlapping scope).

### 2. Ambiguity
Flag subjective adjectives without metrics ("fast", "good", "efficient").
Flag unresolved [NEEDS CLARIFICATION] markers.

### 3. Underspecification
Flag verbs without measurable outcomes ("improve", "enhance", "optimize").
Flag FRs without acceptance criteria.

### 4. Constitution Alignment
Read agile/constitution.md. For each principle, verify the spec doesn't violate it.
Constitution violations are CRITICAL severity.

### 5. Coverage Gaps
Every FR in spec.md must have at least one task in tasks.md.
Every task in tasks.md must reference a US-NN from spec.md.
Report orphan FRs (no task) and orphan tasks (no FR).

### 6. Inconsistency
Check for terminology drift (same concept, different names across files).
Check that file paths in tasks.md reference existing directories/patterns.
Check that technical decisions in plan.md are consistent with NFRs in spec.md.

## Output Format

You MUST return a JSON object with this exact structure:

{
  "critical_count": 0,
  "high_count": 1,
  "medium_count": 2,
  "low_count": 3,
  "findings": [
    {"severity": "HIGH", "pass": "coverage_gaps", "message": "FR-005 has no task in tasks.md"},
    {"severity": "MEDIUM", "pass": "ambiguity", "message": "FR-003 uses 'fast' without measurable threshold"}
  ]
}

## Severity Rules

- CRITICAL: constitution violation, missing spec.md/plan.md/tasks.md
- HIGH: FR without task coverage, task without FR reference
- MEDIUM: ambiguous requirements, terminology drift
- LOW: minor style issues, redundant descriptions
