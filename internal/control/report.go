package control

import (
	"fmt"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/notify"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

const waiterOption = "@cockpit_waited_by"

var reports = map[state.Status]string{
	state.Done:       "is done%s. Read it with get_output / get_diff.",
	state.NeedsInput: "needs input%s. Read it with get_output and answer with send_task.",
}

func Waiting(pane, waiter string) func() {
	setOption(pane, waiterOption, waiter)
	return func() { _ = tmux.UnsetPaneOption(pane, waiterOption) }
}

func Report(prev, cur state.Agent) error {
	if prev.Status == cur.Status || reports[cur.Status] == "" {
		return nil
	}
	cfg, err := config.Load()
	if err != nil || !cfg.Spawn.NotifyParent {
		return err
	}
	opts, err := tmux.PaneOptions(cur.Pane, parentOption, waiterOption, "@cockpit_label")
	if err != nil {
		return err
	}
	parent, waiter, label := opts[0], opts[1], opts[2]
	if parent == "" || waiter == parent {
		return nil
	}
	if s, err := tmux.SessionOf(parent); err != nil || s != tmux.Session {
		return nil
	}
	p, err := state.Load(parent)
	if err != nil || p.Kind != "claude" || p.Status == state.NeedsInput {
		return nil
	}
	return tmux.SendText(parent, report(cur, label))
}

func report(a state.Agent, label string) string {
	who := a.Pane
	if label != "" {
		who += " (" + label + ")"
	}
	msg := strings.TrimRight(notify.Summary(a.Message), ".")
	if msg != "" {
		msg = ": " + msg
	}
	return fmt.Sprintf("[cockpit] child %s "+reports[a.Status], who, msg)
}
