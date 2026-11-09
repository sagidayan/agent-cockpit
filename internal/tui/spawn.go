package tui

import (
	"cmp"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/worktree"
)

type spawnStep int

const (
	stepMode spawnStep = iota
	stepKind
	stepProject
	stepDir
	stepURL
	stepTicket
	stepWorktree
	stepDetails
)

type suggestion struct{ label, hint, value string }

type spawnForm struct {
	modes      []config.Mode
	mode       int
	kinds      []string
	kind       int
	kindPicked bool
	projects   []projects.Project
	project    string
	repo       bool
	worktree   bool
	filter     textinput.Model
	dir        textinput.Model
	url        textinput.Model
	ticket     textinput.Model
	task       textinput.Model
	name       textinput.Model
	onName     bool
	step       spawnStep
	matches    []suggestion
	pick       int
	err        string
}

type spawnedMsg struct {
	pane, name string
	err        error
}

func spawnWith(r control.Request) tea.Cmd {
	return func() tea.Msg {
		session, err := control.Session()
		if err != nil {
			return spawnedMsg{err: err}
		}
		pane, err := control.Spawn(session, r)
		return spawnedMsg{pane, cmp.Or(r.Label, r.Name), err}
	}
}

func installedKinds() []string {
	var kinds []string
	for _, k := range []string{"claude", "codex", "opencode"} {
		if _, err := exec.LookPath(k); err == nil {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

func newSpawnForm(dir string, list []projects.Project, width int) spawnForm {
	cfg, _ := config.Load()
	f := spawnForm{
		modes: modes.All(cfg.Modes), kinds: installedKinds(), projects: list,
		filter: newField(width), dir: newField(width), url: newField(width), ticket: newField(width), task: newField(width), name: newField(width),
	}
	f.filter.Placeholder = "type to filter"
	f.url.Placeholder = "https://github.com/org/repo/pull/123"
	f.ticket.Placeholder = "SP-1234 or an issue URL"
	f.dir.SetValue(paths.Contract(dir) + "/")
	f.dir.CursorEnd()
	return f.applyMode()
}

func newField(width int) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetWidth(max(width-14, 10))
	s := textinput.DefaultDarkStyles()
	s.Focused.Placeholder = mutedStyle
	s.Blurred.Placeholder = mutedStyle
	s.Cursor.Color = accent
	ti.SetStyles(s)
	return ti
}

func (f spawnForm) currentMode() config.Mode {
	return f.modes[f.mode]
}

func (f spawnForm) steps() []spawnStep {
	steps := []spawnStep{stepMode}
	if len(f.kinds) > 1 {
		steps = append(steps, stepKind)
	}
	switch f.currentMode().Input {
	case modes.InputURL:
		steps = append(steps, stepURL)
	case modes.InputNone, modes.InputTemp:
	case modes.InputTicket:
		steps = append(append(steps, stepTicket), f.folderSteps()...)
	default:
		steps = append(steps, f.folderSteps()...)
	}
	return append(steps, stepDetails)
}

func (f spawnForm) folderSteps() []spawnStep {
	var steps []spawnStep
	if len(f.projects) > 0 {
		steps = append(steps, stepProject)
	}
	if f.project == "" {
		steps = append(steps, stepDir)
	}
	if f.repo {
		steps = append(steps, stepWorktree)
	}
	return steps
}

func (f spawnForm) first() bool {
	return f.step == stepMode
}

func (f spawnForm) last() bool {
	return f.step == stepDetails
}

func (f spawnForm) dismissible() bool {
	return f.step == stepDir && len(f.matches) > 0
}

func (f spawnForm) dismiss() spawnForm {
	f.matches = nil
	return f
}

func (f spawnForm) applyMode() spawnForm {
	if i := slices.Index(f.kinds, f.currentMode().Agent); i >= 0 {
		f.kind = i
	}
	return f
}

func (f spawnForm) next() (spawnForm, tea.Cmd) {
	f.err = ""
	switch f.step {
	case stepKind:
		f.kindPicked = true
	case stepProject:
		f = f.applyProject(f.matches[f.pick].value)
		f.repo = worktree.InRepo(paths.Expand(f.dir.Value()))
	case stepDir:
		dir, err := resolveDir(f.dir.Value())
		if err != nil {
			f.err = err.Error()
			return f, nil
		}
		f.repo = worktree.InRepo(dir)
	case stepURL:
		if strings.TrimSpace(f.url.Value()) == "" {
			f.err = "a PR URL is required"
			return f, nil
		}
	case stepTicket:
		if strings.TrimSpace(f.ticket.Value()) == "" {
			f.err = "a ticket is required"
			return f, nil
		}
	}
	steps := f.steps()
	return f.enter(steps[min(slices.Index(steps, f.step)+1, len(steps)-1)])
}

func (f spawnForm) back() (spawnForm, tea.Cmd) {
	f.err = ""
	steps := f.steps()
	return f.enter(steps[max(slices.Index(steps, f.step)-1, 0)])
}

func (f spawnForm) applyProject(name string) spawnForm {
	f.project = ""
	p, ok := projects.Find(f.projects, name)
	if !ok {
		return f
	}
	f.project = p.Name
	f.dir.SetValue(p.Dir)
	if i := slices.Index(f.kinds, p.Agent); i >= 0 && !f.kindPicked && f.currentMode().Agent == "" {
		f.kind = i
	}
	return f
}

func (f spawnForm) enter(step spawnStep) (spawnForm, tea.Cmd) {
	f.step, f.onName = step, false
	if step == stepDetails {
		f.name.Placeholder = "defaults to " + f.defaultName()
		f.task.Placeholder = cmp.Or(taskPlaceholders[f.currentMode().Input], taskPlaceholders[modes.InputDir])
	}
	return f.focusInput()
}

var taskPlaceholders = map[string]string{
	modes.InputDir:    "optional - sent as the first prompt",
	modes.InputURL:    "optional - extra review instructions",
	modes.InputTicket: "optional - hints, e.g. where to look",
	modes.InputNone:   "optional - otherwise it waits for you",
	modes.InputTemp:   "optional - or ask in the chat",
}

func (f spawnForm) focusInput() (spawnForm, tea.Cmd) {
	for _, in := range []*textinput.Model{&f.filter, &f.dir, &f.url, &f.ticket, &f.task, &f.name} {
		in.Blur()
	}
	f = f.suggest()
	if in := f.input(); in != nil {
		return f, in.Focus()
	}
	return f, nil
}

func (f spawnForm) defaultName() string {
	m := f.currentMode()
	switch m.Input {
	case modes.InputURL:
		return modes.ReviewName(strings.TrimSpace(f.url.Value()))
	case modes.InputTicket:
		return modes.TicketName(strings.TrimSpace(f.ticket.Value()))
	case modes.InputNone, modes.InputTemp:
		return m.Name
	}
	return cmp.Or(f.project, filepath.Base(filepath.Clean(paths.Expand(f.dir.Value()))))
}

func (f *spawnForm) input() *textinput.Model {
	switch f.step {
	case stepProject:
		return &f.filter
	case stepDir:
		return &f.dir
	case stepURL:
		return &f.url
	case stepTicket:
		return &f.ticket
	case stepDetails:
		if f.onName {
			return &f.name
		}
		return &f.task
	}
	return nil
}

func (f spawnForm) suggest() spawnForm {
	f.pick = 0
	switch f.step {
	case stepProject:
		f.matches = append(matchProjects(f.projects, f.filter.Value()), suggestion{"other folder…", "pick any directory", ""})
	case stepDir:
		f.matches = nil
		for _, d := range matchDirs(f.dir.Value()) {
			f.matches = append(f.matches, suggestion{filepath.Base(d) + "/", "", d})
		}
	default:
		f.matches = nil
	}
	return f
}

func (f spawnForm) update(msg tea.KeyPressMsg) (spawnForm, tea.Cmd) {
	f.err = ""
	key := msg.String()
	switch f.step {
	case stepMode:
		if i, ok := move(f.mode, len(f.modes), key); ok {
			f.mode = i
			return f.applyMode(), nil
		}
		return f, nil
	case stepKind:
		if i, ok := move(f.kind, len(f.kinds), key); ok {
			f.kind = i
		}
		return f, nil
	case stepWorktree:
		if _, ok := move(0, 2, key); ok {
			f.worktree = !f.worktree
		}
		return f, nil
	case stepDetails:
		if key == "tab" || key == "shift+tab" || key == "up" || key == "down" {
			f.onName = !f.onName
			return f.focusInput()
		}
	case stepProject:
		if key == "up" || key == "down" {
			f.pick, _ = move(f.pick, len(f.matches), key)
			return f, nil
		}
	case stepDir:
		switch {
		case (key == "up" || key == "down") && len(f.matches) > 0:
			f.pick, _ = move(f.pick, len(f.matches), key)
			return f, nil
		case key == "right" && len(f.matches) > 0 && f.atEnd():
			return f.accept(), nil
		case key == "left" && f.atEnd():
			return f.parentDir(), nil
		}
	}
	return f.updateInput(msg)
}

func move(i, n int, key string) (int, bool) {
	switch key {
	case "up", "k":
		return (i + n - 1) % n, true
	case "down", "j":
		return (i + 1) % n, true
	}
	return i, false
}

func (f spawnForm) accept() spawnForm {
	f.dir.SetValue(f.matches[f.pick].value)
	f.dir.CursorEnd()
	return f.suggest()
}

func (f spawnForm) atEnd() bool {
	in := f.input()
	return in != nil && in.Position() == len([]rune(in.Value()))
}

func (f spawnForm) parentDir() spawnForm {
	trimmed := strings.TrimSuffix(f.dir.Value(), "/")
	i := strings.LastIndex(trimmed, "/")
	if i < 0 {
		return f
	}
	f.dir.SetValue(trimmed[:i+1])
	f.dir.CursorEnd()
	return f.suggest()
}

func (f spawnForm) updateInput(msg tea.Msg) (spawnForm, tea.Cmd) {
	in := f.input()
	if in == nil {
		return f, nil
	}
	before := in.Value()
	var cmd tea.Cmd
	*in, cmd = in.Update(msg)
	if in.Value() != before {
		f = f.suggest()
	}
	return f, cmd
}

func (f spawnForm) request() (control.Request, error) {
	if len(f.kinds) == 0 {
		return control.Request{}, errors.New("no agent CLI found on PATH")
	}
	m := f.currentMode()
	spec := modes.Spec{
		Kind:   f.kinds[f.kind],
		Name:   strings.TrimSpace(f.name.Value()),
		Task:   strings.TrimSpace(f.task.Value()),
		URL:    strings.TrimSpace(f.url.Value()),
		Ticket: strings.TrimSpace(f.ticket.Value()),
	}
	if modes.TakesDir(m) {
		dir, err := resolveDir(f.dir.Value())
		if err != nil {
			return control.Request{}, err
		}
		spec.Dir, spec.Name, spec.Worktree = dir, cmp.Or(spec.Name, f.defaultName()), f.repo && f.worktree
	}
	return modes.Build(m, spec)
}

func resolveDir(input string) (string, error) {
	dir := filepath.Clean(paths.Expand(strings.TrimSpace(input)))
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errors.New("not a directory: " + dir)
	}
	return dir, nil
}

func matchProjects(list []projects.Project, query string) []suggestion {
	var found []suggestion
	for _, p := range list {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(strings.TrimSpace(query))) {
			found = append(found, suggestion{p.Name, p.Dir, p.Name})
		}
	}
	return found
}

func matchDirs(input string) []string {
	parent := input[:strings.LastIndex(input, "/")+1]
	if parent == "" {
		return nil
	}
	entries, err := os.ReadDir(paths.Expand(parent))
	if err != nil {
		return nil
	}
	prefix := strings.ToLower(input[len(parent):])
	var dirs []string
	for _, e := range entries {
		name := e.Name()
		hidden := strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".")
		if e.IsDir() && !hidden && strings.HasPrefix(strings.ToLower(name), prefix) {
			dirs = append(dirs, parent+name+"/")
		}
	}
	return dirs
}
