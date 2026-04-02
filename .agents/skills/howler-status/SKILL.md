---
name: howler-status
description: "Check the status of howler-agents evolution runs. Shows generation progress, population statistics, best scores, and agent rankings. Works with both active and completed runs."
---

# Howler Status

## What This Skill Does

Retrieves and displays the current state of one or more howler-agents evolution runs. Shows generation progress, population health, best performers, and any errors.

## Prerequisites

- howler-agents MCP server running (run `/howler-setup` first)
- At least one evolution run started (via `/howler-evolve` or direct MCP call)

## Execution

### If the user provides a run_id:

1. Call the `howler_status` MCP tool with the provided `run_id`.
2. Call the `howler_list_agents` MCP tool with the same `run_id` and `top_k=5`.
3. Present the combined results.

### If the user does NOT provide a run_id:

1. Read the `.howler-agents/` directory for any cached run IDs.
2. If a `run_id` is found in a recent memory file under `.howler-agents/memory/`, use it.
3. If no run_id can be determined, inform the user: "No active runs found. Start one with /howler-evolve."

### Both calls can be made in parallel since they are independent.

## Output Format

```
--- Howler Evolution Status ---
Run ID       : <run_id>
Status       : <pending | running | completed | failed>
Domain       : <task_domain>
Generation   : <current> / <total>
Population   : <size> agents

Scores:
  Best score   : <best_score>
  Mean score   : <mean_score>

Timeline:
  Started      : <started_at>
  Finished     : <finished_at or "still running">

Top 5 Agents:
  #1 <agent_id_short>  score=<combined>  perf=<perf>  novelty=<nov>  gen=<gen>
  #2 <agent_id_short>  score=<combined>  perf=<perf>  novelty=<nov>  gen=<gen>
  #3 <agent_id_short>  score=<combined>  perf=<perf>  novelty=<nov>  gen=<gen>
  #4 <agent_id_short>  score=<combined>  perf=<perf>  novelty=<nov>  gen=<gen>
  #5 <agent_id_short>  score=<combined>  perf=<perf>  novelty=<nov>  gen=<gen>
```

If the run has failed, display the error message prominently.

If the run is still in progress (status = "running"), note the current generation and estimated remaining time if inferable.

## MCP Tools Used

| Tool | Purpose |
|------|---------|
| `howler_status` | Get run progress and scores |
| `howler_list_agents` | Get ranked agent list |

## Error Handling

- If `howler_status` returns an error for an unknown run_id, display: "Run not found: <run_id>. It may have been from a previous MCP server session."
- If the MCP server is unreachable, suggest running `/howler-setup` to verify the configuration.
