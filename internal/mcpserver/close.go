package mcpserver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/control"
)

func closeAgent(_ context.Context, _ *mcp.CallToolRequest, in paneArgs) (*mcp.CallToolResult, any, error) {
	live, err := agents.List()
	if err != nil {
		return nil, nil, err
	}
	tree, err := closable(live, in.Pane, os.Getenv("TMUX_PANE"))
	if err != nil {
		return nil, nil, err
	}
	var closed []string
	for _, a := range tree {
		if err := control.Kill(a); err != nil {
			return nil, nil, err
		}
		closed = append(closed, a.ID)
	}
	return text("closed " + strings.Join(closed, ", ")), nil, nil
}

func closable(live []agents.Live, pane, caller string) ([]agents.Live, error) {
	for _, a := range live {
		if a.ID != pane {
			continue
		}
		if caller == "" || a.Parent != caller {
			return nil, fmt.Errorf("%s is not your child agent; you can only close agents you spawned", pane)
		}
		return agents.Subtree(live, a), nil
	}
	return nil, fmt.Errorf("no agent in pane %s", pane)
}
