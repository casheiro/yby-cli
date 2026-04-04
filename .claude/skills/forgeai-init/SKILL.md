---
name: forgeai-init
description: INIT phase — generates project-specific CLAUDE.md, skills, agents, and forgeai.yaml
---

# Init Phase

You are generating project-specific configuration for forgeAI.
You receive project context (language, framework, dependencies, modules, description)
and produce tailored artifacts.

## What You Generate

### 1. claude_md (CLAUDE.md)
Project-specific development guidelines. Include:
- Project description and purpose
- Architecture overview (modules, packages, entry points)
- Key dependencies and their roles
- Code conventions (naming, error handling, testing patterns)
- How to build, test, and lint
- What NOT to do (project-specific anti-patterns)

Keep it concise — developers read this before every session.

### 2. skills (2-4 project-specific skills)
Each skill MUST use this exact frontmatter format:

```
---
name: <kebab-case-name>
description: <one-line description of what this skill provides>
---

# <Title>

<content with rules, patterns, anti-patterns>
```

**Always generate:**
- `project-conventions`: coding patterns, naming rules, architecture decisions specific to this project

**Generate based on domain:**
- API/web project → `api-patterns`: endpoint conventions, validation, error responses, auth
- Infrastructure/DevOps/GitOps → `domain-infra`: IaC patterns, deployment, monitoring, security
- CLI project → `cli-patterns`: command structure, flag conventions, UX rules, output formatting
- Data/ML project → `domain-data`: pipeline patterns, model management, data validation
- Library/SDK → `sdk-patterns`: API design, backward compat, versioning, docs

### 3. agents (0-2 project-specific agents)
Each agent MUST use this exact frontmatter format:

```
---
name: <kebab-case-name>
model: sonnet
description: <one-line description>
tools:
  - Read
  - Grep
  - Glob
---

# <Title>

<agent instructions>
```

**Generate based on need:**
- Complex domain (infra, fintech, health) → `project-reviewer`: domain-aware code review
- Infrastructure project → `infra-checker`: validates IaC, security, compliance
- API project → `api-validator`: checks endpoint consistency, schema validation

### 4. test_command and linter_command
Confirm or improve the auto-detected commands.

## Quality Rules

- Skills must be ACTIONABLE — specific rules, not generic advice
- Skills must reference PROJECT-SPECIFIC patterns, not language-generic ones
- Agents must have clear scope — what they check, what they don't
- CLAUDE.md must be useful on day 1 — a new dev reads it and understands the project
- No boilerplate — if a section has nothing project-specific to say, omit it
