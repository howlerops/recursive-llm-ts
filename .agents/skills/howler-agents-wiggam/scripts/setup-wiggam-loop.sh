#!/bin/bash

# Howler Agents Wiggam Loop Setup Script
# Creates state file for ralph-wiggum loop enhanced with howler-agents hive-mind + GEA

set -euo pipefail

# Parse arguments
PROMPT_PARTS=()
MAX_ITERATIONS=10
COMPLETION_PROMISE="null"
DOMAIN="coding"
DEPTH="quick"
MODEL="claude-sonnet-4-20250514"

while [[ $# -gt 0 ]]; do
  case $1 in
    -h|--help)
      cat << 'HELP_EOF'
Howler Agents Wiggam - Ralph Loop + Hive-Mind + GEA Evolution

USAGE:
  /howler-agents-wiggam [PROMPT...] [OPTIONS]

ARGUMENTS:
  PROMPT...    Task description (can be multiple words without quotes)

OPTIONS:
  --completion-promise '<text>'  Promise phrase to signal completion (REQUIRED)
  --max-iterations <n>           Max iterations (default: 10)
  --domain <type>                coding|math|writing|general (default: coding)
  --depth <level>                quick|standard|deep (default: quick)
  --model <name>                 LLM model (default: claude-sonnet-4-20250514)
  -h, --help                     Show this help

DESCRIPTION:
  Combines Ralph Wiggum iterative loops with howler-agents hive-mind + GEA.
  Each iteration runs a full howler-agents pass with collective intelligence.
  The loop continues until the completion promise is genuinely TRUE.

EXAMPLES:
  /howler-agents-wiggam Fix all tests --completion-promise 'ALL TESTS PASSING'
  /howler-agents-wiggam Refactor auth module --completion-promise 'REFACTORING COMPLETE' --depth standard
  /howler-agents-wiggam Add caching layer --completion-promise 'CACHE WORKING' --max-iterations 15
HELP_EOF
      exit 0
      ;;
    --max-iterations)
      [[ -z "${2:-}" ]] && { echo "Error: --max-iterations requires a number" >&2; exit 1; }
      [[ ! "$2" =~ ^[0-9]+$ ]] && { echo "Error: --max-iterations must be a positive integer, got: $2" >&2; exit 1; }
      MAX_ITERATIONS="$2"
      shift 2
      ;;
    --completion-promise)
      [[ -z "${2:-}" ]] && { echo "Error: --completion-promise requires a text argument" >&2; exit 1; }
      COMPLETION_PROMISE="$2"
      shift 2
      ;;
    --domain)
      [[ -z "${2:-}" ]] && { echo "Error: --domain requires a value" >&2; exit 1; }
      DOMAIN="$2"
      shift 2
      ;;
    --depth)
      [[ -z "${2:-}" ]] && { echo "Error: --depth requires a value" >&2; exit 1; }
      DEPTH="$2"
      shift 2
      ;;
    --model)
      [[ -z "${2:-}" ]] && { echo "Error: --model requires a value" >&2; exit 1; }
      MODEL="$2"
      shift 2
      ;;
    *)
      PROMPT_PARTS+=("$1")
      shift
      ;;
  esac
done

PROMPT="${PROMPT_PARTS[*]}"

if [[ -z "$PROMPT" ]]; then
  echo "Error: No task description provided" >&2
  echo "" >&2
  echo "  Usage: /howler-agents-wiggam <TASK> --completion-promise '<PROMISE>'" >&2
  echo "" >&2
  echo "  Example: /howler-agents-wiggam Fix failing tests --completion-promise 'ALL TESTS PASSING'" >&2
  exit 1
fi

if [[ "$COMPLETION_PROMISE" == "null" ]]; then
  echo "Error: --completion-promise is required for wiggam loops" >&2
  echo "" >&2
  echo "  The completion promise defines when the loop stops." >&2
  echo "  Example: --completion-promise 'ALL TESTS PASSING'" >&2
  exit 1
fi

# Create state file
mkdir -p .claude

if [[ -n "$COMPLETION_PROMISE" ]] && [[ "$COMPLETION_PROMISE" != "null" ]]; then
  COMPLETION_PROMISE_YAML="\"$COMPLETION_PROMISE\""
else
  COMPLETION_PROMISE_YAML="null"
fi

cat > .claude/ralph-loop.local.md <<EOF
---
active: true
iteration: 1
max_iterations: $MAX_ITERATIONS
completion_promise: $COMPLETION_PROMISE_YAML
started_at: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
howler_mode: true
howler_domain: "$DOMAIN"
howler_depth: "$DEPTH"
howler_model: "$MODEL"
---

$PROMPT
EOF

cat <<EOF
Howler Agents Wiggam loop activated!

Task:               $PROMPT
Domain:             $DOMAIN
Depth:              $DEPTH (per-iteration evolution)
Model:              $MODEL
Max iterations:     $MAX_ITERATIONS
Completion promise: ${COMPLETION_PROMISE//\"/} (ONLY output when TRUE)

Each iteration will:
  1. Query hive-mind memory for relevant lessons
  2. Run GEA evolution (agents compete to find best approach)
  3. Apply the winning strategy
  4. Store lessons for the next iteration

The ralph-wiggum stop hook feeds the same prompt back on each exit.
Previous iterations' work is visible in files and git history.

To monitor: head -15 .claude/ralph-loop.local.md

WARNING: Loop runs until completion promise is TRUE or max iterations reached.
EOF

echo ""
echo "$PROMPT"
