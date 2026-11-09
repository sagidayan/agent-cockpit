package snapshot

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

func FromLive(live []agents.Live) []Entry {
	ids := make(map[string]bool, len(live))
	for _, a := range live {
		ids[a.ID] = true
	}
	var roots []Entry
	for _, a := range live {
		if a.Parent == "" || !ids[a.Parent] {
			roots = append(roots, Tree(a, live))
		}
	}
	return roots
}

func Tree(a agents.Live, live []agents.Live) Entry {
	e := Entry{Kind: a.Kind, Dir: a.Pane.Cwd, Name: a.WindowName, Label: a.Label, SessionID: a.SessionID, Mode: a.Mode, Paused: a.Paused}
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
	return os.WriteFile(path(), data, 0o644)
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

func Clear() error {
	if err := os.Remove(path()); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
