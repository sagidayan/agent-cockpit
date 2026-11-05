package favorites

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
)

func Path() string {
	return filepath.Join(filepath.Dir(config.Path()), "favorites.json")
}

func Load() ([]snapshot.Entry, error) {
	data, err := os.ReadFile(Path())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []snapshot.Entry
	return list, json.Unmarshal(data, &list)
}

func save(list []snapshot.Entry) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0o644)
}

func Index(list []snapshot.Entry, a agents.Live) int {
	return slices.IndexFunc(list, func(e snapshot.Entry) bool { return snapshot.Matches(e, a) })
}

func Toggle(a agents.Live, live []agents.Live) (bool, error) {
	list, err := Load()
	if err != nil {
		return false, err
	}
	if i := Index(list, a); i >= 0 {
		return false, save(slices.Delete(list, i, i+1))
	}
	return true, save(append(list, snapshot.Tree(a, live)))
}

func Sync(live []agents.Live) error {
	list, err := Load()
	if err != nil || len(list) == 0 {
		return err
	}
	updated := slices.Clone(list)
	for i, e := range updated {
		for _, a := range live {
			if snapshot.Matches(e, a) {
				t := snapshot.Tree(a, live)
				t.Name, t.Label = e.Name, e.Label
				updated[i] = t
				break
			}
		}
	}
	if reflect.DeepEqual(updated, list) {
		return nil
	}
	return save(updated)
}

func Remove(e snapshot.Entry) error {
	list, err := Load()
	if err != nil {
		return err
	}
	return save(slices.DeleteFunc(list, func(x snapshot.Entry) bool { return reflect.DeepEqual(x, e) }))
}

func Running(e snapshot.Entry, live []agents.Live) bool {
	return slices.ContainsFunc(live, func(a agents.Live) bool { return snapshot.Matches(e, a) })
}
