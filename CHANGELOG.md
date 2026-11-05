# Changelog

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
