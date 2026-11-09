package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/changes"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/modes"
)

const reviewTask = `Review the changes made by agent %s. Read them with the cockpit get_diff tool on pane %s: it covers every repo the agent and its children touched.
%sThis is a read-only review: do not edit files, commit or push.
Look for bugs, risks and missing tests. Verify each finding against the code (read the file, run the test) before reporting it.
Report findings grouped by severity (critical, high, medium, low), each with file:line and a one-line suggested fix. Say so if you found nothing.`

func reviewAgent(_ context.Context, _ *mcp.CallToolRequest, in paneArgs) (*mcp.CallToolResult, spawnResult, error) {
	live, err := agents.List()
	if err != nil {
		return nil, spawnResult{}, err
	}
	l, err := pick(live, in.Pane)
	if err != nil {
		return nil, spawnResult{}, err
	}
	diff := changes.Diff(l, live)
	if diff == "" {
		return nil, spawnResult{}, fmt.Errorf("no changes to review in the repos %s touched", in.Pane)
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, spawnResult{}, err
	}
	caller := os.Getenv("TMUX_PANE")
	if err := checkDepth(caller, cfg.Spawn.MaxDepth); err != nil {
		return nil, spawnResult{}, err
	}
	res, out, err := start(control.Request{
		Kind:   cmp.Or(callerKind(caller), "claude"),
		Dir:    cmp.Or(l.Agent.Cwd, l.Pane.Cwd),
		Label:  "review " + cmp.Or(l.Label, l.ID),
		Prompt: fmt.Sprintf(reviewTask, in.Pane, in.Pane, changes.Summary(diff)),
		Mode:   modes.Review,
		Parent: caller,
	})
	if out.Pane != "" {
		out.Next = fmt.Sprintf("wait_for_agent(%q), read the findings with get_output, forward them to %s with send_task, then close_agent(%q)", out.Pane, in.Pane, out.Pane)
	}
	return res, out, err
}
