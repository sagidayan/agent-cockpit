package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/changes"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

const (
	messageLimit = 300
	diffLimit    = 50_000
)

type paneArgs struct {
	Pane string `json:"pane" jsonschema:"tmux pane id from list_agents, e.g. %12"`
}

type sendArgs struct {
	Pane   string `json:"pane" jsonschema:"tmux pane id from list_agents, e.g. %12"`
	Prompt string `json:"prompt" jsonschema:"the task to give the agent"`
}

type waitArgs struct {
	Pane           string `json:"pane" jsonschema:"tmux pane id from list_agents, e.g. %12"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"max seconds to wait (default 300, max 1800)"`
}

type outputArgs struct {
	Pane  string `json:"pane" jsonschema:"tmux pane id from list_agents, e.g. %12"`
	Lines int    `json:"lines,omitempty" jsonschema:"terminal lines to capture (default 100)"`
}

type agentInfo struct {
	Pane       string       `json:"pane"`
	Kind       string       `json:"kind"`
	Session    string       `json:"session"`
	WindowName string       `json:"window_name"`
	Cwd        string       `json:"cwd"`
	Mode       string       `json:"mode,omitempty"`
	Parent     string       `json:"parent,omitempty"`
	Status     state.Status `json:"status"`
	Message    string       `json:"message"`
	UpdatedAt  time.Time    `json:"updated_at"`
	Stats      *state.Stats `json:"stats,omitempty"`
}

type agentList struct {
	Agents []agentInfo `json:"agents"`
}

func toInfo(l agents.Live) agentInfo {
	info := agentInfo{
		Pane:       l.Pane.ID,
		Kind:       l.Kind,
		Session:    l.Session,
		WindowName: l.WindowName,
		Cwd:        l.Pane.Cwd,
		Mode:       l.Mode,
		Parent:     l.Parent,
		Status:     l.Status,
		Message:    truncate(l.Message, messageLimit),
		UpdatedAt:  l.UpdatedAt,
	}
	if l.HasStats {
		info.Stats = &l.Stats
	}
	return info
}

func find(pane string) (agents.Live, error) {
	live, err := agents.List()
	if err != nil {
		return agents.Live{}, err
	}
	return pick(live, pane)
}

func pick(live []agents.Live, pane string) (agents.Live, error) {
	for _, l := range live {
		if l.Pane.ID == pane {
			return l, nil
		}
	}
	return agents.Live{}, fmt.Errorf("unknown pane %q: no tracked agent there, call list_agents for valid pane ids", pane)
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func listAgents(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, agentList, error) {
	live, err := agents.List()
	if err != nil {
		return nil, agentList{}, err
	}
	out := agentList{Agents: make([]agentInfo, 0, len(live))}
	for _, l := range live {
		out.Agents = append(out.Agents, toInfo(l))
	}
	return nil, out, nil
}

func sendTask(_ context.Context, _ *mcp.CallToolRequest, in sendArgs) (*mcp.CallToolResult, agentInfo, error) {
	l, err := find(in.Pane)
	if err != nil {
		return nil, agentInfo{}, err
	}
	if err := tmux.SendText(in.Pane, in.Prompt); err != nil {
		return nil, agentInfo{}, err
	}
	l.Status, l.Prompt, l.Message = state.Working, in.Prompt, ""
	if err := state.Save(l.Agent); err != nil {
		return nil, agentInfo{}, err
	}
	return nil, toInfo(l), nil
}

func waitForAgent(ctx context.Context, _ *mcp.CallToolRequest, in waitArgs) (*mcp.CallToolResult, agentInfo, error) {
	l, err := find(in.Pane)
	if err != nil {
		return nil, agentInfo{}, err
	}
	deadline := time.After(time.Duration(waitSeconds(in.TimeoutSeconds)) * time.Second)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for l.Status == state.Working || l.Status == state.Background {
		select {
		case <-ctx.Done():
			return nil, agentInfo{}, ctx.Err()
		case <-deadline:
			return nil, toInfo(l), nil
		case <-tick.C:
		}
		if l.Agent, err = state.Load(in.Pane); err != nil {
			return nil, agentInfo{}, err
		}
		l.Agent = agents.Settle(l.Agent)
	}
	return nil, toInfo(l), nil
}

func waitSeconds(requested int) int {
	if requested <= 0 {
		return 300
	}
	return min(requested, 1800)
}

func getOutput(_ context.Context, _ *mcp.CallToolRequest, in outputArgs) (*mcp.CallToolResult, any, error) {
	l, err := find(in.Pane)
	if err != nil {
		return nil, nil, err
	}
	lines := in.Lines
	if lines <= 0 {
		lines = 100
	}
	screen, err := tmux.Capture(in.Pane, lines)
	if err != nil {
		return nil, nil, err
	}
	return text(fmt.Sprintf("status: %s\n\nlast message:\n%s\n\npane output:\n%s", l.Status, l.Message, screen)), nil, nil
}

func getDiff(_ context.Context, _ *mcp.CallToolRequest, in paneArgs) (*mcp.CallToolResult, any, error) {
	live, err := agents.List()
	if err != nil {
		return nil, nil, err
	}
	l, err := pick(live, in.Pane)
	if err != nil {
		return nil, nil, err
	}
	diff := changes.Diff(l, live)
	if diff == "" {
		return text("no changes found in the repos " + in.Pane + " or its children touched"), nil, nil
	}
	return text(truncate(changes.Summary(diff)+diff, diffLimit)), nil, nil
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return strings.ToValidUTF8(s[:limit], "") + fmt.Sprintf("\n[truncated: showing %d of %d bytes]", limit, len(s))
}
