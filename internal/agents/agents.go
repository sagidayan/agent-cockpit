package agents

import (
	"slices"
	"sort"
	"strconv"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/notify"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/sagidayan/agent-cockpit/internal/transcript"
)

var kinds = []string{"claude", "codex", "opencode"}

type Live struct {
	state.Agent
	tmux.Pane
	Stats      state.Stats
	HasStats   bool
	MCPDown    bool
	OverBudget bool
}

func List() ([]Live, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return nil, err
	}
	stored, err := state.List()
	if err != nil {
		return nil, err
	}
	server := tmux.ServerPID()
	live, stale := join(panes, stored, server)
	for _, pane := range stale {
		_ = state.Remove(pane)
	}
	for i := range live {
		live[i].Agent = Current(live[i].Agent)
		s, ok := state.LoadStats(live[i].ID)
		live[i].Stats, live[i].HasStats = s, ok && s.Server == server
	}
	Sort(live)
	return live, nil
}

func Current(a state.Agent) state.Agent {
	a = settle(a)
	if a.Paused {
		a.Status = state.Paused
	}
	return a
}

func settle(a state.Agent) state.Agent {
	s := transcript.Settle(a)
	if s.Status != a.Status {
		_, _, _ = notify.Record(a.Pane, settled)
	}
	return s
}

func settled(prev state.Agent) (state.Agent, bool) {
	s := transcript.Settle(prev)
	if s.Status == prev.Status {
		return prev, false
	}
	prev.Status, prev.Message, prev.Prompt = s.Status, s.Message, s.Prompt
	return prev, true
}

func Sort(live []Live) {
	sort.SliceStable(live, func(i, j int) bool {
		a, b := live[i].Pane, live[j].Pane
		if a.Session != b.Session {
			return a.Session < b.Session
		}
		wa, _ := strconv.Atoi(a.Window)
		wb, _ := strconv.Atoi(b.Window)
		return wa < wb
	})
}

func join(panes []tmux.Pane, stored []state.Agent, server string) (live []Live, stale []string) {
	byPane := make(map[string]state.Agent, len(stored))
	for _, a := range stored {
		byPane[a.Pane] = a
	}
	for _, p := range panes {
		a, tracked := byPane[p.ID]
		isAgent := slices.Contains(kinds, p.Command)
		if tracked && (a.Server != "" && a.Server != server || a.Server == "" && !isAgent) {
			stale = append(stale, p.ID)
			tracked = false
		}
		delete(byPane, p.ID)
		if !tracked && !isAgent {
			continue
		}
		if !tracked {
			a = state.Agent{Pane: p.ID, Kind: p.Command, Cwd: p.Cwd}
		}
		live = append(live, Live{Agent: a, Pane: p})
	}
	for pane := range byPane {
		stale = append(stale, pane)
	}
	return live, stale
}

func Subtree(live []Live, root Live) []Live {
	var out []Live
	for _, a := range live {
		if a.Parent == root.ID && a.ID != root.ID {
			out = append(out, Subtree(live, a)...)
		}
	}
	return append(out, root)
}

func Spent(a Live) float64 {
	if !a.HasStats {
		return a.ClosedCost
	}
	return a.ClosedCost + a.Stats.CostUSD
}

func TreeCost(live []Live, root Live) float64 {
	total := 0.0
	for _, a := range Subtree(live, root) {
		total += Spent(a)
	}
	return total
}

func WarnBudget(live []Live, cfg config.Config) {
	if cfg.Budget.TreeUSD <= 0 {
		return
	}
	for i, a := range live {
		if slices.ContainsFunc(live, func(p Live) bool { return p.ID == a.Parent }) {
			continue
		}
		total := TreeCost(live, a)
		live[i].OverBudget = total > cfg.Budget.TreeUSD
		if live[i].OverBudget != a.BudgetWarned {
			_ = notify.Budget(cfg.Notify, a.ID, total, cfg.Budget.TreeUSD)
		}
	}
}
