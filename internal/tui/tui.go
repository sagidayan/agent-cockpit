package tui

import (
	"cmp"
	"errors"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/favorites"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

type tickMsg time.Time

type frameMsg time.Time

const (
	frameRate  = 120 * time.Millisecond
	flashFor   = 2 * time.Second
	noticeFor  = 4 * time.Second
	noticeFade = time.Second
)

type agentsMsg struct {
	agents    []agents.Live
	projects  []projects.Project
	favorites []snapshot.Entry
	err       error
}

type actionMsg struct {
	err    error
	notice string
}

type sentMsg struct {
	to  string
	err error
}

type model struct {
	all            []agents.Live
	agents         []agents.Live
	filter         string
	sortBy         string
	order          []string
	chord          string
	prompt         rune
	line           textinput.Model
	confirm        *confirmation
	question       *question
	overlay        string
	projects       []projects.Project
	blurred        bool
	frame          int
	changed        map[string]time.Time
	noticed        string
	noticeAt       time.Time
	saved          []snapshot.Entry
	restorable     []snapshot.Entry
	restoreChecked bool
	clickedAt      time.Time
	favorites      []snapshot.Entry
	favCursor      int
	closed         []state.Closed
	closedCursor   int
	grouped        bool
	depth          map[string]int
	expanded       map[string]bool
	cursor         int
	loadErr        error
	actionErr      error
	view           *viewer
	detail         detail
	sending        bool
	input          textarea.Model
	spawning       bool
	form           spawnForm
	pending        string
	notice         string
	width          int
	height         int
}

func Run() error {
	if cfg, err := config.Load(); err == nil && !cfg.UI.Icons {
		icons = plainIcons
	}
	saved, err := snapshot.Load()
	if err != nil {
		return err
	}
	v := newViewer()
	final, err := tea.NewProgram(model{view: v, input: newComposer(), line: newPrompt(), changed: map[string]time.Time{}, expanded: map[string]bool{}, saved: saved, order: state.LoadOrder(), grouped: state.LoadGrouped()}).Run()
	v.close()
	if m, ok := final.(model); ok && err == nil {
		return saveSnapshot(m)
	}
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(load, tick(), frame())
}

func frame() tea.Cmd {
	return tea.Tick(frameRate, func(t time.Time) tea.Msg { return frameMsg(t) })
}

func (m model) animate(now time.Time) model {
	m.frame++
	if m.notice != m.noticed {
		m.noticed, m.noticeAt = m.notice, now
	}
	if m.notice != "" && now.Sub(m.noticeAt) > noticeFor {
		m.notice, m.noticed = "", ""
	}
	for id, at := range m.changed {
		if now.Sub(at) > flashFor {
			delete(m.changed, id)
		}
	}
	return m
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func load() tea.Msg {
	a, err := agents.List()
	if err != nil {
		return agentsMsg{err: err}
	}
	agents.MarkMCP(a)
	p, err := projects.Load()
	if err != nil {
		return agentsMsg{err: err}
	}
	if err := favorites.Sync(a); err != nil {
		return agentsMsg{err: err}
	}
	f, err := favorites.Load()
	return agentsMsg{a, p, f, err}
}

func send(a agents.Live, text string) tea.Cmd {
	return func() tea.Msg {
		return sentMsg{to: name(a), err: control.Send(a, text)}
	}
}

func (m model) selected() (agents.Live, bool) {
	if m.cursor < 0 || m.cursor >= len(m.agents) {
		return agents.Live{}, false
	}
	return m.agents[m.cursor], true
}

func (m model) showSelected() model {
	if a, ok := m.selected(); ok {
		m.actionErr = m.view.show(a.Pane)
	}
	return m
}

func (m model) goTo() model {
	a, ok := m.selected()
	if !ok {
		return m
	}
	if m.view == nil {
		m.actionErr = tmux.Focus(a.ID)
		return m
	}
	m.actionErr = m.view.goTo(a.Pane)
	return m
}

func (m model) interact() model {
	if m.view == nil {
		return m.goTo()
	}
	m.actionErr = m.view.focus()
	m.blurred = m.actionErr == nil
	return m
}

func (m model) setAgents(live []agents.Live) model {
	viewed := false
	for i := range live {
		if m.view != nil && live[i].ID == m.view.viewing {
			home := m.view.home
			home.Label = live[i].Label
			live[i].Pane = home
			viewed = true
		}
	}
	if !viewed && m.view != nil && m.view.viewing != "" {
		m.actionErr = m.view.restore()
	}
	m.order = arrange(m.order, live)
	byOrder(live, m.order)
	m.markChanged(live)
	m.all = live
	return m.checkRestore(live).refreshView()
}

func (m model) markChanged(live []agents.Live) {
	prev := make(map[string]state.Status, len(m.all))
	for _, a := range m.all {
		prev[a.ID] = a.Status
	}
	for _, a := range live {
		if s, ok := prev[a.ID]; ok && s != a.Status {
			m.changed[a.ID] = time.Now()
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(m.width-4, 10))
	case tickMsg:
		return m, tea.Batch(load, tick(), m.reloadDetail())
	case frameMsg:
		return m.animate(time.Time(msg)), frame()
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.FocusMsg:
		m.blurred = false
		m.view.giveBack()
	case tea.BlurMsg:
		m.blurred = true
	case agentsMsg:
		if m.loadErr = msg.err; msg.err == nil {
			m.projects, m.favorites = msg.projects, msg.favorites
			m = m.setAgents(msg.agents)
		}
	case actionMsg:
		m.actionErr = msg.err
		if msg.notice != "" {
			m.notice = msg.notice
		}
	case sentMsg:
		m.actionErr = msg.err
		if msg.err == nil {
			m.notice = "✓ sent to " + msg.to
		}
	case restoredMsg:
		m.actionErr = msg.err
		if msg.n > 0 {
			m.notice = restoredNotice(msg.n)
		}
		return m, load
	case detailMsg:
		m = m.setDetail(msg)
	case spawnedMsg:
		m.actionErr = msg.err
		if msg.err == nil {
			m.pending, m.notice = msg.pane, "✓ started "+msg.name
		}
	case tea.KeyPressMsg:
		m.notice = ""
		if m.confirm != nil {
			return m.updateConfirm(msg)
		}
		if m.question != nil {
			return m.updateQuestion(msg)
		}
		if m.overlay == "favorites" {
			return m.updateFavorites(msg)
		}
		if m.overlay == "closed" {
			return m.updateClosed(msg)
		}
		if m.overlay != "" {
			m.overlay = ""
			return m, nil
		}
		if m.prompt != 0 {
			return m.updatePrompt(msg)
		}
		if m.spawning {
			return m.updateSpawn(msg)
		}
		if m.sending {
			return m.updateInput(msg)
		}
		if m.detail.open {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}
	if m.prompt != 0 {
		var cmd tea.Cmd
		m.line, cmd = m.line.Update(msg)
		return m, cmd
	}
	if m.spawning {
		var cmd tea.Cmd
		m.form, cmd = m.form.updateInput(msg)
		return m, cmd
	}
	if m.sending {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	a, ok := m.selected()
	key := m.chord + msg.String()
	m.chord = ""
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m.cursor = min(m.cursor+1, max(0, len(m.agents)-1))
		return m.showSelected(), nil
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
		return m.showSelected(), nil
	case "G", "end":
		m.cursor = max(0, len(m.agents)-1)
		return m.showSelected(), nil
	case "gg", "home":
		m.cursor = 0
		return m.showSelected(), nil
	case "J", "pgdown":
		return m.shift(1)
	case "K", "pgup":
		return m.shift(-1)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		if i := int(msg.String()[0] - '1'); i < len(m.agents) {
			m.cursor = i
		}
		return m.showSelected(), nil
	case "/", ":":
		return m.openPrompt(rune(msg.String()[0]))
	case "r":
		if ok {
			return m.openPrompt('r')
		}
	case "?":
		m.overlay = "help"
	case "R":
		return m.restoreAll()
	case "f":
		return m.toggleFavorite()
	case "F":
		return m.openFavorites(), nil
	case "x":
		m.restorable = nil
	case "esc":
		m.filter = ""
		return m.refreshView(), nil
	case "enter", "right", "l":
		return m.interact(), nil
	case "i":
		return m.openDetail()
	case "g":
		m.chord = "g"
	case "gt":
		return m.goTo(), nil
	case "space":
		return m.toggleExpanded(), nil
	case "d":
		return cmdDiff(m, "")
	case "m":
		return cmdMove(m, "")
	case "s":
		if ok {
			m.sending = true
			return m, m.input.Focus()
		}
	case "n":
		return m.openSpawn()
	case "p":
		if ok && canAnswer(a) {
			return m.openQuestion()
		}
	}
	return m, nil
}

func (m model) openPrompt(kind rune) (tea.Model, tea.Cmd) {
	m.prompt = kind
	m.line.Reset()
	m.line.SetWidth(max(m.width-4, 10))
	switch kind {
	case '/':
		m.line.SetValue(m.filter)
	case 'r':
		a, _ := m.selected()
		m.line.SetValue(a.Label)
	}
	m.line.CursorEnd()
	return m, m.line.Focus()
}

func (m model) updatePrompt(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.prompt == '/' {
			m.filter = ""
			m = m.refreshView()
		}
		return m.closePrompt(), nil
	case "enter":
		kind, value := m.prompt, m.line.Value()
		m = m.closePrompt()
		switch kind {
		case ':':
			return m.execute(value)
		case 'r':
			if a, ok := m.selected(); ok {
				return m.setLabel(a, value)
			}
		}
		return m, nil
	case "tab":
		if c := completions(m.line.Value()); m.prompt == ':' && len(c) > 0 {
			m.line.SetValue(c[0].name + " ")
			m.line.CursorEnd()
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.line, cmd = m.line.Update(msg)
	if m.prompt == '/' {
		m.filter = m.line.Value()
		m = m.refreshView()
	}
	return m, cmd
}

func (m model) setLabel(a agents.Live, label string) (model, tea.Cmd) {
	label = strings.Join(strings.Fields(label), " ")
	if m.actionErr = control.SetLabel(a, label); m.actionErr == nil {
		m.notice = "✓ labeled " + cmp.Or(label, a.WindowName)
	}
	return m, load
}

func (m model) closePrompt() model {
	m.prompt = 0
	m.line.Blur()
	return m
}

func (m model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	c := m.confirm
	m.confirm = nil
	if msg.String() == "y" {
		return c.run(m)
	}
	return m, nil
}

func (m model) openSpawn() (model, tea.Cmd) {
	dir, _ := os.Getwd()
	if a, ok := m.selected(); ok {
		dir = a.Pane.Cwd
	}
	m.form = newSpawnForm(dir, m.projects, m.width)
	m.spawning = true
	return m, nil
}

func (m model) updateSpawn(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		switch {
		case m.form.dismissible():
			m.form = m.form.dismiss()
		case m.form.first():
			m.spawning = false
		default:
			var cmd tea.Cmd
			m.form, cmd = m.form.back()
			return m, cmd
		}
		return m, nil
	case "enter":
		if m.form.last() {
			return m.startSpawn()
		}
		var cmd tea.Cmd
		m.form, cmd = m.form.next()
		return m, cmd
	}
	var cmd tea.Cmd
	m.form, cmd = m.form.update(msg)
	return m, cmd
}

func (m model) startSpawn() (tea.Model, tea.Cmd) {
	if m.view == nil {
		m.actionErr = errors.New("run cockpit inside tmux to start sessions")
		return m, nil
	}
	req, err := m.form.request()
	if m.actionErr = err; err != nil {
		return m, nil
	}
	m.spawning = false
	return m, spawnWith(req)
}

func (m model) updateInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m.closeInput(), nil
	case "enter":
		a, ok := m.selected()
		text := strings.TrimSpace(m.input.Value())
		m = m.closeInput()
		if !ok || text == "" {
			return m, nil
		}
		return m, send(a, text)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) closeInput() model {
	m.sending = false
	m.input.Reset()
	m.input.Blur()
	return m
}
