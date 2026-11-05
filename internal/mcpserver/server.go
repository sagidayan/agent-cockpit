package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Run() error {
	return newServer().Run(context.Background(), &mcp.StdioTransport{})
}

func newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agent-cockpit", Version: "0.1.0"}, &mcp.ServerOptions{
		Instructions: "Dispatch tasks to coding agents running in tmux panes and review their results. " +
			"New work: list_projects, spawn_agent, wait_for_agent, get_output and get_diff to review, send_task for feedback. " +
			"Existing agents: list_agents to pick an idle one, then send_task.",
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_projects",
		Description: "List the named project folders (name, dir, default agent) you can spawn agents in.",
	}, listProjects)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "spawn_agent",
		Description: "Start a new child agent in a project or directory with a first task. Use mode master for a sub-master that coordinates its own children on a large chunk of work (nesting depth is capped). Returns its pane id; follow with wait_for_agent.",
	}, spawnAgent)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_agents",
		Description: "List coding agents (claude-code, codex, opencode) running in tmux panes, with their repo cwd, mode, parent pane and status (working, needs_input, done, idle). Use the pane id with the other tools. Prefer agents that are done or idle; don't send to a working agent.",
	}, listAgents)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "send_task",
		Description: "Type a prompt into an agent's pane and submit it, marking the agent as working. The agent works in its own repo (see cwd from list_agents), so write a self-contained prompt. Follow with wait_for_agent.",
	}, sendTask)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "wait_for_agent",
		Description: "Block until the agent stops working (done, needs_input or idle; background tasks count as still working) or the timeout expires, then return its state. If still working after the timeout, call again. If needs_input, read get_output and answer with send_task.",
	}, waitForAgent)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_output",
		Description: "Get the agent's last reported message plus the recent visible text of its terminal pane. Use after wait_for_agent to read what it did or what it is asking.",
	}, getOutput)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_diff",
		Description: "Get uncommitted changes (staged and unstaged diff vs HEAD, plus untracked file names) in the agent's working directory. Use to review an agent's code changes after it finishes.",
	}, getDiff)
	return s
}
