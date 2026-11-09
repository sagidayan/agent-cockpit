package modes

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/worktree"
)

const (
	Code   = "code"
	Review = "review"
	Bug    = "bug"
	Chat   = "chat"

	InputDir    = "dir"
	InputURL    = "url"
	InputTicket = "ticket"
	InputNone   = "none"
	InputTemp   = "temp"
)

const reviewPrompt = `Review the pull request {{.URL}}
Use the gh CLI to read the description, diff and comments. Clone the repo into this folder if you need the code.
Report: a short summary, bugs and risks (with file:line), and concrete suggestions.
Do not push, approve or comment on the PR unless I ask.{{if .Task}}

Also: {{.Task}}{{end}}`

const bugPrompt = `Fix the bug described in {{.Ticket}}
Read the ticket first with the matching CLI (jira for a Jira key or URL, gh for a GitHub issue).
Reproduce it, find the root cause, fix it and add a test that fails without the fix.
Do not commit, push or update the ticket unless I ask.{{if .Task}}

Also: {{.Task}}{{end}}`

var builtin = []config.Mode{
	{Name: Code, Title: "Coding", Input: InputDir, Prompt: "{{.Task}}"},
	{Name: Review, Title: "PR review", Input: InputURL, Prompt: reviewPrompt},
	{Name: Bug, Title: "Bug fix", Input: InputTicket, Prompt: bugPrompt},
	{Name: Chat, Title: "Chat", Input: InputTemp, Prompt: "{{.Task}}"},
}

type Inputs struct {
	Task, URL, Ticket string
}

func All(custom []config.Mode) []config.Mode {
	all := slices.Clone(builtin)
	for _, c := range custom {
		if i := slices.IndexFunc(all, func(m config.Mode) bool { return m.Name == c.Name }); i >= 0 {
			all[i] = merge(all[i], c)
		} else {
			all = append(all, merge(config.Mode{Title: c.Name, Input: InputDir}, c))
		}
	}
	return all
}

func merge(base, over config.Mode) config.Mode {
	return config.Mode{
		Name:   cmp.Or(over.Name, base.Name),
		Title:  cmp.Or(over.Title, base.Title),
		Input:  cmp.Or(over.Input, base.Input),
		Agent:  cmp.Or(over.Agent, base.Agent),
		Prompt: cmp.Or(over.Prompt, base.Prompt),
	}
}

func Find(all []config.Mode, name string) (config.Mode, bool) {
	i := slices.IndexFunc(all, func(m config.Mode) bool { return m.Name == name })
	if i < 0 {
		return config.Mode{}, false
	}
	return all[i], true
}

func Prompt(m config.Mode, in Inputs) (string, error) {
	t, err := template.New(m.Name).Parse(m.Prompt)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, in); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func repoAndNumber(raw string) (repo, number string) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", ""
	}
	for i, p := range parts[:len(parts)-1] {
		if p == "pull" || p == "merge_requests" {
			return parts[1], parts[i+1]
		}
	}
	return parts[1], ""
}

func ReviewDir(raw string) string {
	repo, number := repoAndNumber(raw)
	id := strings.Trim(repo+"-"+number, "-")
	return filepath.Join(paths.ReviewRoot, cmp.Or(id, "review"))
}

func ReviewName(raw string) string {
	repo, number := repoAndNumber(raw)
	switch {
	case repo != "" && number != "":
		return "review " + repo + "#" + number
	case repo != "":
		return "review " + repo
	}
	return "review"
}

var ticketKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-\d+$`)

func TicketName(ticket string) string {
	ticket = strings.TrimSpace(ticket)
	if strings.Contains(ticket, "/") && !strings.Contains(ticket, "://") {
		ticket = "https://" + ticket
	}
	u, err := url.Parse(ticket)
	if err != nil || u.Host == "" {
		return strings.TrimSpace("bug " + ticket)
	}
	parts := slices.DeleteFunc(strings.Split(u.Path, "/"), func(p string) bool { return p == "" || p == "-" })
	if i := slices.Index(parts, "issues"); i > 0 && i+1 < len(parts) {
		return "bug " + parts[i-1] + "#" + parts[i+1]
	}
	key := u.Query().Get("selectedIssue")
	if i := slices.IndexFunc(parts, ticketKey.MatchString); i >= 0 {
		key = cmp.Or(key, parts[i])
	}
	if key == "" && len(parts) > 0 {
		key = parts[len(parts)-1]
	}
	return strings.TrimSpace("bug " + key)
}

type Spec struct {
	Kind, Dir, Name, Label, Task, URL, Ticket string
	Worktree                                  bool
}

func TakesDir(m config.Mode) bool {
	return !slices.Contains([]string{InputURL, InputNone, InputTemp}, m.Input)
}

func Build(m config.Mode, s Spec) (control.Request, error) {
	if s.Worktree && !TakesDir(m) {
		return control.Request{}, fmt.Errorf("a worktree needs a mode that works in a folder, not %s", m.Name)
	}
	prompt, err := Prompt(m, Inputs{Task: s.Task, URL: s.URL, Ticket: s.Ticket})
	if err != nil {
		return control.Request{}, err
	}
	dir, name := s.Dir, s.Name
	switch m.Input {
	case InputURL:
		if s.URL == "" {
			return control.Request{}, errors.New("a PR URL is required")
		}
		dir, name = cmp.Or(dir, ReviewDir(s.URL)), cmp.Or(name, ReviewName(s.URL))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return control.Request{}, err
		}
	case InputTemp:
		if dir == "" {
			if dir, err = paths.NewChat(); err != nil {
				return control.Request{}, err
			}
		}
		name = cmp.Or(name, m.Name)
	case InputNone:
		home, _ := os.UserHomeDir()
		dir, name = cmp.Or(dir, home), cmp.Or(name, m.Name)
	case InputTicket:
		if s.Ticket == "" {
			return control.Request{}, errors.New("a ticket is required")
		}
		name = cmp.Or(name, TicketName(s.Ticket))
		fallthrough
	default:
		if dir == "" {
			return control.Request{}, errors.New("a directory is required")
		}
		if s.Worktree {
			if dir, err = worktree.Create(dir, cmp.Or(s.Label, name, filepath.Base(dir))); err != nil {
				return control.Request{}, err
			}
		}
	}
	return control.Request{Kind: cmp.Or(s.Kind, m.Agent), Dir: dir, Name: name, Label: s.Label, Prompt: prompt, Mode: m.Name}, nil
}
