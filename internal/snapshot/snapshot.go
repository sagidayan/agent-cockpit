package snapshot

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

type Entry struct {
	Kind      string  `json:"kind"`
	Dir       string  `json:"dir"`
	Name      string  `json:"name"`
	Label     string  `json:"label"`
	SessionID string  `json:"session_id"`
	Mode      string  `json:"mode,omitempty"`
	Paused    bool    `json:"paused,omitempty"`
	Children  []Entry `json:"children,omitempty"`
}

func path() string {
	return filepath.Join(filepath.Dir(state.Dir()), "snapshot.json")
}

func Merge(saved []Entry, live []agents.Live) []Entry {
	m := merger{live: live, used: map[string]bool{}}
	out := m.placeAll(saved)
	for _, a := range live {
		if !m.used[a.ID] && !m.hasLiveParent(a) {
			out = append(out, m.tree(a, nil))
		}
	}
	return out
}

type merger struct {
	live []agents.Live
	used map[string]bool
}

func (m merger) placeAll(saved []Entry) []Entry {
	var out []Entry
	for _, e := range saved {
		out = append(out, m.place(e)...)
	}
	return out
}

func (m merger) place(e Entry) []Entry {
	i := slices.IndexFunc(m.live, func(a agents.Live) bool { return Matches(e, a) })
	switch {
	case i < 0:
		e.Children = m.placeAll(e.Children)
		return []Entry{e}
	case m.used[m.live[i].ID] || m.hasLiveParent(m.live[i]):
		return nil
	}
	return []Entry{m.tree(m.live[i], e.Children)}
}

func (m merger) tree(a agents.Live, saved []Entry) Entry {
	m.used[a.ID] = true
	t := entry(a)
	for _, c := range m.live {
		if c.Parent == a.ID && !m.used[c.ID] {
			t.Children = append(t.Children, m.tree(c, savedChildren(saved, c)))
		}
	}
	t.Children = append(t.Children, m.placeAll(saved)...)
	return t
}

func (m merger) hasLiveParent(a agents.Live) bool {
	return a.Parent != "" && a.Parent != a.ID && slices.ContainsFunc(m.live, func(p agents.Live) bool { return p.ID == a.Parent })
}

func savedChildren(saved []Entry, a agents.Live) []Entry {
	for _, e := range saved {
		if Matches(e, a) {
			return e.Children
		}
	}
	return nil
}

func entry(a agents.Live) Entry {
	return Entry{Kind: a.Kind, Dir: a.Pane.Cwd, Name: a.WindowName, Label: a.Label, SessionID: a.SessionID, Mode: a.Mode, Paused: a.Paused}
}

func Tree(a agents.Live, live []agents.Live) Entry {
	e := entry(a)
	for _, c := range live {
		if c.Parent != "" && c.Parent == a.ID && c.ID != a.ID {
			e.Children = append(e.Children, Tree(c, live))
		}
	}
	return e
}

func Save(entries []Entry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path()), 0o755); err != nil {
		return err
	}
	tmp := path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path())
}

func Load() ([]Entry, error) {
	data, err := os.ReadFile(path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	return entries, json.Unmarshal(data, &entries)
}

func Missing(saved []Entry, live []agents.Live) []Entry {
	var missing []Entry
	for _, e := range saved {
		if !slices.ContainsFunc(live, func(a agents.Live) bool { return Matches(e, a) }) {
			missing = append(missing, e)
		}
	}
	return missing
}

func Matches(e Entry, a agents.Live) bool {
	if e.SessionID != "" {
		return e.SessionID == a.SessionID
	}
	return e.Kind == a.Kind && e.Dir == a.Pane.Cwd && (e.Label == "" || e.Label == a.Label)
}

func Sync() ([]agents.Live, error) {
	var live []agents.Live
	return live, update(func(saved []Entry) ([]Entry, error) {
		var err error
		live, err = agents.List()
		return Merge(saved, live), err
	})
}

func Forget(a agents.Live) error {
	return update(func(saved []Entry) ([]Entry, error) { return without(saved, a), nil })
}

func Dismiss(entries []Entry) error {
	return update(func(saved []Entry) ([]Entry, error) {
		return slices.DeleteFunc(saved, func(e Entry) bool { return slices.ContainsFunc(entries, func(d Entry) bool { return same(e, d) }) }), nil
	})
}

func without(entries []Entry, a agents.Live) []Entry {
	var out []Entry
	for _, e := range entries {
		e.Children = without(e.Children, a)
		if Matches(e, a) {
			out = append(out, e.Children...)
			continue
		}
		out = append(out, e)
	}
	return out
}

func same(a, b Entry) bool {
	return a.Kind == b.Kind && a.Dir == b.Dir && a.Label == b.Label && a.SessionID == b.SessionID
}

func update(change func([]Entry) ([]Entry, error)) error {
	unlock, err := state.Lock(".snapshot.lock")
	if err != nil {
		return err
	}
	defer unlock()
	saved, err := Load()
	if err != nil {
		return err
	}
	next, err := change(slices.Clone(saved))
	if err != nil || reflect.DeepEqual(next, saved) {
		return err
	}
	return Save(next)
}

func Clear() error {
	if err := os.Remove(path()); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
