package tui

import (
	"os"

	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

const (
	viewerWidth = "70%"
	slotCommand = `printf '\n  agent-cockpit viewer slot\n'; exec tail -f /dev/null`
)

type viewer struct {
	self     string
	slot     string
	viewing  string
	home     tmux.Pane
	borrowed string
}

func newViewer() *viewer {
	self := os.Getenv("TMUX_PANE")
	if self == "" {
		return nil
	}
	return &viewer{self: self}
}

func (v *viewer) show(p tmux.Pane) error {
	if v == nil || v.viewing == p.ID {
		return nil
	}
	if err := v.restore(); err != nil {
		return err
	}
	if err := v.ensureSlot(); err != nil {
		return err
	}
	if err := tmux.Swap(p.ID, v.slot); err != nil {
		v.slot = ""
		return err
	}
	v.viewing, v.home = p.ID, p
	return nil
}

func (v *viewer) ensureSlot() error {
	if v.slot != "" {
		return nil
	}
	slot, err := tmux.SplitRight(v.self, viewerWidth, slotCommand)
	v.slot = slot
	return err
}

func (v *viewer) restore() error {
	if v == nil || v.viewing == "" {
		return nil
	}
	v.viewing = ""
	if err := tmux.Swap(v.slot, v.home.ID); err != nil {
		return v.reclaimSlot()
	}
	return nil
}

func (v *viewer) reclaimSlot() error {
	if err := tmux.JoinRight(v.slot, v.self, viewerWidth); err != nil {
		v.slot = ""
		return err
	}
	return nil
}

func (v *viewer) focus() error {
	if v == nil || v.viewing == "" {
		return nil
	}
	return tmux.Select(v.viewing)
}

func (v *viewer) goTo(p tmux.Pane) error {
	if err := v.restore(); err != nil {
		return err
	}
	session, err := tmux.SessionOf(v.self)
	if err != nil {
		return err
	}
	window, err := tmux.WindowOf(p.ID)
	if err != nil {
		return err
	}
	target := session + ":" + window
	if v.borrowed != target {
		v.giveBack()
	}
	here, err := tmux.HasWindow(session, window)
	if err != nil {
		return err
	}
	if !here {
		if err := v.link(session, window); err != nil {
			return err
		}
		v.borrowed = target
	}
	return tmux.SelectWindow(target, p.ID)
}

func (v *viewer) link(session, window string) error {
	own, err := tmux.WindowOf(v.self)
	if err != nil {
		return err
	}
	return tmux.Link(window, session+":"+own)
}

func (v *viewer) giveBack() {
	if v == nil || v.borrowed == "" {
		return
	}
	_ = tmux.Unlink(v.borrowed)
	v.borrowed = ""
}

func (v *viewer) close() {
	if v == nil {
		return
	}
	v.giveBack()
	_ = v.restore()
	if v.slot != "" {
		_ = tmux.Kill(v.slot)
	}
}
