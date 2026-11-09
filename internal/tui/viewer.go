package tui

import (
	"os"

	"github.com/sagidayan/agent-cockpit/internal/launch"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

const (
	dashboardOption = "@cockpit_dashboard"
	viewerWidth     = "70%"
	slotCommand     = `printf '\n  agent-cockpit viewer slot\n'; exec tail -f /dev/null`
)

type viewer struct {
	self    string
	slot    string
	viewing string
	home    tmux.Pane
}

func newViewer() *viewer {
	self := os.Getenv("TMUX_PANE")
	if self == "" {
		return nil
	}
	if s, err := tmux.SessionOf(self); err != nil || s != tmux.Session {
		return nil
	}
	return &viewer{self: self}
}

func (v *viewer) show(p tmux.Pane) error {
	if v == nil || v.viewing == p.ID {
		return nil
	}
	if err := v.restore(); err != nil || p.Session != tmux.Session {
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

func openInCockpit(self string) (bool, error) {
	if self == "" {
		return false, nil
	}
	if s, err := tmux.SessionOf(self); err != nil || s == tmux.Session {
		return false, err
	}
	if err := tmux.EnsureSession(); err != nil {
		return true, err
	}
	panes, err := tmux.PanesWithOption(dashboardOption, "1")
	if err != nil {
		return true, err
	}
	if len(panes) == 0 {
		exe, err := os.Executable()
		if err != nil {
			return true, err
		}
		pane, err := tmux.NewWindow(tmux.Session, "dashboard", paths.Expand("~"), launch.Quote(exe))
		if err != nil {
			return true, err
		}
		panes = []string{pane}
	}
	return true, tmux.Focus(panes[0])
}

func (v *viewer) goTo(p tmux.Pane) error {
	if err := v.restore(); err != nil {
		return err
	}
	return tmux.Focus(p.ID)
}

func (v *viewer) close() {
	if v == nil {
		return
	}
	_ = v.restore()
	if v.slot != "" {
		_ = tmux.Kill(v.slot)
	}
}
