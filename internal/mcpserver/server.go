package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

func Run() error {
	logf("start, pane %s", os.Getenv("TMUX_PANE"))
	err := newServer().Run(context.Background(), &mcp.StdioTransport{})
	logf("exit, pane %s: %v", os.Getenv("TMUX_PANE"), err)
	return err
}

func addTool[In, Out any](s *mcp.Server, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(s, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out Out, err error) {
		defer func() {
			if r := recover(); r != nil {
				logf("panic in %s: %v\n%s", t.Name, r, debug.Stack())
				err = fmt.Errorf("cockpit %s crashed: %v", t.Name, r)
			}
		}()
		return h(ctx, req, in)
	})
}

func logf(format string, args ...any) {
	path := state.MCPLog()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s pid %d: %s\n", time.Now().Format(time.RFC3339), os.Getpid(), fmt.Sprintf(format, args...))
}

func newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agent-cockpit", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Delegate work to other coding agents running in tmux panes, and review their results. " +
			"Use it when a task splits into independent chunks, spans several repos, or the user asks you to fan out. " +
			"New work: list_projects for the folders you can work in, spawn_agent with a focused, self-contained task, wait_for_agent, then get_output and get_diff to review and send_task for feedback. " +
			"Run children in parallel when you can. When several work in the same repo at once, pass worktree: true so each gets its own branch, and name the branches in your report. " +
			"A child can spawn its own children for a large chunk. " +
			"Close each child with close_agent once its work is done and reviewed, and close all your children before you report back. " +
			"Existing agents: list_agents to pick an idle one, then send_task.",
	})
	addTool(s, &mcp.Tool{
		Name:        "list_projects",
		Description: "List the named project folders (name, dir, default agent) you can spawn agents in.",
	}, listProjects)
	addTool(s, &mcp.Tool{
		Name:        "spawn_agent",
		Description: "Start a new child agent in a project or directory with a first task. Use mode bug with a ticket to have it fix a reported bug. Children can spawn their own children (nesting depth is capped). Pass worktree: true when several agents work in one repo. Returns its pane id; follow with wait_for_agent, review with get_output and get_diff, and close it with close_agent once its work is done.",
	}, spawnAgent)
	addTool(s, &mcp.Tool{
		Name:        "list_agents",
		Description: "List coding agents (claude-code, codex, opencode) running in tmux panes, with their repo cwd, mode, parent pane and status (working, needs_input, done, idle). Use the pane id with the other tools. Prefer agents that are done or idle; don't send to a working agent.",
	}, listAgents)
	addTool(s, &mcp.Tool{
		Name:        "send_task",
		Description: "Type a prompt into an agent's pane and submit it, marking the agent as working. The agent works in its own repo (see cwd from list_agents), so write a self-contained prompt. Follow with wait_for_agent.",
	}, sendTask)
	addTool(s, &mcp.Tool{
		Name:        "wait_for_agent",
		Description: "Block until the agent stops working (done, needs_input or idle; background tasks count as still working) or the timeout expires, then return its state. If still working after the timeout, call again. If needs_input, read get_output and answer with send_task.",
	}, waitForAgent)
	addTool(s, &mcp.Tool{
		Name:        "get_output",
		Description: "Get the agent's last reported message plus the recent visible text of its terminal pane. Use after wait_for_agent to read what it did or what it is asking.",
	}, getOutput)
	addTool(s, &mcp.Tool{
		Name:        "close_agent",
		Description: "Close a child agent you spawned, and any agents it spawned, once its work is done and reviewed. Kills its tmux pane; it cannot be undone. Only works on your own children.",
	}, closeAgent)
	addTool(s, &mcp.Tool{
		Name:        "get_diff",
		Description: "Get the agent's changes in every repo it touched (its folder, repos it cloned into a temp folder, files it edited elsewhere) and in its children's repos, including closed children: committed work on each branch since the merge-base with the default branch, plus uncommitted and untracked changes, with a file and line summary on top. Use to review an agent's code after it finishes.",
	}, getDiff)
	return s
}
