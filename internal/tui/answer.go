package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/permission"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

type question struct {
	agent  agents.Live
	prompt permission.Prompt
}

var answerKeys = []struct {
	key    string
	choice permission.Choice
	hint   string
	done   string
}{
	{"y", permission.Once, "approve", "approved"},
	{"a", permission.Always, "always", "always approved"},
	{"n", permission.Deny, "deny", "denied"},
}

func canAnswer(a agents.Live) bool {
	return a.Status == state.NeedsInput
}

func (m model) openQuestion() (model, tea.Cmd) {
	return withSelected(m, func(a agents.Live) (model, tea.Cmd) {
		if !permission.Supported(a.Kind) {
			m.notice = "answering prompts isn't supported for " + a.Kind
			return m, nil
		}
		p, err := permission.Read(a.Kind, a.ID)
		if m.actionErr = err; err == nil {
			m.question = &question{a, p}
		}
		return m, nil
	})
}

func (m model) updateQuestion(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	q := m.question
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.question = nil
		return m, nil
	}
	for _, k := range answerKeys {
		if _, ok := q.prompt.Option(k.choice); ok && msg.String() == k.key {
			m.question = nil
			if m.actionErr = permission.Answer(q.agent.Kind, q.agent.ID, q.prompt, k.choice); m.actionErr != nil {
				return m, nil
			}
			m.notice = "✓ " + k.done + " " + name(q.agent)
			return m, load
		}
	}
	return m, nil
}

func (m model) questionView() string {
	q := m.question
	rows := []string{" " + titleStyle.Render(ansi.Truncate(name(q.agent)+" asks", m.width-1, "…")), ""}
	for _, l := range q.prompt.Lines {
		rows = append(rows, m.indented(l)...)
	}
	rows = append(rows, "")
	for _, k := range answerKeys {
		if o, ok := q.prompt.Option(k.choice); ok {
			rows = append(rows, m.indented(keyStyle.Render(k.key)+" "+mutedStyle.Render(o.Label))...)
		}
	}
	return strings.Join(append(rows, "   "+keyStyle.Render("esc")+" "+mutedStyle.Render("close")), "\n")
}

func (m model) indented(s string) []string {
	var lines []string
	for l := range strings.SplitSeq(ansi.Wrap(s, max(m.width-4, 10), ""), "\n") {
		lines = append(lines, "   "+l)
	}
	return lines
}

func (m model) questionKeys() [][2]string {
	var keys [][2]string
	for _, k := range answerKeys {
		if _, ok := m.question.prompt.Option(k.choice); ok {
			keys = append(keys, [2]string{k.key, k.hint})
		}
	}
	return append(keys, [2]string{"esc", "close"})
}
