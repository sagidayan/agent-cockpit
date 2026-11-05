package cli

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

type agentJSON struct {
	Pane    string       `json:"pane"`
	Kind    string       `json:"kind"`
	Status  state.Status `json:"status"`
	Session string       `json:"session"`
	Window  string       `json:"window"`
	Label   string       `json:"label,omitempty"`
	Mode    string       `json:"mode,omitempty"`
	Parent  string       `json:"parent,omitempty"`
	Dir     string       `json:"dir"`
	Project string       `json:"project,omitempty"`
	Message string       `json:"message,omitempty"`
	Stats   *state.Stats `json:"stats,omitempty"`
}

func list(args []string) error {
	var asJSON bool
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.BoolVar(&asJSON, "json", false, "")
	if err := parse(fs, args); err != nil {
		return err
	}
	live, err := agents.List()
	if err != nil {
		return err
	}
	projs, err := projects.Load()
	if err != nil {
		return err
	}
	if asJSON {
		return printJSON(live, projs)
	}
	fmt.Print(table(live, projs))
	return nil
}

func printJSON(live []agents.Live, projs []projects.Project) error {
	out := make([]agentJSON, len(live))
	for i, a := range live {
		out[i] = toJSON(a, projs)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func toJSON(a agents.Live, projs []projects.Project) agentJSON {
	p, _ := projects.ForDir(projs, a.Pane.Cwd)
	out := agentJSON{a.ID, a.Kind, a.Status, a.Session, a.WindowName, a.Label, a.Mode, a.Parent, a.Pane.Cwd, p.Name, a.Message, nil}
	if a.HasStats {
		out.Stats = &a.Stats
	}
	return out
}

type node struct {
	agents.Live
	depth int
}

func table(live []agents.Live, projs []projects.Project) string {
	withMode := slices.ContainsFunc(live, func(a agents.Live) bool { return cmp.Or(a.Mode, modes.Code) != modes.Code })
	rows := [][]string{columns(withMode, "PANE", "STATUS", "AGENT", "MODE", "NAME", "WHERE", "COST")}
	for _, n := range tree(live) {
		rows = append(rows, columns(withMode, n.ID, cmp.Or(string(n.Status), "unknown"), n.Kind, cmp.Or(n.Mode, modes.Code), indent(n.depth)+cmp.Or(n.Label, n.WindowName), where(n.Pane.Cwd, projs), cost(n.Live)))
	}
	return render(rows)
}

func columns(withMode bool, cells ...string) []string {
	if withMode {
		return cells
	}
	return slices.Delete(cells, 3, 4)
}

func indent(depth int) string {
	if depth == 0 {
		return ""
	}
	return strings.Repeat("  ", depth-1) + "└ "
}

func tree(live []agents.Live) []node {
	ids := make(map[string]bool, len(live))
	for _, a := range live {
		ids[a.ID] = true
	}
	var out []node
	for _, a := range live {
		if !ids[a.Parent] {
			out = appendTree(out, a, live, 0)
		}
	}
	return out
}

func appendTree(out []node, a agents.Live, live []agents.Live, depth int) []node {
	out = append(out, node{a, depth})
	for _, c := range live {
		if c.Parent == a.ID && c.ID != a.ID {
			out = appendTree(out, c, live, depth+1)
		}
	}
	return out
}

func where(dir string, projs []projects.Project) string {
	if p, ok := projects.ForDir(projs, dir); ok {
		return p.Name
	}
	return paths.Contract(dir)
}

func cost(a agents.Live) string {
	if !a.HasStats || a.Stats.CostUSD == 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", a.Stats.CostUSD)
}
