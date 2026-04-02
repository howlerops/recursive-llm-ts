---
name: howler-auto-evolve
description: "Run an auto-evolution loop that deploys evolved agents via Codex-flow orchestration, executes real tasks, and feeds outcomes back into the GEA evolutionary cycle. Agents improve automatically based on real-world performance."
---

# Howler Auto-Evolve -- Continuous Evolution via Orchestration

## What This Skill Does

Runs a closed-loop evolution cycle where:
1. GEA evolves a population of agents (howler-agents)
2. Top-performing agents are deployed as real workers (via Codex-flow or local LLM)
3. Workers execute actual tasks in the user's project
4. Task outcomes feed back as experience traces
5. The experience pool informs the next generation of mutations
6. Repeat -- agents get smarter each cycle

This is the "self-improving agent swarm" skill.

## Prerequisites

- howler-agents MCP server running (`/howler-setup`)
- For full orchestration: Codex-flow installed (`npm install -g Codex-flow@alpha`)
- Without Codex-flow: falls back to local LLM execution (still works, just less rich)

## Parameters (User-Provided, All Optional)

- **domain**: coding, math, writing, or general (default: coding)
- **population**: number of agents (default: 6)
- **generations**: number of evolution cycles (default: 3)
- **tasks**: specific tasks for agents to execute (default: domain-appropriate tasks)
- **model**: LLM model string (default: Codex-sonnet-4-20250514)
- **orchestrator**: auto, local, or Codex-flow (default: auto -- uses Codex-flow if available)

## Execution Protocol

### Phase 1: Detect Orchestrator

1. Check if Codex-flow is available: run `which Codex-flow` or check `howler_orchestrator_status` MCP tool
2. If available, log: "Using Codex-flow orchestration (preferred)"
3. If not available, log: "Using local LLM orchestration (fallback)"
4. Store the detection result for subsequent phases

### Phase 2: Initialize Evolution

1. Call `howler_configure` MCP tool to set model assignments if user specified non-defaults
2. Call `howler_evolve` MCP tool with parameters:
   - task_domain = domain
   - population_size = population
   - num_iterations = generations
   - model = model
3. Capture the returned `run_id`
4. Log: "Evolution run started: {run_id}"

### Phase 3: Monitor and Collect Outcomes

Wait for the evolution run to complete by polling `howler_status` with the run_id.

While running, check status every 15 seconds. Log generation progress:
```
Generation 1/3: best=0.72, mean=0.58, population=6
Generation 2/3: best=0.81, mean=0.65, population=6
Generation 3/3: best=0.89, mean=0.71, population=6
```

### Phase 4: Deploy Top Agents

Once the evolution completes:

1. Call `howler_list_agents` with `run_id` and `top_k=3` to get the best agents
2. For each top agent, extract its evolved configuration
3. If using Codex-flow orchestration:
   - Call `mcp__claude-flow__agent_spawn` for each agent with its evolved prompt
   - Call `mcp__claude-flow__task_orchestrate` with user-specified tasks
   - Collect outcomes from each spawned agent
4. If using local orchestration:
   - Execute tasks directly via LLM calls using the evolved agent prompts
   - Record outcomes locally

### Phase 5: Feed Back Outcomes

For each task outcome:
1. Call `howler_submit_experience` MCP tool with:
   - run_id = the current run
   - agent_id = the agent that executed it
   - task_description = what the task was
   - outcome = "success" or "failure"
   - score = numeric score
   - key_decisions = decisions made during execution
   - lessons_learned = what was learned
2. This enriches the experience pool for future evolution runs

### Phase 6: Seed Hive-Mind

1. Call `howler_hivemind` with action="seed" and run_id to extract lessons
2. Call `howler_memory` with action="store" to persist key insights:
   - Namespace: "auto-evolve"
   - Key: "run-{run_id}-summary"
   - Value: JSON summary of the run results
   - Tags: ["auto-evolve", domain, "generation-{n}"]

### Phase 7: Report Results

Display a comprehensive report:

```
--- Auto-Evolution Report ---
Run ID        : {run_id}
Orchestrator  : {Codex-flow | local}
Domain        : {domain}
Generations   : {n}
Population    : {size}

Evolution Results:
  Best score   : {best_score}
  Mean score   : {mean_score}
  Improvement  : {first_gen_best} -> {last_gen_best} ({pct}%)

Top Evolved Agents:
  #1 {agent_id_short}  score={score}  gen={gen}
  #2 {agent_id_short}  score={score}  gen={gen}
  #3 {agent_id_short}  score={score}  gen={gen}

Task Execution (Post-Evolution):
  Tasks run    : {count}
  Success rate : {pct}%
  Avg score    : {avg}

Hive-Mind:
  Lessons stored  : {n}
  Memory entries  : {n}
  Consensus items : {n}

Next Steps:
  - Run again to continue evolving: /howler-auto-evolve
  - View collective memory: /howler-memory
  - Check full history: howler_history action="runs"
```

## Important Rules

- Auto-detect Codex-flow by default; do not require explicit user config
- If Codex-flow is available, USE IT -- it is the preferred backend
- If any MCP tool call fails, log the error and continue with remaining tasks
- Always feed outcomes back via howler_submit_experience, even on failure
- Always seed the hive-mind after a completed run
- Keep status updates concise -- one line per generation

## Error Handling

- If `howler_evolve` fails: report the error and suggest running `/howler-setup`
- If Codex-flow agents fail to spawn: fall back to local execution for that agent
- If hive-mind seeding fails: log warning but do not fail the overall skill
- If the evolution run times out (>10min): report partial results from whatever completed
