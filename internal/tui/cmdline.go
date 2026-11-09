package tui

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/changes"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/launch"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

type command struct {
	name, args, help string
	run              func(m model, args string) (model, tea.Cmd)
}

type confirmation struct {
	prompt string
	run    func(m model) (model, tea.Cmd)
}

func commands() []command {
	return []command{
		{"new", "[agent] [project|dir]", "start a new session", cmdNew},
		{"send", "<text>", "send text to the selected agent", cmdSend},
		{"broadcast", "<text>", "send text to every listed agent", cmdBroadcast},
		{"stop", "", "interrupt the selected agent", cmdStop},
		{"kill", "", "close the selected agent's pane", cmdKill},
		{"restart", "", "relaunch the selected agent, resuming its session", cmdRestart},
		{"clean", "", "remove leftover cockpit worktrees with no changes", cmdClean},
		{"move", "", "hand the agent to cockpit: move it into the cockpit tmux session", cmdMove},
		{"rename", "[name]", "label the agent in cockpit (empty clears)", cmdRename},
		{"sort", "status|recent|name|cost|location", "change the list order", cmdSort},
		{"project", "[add <name> [dir] | rm <name>]", "list or manage projects", cmdProject},
		{"diff", "", "review the agent's changes in your editor", cmdDiff},
		{"shell", "", "open a shell in the agent's directory", cmdShell},
		{"goto", "", "jump to the agent's own window", cmdGoto},
		{"restore", "", "restore sessions from last time", cmdRestore},
		{"fav", "", "toggle favorite on the selected agent", cmdFav},
		{"favorites", "", "list favorites and resume old ones", cmdFavorites},
		{"closed", "", "list recently closed agents and resume them", cmdClosed},
		{"group", "", "toggle grouping the list by project", cmdGroup},
		{"help", "", "show keys and commands", cmdHelp},
		{"quit", "", "quit cockpit", cmdQuit},
	}
}

func newPrompt() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	s := textinput.DefaultDarkStyles()
	s.Cursor.Color = accent
	ti.SetStyles(s)
	return ti
}

func lookup(name string) (command, error) {
	var found []command
	for _, c := range commands() {
		if c.name == name {
			return c, nil
		}
		if strings.HasPrefix(c.name, name) {
			found = append(found, c)
		}
	}
	switch len(found) {
	case 0:
		return command{}, fmt.Errorf("unknown command: %s", name)
	case 1:
		return found[0], nil
	}
	names := make([]string, len(found))
	for i, c := range found {
		names[i] = c.name
	}
	return command{}, fmt.Errorf("ambiguous command %q: %s", name, strings.Join(names, ", "))
}

func completions(input string) []command {
	if strings.ContainsAny(input, " !") {
		return nil
	}
	var found []command
	for _, c := range commands() {
		if strings.HasPrefix(c.name, input) {
			found = append(found, c)
		}
	}
	return found
}

func (m model) execute(line string) (model, tea.Cmd) {
	line = strings.TrimSpace(line)
	if rest, ok := strings.CutPrefix(line, "!"); ok {
		return cmdRun(m, rest)
	}
	name, args, _ := strings.Cut(line, " ")
	if name == "" {
		return m, nil
	}
	c, err := lookup(name)
	if err != nil {
		m.actionErr = err
		return m, nil
	}
	return c.run(m, strings.TrimSpace(args))
}

func withSelected(m model, run func(a agents.Live) (model, tea.Cmd)) (model, tea.Cmd) {
	a, ok := m.selected()
	if !ok {
		m.actionErr = errors.New("no agent selected")
		return m, nil
	}
	return run(a)
}

func requireArgs(m model, args, usage string) (model, bool) {
	if args == "" {
		m.actionErr = errors.New("usage: " + usage)
		return m, false
	}
	return m, true
}

func cmdNew(m model, args string) (model, tea.Cmd) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return m.openSpawn()
	}
	kinds := installedKinds()
	if len(kinds) == 0 {
		m.actionErr = errors.New("no agent CLI found on PATH")
		return m, nil
	}
	kind := kinds[0]
	if slices.Contains(kinds, fields[0]) {
		kind, fields = fields[0], fields[1:]
	}
	dir, _ := os.Getwd()
	name := ""
	if len(fields) > 0 {
		dir = fields[0]
		if p, ok := projects.Find(m.projects, fields[0]); ok {
			dir, name = paths.Expand(p.Dir), p.Name
			if slices.Contains(kinds, p.Agent) && !slices.Contains(kinds, strings.Fields(args)[0]) {
				kind = p.Agent
			}
		}
	}
	dir, err := resolveDir(dir)
	if err != nil || m.view == nil {
		m.actionErr = cmp.Or(err, errors.New("run cockpit inside tmux to start sessions"))
		return m, nil
	}
	return m, spawnWith(control.Request{Kind: kind, Dir: dir, Name: cmp.Or(name, filepath.Base(dir)), Mode: modes.Code})
}

func cmdSend(m model, args string) (model, tea.Cmd) {
	m, ok := requireArgs(m, args, ":send <text>")
	if !ok {
		return m, nil
	}
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		return m, send(a, args)
	})
}

func cmdBroadcast(m model, args string) (model, tea.Cmd) {
	m, ok := requireArgs(m, args, ":broadcast <text>")
	if !ok {
		return m, nil
	}
	targets := slices.Clone(m.agents)
	return m, func() tea.Msg {
		for _, a := range targets {
			if err := control.Send(a, args); err != nil {
				return sentMsg{err: err}
			}
		}
		return sentMsg{to: fmt.Sprintf("%d agents", len(targets))}
	}
}

func cmdStop(m model, _ string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		m.actionErr = control.Stop(a)
		if m.actionErr == nil {
			m.notice = "✓ interrupted " + name(a)
		}
		return m, nil
	})
}

func cmdKill(m model, _ string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		m.confirm = &confirmation{
			prompt: "kill " + name(a) + "? (y/N)",
			run: func(m model) (model, tea.Cmd) {
				if m.view != nil && m.view.viewing == a.ID {
					_ = m.view.restore()
				}
				return m, tea.Sequence(func() tea.Msg {
					note, err := control.Kill(a)
					return actionMsg{err: err, notice: note}
				}, load)
			},
		}
		return m, nil
	})
}

func cmdRename(m model, args string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		return m.setLabel(a, args)
	})
}

func cmdSort(m model, args string) (model, tea.Cmd) {
	if _, ok := sorters[args]; !ok {
		m.actionErr = errors.New("usage: :sort status|recent|name|cost|location")
		return m, nil
	}
	m.sortBy = args
	m.notice = "✓ sorted by " + args
	return m.refreshView(), nil
}

func popup(dir, title, command string) tea.Cmd {
	return func() tea.Msg { return actionMsg{err: tmux.Popup(dir, title, command)} }
}

func cmdDiff(m model, _ string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		return m, review(a, slices.Clone(m.all))
	})
}

func review(a agents.Live, live []agents.Live) tea.Cmd {
	return func() tea.Msg {
		diff := changes.Diff(a, live)
		if diff == "" {
			return actionMsg{notice: "no changes found for " + name(a)}
		}
		f, err := os.CreateTemp("", "cockpit-*.diff")
		if err != nil {
			return actionMsg{err: err}
		}
		_, err = f.WriteString(diff)
		if err = errors.Join(err, f.Close()); err != nil {
			return actionMsg{err: err}
		}
		file := launch.Quote(f.Name())
		return actionMsg{err: tmux.Popup(a.Pane.Cwd, "diff · "+name(a), diffViewer()+" "+file+"; rm -f "+file)}
	}
}

func diffViewer() string {
	cfg, _ := config.Load()
	return cmp.Or(cfg.Editor.Command, os.Getenv("VISUAL"), os.Getenv("EDITOR"), "less -R")
}

func cmdShell(m model, _ string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		return m, popup(a.Pane.Cwd, "shell · "+name(a), cmp.Or(os.Getenv("SHELL"), "sh"))
	})
}

func cmdRun(m model, args string) (model, tea.Cmd) {
	m, ok := requireArgs(m, args, ":!<command>")
	if !ok {
		return m, nil
	}
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		wrapped := args + "; printf '\\n[press enter to close]'; read _"
		return m, popup(a.Pane.Cwd, args, wrapped)
	})
}

func cmdGoto(m model, _ string) (model, tea.Cmd) {
	return m.goTo(), nil
}

func cmdFav(m model, _ string) (model, tea.Cmd) {
	return m.toggleFavorite()
}

func cmdFavorites(m model, _ string) (model, tea.Cmd) {
	return m.openFavorites(), nil
}

func cmdHelp(m model, _ string) (model, tea.Cmd) {
	m.overlay = "help"
	return m, nil
}

func cmdProject(m model, args string) (model, tea.Cmd) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		m.overlay = "projects"
		return m, nil
	}
	switch {
	case fields[0] == "add" && len(fields) >= 2:
		return addProject(m, fields[1], fields[2:])
	case fields[0] == "rm" && len(fields) == 2:
		m.actionErr = projects.Remove(fields[1])
		if m.actionErr == nil {
			m.notice = "✓ removed project " + fields[1]
		}
		return m, load
	}
	m.actionErr = errors.New("usage: :project [add <name> [dir] | rm <name>]")
	return m, nil
}

func addProject(m model, name string, rest []string) (model, tea.Cmd) {
	dir, _ := os.Getwd()
	if a, ok := m.selected(); ok {
		dir = a.Pane.Cwd
	}
	if len(rest) > 0 {
		dir = rest[0]
	}
	dir, err := resolveDir(dir)
	if err == nil {
		err = projects.Add(projects.Project{Name: name, Dir: paths.Contract(dir)})
	}
	if m.actionErr = err; err == nil {
		m.notice = "✓ project " + name + " → " + paths.Contract(dir)
	}
	return m, load
}

func cmdMove(m model, _ string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		if m.actionErr = m.view.restore(); m.actionErr != nil {
			return m, nil
		}
		n := 0
		for _, f := range m.family(a) {
			moved, err := control.Move(f)
			if m.actionErr = err; err != nil {
				return m, load
			}
			if moved {
				n++
			}
		}
		m.notice = fmt.Sprintf("✓ moved %d agent(s) to the cockpit session", n)
		if n == 0 {
			m.notice = name(a) + " is already in the cockpit session"
		}
		return m, load
	})
}

func (m model) family(a agents.Live) []agents.Live {
	all := []agents.Live{a}
	for _, c := range m.all {
		if c.Parent == a.ID && c.ID != a.ID {
			all = append(all, m.family(c)...)
		}
	}
	return all
}

func cmdQuit(m model, _ string) (model, tea.Cmd) {
	return m, tea.Quit
}

func cmdRestart(m model, _ string) (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		m.confirm = &confirmation{
			prompt: "restart " + name(a) + "? (y/N)",
			run: func(m model) (model, tea.Cmd) {
				m.actionErr = control.Restart(a)
				if m.actionErr == nil {
					m.notice = "✓ restarted " + name(a)
				}
				return m, load
			},
		}
		return m, nil
	})
}

func cmdClean(m model, _ string) (model, tea.Cmd) {
	dirs := make([]string, 0, len(m.projects)+len(m.all))
	for _, p := range m.projects {
		dirs = append(dirs, paths.Expand(p.Dir))
	}
	for _, a := range m.all {
		dirs = append(dirs, a.Pane.Cwd)
	}
	live := slices.Clone(m.all)
	return m, func() tea.Msg {
		removed, kept := control.CleanWorktrees(dirs, live)
		notice := fmt.Sprintf("✓ removed %d worktree(s)", removed)
		if len(kept) > 0 {
			notice += ", kept " + strings.Join(kept, ", ")
		}
		return actionMsg{notice: notice}
	}
}
