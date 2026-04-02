---
name: howler-agents
description: "Combine hive-mind collective intelligence with GEA evolution to produce the best possible solution on any task in a single pass. Pulls lessons from collective memory, spawns evolved agents, and synthesizes their outputs into one optimal result."
---

# Howler Agents -- Hive-Mind + GEA First-Pass Optimization

## What This Skill Does

Produces the best possible solution for a given task by combining two systems:

1. **Hive-Mind Memory** -- Retrieves collective lessons, patterns, and past solutions from persistent memory to inform the approach
2. **GEA Evolution** -- Runs a rapid evolution cycle where multiple agents compete and collaborate, with the best strategies surfacing through performance + novelty selection

The result is a single, high-quality output that benefits from the entire collective intelligence of past runs plus real-time evolutionary optimization.

## Prerequisites

- howler-agents MCP server registered (run `/howler-setup` first)
- `.howler-agents/` directory exists
- MCP tools accessible: `howler_memory`, `howler_hivemind`, `howler_evolve`, `howler_status`, `howler_list_agents`, `howler_get_experience`

## Invocation

The user provides:
- **task**: The task description (required)
- **domain**: coding, math, writing, or general (default: coding)
- **model**: LLM model string (default: Codex-sonnet-4-20250514)
- **depth**: quick (3 agents, 2 gens), standard (6 agents, 3 gens), deep (10 agents, 5 gens) (default: standard)

Example: `/howler-agents Fix the authentication token refresh logic in auth.ts`

## Execution Protocol

### Phase 1: Gather Collective Intelligence

1. Call `howler_memory` MCP tool with `action: "search"` and a query derived from the task description. Retrieve relevant lessons, patterns, and past solutions.
2. Call `howler_hivemind` MCP tool with `action: "consensus"` and the task description to get any consensus knowledge about similar problems.
3. Compile the retrieved intelligence into a **context brief** -- a structured summary of:
   - Relevant past lessons (what worked, what failed)
   - Known patterns for this type of task
   - Recommended strategies from collective memory

If no relevant memory exists, note this and proceed with a fresh approach.

### Phase 2: Configure and Launch Evolution

Based on the depth parameter, set population and iteration counts:

| Depth | Population | Groups | Iterations | Alpha |
|-------|-----------|--------|------------|-------|
| quick | 3 | 1 | 2 | 0.7 |
| standard | 6 | 2 | 3 | 0.6 |
| deep | 10 | 3 | 5 | 0.5 |

1. Call `howler_configure` to set the model if non-default.
2. Call `howler_evolve` with:
   - `domain`: from user parameter
   - `population_size`: from depth table
   - `group_size`: from depth table
   - `iterations`: from depth table
   - `alpha`: from depth table
   - `task_description`: the user's task, prefixed with the context brief from Phase 1

Capture the `run_id`.

### Phase 3: Monitor Evolution

Poll `howler_status` with the `run_id` until status is `"completed"` or `"failed"`. Report progress to the user as generations advance:

```
Howler Agents: Generation 1/3 complete (best score: 0.72)
```

If the run fails, report the error and attempt to solve the task directly using the hive-mind context brief alone.

### Phase 4: Extract Best Solution

Once evolution completes:

1. Call `howler_list_agents` with `run_id` and `top_k=3` to get the top-performing agents.
2. Call `howler_get_experience` with `run_id` to retrieve the collective experience traces.
3. Analyze the top agents' strategies and patches:
   - What approach did the best agent take?
   - What did the group learn during evolution?
   - What patterns emerged across the top performers?

### Phase 5: Synthesize and Execute

Using the combined intelligence from:
- Hive-mind memory (Phase 1)
- Evolution results (Phase 4)
- Top agent strategies and experience traces

Produce the final solution:

1. **For coding tasks**: Apply the best agent's approach, enhanced with collective lessons. Write the actual code changes.
2. **For writing tasks**: Compose the output using the highest-scoring agent's style and structure.
3. **For math tasks**: Present the solution path that scored highest, validated against known patterns.
4. **For general tasks**: Synthesize the top strategies into a single coherent output.

Present the solution to the user with a brief explanation of what informed the approach.

### Phase 6: Update Collective Memory

After delivering the solution:

1. Call `howler_memory` with `action: "store"` to persist:
   - The task description
   - The approach that worked (or didn't)
   - Key lessons from this evolution run
   - Namespace: `"lessons"`

2. Call `howler_memory` with `action: "store"` to persist any new patterns discovered:
   - Namespace: `"patterns"`

This ensures future invocations of `/howler-agents` benefit from this run's experience.

## Output Format

```
--- Howler Agents: Task Complete ---
Intelligence: <N> lessons retrieved from hive-mind
Evolution:    <M> agents, <G> generations, best score: <S>
Strategy:     <one-line summary of winning approach>

<actual solution output -- code changes, text, etc.>

Lessons stored: <count> new entries added to collective memory
```

## Important Rules

- ALWAYS check hive-mind memory first -- past runs may have already solved a similar problem
- If evolution produces a score > 0.9, trust the top agent's approach directly
- If evolution produces a score < 0.5, combine top 3 agents' approaches rather than using just one
- NEVER skip the memory storage phase -- collective intelligence improves with every run
- For coding tasks, ALWAYS validate the solution by running tests if available
