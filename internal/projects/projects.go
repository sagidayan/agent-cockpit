package projects

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/paths"
)

type Project struct {
	Name  string `toml:"name"`
	Dir   string `toml:"dir"`
	Agent string `toml:"agent,omitempty"`
}

type file struct {
	Projects []Project `toml:"project"`
}

func Path() string {
	return filepath.Join(filepath.Dir(config.Path()), "projects.toml")
}

func Load() ([]Project, error) {
	var f file
	_, err := toml.DecodeFile(Path(), &f)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return f.Projects, err
}

func save(list []Project) error {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = ""
	if err := enc.Encode(file{list}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(), buf.Bytes(), 0o644)
}

func Add(p Project) error {
	if strings.ContainsAny(p.Name, " \t") || p.Name == "" {
		return fmt.Errorf("invalid project name %q", p.Name)
	}
	list, err := Load()
	if err != nil {
		return err
	}
	list = slices.DeleteFunc(list, func(e Project) bool { return e.Name == p.Name })
	return save(append(list, p))
}

func Remove(name string) error {
	list, err := Load()
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(list), func(e Project) bool { return e.Name == name })
	if len(kept) == len(list) {
		return fmt.Errorf("no project %q", name)
	}
	return save(kept)
}

func Find(list []Project, name string) (Project, bool) {
	i := slices.IndexFunc(list, func(p Project) bool { return p.Name == name })
	if i < 0 {
		return Project{}, false
	}
	return list[i], true
}

func ForDir(list []Project, dir string) (Project, bool) {
	for _, p := range list {
		if paths.Expand(p.Dir) == dir {
			return p, true
		}
	}
	return Project{}, false
}
