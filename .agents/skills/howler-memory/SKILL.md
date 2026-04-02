---
name: howler-memory
description: "Interact with the howler-agents collective memory. Browse, search, and retrieve lessons learned from past evolution runs. The hive-mind memory stores experience traces, evolutionary insights, and agent performance patterns across all runs."
---

# Howler Memory

## What This Skill Does

Provides access to the howler-agents hive-mind collective memory. This includes:
- Experience traces from past evolution runs
- Lessons learned and evolutionary insights
- Agent performance patterns and capability progressions
- Cross-run knowledge that informs future evolution

## Prerequisites

- howler-agents MCP server running
- At least one completed evolution run (for meaningful memory content)

## Operations

### 1. Browse Memory (default action)

List all stored memory entries from `.howler-agents/memory/`:

```bash
ls -la .howler-agents/memory/
```

For each memory file found, read and summarize:
- Run ID
- Domain
- Completion date
- Best score achieved
- Number of lessons learned

Present as a table:

```
--- Howler Hive-Mind Memory ---
Entries: <count>

| Run ID (short) | Domain  | Date       | Best Score | Lessons |
|----------------|---------|------------|------------|---------|
| abc123...      | coding  | 2026-02-20 | 0.8542     | 5       |
| def456...      | general | 2026-02-21 | 0.7231     | 3       |
```

### 2. Retrieve Experience for a Run

If the user provides a `run_id`:

1. Call the `howler_get_experience` MCP tool with the `run_id`.
2. Also read `.howler-agents/memory/<run_id>.json` if it exists for the persistent summary.
3. Present the experience context in a readable format.

### 3. Search Lessons

If the user asks to search for specific lessons or patterns:

1. Read all files in `.howler-agents/memory/`.
2. Search through `lessons_learned` arrays across all runs.
3. Present matching lessons with their source run context.

### 4. Submit External Experience

If the user wants to add experience to an active run:

1. Collect the required fields:
   - `run_id` -- which run to submit to
   - `agent_id` -- which agent performed the task
   - `task_description` -- what was done
   - `outcome` -- success, failure, or custom label
   - `score` -- 0.0 to 1.0
   - `key_decisions` -- (optional) list of decisions made
   - `lessons_learned` -- (optional) list of lessons

2. Call `howler_submit_experience` with these parameters.
3. Confirm the submission with the returned `trace_id`.

### 5. Aggregate Insights

If the user asks for a summary or "what has the hive learned":

1. Read all memory files from `.howler-agents/memory/`.
2. Aggregate all `lessons_learned` entries.
3. Group by domain.
4. Identify recurring patterns and themes.
5. Present a synthesized narrative.

## MCP Tools Used

| Tool | Purpose |
|------|---------|
| `howler_get_experience` | Retrieve experience context for a run |
| `howler_submit_experience` | Add new experience to an active run |

## Data Location

- Persistent memory: `.howler-agents/memory/<run_id>.json`
- Active run experience: Accessed via MCP tools (in-memory during run)
- Evolution database: `.howler-agents/evolution.db` (when persistence layer is active)

## Memory File Schema

Each file in `.howler-agents/memory/` follows this structure:

```json
{
  "run_id": "uuid",
  "domain": "coding",
  "completed_at": "2026-02-22T10:30:00Z",
  "best_score": 0.85,
  "population_size": 10,
  "generations_completed": 5,
  "top_agents": [
    {
      "agent_id": "uuid",
      "score": 0.85,
      "generation": 4,
      "capability_vector": [1, 0, 1, 1, 0]
    }
  ],
  "lessons_learned": [
    "Agents that integrated error-handling tools early performed better in later generations",
    "Group diversity (high novelty scores) correlated with faster convergence"
  ],
  "experience_summary": "Free-form narrative from the experience pool aggregation"
}
```
