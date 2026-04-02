---
name: howler-init
description: "Analyze the current repository using howler-agents to generate GEA knowledge events. Builds a structured understanding of the codebase -- architecture, patterns, dependencies, test coverage, conventions -- and stores it in the hive-mind so future howler-agents runs operate with full repo context."
---

# Howler Init -- Repository Intelligence Seeding

## What This Skill Does

Performs a deep analysis of the current repository and seeds the hive-mind collective memory with structured knowledge about:

1. **Repository structure** -- directory layout, packages, key entry points
2. **Architecture patterns** -- design patterns, abstractions, module boundaries
3. **Dependencies** -- external libraries, internal packages, version constraints
4. **Code conventions** -- naming, file organization, import style, error handling
5. **Test coverage** -- test frameworks, test locations, coverage patterns
6. **Build system** -- build tools, scripts, CI configuration
7. **Known issues** -- TODO comments, deprecation warnings, tech debt

This intelligence allows `/howler-agents` and `/howler-agents-wiggam` to produce better solutions immediately because the evolved agents understand the repo context.

## Prerequisites

- howler-agents MCP server registered (run `/howler-setup` first)
- Must be run from within a git repository

## Invocation

The user can optionally specify:
- **focus**: A specific area to analyze deeply (e.g., "auth", "database", "frontend")
- **depth**: quick (structure only), standard (structure + patterns), deep (full analysis) (default: standard)

Example: `/howler-init` or `/howler-init --focus auth --depth deep`

## Execution Protocol

### Phase 1: Repository Discovery

Gather basic repository metadata:

1. Read `package.json`, `pyproject.toml`, `Cargo.toml`, `go.mod`, or equivalent to identify the language ecosystem and dependencies.
2. Run `git log --oneline -20` to understand recent development activity.
3. List the top-level directory structure.
4. Identify the monorepo structure if applicable (workspaces, packages).
5. Read `README.md`, `AGENTS.md`, or equivalent project documentation.

Store findings as:
```
Namespace: "repo-intel"
Key: "repo-structure-<repo-name>"
```

### Phase 2: Architecture Analysis

Analyze code architecture and patterns:

1. Identify entry points (main files, CLI commands, API routes, exports).
2. Map module boundaries and their dependencies (imports graph).
3. Identify design patterns in use:
   - Dependency injection, factory patterns, strategy patterns
   - Middleware chains, plugin systems, event-driven architecture
   - ORM models, data access layers, repository patterns
4. Identify public API surfaces (exported functions, classes, types).
5. Look for architectural documentation (ADRs, architecture.md, diagrams).

If a **focus** area is specified, go deeper on that area:
- Trace call chains through the focused module
- Map all external integrations
- Document configuration options and environment variables

Store findings as:
```
Namespace: "repo-intel"
Key: "architecture-<repo-name>"
```

### Phase 3: Convention Detection

Detect coding conventions by sampling files:

1. **Naming conventions**: camelCase vs snake_case, file naming, class naming.
2. **Import organization**: absolute vs relative, grouping, barrel exports.
3. **Error handling**: exception patterns, Result types, error codes.
4. **Logging**: structured logging, log levels, logger names.
5. **Type safety**: strict mode, type annotations, generics usage.
6. **Documentation**: docstrings, JSDoc, comments style.

Store findings as:
```
Namespace: "repo-intel"
Key: "conventions-<repo-name>"
```

### Phase 4: Test & Build Intelligence

Analyze the testing and build infrastructure:

1. Identify test frameworks (pytest, vitest, jest, go test, cargo test).
2. Map test file locations and naming patterns.
3. Count tests and identify test categories (unit, integration, e2e).
4. Read CI configuration (.github/workflows, .gitlab-ci.yml).
5. Identify build scripts and their purpose.
6. Note any test fixtures, factories, or shared test utilities.

Store findings as:
```
Namespace: "repo-intel"
Key: "testing-<repo-name>"
```

### Phase 5: Quick GEA Evolution (standard/deep only)

Run a rapid GEA evolution focused on repo understanding:

1. Call `howler_evolve` with:
   - `domain`: "coding"
   - `population_size`: 3
   - `group_size`: 1
   - `iterations`: 2
   - `task_description`: "Analyze this repository and identify the most important patterns and conventions for working effectively in it: <repo summary from phases 1-4>"

2. Wait for completion via `howler_status`.
3. Call `howler_list_agents` to get the top agent's analysis.
4. Store the evolved understanding as additional intelligence.

### Phase 6: Generate Summary Report

Compile all findings into a structured report:

```
--- Howler Init: Repository Intelligence Report ---
Repository: <name>
Language:   <primary language(s)>
Structure:  <monorepo/single-package>

Architecture:
  Patterns:     <list of identified patterns>
  Entry points: <list of main entry points>
  Modules:      <count> modules across <count> packages

Conventions:
  Naming:       <camelCase/snake_case/etc>
  Imports:      <absolute/relative>
  Error style:  <exceptions/results/error-codes>
  Type safety:  <strict/loose>

Testing:
  Framework:    <pytest/vitest/etc>
  Tests:        <count> tests in <count> files
  Coverage:     <estimated from test file analysis>

Build:
  Tools:        <make/npm/uv/etc>
  CI:           <GitHub Actions/GitLab CI/etc>

Hive-mind entries created: <count>
```

### Phase 7: Store Master Intelligence Entry

Create a master entry that links all the detailed entries:

```
Namespace: "repo-intel"
Key: "master-<repo-name>"
Value: JSON with keys for structure, architecture, conventions, testing, build
       and cross-references to detailed entries
```

## Important Rules

- ALWAYS read files before making conclusions -- never guess based on file names alone
- For large repos, sample representative files rather than reading everything
- Focus on patterns that would help an AI agent write better code in this repo
- Do NOT modify any files -- this skill is read-only analysis
- If the repo already has a master intelligence entry, UPDATE it rather than creating duplicates
- Report findings to the user as you go, not just at the end
