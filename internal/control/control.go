package control

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/changes"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/launch"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

const (
	defaultSession = "cockpit"
	editorOption   = "@cockpit_editor_for"
	modeOption     = "@cockpit_mode"
	parentOption   = "@cockpit_parent"
)

type Request struct {
	Kind, Dir, Name, Prompt, ResumeID, Label, Mode, Parent string
}

func Session() (string, error) {
	return defaultSession, tmux.EnsureSession(defaultSession)
}

func Spawn(session string, r Request) (string, error) {
	if paths.IsChat(r.Dir) {
		if err := os.MkdirAll(r.Dir, 0o755); err != nil {
			return "", err
		}
	}
	command := launch.Command(r.Kind, r.ResumeID)
	if r.ResumeID == "" {
		command = launch.Start(r.Kind, r.Prompt)
	}
	if command == "" {
		return "", fmt.Errorf("unknown agent %q", r.Kind)
	}
	name := cmp.Or(r.Name, r.Label, filepath.Base(r.Dir))
	pane, err := tmux.NewWindow(session, name, r.Dir, command)
	if err != nil {
		return "", err
	}
	if r.Label != "" {
		_ = tmux.SetLabel(pane, r.Label)
	}
	setOption(pane, modeOption, r.Mode)
	setOption(pane, parentOption, r.Parent)
	_ = state.Save(state.Agent{Pane: pane, Kind: r.Kind, Cwd: r.Dir, Status: state.Idle, SessionID: r.ResumeID})
	if r.Mode != "" && r.Mode != "code" {
		return pane, nil
	}
	return pane, openEditor(pane, r.Dir)
}

func setOption(pane, name, value string) {
	if value != "" {
		_ = tmux.SetPaneOption(pane, name, value)
	}
}

func Resume(session string, e snapshot.Entry, parent string, live []agents.Live) (string, error) {
	pane, err := resumeOne(session, e, parent, live)
	if err != nil {
		return "", err
	}
	for _, c := range e.Children {
		if _, err := Resume(session, c, pane, live); err != nil {
			return pane, err
		}
	}
	return pane, nil
}

func resumeOne(session string, e snapshot.Entry, parent string, live []agents.Live) (string, error) {
	for _, a := range live {
		if snapshot.Matches(e, a) {
			setOption(a.ID, parentOption, parent)
			return a.ID, nil
		}
	}
	return Spawn(session, Request{
		Kind: e.Kind, Dir: e.Dir, Name: cmp.Or(e.Name, filepath.Base(e.Dir)),
		ResumeID: e.SessionID, Label: e.Label, Mode: e.Mode, Parent: parent,
	})
}

func openEditor(pane, dir string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	fields := strings.Fields(cfg.Editor.Command)
	if len(fields) == 0 {
		return nil
	}
	if _, err := exec.LookPath(fields[0]); err != nil {
		return nil
	}
	editor, err := tmux.SplitLeft(pane, cmp.Or(cfg.Editor.Width, "50%"), dir, cfg.Editor.Command)
	if err != nil {
		return err
	}
	return tmux.SetPaneOption(editor, editorOption, pane)
}

func Find(target string) (agents.Live, error) {
	live, err := agents.List()
	if err != nil {
		return agents.Live{}, err
	}
	return match(live, target)
}

func match(live []agents.Live, target string) (agents.Live, error) {
	for _, by := range []func(a agents.Live) bool{
		func(a agents.Live) bool { return a.ID == target },
		func(a agents.Live) bool { return strings.EqualFold(a.Label, target) },
		func(a agents.Live) bool { return strings.EqualFold(a.WindowName, target) },
	} {
		var found []agents.Live
		for _, a := range live {
			if by(a) {
				found = append(found, a)
			}
		}
		switch len(found) {
		case 1:
			return found[0], nil
		case 0:
			continue
		default:
			return agents.Live{}, fmt.Errorf("%q matches %d agents, use a pane id", target, len(found))
		}
	}
	return agents.Live{}, fmt.Errorf("no agent matches %q", target)
}

func Send(a agents.Live, text string) error {
	if err := tmux.SendText(a.ID, text); err != nil {
		return err
	}
	a.Status, a.Prompt, a.Message = state.Working, text, ""
	return state.Save(a.Agent)
}

func Stop(a agents.Live) error {
	return tmux.SendKeys(a.ID, "Escape")
}

func Kill(a agents.Live) error {
	handOver(a)
	_ = state.AddClosed(record(a))
	editors, _ := tmux.PanesWithOption(editorOption, a.ID)
	for _, e := range editors {
		_ = tmux.Kill(e)
	}
	_ = state.Remove(a.ID)
	if err := tmux.Kill(a.ID); err != nil {
		return err
	}
	if paths.IsChat(a.Pane.Cwd) {
		return os.RemoveAll(a.Pane.Cwd)
	}
	return nil
}

func record(a agents.Live) state.Closed {
	return state.Closed{
		Name: a.WindowName, Label: a.Label, Kind: a.Kind, Dir: a.Pane.Cwd, Mode: a.Mode,
		Parent: parentName(a.Parent), SessionID: a.SessionID, Message: a.Message,
		Diff: diffSummary(a), CostUSD: a.Stats.CostUSD, ClosedAt: time.Now(),
	}
}

func parentName(pane string) string {
	if pane == "" {
		return ""
	}
	label, window, _ := tmux.Names(pane)
	return cmp.Or(label, window)
}

func diffSummary(a agents.Live) string {
	diff := changes.Diff(a, []agents.Live{a})
	if diff == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(changes.Summary(diff), "# "))
}

func handOver(a agents.Live) {
	if a.Parent == "" {
		return
	}
	if saved, err := state.Load(a.ID); err == nil {
		a.Touched = saved.Touched
	}
	touched := changes.Touched(a)
	if len(touched) == 0 {
		return
	}
	_, _, _, _ = state.Update(a.Parent, func(p state.Agent) (state.Agent, bool) {
		if p.Pane == "" {
			return p, false
		}
		p.Touched = slices.Compact(slices.Sorted(slices.Values(append(p.Touched, touched...))))
		return p, true
	})
}

func Move(a agents.Live, cockpitPane string) (bool, error) {
	session, err := Session()
	if err != nil {
		return false, err
	}
	window, err := tmux.WindowOf(a.ID)
	if err != nil {
		return false, err
	}
	if here, err := tmux.HasWindow(session, window); err != nil || here {
		return false, err
	}
	if own, _ := tmux.WindowOf(cockpitPane); own == window {
		return true, tmux.BreakPane(a.ID, session, a.WindowName)
	}
	return true, tmux.MoveWindow(window, session)
}

func SetLabel(a agents.Live, label string) error {
	return tmux.SetLabel(a.ID, strings.Join(strings.Fields(label), " "))
}
