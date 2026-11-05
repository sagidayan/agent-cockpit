package cli

import (
	"fmt"
	"os"

	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
)

func project(args []string) error {
	switch {
	case len(args) == 0 || len(args) == 1 && args[0] == "list":
		return listProjects()
	case args[0] == "add" && (len(args) == 2 || len(args) == 3):
		return addProject(args[1], args[2:])
	case args[0] == "rm" && len(args) == 2:
		return projects.Remove(args[1])
	}
	return errUsage
}

func listProjects() error {
	list, err := projects.Load()
	if err != nil {
		return err
	}
	rows := make([][]string, len(list))
	for i, p := range list {
		rows[i] = []string{p.Name, p.Dir, p.Agent}
	}
	fmt.Print(render(rows))
	return nil
}

func addProject(name string, rest []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		dir = rest[0]
	}
	if dir, err = resolveDir(dir); err != nil {
		return err
	}
	return projects.Add(projects.Project{Name: name, Dir: paths.Contract(dir)})
}
