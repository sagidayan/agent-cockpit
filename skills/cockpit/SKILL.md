---
name: cockpit
description: Drive other coding agents (claude, codex, opencode) running in tmux via the agent-cockpit CLI (`cockpit`). Use to list running agents and their status, spawn a new agent in a project/dir, start a PR review agent or a quick chat, send a prompt or task to another agent, wait for it, read its output, interrupt/kill it, label or favorite it, restore last session's agents, or manage cockpit projects. Triggers on "cockpit", "spawn an agent", "start an agent in <repo>", "tell the <x> agent to", "what are my agents doing", "review this PR in a new agent", "fan out to agents".
---

# Cockpit

`cockpit` drives coding agents running in tmux. `TARGET` = pane id (`%12`), label, or window name.

If the `cockpit` MCP server is connected, prefer its tools (`list_agents`, `list_projects`, `spawn_agent`, `send_task`, `wait_for_agent`, `get_output`, `get_diff`) - agents spawned that way become children of this session. Otherwise use the CLI below.

## Commands

```sh
cockpit list --json                # pane, kind, status, session, window, label, mode, parent, dir, project, message, stats
cockpit project list               # named dirs; add NAME [DIR] | rm NAME

cockpit new [--mode code|review|master|chat] [--agent claude|codex|opencode] \
  [--project NAME | --dir DIR] [--url PR_URL] [--name WINDOW] [--label L] [--prompt TASK]
                                   # prints the new pane id
cockpit send TARGET "text"         # or: ... | cockpit send TARGET -
cockpit stop TARGET                # interrupt (Esc)
cockpit kill TARGET                # close pane (+ its editor pane; deletes a chat's temp folder)
cockpit label TARGET [TEXT]        # no TEXT clears
cockpit fav [TARGET | resume N|LABEL|NAME | rm N]
cockpit restore                    # resume agents from last snapshot
```

Modes:
- `code` (default): works in `--project`/`--dir`; the prompt is its first task.
- `review`: reviews `--url` in `/tmp/prs/<repo>-<number>`.
- `master`: coordinates child agents via the MCP tools.
- `chat`: a plain Q&A session in a disposable temp folder.

Statuses in JSON: `working`, `background`, `needs_input`, `done`, `idle`.

## CLI equivalents for MCP-only tools

- **Wait**: poll `cockpit list --json | jq -r '.[] | select(.pane=="%12") | .status'` until it is not `working`/`background` (use a monitor/until-loop, not a fixed sleep).
- **Output**: the `message` field from `list --json`, or `tmux capture-pane -p -t %12 -S -200` for the raw screen.
- **Diff**: `git -C <dir> diff` using the agent's `dir`; add `$(git -C <dir> merge-base HEAD origin/HEAD)` to include its commits.

## Rules

- Find this session's own pane first (`$TMUX_PANE`) and never send/stop/kill it.
- `list`, `project list` and `fav` (no args) are read-only - run freely.
- `new`, `send`, `stop`, `kill`, `label`, `fav TARGET`, `restore` and `project add/rm` affect other sessions - confirm with the user first unless they explicitly asked for that action. Always confirm before `kill`.
- Resolve the target from `list --json` before acting; if a label or name is ambiguous, use the pane id.
