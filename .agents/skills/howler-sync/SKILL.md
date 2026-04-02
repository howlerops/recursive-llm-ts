---
name: howler-sync
description: "Synchronize howler-agents evolution data between local SQLite storage and a remote team database (Postgres). Push local runs to shared storage, pull team results, and manage the dual-mode persistence layer."
---

# Howler Sync

## What This Skill Does

Manages the dual-mode persistence layer of howler-agents. The system operates locally with SQLite (`.howler-agents/evolution.db`) and can optionally synchronize with a remote Postgres database for team collaboration. This skill handles push, pull, and status operations.

## Prerequisites

- howler-agents MCP server running
- `.howler-agents/` directory exists
- For remote sync: `HOWLER_API_URL` environment variable set, or configured in `.howler-agents/config.json`

## Operations

### 1. Sync Status (default action)

Check the current synchronization state:

1. Read `.howler-agents/config.json` to determine if a remote URL is configured.
2. Check if `.howler-agents/evolution.db` exists (local persistence active).
3. Report the current mode.

```
--- Howler Sync Status ---
Local DB     : .howler-agents/evolution.db (<size> KB)
Remote URL   : <url or "not configured">
Mode         : <local-only | dual-mode>
Last sync    : <timestamp or "never">
Pending push : <count> runs
```

### 2. Push to Remote

Push local evolution data to the remote database:

1. Verify `HOWLER_API_URL` is set or configured in `.howler-agents/config.json`.
2. Read all memory files from `.howler-agents/memory/`.
3. For each run that has not been synced:
   - Read the local memory file
   - POST the run data to the remote API
   - Mark as synced in a local sync manifest

```
--- Howler Push ---
Pushing <count> runs to <remote_url>...
  Run abc123... (coding, score=0.85) -- pushed
  Run def456... (general, score=0.72) -- pushed
Push complete. <count> runs synced.
```

### 3. Pull from Remote

Pull team evolution data from the remote database:

1. Verify `HOWLER_API_URL` is set.
2. GET the list of runs from the remote API.
3. For each run not present locally:
   - Download the run data
   - Write to `.howler-agents/memory/<run_id>.json`

```
--- Howler Pull ---
Pulling from <remote_url>...
  Run xyz789... (coding, score=0.91) -- new, saved locally
  Run abc123... (coding, score=0.85) -- already local, skipped
Pull complete. <count> new runs imported.
```

### 4. Configure Remote

Set up the remote sync target:

1. Ask the user for the remote URL if not provided.
2. Update `.howler-agents/config.json` with the `sync.remote_url` field.
3. Optionally set `sync.auto_sync` to true for automatic sync after each run.

### 5. Full Sync (bidirectional)

Run both push and pull in sequence:

1. Push all unsynced local runs
2. Pull all missing remote runs
3. Report the delta

## Sync Manifest

The sync state is tracked in `.howler-agents/sync_manifest.json`:

```json
{
  "remote_url": "https://api.howler-agents.example.com",
  "last_sync": "2026-02-22T10:30:00Z",
  "synced_runs": ["abc123", "def456"],
  "pending_push": ["ghi789"]
}
```

## Dual-Mode Architecture

```
Local (always available)          Remote (optional, for teams)
========================          ============================
.howler-agents/evolution.db  -->  Postgres + pgvector
.howler-agents/memory/*.json -->  Remote experience store
                                  (shared across team members)
```

- Local mode is always active. Every run writes to local storage.
- Remote mode is opt-in. When configured, sync operations replicate data.
- The MCP server can proxy all calls to the remote API when `HOWLER_API_URL` is set.
- Conflict resolution: remote wins for score/status updates; local wins for experience traces (append-only).

## MCP Integration

When `HOWLER_API_URL` is set in the environment, the howler-agents MCP server automatically proxies all tool calls to the remote API. This means:
- `howler_evolve` creates runs on the remote server
- `howler_status` reads from the remote server
- Experience traces are stored remotely

To switch between modes:
- **Local only**: Unset `HOWLER_API_URL`
- **Remote proxy**: Set `HOWLER_API_URL=https://your-server.com`
- **Dual mode**: Use this `/howler-sync` skill to manually replicate between local and remote

## Error Handling

- If the remote server is unreachable, report the error and continue in local-only mode.
- If a push fails for a specific run, skip it and continue with the remaining runs.
- Network timeouts default to 30 seconds; configurable in `.howler-agents/config.json`.
