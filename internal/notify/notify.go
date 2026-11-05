package notify

import (
	"cmp"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gen2brain/beeep"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

const maxBody = 160

var agentNames = map[string]string{"claude": "Claude", "codex": "Codex", "opencode": "OpenCode"}

var headlines = map[state.Status]string{
	state.Done:       "✅ %s finished",
	state.NeedsInput: "🙋 %s needs you",
	state.Working:    "⏳ %s is working",
	state.Background: "◷ %s is waiting on background work",
	state.Idle:       "💤 %s is idle",
}

var markdown = regexp.MustCompile("(?m)\\*\\*|__|`|^\\s*#+\\s*|^\\s*[-*]\\s+")

type note struct {
	who, title, body string
}

func Wanted(cfg config.Notify, prev, cur state.Agent) bool {
	changed := prev.Status != cur.Status || prev.Message != cur.Message
	return changed && slices.Contains(cfg.Events, cur.Status)
}

func Record(pane string, change func(prev state.Agent) (state.Agent, bool)) error {
	prev, cur, saved, err := state.Update(pane, change)
	if err != nil || !saved {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !Wanted(cfg.Notify, prev, cur) {
		return nil
	}
	return Send(cfg.Notify, cur)
}

func Send(cfg config.Notify, a state.Agent) error {
	clients, err := tmux.Clients()
	if err != nil {
		return err
	}
	if watched(clients, a.Pane) {
		return nil
	}
	n := compose(a, describe(a))
	var errs []error
	if cfg.Desktop {
		errs = append(errs, desktop(a.Status, n))
	}
	if cfg.Tmux {
		for _, c := range clients {
			errs = append(errs, tmux.Display(c.Name, n.title+" · "+firstLine(n.body)))
		}
	}
	if cfg.Command != "" {
		errs = append(errs, command(cfg.Command, a, n))
	}
	return errors.Join(errs...)
}

func desktop(s state.Status, n note) error {
	beeep.AppName = "agent-cockpit"
	if s == state.NeedsInput {
		return beeep.Alert(n.title, n.body, "")
	}
	return beeep.Notify(n.title, n.body, "")
}

func watched(clients []tmux.Client, pane string) bool {
	return slices.ContainsFunc(clients, func(c tmux.Client) bool { return c.Pane == pane })
}

type identity struct {
	label, window, project string
}

func describe(a state.Agent) identity {
	label, window, _ := tmux.Names(a.Pane)
	list, _ := projects.Load()
	p, _ := projects.ForDir(list, a.Cwd)
	return identity{label, window, p.Name}
}

func compose(a state.Agent, id identity) note {
	who := friendlyName(a, id)
	title := strings.Replace(cmp.Or(headlines[a.Status], "%s: "+string(a.Status)), "%s", who, 1)
	context := cmp.Or(agentNames[a.Kind], a.Kind)
	if place := cmp.Or(id.project, filepath.Base(a.Cwd)); place != "" && place != who && place != "." {
		context += " · " + place
	}
	body := context
	if msg := summary(cmp.Or(a.Message, a.Prompt)); msg != "" {
		body = msg + "\n" + context
	}
	return note{who, title, body}
}

func friendlyName(a state.Agent, id identity) string {
	generic := id.window == "" || id.window == a.Kind || slices.Contains([]string{"node", "zsh", "bash", "sh", "fish"}, id.window)
	window := id.window
	if generic {
		window = ""
	}
	return cmp.Or(id.label, id.project, window, filepath.Base(a.Cwd), agentNames[a.Kind], "Your agent")
}

func summary(msg string) string {
	text := strings.Join(strings.Fields(markdown.ReplaceAllString(msg, "")), " ")
	if len([]rune(text)) <= maxBody {
		return text
	}
	cut := string([]rune(text)[:maxBody])
	if i := strings.LastIndex(cut, " "); i > maxBody/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, ",.;: ") + "…"
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func command(script string, a state.Agent, n note) error {
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(),
		"COCKPIT_PANE="+a.Pane,
		"COCKPIT_KIND="+a.Kind,
		"COCKPIT_STATUS="+string(a.Status),
		"COCKPIT_MESSAGE="+a.Message,
		"COCKPIT_NAME="+n.who,
		"COCKPIT_TITLE="+n.title,
		"COCKPIT_BODY="+n.body,
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
