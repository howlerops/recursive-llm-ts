---
name: howler-agents-wiggam
description: "Combine the Ralph Wiggum iterative loop with howler-agents hive-mind + GEA evolution. Each iteration runs a full howler-agents pass, sees previous work in files, and uses collective intelligence to iteratively improve until a completion promise is met."
---

# Howler Agents Wiggam -- Ralph Loop + Hive-Mind + GEA Evolution

## What This Skill Does

Combines two techniques for maximum solution quality:

1. **Howler Agents** (hive-mind + GEA) -- Each iteration gets collective intelligence and evolutionary optimization
2. **Ralph Wiggum Loop** -- Iterates on the same task, with each pass seeing previous work and building on it

The result: each iteration doesn't just retry blindly. It pulls collective lessons from all previous iterations via hive-mind memory, runs a rapid evolution to find the best improvement strategy, then applies it. The loop continues until the completion promise is genuinely true.

## Prerequisites

- howler-agents MCP server registered (run `/howler-setup` first)
- `.howler-agents/` directory exists
- MCP tools accessible (same as `/howler-agents`)

## Invocation

The user provides:
- **task**: The task description (required)
- **completion-promise**: A statement that must be TRUE to exit (required)
- **max-iterations**: Maximum loop iterations (default: 10)
- **domain**: coding, math, writing, or general (default: coding)
- **model**: LLM model string (default: Codex-sonnet-4-20250514)
- **depth**: quick, standard, deep (default: quick -- faster per-iteration for loops)

Example: `/howler-agents-wiggam Fix all failing tests in the auth module --completion-promise "ALL TESTS PASSING" --max-iterations 15`

## Execution Protocol

### Phase 1: Initialize the Loop State

Create the ralph-wiggum loop state file at `.Codex/ralph-loop.local.md`:

```bash
mkdir -p .Codex
```

Write the state file with YAML frontmatter:

```yaml
---
active: true
iteration: 1
max_iterations: <max-iterations>
completion_promise: "<completion-promise>"
started_at: "<ISO timestamp>"
howler_mode: true
howler_domain: "<domain>"
howler_depth: "<depth>"
howler_model: "<model>"
---

<task description>
```

The `howler_mode: true` flag signals that this is a howler-enhanced ralph loop.

### Phase 2: Execute First Howler-Agents Pass

For each iteration (starting with iteration 1):

#### 2a. Gather Iteration Context

1. Read the current state of the codebase (files modified, test results, git diff).
2. Call `howler_memory` with `action: "search"` using the task description + iteration number. Retrieve:
   - Lessons from previous iterations of THIS task
   - General lessons from similar tasks
   - Patterns that apply to this type of work

3. If iteration > 1, also call `howler_get_experience` from the previous iteration's `run_id` to get the evolutionary traces.

#### 2b. Run Rapid Evolution

Call the howler-agents protocol (Phase 2-4 from `/howler-agents`) with:
- **depth**: Use the configured depth (default: `quick` for loop speed)
- **task**: The original task, augmented with:
  - Current iteration number and max iterations
  - Summary of what previous iterations accomplished
  - What still needs to be done (delta from completion promise)
  - Relevant hive-mind lessons

#### 2c. Apply Best Solution

Take the evolution winner's approach and apply it:
- For coding: write the code changes
- For writing: produce the content
- Run any available tests or validation

#### 2d. Store Iteration Lessons

Call `howler_memory` with `action: "store"`:
- What was attempted this iteration
- What worked and what didn't
- Current progress toward the completion promise
- Namespace: `"iterations"`
- Key: `"wiggam-<task-hash>-iter-<N>"`

### Phase 3: Check Completion

After applying the solution, evaluate the completion promise:

1. Assess whether the promise statement is now genuinely TRUE.
2. If TRUE: output `<promise>COMPLETION_PROMISE_TEXT</promise>` to signal the ralph-wiggum stop hook.
3. If NOT TRUE: explain what remains to be done and let the ralph-wiggum stop hook feed the prompt back for the next iteration.

**CRITICAL**: NEVER output the promise tag unless the statement is genuinely and completely TRUE. The loop is designed to continue until real completion. Do not lie to exit the loop.

### Phase 4: Subsequent Iterations

When the stop hook feeds the prompt back (iteration 2+):

1. Read `.Codex/ralph-loop.local.md` to get the current iteration number.
2. Review what was accomplished in previous iterations by:
   - Reading modified files and git history
   - Querying hive-mind memory for iteration-specific lessons
3. Run Phase 2 again with the accumulated context.

Each iteration benefits from:
- All previous iterations' file changes (visible in workspace)
- All previous iterations' lessons (stored in hive-mind)
- Fresh evolution with the accumulated knowledge
- Progressively narrowing focus on what remains

### Phase 5: Final Report

When the completion promise is met and the loop ends:

```
--- Howler Agents Wiggam: Complete ---
Task:         <task description>
Iterations:   <completed> / <max>
Promise:      <completion-promise> -- TRUE

Evolution Summary:
  Total evolution runs: <count>
  Best agent score:     <score>
  Lessons accumulated:  <count>

Iteration History:
  #1: <what was done, score>
  #2: <what was done, score>
  ...

Final Solution:
  <summary of the complete solution>

Hive-mind updated with <N> new lessons from this session.
```

## Important Rules

- Use `quick` depth by default in loops -- each iteration should be fast (3 agents, 2 gens)
- NEVER lie about the completion promise to exit the loop early
- ALWAYS store iteration lessons in hive-mind -- this is what makes each iteration smarter
- If an iteration makes no progress, change strategy: try a different approach, adjust depth to `standard`, or decompose the problem
- If stuck for 3+ iterations with no progress, store a "stuck" lesson and try a fundamentally different approach
- The ralph-wiggum stop hook handles loop mechanics -- focus on solving the task and storing lessons
- After the loop completes, store a comprehensive summary lesson covering the full session
