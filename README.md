# agent-cockpit

TUI cockpit for running and steering Claude Code, Codex and OpenCode agents in tmux, with an MCP server so agents can run agents.

![agent-cockpit dashboard](docs/screenshot.png)

## Install

Download a binary for macOS or Linux (amd64 / arm64) from [Releases](https://github.com/sagidayan/agent-cockpit/releases), unpack it and put `cockpit` on your `PATH`:

```sh
tar -xzf cockpit_v0.0.1_darwin_arm64.tar.gz
mv cockpit_v0.0.1_darwin_arm64/cockpit ~/bin/
```

Or with Go:

```sh
go install github.com/sagidayan/agent-cockpit/cmd/cockpit@latest
# or from a clone:
make init   # install + cockpit init
```

When upgrading on macOS, replace the binary (`mv`) rather than `cp` over it, or the kernel kills it for an invalid code signature. `cockpit version` prints the installed version.

Other targets: `make build`, `make test`, `make lint`, `make clean`.

Runs on macOS and Linux (Windows via WSL). Requires tmux.

## Setup

```sh
cockpit init      # asks per agent; -y to accept all
```

For each installed agent it adds the status hooks (keeping your existing ones, with a `.bak` backup) and registers the `cockpit` MCP server. For claude it also routes `statusLine` through `cockpit statusline` to collect model, context, cost and lines changed - your own statusline command is saved in the config and keeps running as before. It also writes a default config. Safe to re-run. Run opencode with `--standalone` so cockpit can track it.

To do it by hand instead, `cockpit setup <claude|codex|opencode>` prints the snippets.

MCP tools: `list_agents`, `list_projects`, `spawn_agent`, `send_task`, `wait_for_agent`, `get_output`, `get_diff` (same diff as `:diff`, with a file and line summary on top), `close_agent`. Every agent can delegate: agents started with `spawn_agent` are children of the calling agent, and children can spawn their own, up to `[spawn] max_depth` levels. `spawn_agent` takes `mode: "bug"` with a `ticket`, and `worktree: true` to start the child in its own [worktree](#worktrees). An agent closes each child (and the child's own children) with `close_agent` once its work is done and reviewed; it can't close agents it didn't spawn.

## Skill

`skills/cockpit/SKILL.md` teaches an agent to drive cockpit from its shell: list agents, spawn, send, wait, review. It's useful in sessions where the MCP server isn't connected, and it carries the safety rules (never touch its own pane, confirm before acting on other agents). For Claude Code:

```sh
mkdir -p ~/.claude/skills && cp -r skills/cockpit ~/.claude/skills/
```

## Use

```sh
cockpit   # run inside tmux
```

The cockpit window splits into the agent list and a live viewer. Moving through the list swaps each agent's real pane into the viewer, and everything is swapped back on quit.

| Key | Action |
|---|---|
| `j`/`k`, `↑`/`↓` | select agent (shown live in the viewer) |
| `gg` / `G` (or `home`/`end`) | first / last agent |
| `J`/`K` (or `pgdn`/`pgup`) | move the agent down / up the list. New agents join at the bottom and the order is kept across restarts |
| `enter`, `→`, `l` | focus the viewer to interact; return with your tmux pane keys |
| `i` | details: the agent's full last prompt and reply plus the recent conversation (from the claude transcript, or the bottom of the pane for other agents). Updates live; `↑`/`↓`, `pgup`/`pgdn`, `home`/`end` or the mouse wheel scroll, `esc` closes |
| `gt` | go-to: show the agent's window in your current session (linked from its own session, unlinked when you return to the cockpit) |
| `d` | review the agent's changes in your editor (see `:diff`) |
| `m` | move the agent and its children into the dedicated `cockpit` tmux session (its whole window, or just the agent pane if it shares a window with the cockpit) |
| `s` | send a prompt without leaving the list |
| `p` | answer a permission prompt (shown when the agent needs input): see what it asks, then `y` approve once, `a` approve always (when offered) or `n` deny. Claude and OpenCode only |
| `r` | label the agent (cockpit only, tmux window name is untouched; empty clears) |
| `n` | start a new agent session (see below) |
| `1`-`9` | jump to an agent |
| `/` | filter the list live (name, repo, agent, status, prompt, reply); `esc` clears |
| `:` | run a command (tab completes, any unique prefix works) |
| `f` / `F` | toggle favorite (★) / favorites list: `enter` jumps to a running one or resumes it, `d` removes |
| `R` / `x` | restore / dismiss sessions from last time |
| `?` | help |
| `q` | quit and restore all panes |

Commands:

| Command | Action |
|---|---|
| `:new [agent] [project\|dir]` | start a session (no args opens the wizard) |
| `:send <text>` | send a prompt to the selected agent |
| `:broadcast <text>` | send a prompt to every listed agent (combine with `/` to target a subset) |
| `:stop` | interrupt the selected agent (Esc) |
| `:kill` | close the selected agent's pane (asks to confirm) |
| `:rename [name]` | label the agent in cockpit (same as `r`) |
| `:sort status\|recent\|name\|cost\|location` | change the list order (favorites always stay on top); `location` is your own order |
| `:project [add <name> [dir] \| rm <name>]` | list or manage projects (dir defaults to the selected agent's) |
| `:diff` | open the agent's changes in a popup with `[editor] command`, else `$VISUAL`, `$EDITOR` or `less`. Covers every repo it touched: its folder (including a worktree it moved into), repos cloned inside a temp folder (any folder under `/tmp` or `$TMPDIR`, like chat and review folders), repos of files it edited elsewhere (claude, including its subagents) and its children's repos, also after they were closed. Each repo is diffed against the merge-base with the default branch, so committed, uncommitted and untracked changes all show. Edited files outside git are listed |
| `:shell` | open a shell in the agent's directory in a popup |
| `:!<cmd>` | run a command in the agent's directory in a popup |
| `:restore` | restore sessions from last time |
| `:fav`, `:favorites` | toggle favorite / open the favorites list |
| `:goto`, `:move`, `:help`, `:quit` | |

Mouse: click a card to view it, double-click to interact, scroll to move through the list (needs `set -g mouse on` in tmux; hold shift to select text).

Statuses: working, background (the turn ended but background tasks or scheduled wakeups are still pending), needs input, done, idle.

A claude or opencode agent whose `cockpit mcp` server isn't running (30s after it started) shows **⚠ MCP down**, and the footer says how to reconnect: `/mcp` → cockpit → reconnect in claude, a restart in opencode. The server logs its start, exit and any tool panic to `~/.local/state/agent-cockpit/mcp.log`.

When the cockpit pane loses focus its header dims and shows "viewer has focus". This needs `set -g focus-events on` in your tmux config.

## CLI

Every action is also a subcommand, for scripts and other agents. `TARGET` is a pane id, label or window name.

```sh
pane=$(cockpit new --project payments --label refunds --prompt "fix the flaky refund test")
cockpit new --mode review --url https://github.com/acme/payments-service/pull/42
cockpit new --mode bug --project api --ticket SP-1234 --worktree
cockpit new --mode chat --prompt "what does SIGKILL (Code Signature Invalid) mean?"
cockpit new --project payments --label refunds --worktree --prompt "..."   # own git worktree, see below
cockpit list                      # children indented under their parent; or --json
cockpit send refunds "also add a regression test"
git diff | cockpit send %12 -     # - reads stdin
cockpit stop refunds; cockpit kill refunds
cockpit label %12 reviewer        # no text clears
cockpit fav refunds               # toggle; `cockpit fav` lists, `fav resume 1`, `fav rm 1`
cockpit restore                   # bring back agents from last time (with their children)
cockpit project add payments ~/Development/payments-service
```

New agents always go to a dedicated detached `cockpit` tmux session, so your own sessions stay uncluttered. Agents running elsewhere still show up in the list.

New sessions (TUI or CLI) open the configured editor in a pane to the left of the agent, in the agent's directory. It's skipped when the editor isn't installed, and closed together with the agent on `kill`.

## Modes

A mode is a session template.

| Mode | Input | What it does |
|---|---|---|
| `code` | dir | a coding agent, the task is its first prompt (default) |
| `review` | url | reviews a PR in `/tmp/prs/<repo>-<number>` |
| `bug` | ticket + dir | fixes the bug in a ticket (key or URL, read with `jira` or `gh`), with a regression test |
| `chat` | temp | a plain chat for questions, in a fresh `/tmp/cockpit-chats/chat-*` folder that `kill` deletes. Favorites still resume it: the history lives with the agent and the folder is recreated |

Add your own (or override a built-in) in `config.toml`. `{{.Task}}`, `{{.URL}}` and `{{.Ticket}}` are filled from the form or `--prompt`/`--url`/`--ticket`:

```toml
[[mode]]
name = "docs"
title = "Docs"
input = "dir"      # dir, url, ticket, none or temp
agent = "claude"   # optional
prompt = "Update the docs for: {{.Task}}"
```

In the TUI, the mode is the first step of the new-session wizard. Any agent can split its work across children through the MCP tools; children appear indented under their parent.

Favoriting or restoring an agent brings back its children too.

## New session wizard

`n` opens a short wizard, one question per step. Steps that don't apply are skipped:

1. **Type**: coding, PR review, bug fix, chat or a custom mode.
2. **Agent**: skipped when only one agent CLI is installed.
3. **Where**: coding picks a project (or "other folder…" for any directory). PR review asks for the PR URL. Bug fix asks for the ticket, then a project. Chat skips this.
4. **Worktree**: when the folder is in a git repo, choose whether the agent gets its own [worktree](#worktrees).
5. **Details**: the first prompt (task, notes or question) and an optional name.

| Key | Action |
|---|---|
| `↑`/`↓` | choose |
| `enter` | next step; starts the session on the last step |
| `esc` | close the folder list, else back a step; cancels on the first step |
| `→` / `←` | in the folder step: open the highlighted directory / go up (cursor at end) |
| `tab` | switch between the prompt and name on the last step |

A project's configured agent is used unless you picked one in the agent step.

## Worktrees

A coding or bug-fix agent can run in its own git worktree, so several agents can work on one repo in parallel without sharing a checkout. Use the wizard's worktree step, `cockpit new --worktree` or `spawn_agent` with `worktree: true`.

The worktree is created at `<repo>-worktrees/<slug>` next to the main checkout, on a new `cockpit/<slug>` branch from the chosen folder's current HEAD. The slug comes from the label, else the session name, else the folder name. If the folder or branch already exists, `-2`, `-3`, ... is appended.

`kill` leaves worktrees and branches alone since they may hold unmerged work. Clean up by hand once merged:

```sh
git worktree list
git worktree remove ../payments-service-worktrees/refunds
git branch -d cockpit/refunds   # -D if it was never merged
```

## Restore

On quit, cockpit saves the running agents (agent, directory, window name, label, conversation id) to `~/.local/state/agent-cockpit/snapshot.json`. If some of them aren't running next time (closed, tmux restarted, reboot), a banner offers `R` to bring them back, resuming each conversation:

| Agent | Resume command |
|---|---|
| claude | `claude --resume <id>` |
| codex | `codex resume <id>` |
| opencode | `opencode --standalone --session <id>` |

## Projects

A project is a named directory, stored in `~/.config/agent-cockpit/projects.toml`:

```toml
[[project]]
name = "payments"
dir = "~/Development/payments-service"
agent = "claude"   # optional default agent
```

Picking a project in the new-session wizard (or `:new payments`) fills in the directory, window name and agent. Cards for agents running in a project's directory show `◆ <project>`, and `/` matches project names.

## Config

`~/.config/agent-cockpit/config.toml` (optional):

```toml
[notify]
events = ["needs_input", "done"]
desktop = true
tmux = true
command = ""   # sh -c, env: COCKPIT_TITLE, COCKPIT_BODY, COCKPIT_NAME, COCKPIT_STATUS, COCKPIT_MESSAGE, COCKPIT_KIND, COCKPIT_PANE

[claude]
statusline = ""   # your original claude statusLine command, set by cockpit init

[ui]
icons = true   # Nerd Font icons; set false for plain symbols

[editor]
command = "nvim"   # "" disables the editor pane
width = "50%"

[spawn]
max_depth = 3   # agent tree levels (agent -> child -> grandchild); 0 = no limit
```

Notifications name the agent by its label, project, window or folder (e.g. "✅ billing bug finished", "🙋 payments needs you") and show its last message. "Needs you" notifications are urgent: with a sound on macOS, and on Linux they stay until dismissed. Desktop notifications use `osascript` (or `terminal-notifier` if installed) on macOS and D-Bus / `notify-send` on Linux. They're skipped while you're looking at the agent's pane. When claude ends a turn without running its hooks in the agent's pane, cockpit picks the end up from the session transcript and notifies once.

## Releasing

Push a `v*` tag. The release workflow runs lint and tests, builds macOS and Linux binaries (amd64 / arm64) and publishes them with checksums:

```sh
git tag v0.0.2 && git push origin v0.0.2
```

## License

[GPL-3.0](LICENSE)
