---
name: howler-setup
description: "Initialize howler-agents: install the package, register the MCP server, create the .howler-agents/ directory, and verify the setup. Run this first before using any other howler skills."
---

# Howler Setup

## What This Skill Does

Performs zero-config initialization of the howler-agents system. Handles package installation, MCP server registration, local data directory creation, and verification that the MCP tools are reachable.

## Setup Steps

Execute these steps in order:

### 1. Install howler-agents

**Option A — npx (zero-config, recommended):**

No install needed. The npm wrapper auto-installs the Python package on first use:

```bash
npx howler-agents serve --transport stdio
```

**Option B — uv/pip (direct Python install):**

```bash
pip install 'howler-agents-core[mcp]'
# or
uv pip install 'howler-agents-core[mcp]'
```

**Option C — from local monorepo (development):**

```bash
pip install -e /Users/jacob/projects/howler-agents/packages/howler-agents-core
```

### 2. Register the MCP Server

Use the built-in install command to register with your AI coding tool:

```bash
# Auto-detect the host and register (npx mode — recommended):
howler-agents install --command npx

# Or specify the host explicitly:
howler-agents install --host Codex --command npx

# Or with direct Python install:
howler-agents install --host Codex
```

Alternatively, manually add via `Codex mcp add`:

```bash
# npx mode (no Python install needed):
Codex mcp add howler-agents -- npx howler-agents@latest serve --transport stdio

# Direct Python mode:
Codex mcp add howler-agents -- howler-agents serve --transport stdio
```

This registers the entry in `.mcp.json`. If `.mcp.json` already contains a `howler-agents` entry, skip this step.

Verify the `.mcp.json` contains one of:

```json
{
  "mcpServers": {
    "howler-agents": {
      "command": "npx",
      "args": ["howler-agents@latest", "serve", "--transport", "stdio"]
    }
  }
}
```

Or for direct Python installs:

```json
{
  "mcpServers": {
    "howler-agents": {
      "command": "howler-agents",
      "args": ["serve", "--transport", "stdio"]
    }
  }
}
```

### 3. Create the Local Data Directory

```bash
mkdir -p .howler-agents
```

The `.howler-agents/` directory stores:
- `evolution.db` -- SQLite database for persistent evolution state
- `memory/` -- Hive-mind collective memory snapshots
- `config.json` -- Local configuration overrides

Initialize the config file if it does not exist:

```json
{
  "default_model": "Codex-sonnet-4-20250514",
  "acting_model": null,
  "evolving_model": null,
  "reflecting_model": null,
  "default_population": 10,
  "default_iterations": 5,
  "default_domain": "general",
  "sync": {
    "remote_url": null,
    "auto_sync": false
  }
}
```

Write this to `.howler-agents/config.json`.

### 4. Verify MCP Tools Are Available

Call the `howler_configure` MCP tool with no arguments to verify connectivity:

```
Use the howler_configure tool with {} (empty arguments).
```

If it returns a configuration object, setup is complete. If it errors, troubleshoot:
- Ensure `howler-agents` is installed: `which howler-agents`
- Ensure the MCP server entry exists in `.mcp.json`
- Try running manually: `howler-agents serve --transport stdio`

### 5. Report Setup Status

After all steps complete, report:
- Installation status (pip package found or not)
- MCP registration status (entry in .mcp.json)
- Data directory status (.howler-agents/ exists)
- MCP connectivity status (howler_configure responded)

## Available MCP Tools After Setup

| Tool | Purpose |
|------|---------|
| `howler_evolve` | Start an evolution run |
| `howler_auto_evolve` | One-shot evolution with auto-deployment |
| `howler_status` | Check run progress |
| `howler_list_agents` | List/rank agents in a run |
| `howler_submit_experience` | Submit task experience trace |
| `howler_get_experience` | Retrieve collective experience |
| `howler_configure` | Set model configuration |
| `howler_memory` | Access hive-mind collective memory |
| `howler_history` | Browse past evolution runs |
| `howler_hivemind` | Hive-mind consensus operations |
| `howler_deploy_agents` | Deploy top agents via orchestrator |
| `howler_orchestrator_status` | Check orchestrator backend |
| `howler_sync_push` | Push local data to remote |
| `howler_sync_pull` | Pull remote data to local |

## Available Skills After Setup

| Skill | Purpose |
|-------|---------|
| `/howler-agents` | Best-first-pass solution: hive-mind + GEA evolution combined |
| `/howler-agents-wiggam` | Iterative ralph-wiggum loop enhanced with hive-mind + GEA |
| `/howler-init` | Analyze repo and seed hive-mind with architecture/convention intelligence |
| `/howler-evolve` | Full GEA evolution run with team coordination |
| `/howler-auto-evolve` | One-shot evolution with auto-deployment |
| `/howler-status` | Check evolution run progress |
| `/howler-memory` | Browse and search collective memory |
| `/howler-sync` | Push/pull between local and remote storage |

## Troubleshooting

### MCP server not starting
- Check Python version: requires 3.11+
- Check dependencies: `pip install 'howler-agents-core[mcp]'`
- Check for port conflicts if using SSE transport
- If using npx: ensure Node.js 18+ is installed

### Tools not visible
- Restart Codex after modifying `.mcp.json`
- Verify the command path: `which howler-agents` or `npx howler-agents --help`

### Database errors
- Delete `.howler-agents/evolution.db` and re-run setup
- Check disk permissions on the project directory
