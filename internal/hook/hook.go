package hook

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/sagidayan/agent-cockpit/internal/config"
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
	prev, _ := state.Load(a.Pane)
	if prev.Kind == a.Kind && (a.SessionID == "" || a.SessionID == prev.SessionID) {
		a.SessionID = prev.SessionID
		a.Prompt = cmp.Or(a.Prompt, prev.Prompt)
		a.Transcript = cmp.Or(a.Transcript, prev.Transcript)
	}
	if err := state.Save(a); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !notify.Wanted(cfg.Notify, prev, a) {
		return nil
	}
	return notify.Send(cfg.Notify, a)
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
