package mcpserver

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

type spawnArgs struct {
	Project  string `json:"project,omitempty" jsonschema:"project name from list_projects (or use dir)"`
	Dir      string `json:"dir,omitempty" jsonschema:"absolute directory to work in (or use project)"`
	Task     string `json:"task" jsonschema:"self-contained first prompt for the agent"`
	Agent    string `json:"agent,omitempty" jsonschema:"claude, codex or opencode (default: the project's agent, else yours)"`
	Label    string `json:"label,omitempty" jsonschema:"short label shown in the cockpit"`
	Mode     string `json:"mode,omitempty" jsonschema:"code (default) or bug; bug fixes the bug described in ticket"`
	Ticket   string `json:"ticket,omitempty" jsonschema:"bug mode: ticket key or URL, e.g. SP-1234 or a GitHub issue URL"`
	Worktree bool   `json:"worktree,omitempty" jsonschema:"work in a new git worktree on a cockpit/<label> branch from the folder's HEAD, so parallel agents in one repo don't share a checkout"`
}

type spawnResult struct {
	Pane string `json:"pane"`
	Kind string `json:"kind"`
	Dir  string `json:"dir"`
	Next string `json:"next"`
}

type projectInfo struct {
	Name  string `json:"name"`
	Dir   string `json:"dir"`
	Agent string `json:"agent,omitempty"`
}

type projectList struct {
	Projects []projectInfo `json:"projects"`
}

func listProjects(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, projectList, error) {
	list, err := projects.Load()
	if err != nil {
		return nil, projectList{}, err
	}
	out := projectList{Projects: make([]projectInfo, len(list))}
	for i, p := range list {
		out.Projects[i] = projectInfo{p.Name, paths.Expand(p.Dir), p.Agent}
	}
	return nil, out, nil
}

func spawnAgent(_ context.Context, _ *mcp.CallToolRequest, in spawnArgs) (*mcp.CallToolResult, spawnResult, error) {
	r, err := spawnRequest(in)
	if err != nil {
		return nil, spawnResult{}, err
	}
	session, err := control.Session()
	if err != nil {
		return nil, spawnResult{}, err
	}
	pane, err := control.Spawn(session, r)
	if pane == "" {
		return nil, spawnResult{}, err
	}
	next := fmt.Sprintf("wait_for_agent(%q), then get_output and get_diff; send_task to give feedback", pane)
	return nil, spawnResult{pane, r.Kind, r.Dir, next}, nil
}

func spawnRequest(in spawnArgs) (control.Request, error) {
	cfg, err := config.Load()
	if err != nil {
		return control.Request{}, err
	}
	caller := os.Getenv("TMUX_PANE")
	if err := checkDepth(caller, cfg.Spawn.MaxDepth); err != nil {
		return control.Request{}, err
	}
	m, ok := modes.Find(modes.All(cfg.Modes), cmp.Or(in.Mode, modes.Code))
	if !ok || (m.Name != modes.Code && m.Name != modes.Bug) {
		return control.Request{}, fmt.Errorf("mode must be code or bug, got %q", in.Mode)
	}
	spec, err := spawnSpec(in)
	if err != nil {
		return control.Request{}, err
	}
	spec.Label, spec.Ticket, spec.Worktree = in.Label, in.Ticket, in.Worktree
	r, err := modes.Build(m, spec)
	if err != nil {
		return control.Request{}, err
	}
	r.Kind, r.Parent = cmp.Or(r.Kind, callerKind(caller), "claude"), caller
	return r, nil
}

func checkDepth(caller string, max int) error {
	if caller == "" || max <= 0 {
		return nil
	}
	panes, err := tmux.ListPanes()
	if err != nil {
		return err
	}
	parents := make(map[string]string, len(panes))
	for _, p := range panes {
		parents[p.ID] = p.Parent
	}
	if level(caller, parents) >= max {
		return fmt.Errorf("agent nesting limit reached (%d levels, [spawn] max_depth in config.toml): do this work yourself instead of spawning", max)
	}
	return nil
}

func level(pane string, parents map[string]string) int {
	n := 0
	for id := pane; id != "" && n <= len(parents); id = parents[id] {
		n++
	}
	return n
}

func spawnSpec(in spawnArgs) (modes.Spec, error) {
	if (in.Project == "") == (in.Dir == "") {
		return modes.Spec{}, errors.New("set exactly one of project or dir")
	}
	if in.Project == "" {
		return dirSpec(in)
	}
	list, err := projects.Load()
	if err != nil {
		return modes.Spec{}, err
	}
	p, ok := projects.Find(list, in.Project)
	if !ok {
		return modes.Spec{}, fmt.Errorf("no project %q, call list_projects", in.Project)
	}
	return modes.Spec{Kind: cmp.Or(in.Agent, p.Agent), Dir: paths.Expand(p.Dir), Name: p.Name, Task: in.Task}, nil
}

func dirSpec(in spawnArgs) (modes.Spec, error) {
	dir := filepath.Clean(paths.Expand(in.Dir))
	if info, err := os.Stat(dir); err != nil || !info.IsDir() || !filepath.IsAbs(dir) {
		return modes.Spec{}, fmt.Errorf("not an absolute directory: %s", in.Dir)
	}
	return modes.Spec{Kind: in.Agent, Dir: dir, Task: in.Task}, nil
}

func callerKind(pane string) string {
	if pane == "" {
		return ""
	}
	a, err := state.Load(pane)
	if err != nil {
		return ""
	}
	return a.Kind
}
