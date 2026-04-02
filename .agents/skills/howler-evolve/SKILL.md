---
name: howler-evolve
description: "Start a full GEA (Group-Evolving Agents) evolution run with hive-mind coordination. Spawns a team of specialized sub-agents -- coordinator, actors, evaluator, reproducer -- that collaborate through Codex's team system and howler-agents MCP tools to run the evolutionary loop."
---

# Howler Evolve -- GEA Hive-Mind Evolution

## What This Skill Does

Orchestrates a complete Group-Evolving Agents (GEA) evolution run using Codex's native team system. This is the primary entry point for howler-agents. It:

1. Initializes local persistence (SQLite via MCP)
2. Loads prior evolution state and hive-mind memory
3. Spawns a team of specialized agents
4. Runs the evolution loop across multiple generations
5. Stores results persistently
6. Updates collective memory with lessons learned
7. Reports results to the user

## Prerequisites

- howler-agents MCP server registered (run `/howler-setup` first)
- `.howler-agents/` directory exists
- MCP tools `howler_evolve`, `howler_status`, `howler_list_agents` are accessible

## Invocation

The user provides optional parameters:
- **domain**: coding, math, writing, or general (default: general)
- **population**: number of agents (default: 10)
- **iterations**: number of generations (default: 5)
- **model**: LLM model string (default: Codex-sonnet-4-20250514)
- **alpha**: performance vs novelty balance, 0.0-1.0 (default: 0.5)

If the user does not specify parameters, use the defaults.

## Execution Protocol

### Phase 1: Initialize

1. Read `.howler-agents/config.json` if it exists to load any saved configuration.
2. Call the `howler_configure` MCP tool to set model assignments if the config specifies non-default models.
3. Call the `howler_evolve` MCP tool with the user's parameters (or defaults). Capture the returned `run_id`.
4. Store the `run_id` for reference by the team.

### Phase 2: Create the Hive-Mind Team

Use Codex's `TeamCreate` tool to create a team named `howler-evolution`.

Then create tasks for the team using `TaskCreate`:

**Task 1: Monitor Evolution Progress**
- Assigned to the coordinator
- Periodically call `howler_status` with the `run_id`
- Report generation progress as it advances

**Task 2: Evaluate Agent Performance**
- Assigned to the evaluator
- Once the evolution run completes (status = "completed"), call `howler_list_agents` with `top_k=5`
- Analyze the top agents' capability vectors, scores, and lineage

**Task 3: Extract Collective Lessons**
- Assigned to the reproducer
- Call `howler_get_experience` with the `run_id`
- Synthesize the experience context into actionable lessons
- Write a summary of what the population learned

Spawn teammates using the Task tool with `run_in_background: true`:

**Teammate: howler-coordinator**
- Agent type: `.Codex/agents/howler/coordinator.md`
- Role: Monitor the evolution run, coordinate between evaluator and reproducer
- Receives the `run_id` and polls `howler_status` until completion
- When completed, notifies the evaluator and reproducer to begin their work

**Teammate: howler-evaluator**
- Agent type: `.Codex/agents/howler/evaluator.md`
- Role: Score and analyze the final population
- Waits for coordinator signal, then calls `howler_list_agents`
- Produces a ranked analysis of agent performance

**Teammate: howler-reproducer**
- Agent type: `.Codex/agents/howler/reproducer.md`
- Role: Extract and persist evolutionary lessons
- Calls `howler_get_experience` and synthesizes lessons
- Writes results to `.howler-agents/memory/`

### Phase 3: Wait for Results

After spawning all teammates in ONE message, STOP. Do not poll or check status. Wait for teammate messages to arrive automatically.

### Phase 4: Synthesize and Report

When all teammate results arrive, compile the final report:

```
--- Howler Evolution Report ---
Run ID      : <run_id>
Domain      : <domain>
Generations : <completed> / <total>
Status      : <status>

Population Performance:
  Best score  : <best_score>
  Mean score  : <mean_score>

Top Agents:
  #1 <agent_id> -- score=<score>, gen=<generation>, group=<group_id>
  #2 <agent_id> -- score=<score>, gen=<generation>, group=<group_id>
  #3 <agent_id> -- score=<score>, gen=<generation>, group=<group_id>

Collective Lessons:
  - <lesson_1>
  - <lesson_2>
  - <lesson_3>

Evolutionary Insights:
  <synthesized narrative from experience context>
```

### Phase 5: Persist to Hive-Mind Memory

Write the final report and lessons to `.howler-agents/memory/<run_id>.json`:

```json
{
  "run_id": "<run_id>",
  "domain": "<domain>",
  "completed_at": "<ISO timestamp>",
  "best_score": 0.0,
  "top_agents": [],
  "lessons_learned": [],
  "experience_summary": ""
}
```

Then shut down all teammates gracefully via `SendMessage` with `type: "shutdown_request"`.

## Important Rules

- ALL teammate spawns MUST happen in ONE message
- After spawning, STOP and wait for results
- Never poll `howler_status` from the leader -- the coordinator teammate handles monitoring
- The MCP tools (`howler_evolve`, `howler_status`, etc.) do the actual computation; teammates orchestrate and analyze
- If the evolution run fails, report the error from `howler_status` and do not spawn the evaluator/reproducer

## Fallback: No Team Mode

If Codex's team system is unavailable, fall back to sequential execution:

1. Call `howler_evolve` and capture `run_id`
2. Poll `howler_status` every 10 seconds until status is "completed" or "failed"
3. Call `howler_list_agents` with `top_k=5`
4. Call `howler_get_experience`
5. Synthesize and report results directly
