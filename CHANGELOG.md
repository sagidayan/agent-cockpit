# Changelog

## v0.0.4

- The dashboard runs in the tmux session you start it from again, with agents in the `cockpit` session, and `m` and `gt` work as before.
- Cockpit only touches panes it manages: the `cockpit` session, agents it started, the dashboard and its viewer, agents handed over with `m` and agents that spawn children through it. `m` now moves only the agent's pane when it shares a window with other panes.
- The README notes that cockpit was built with AI coding agents.

## v0.0.3

- Cockpit only ever changes its own `cockpit` tmux session. Agents in your other sessions are listed read-only, every mutating tmux command is checked against its target's session, and `kill-server` / `kill-session` are refused. `cockpit` run from another session opens its dashboard in the `cockpit` session. `m` / `:move` are gone and `gt` switches within the `cockpit` session.
- Every agent can delegate to child agents, so the master mode is gone. The guidance on splitting work and closing children now comes with the cockpit MCP server.
- New bug fix mode: give it a ticket key or URL and a project, it reads the ticket, fixes the bug and adds a regression test. In the wizard, `cockpit new --mode bug --ticket` and `spawn_agent` with `mode: "bug"`.
- `d` and `get_diff` show every repo the agent touched: its folder, repos cloned inside a temp folder, repos of files it edited elsewhere and its children's repos, also after they were closed. Edited files outside git are listed.
- The dashboard flags claude and opencode agents whose cockpit MCP server is gone (**⚠ MCP down**) and tells you how to reconnect.
- The agent list keeps a fixed order: new agents join at the bottom, and `J`/`K` (or `pgdn`/`pgup`) move the selected one down or up. The order survives restarts.
- Vim-style keys: `gg` / `G` jump to the first / last agent, and go-to is now `gt` instead of `g`.
<<<<<<< HEAD
- `d` and `get_diff` list edited files that git ignores. `mcp.log` rotates past 1 MiB, and claude installed through npm is checked for MCP health too.
=======
- `:closed` lists the last 20 closed agents with their last reply, diff summary and cost, and resumes one with `enter`.
- `:group` shows a header per project above its agents, keeping your order inside each group.
>>>>>>> 0510747 (Add a recently closed list and grouping by project)
- `:restart` and `cockpit restart` relaunch an agent in its pane, resuming its session, so an agent whose MCP server died gets a fresh one.
- Closing an agent removes its worktree and branch when they hold no changes; `:clean` removes leftover clean cockpit worktrees.
- A parent's card shows the cost of its whole tree, closed children included (`tree_cost_usd` in `list_agents` and `cockpit list --json`). An optional `[budget] tree_usd` flags and notifies when a tree passes it.
- A claude parent gets a `[cockpit]` line in its pane when a child finishes or needs input, unless it is already waiting on that child. Off with `[spawn] notify_parent = false`.
- New `review_agent` MCP tool: spawns a read-only reviewer of an agent's changes, for a review-then-fix loop.
- The MCP server no longer dies when a tool panics, and logs its start, exit and panics to `~/.local/state/agent-cockpit/mcp.log`.

## v0.0.2

- Answer an agent's permission prompt from the dashboard: `p`, then `y` approve once, `a` approve always or `n` deny (Claude Code and OpenCode).
- Detail panel (`i`): the full last prompt and reply plus the recent conversation, updated live.
- Run a coding agent in its own git worktree: a step in the new-session wizard, `cockpit new --worktree`, and `worktree: true` on `spawn_agent`.
- Masters close finished children with the new `close_agent` MCP tool.
- Notifications are no longer lost when Claude ends a turn from its background daemon.
- `get_diff` shows committed, uncommitted and untracked work since the branch left the default branch, like `d`.
- CI runs lint and tests on Linux and macOS for every push and pull request.

## v0.0.1

First release: the dashboard, the new-session wizard (coding, PR review, master and chat), master agents through the MCP server, the dedicated tmux session, go-to, move, favorites, restore, diff review and the CLI.
