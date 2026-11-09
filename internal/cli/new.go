package cli

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
)

var kinds = []string{"claude", "codex", "opencode"}

func newAgent(args []string) error {
	list, err := projects.Load()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	r, err := request(args, list, modes.All(cfg.Modes), cwd, installed())
	if err != nil {
		return err
	}
	if r.Dir, err = resolveDir(r.Dir); err != nil {
		return err
	}
	return spawn(r)
}

func spawn(r control.Request) error {
	session, err := control.Session()
	if err != nil {
		return err
	}
	pane, err := control.Spawn(session, r)
	if pane != "" {
		fmt.Println(pane)
	}
	return err
}

func installed() []string {
	var found []string
	for _, k := range kinds {
		if _, err := exec.LookPath(k); err == nil {
			found = append(found, k)
		}
	}
	return found
}

func request(args []string, list []projects.Project, all []config.Mode, cwd string, installed []string) (control.Request, error) {
	var s modes.Spec
	var project, mode string
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.StringVar(&s.Kind, "agent", "", "")
	fs.StringVar(&mode, "mode", modes.Code, "")
	fs.StringVar(&s.URL, "url", "", "")
	fs.StringVar(&s.Ticket, "ticket", "", "")
	fs.StringVar(&project, "project", "", "")
	fs.StringVar(&s.Dir, "dir", "", "")
	fs.StringVar(&s.Name, "name", "", "")
	fs.StringVar(&s.Label, "label", "", "")
	fs.StringVar(&s.Task, "prompt", "", "")
	fs.BoolVar(&s.Worktree, "worktree", false, "")
	if err := parse(fs, args); err != nil {
		return control.Request{}, err
	}
	m, ok := modes.Find(all, mode)
	if !ok {
		return control.Request{}, fmt.Errorf("no mode %q", mode)
	}
	if err := applyProject(fs, &s, project, list); err != nil {
		return control.Request{}, err
	}
	if s.Dir != "" || modes.TakesDir(m) {
		s.Dir = absolute(cwd, paths.Expand(s.Dir))
	}
	r, err := modes.Build(m, s)
	if err != nil {
		return control.Request{}, err
	}
	if r.Kind == "" && len(installed) == 0 {
		return control.Request{}, errors.New("no agent CLI installed (claude, codex or opencode)")
	}
	if r.Kind == "" {
		r.Kind = installed[0]
	}
	return r, nil
}

func applyProject(fs *flag.FlagSet, s *modes.Spec, project string, list []projects.Project) error {
	if project == "" {
		return nil
	}
	if s.Dir != "" {
		return usageError(fs, errors.New("use --project or --dir, not both"))
	}
	p, ok := projects.Find(list, project)
	if !ok {
		return fmt.Errorf("no project %q", project)
	}
	s.Dir, s.Name, s.Kind = paths.Expand(p.Dir), cmp.Or(s.Name, p.Name), cmp.Or(s.Kind, p.Agent)
	return nil
}

func absolute(cwd, dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return filepath.Join(cwd, dir)
}
