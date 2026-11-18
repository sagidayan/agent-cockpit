package hook

import (
	"cmp"
	"encoding/json"
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
	deny       string
}

type parser func(args []string, stdin io.Reader) (event, error)

var parsers = map[string]parser{
	"claude":   parseClaude,
	"codex":    parseCodex,
	"opencode": parseOpencode,
}

func Run(kind string, args []string) error {
	ev, err := parse(kind, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cockpit hook:", err)
		return nil
	}
	if ev.deny != "" {
		return writeDeny(ev.deny)
	}
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return nil
	}
	if err := record(kind, pane, ev); err != nil {
		fmt.Fprintln(os.Stderr, "cockpit hook:", err)
	}
	return nil
}

func parse(kind string, args []string) (event, error) {
	p, ok := parsers[kind]
	if !ok {
		return event{}, fmt.Errorf("unknown agent %q", kind)
	}
	return p(args, os.Stdin)
}

func writeDeny(reason string) error {
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName":            "PreToolUse",
		"permissionDecision":       "deny",
		"permissionDecisionReason": reason,
	}})
}

func record(kind, pane string, ev event) error {
	if ev.stats != nil {
		return state.SaveStats(pane, *ev.stats)
	}
	if ev.remove {
		return ignoreMissing(endSession(pane))
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

func endSession(pane string) error {
	_, _, kept, err := state.Update(pane, func(p state.Agent) (state.Agent, bool) {
		if !p.Restarting {
			return p, false
		}
		p.Restarting = false
		return p, true
	})
	if err != nil || kept {
		return err
	}
	return state.Remove(pane)
}

func merge(prev, a state.Agent) state.Agent {
	if prev.Kind == a.Kind {
		a.Touched, a.ClosedCost, a.BudgetWarned, a.Restarting = prev.Touched, prev.ClosedCost, prev.BudgetWarned, prev.Restarting
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
