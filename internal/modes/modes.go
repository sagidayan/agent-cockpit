package modes

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	Master = "master"
	Chat   = "chat"

	InputDir  = "dir"
	InputURL  = "url"
	InputNone = "none"
	InputTemp = "temp"
)

const reviewPrompt = `Review the pull request {{.URL}}
Use the gh CLI to read the description, diff and comments. Clone the repo into this folder if you need the code.
Report: a short summary, bugs and risks (with file:line), and concrete suggestions.
Do not push, approve or comment on the PR unless I ask.{{if .Task}}

Also: {{.Task}}{{end}}`

const masterPrompt = `You are a master agent: you coordinate other coding agents and don't edit code yourself.
Use the cockpit MCP tools: list_projects to see the folders you can work in, spawn_agent to start a child agent in a folder with a focused task, wait_for_agent, get_output and get_diff to follow up and review, and send_task to give feedback.
Split the work across folders, run children in parallel when you can, and report back to me with a summary.
When several children work in the same repo at once, pass worktree: true to spawn_agent so each gets its own git worktree on a new cockpit/<label> branch, and name the branches in your report.
For a large chunk that spans several folders, spawn a sub-master with spawn_agent mode "master"; it runs its own children and reports back to you.
When you decide a child's work is done and reviewed, close it with close_agent. Close all your children before you report back.{{if .Task}}

Task: {{.Task}}{{else}}

Wait for my instructions.{{end}}`

var builtin = []config.Mode{
	{Name: Code, Title: "Coding", Input: InputDir, Prompt: "{{.Task}}"},
	{Name: Review, Title: "PR review", Input: InputURL, Prompt: reviewPrompt},
	{Name: Master, Title: "Master", Input: InputNone, Prompt: masterPrompt},
	{Name: Chat, Title: "Chat", Input: InputTemp, Prompt: "{{.Task}}"},
}

type Inputs struct {
	Task, URL string
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

const reviewRoot = "/tmp/prs"

func ReviewDir(raw string) string {
	repo, number := repoAndNumber(raw)
	id := strings.Trim(repo+"-"+number, "-")
	return filepath.Join(reviewRoot, cmp.Or(id, "review"))
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

type Spec struct {
	Kind, Dir, Name, Label, Task, URL string
	Worktree                          bool
}

func TakesDir(m config.Mode) bool {
	return !slices.Contains([]string{InputURL, InputNone, InputTemp}, m.Input)
}

func Build(m config.Mode, s Spec) (control.Request, error) {
	if s.Worktree && !TakesDir(m) {
		return control.Request{}, fmt.Errorf("a worktree needs a mode that works in a folder, not %s", m.Name)
	}
	prompt, err := Prompt(m, Inputs{Task: s.Task, URL: s.URL})
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
