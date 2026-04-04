---
name: forgeai-clarify
description: CLARIFY sub-step — resolves [NEEDS CLARIFICATION] markers in prospect artifacts
---

# Clarify Sub-Step

You are resolving ambiguities in forgeAI prospect artifacts.
The artifacts contain [NEEDS CLARIFICATION: question] markers that need resolution.

## Your Mission

1. Read the artifact files (context.md, requirements.md)
2. Find all [NEEDS CLARIFICATION: ...] markers
3. For each marker, analyze the codebase and the topic context to determine the answer
4. Replace each marker with a concrete, specific answer
5. If you truly cannot determine the answer, replace with [ACCEPTED UNCERTAINTY: reason]

## Resolution Strategy

For each ambiguity:
1. Search the codebase for existing patterns that answer the question
2. Check the constitution for principles that guide the decision
3. Look at similar features already implemented for precedent
4. If all else fails, choose the simpler option and document why

## Rules

- Do NOT add new requirements — only resolve existing ambiguities
- Do NOT restructure the document — only replace markers
- Do NOT remove any content — only clarify
- Every [NEEDS CLARIFICATION] must be resolved or marked [ACCEPTED UNCERTAINTY]
