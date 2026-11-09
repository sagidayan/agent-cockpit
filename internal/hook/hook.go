package hook

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/notify"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

type event struct {
	status     state.Status
	message    string
	cwd        string
	remove     bool
	stats      *state.Stats
	session    string
	prompt     string
	transcript string
}

type parser func(args []string, stdin io.Reader) (event, error)

var parsers = map[string]parser{
	"claude":   parseClaude,
	"codex":    parseCodex,
	"opencode": parseOpencode,
}

func Run(kind string, args []string) error {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return nil
	}
	if err := record(kind, pane, args); err != nil {
		fmt.Fprintln(os.Stderr, "cockpit hook:", err)
	}
	return nil
}

func record(kind, pane string, args []string) error {
	parse, ok := parsers[kind]
	if !ok {
		return fmt.Errorf("unknown agent %q", kind)
	}
	ev, err := parse(args, os.Stdin)
	if err != nil {
		return err
	}
	if ev.stats != nil {
		return state.SaveStats(pane, *ev.stats)
	}
	if ev.remove {
		return ignoreMissing(state.Remove(pane))
	}
	if ev.status == "" {
		return nil
	}
	return update(state.Agent{Pane: pane, Kind: kind, Cwd: cwd(ev), Status: ev.status, Message: ev.message, Prompt: ev.prompt, Transcript: ev.transcript, SessionID: ev.session})
}

func update(a state.Agent) error {
	prev, cur, err := notify.Record(a.Pane, func(prev state.Agent) (state.Agent, bool) {
		return merge(prev, a), true
	})
	return errors.Join(err, control.Report(prev, cur))
}

func merge(prev, a state.Agent) state.Agent {
	if prev.Kind == a.Kind {
		a.Touched, a.ClosedCost, a.BudgetWarned = prev.Touched, prev.ClosedCost, prev.BudgetWarned
		a.Paused = prev.Paused && !(a.Status == state.Working && a.Prompt != "")
	}
	if prev.Kind == a.Kind && (a.SessionID == "" || a.SessionID == prev.SessionID) {
		a.SessionID = prev.SessionID
		a.Prompt = cmp.Or(a.Prompt, prev.Prompt)
		a.Transcript = cmp.Or(a.Transcript, prev.Transcript)
	}
	return a
}

func cwd(ev event) string {
	if ev.cwd != "" {
		return ev.cwd
	}
	dir, _ := os.Getwd()
	return dir
}

func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
