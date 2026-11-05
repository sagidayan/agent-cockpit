package permission

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

type Choice int

const (
	Once Choice = iota
	Always
	Deny
)

type Option struct {
	Choice Choice
	Label  string
	Keys   []string
}

type Prompt struct {
	Lines   []string
	Options []Option
}

var (
	errNoPrompt = errors.New("no permission prompt on screen")
	errChanged  = errors.New("the prompt changed, press p again")
)

var parsers = map[string]func([]string) (Prompt, bool){
	"claude":   parseClaude,
	"opencode": parseOpencode,
}

var (
	claudeOption = regexp.MustCompile(`^\s*(?:❯\s*)?(\d+)\.\s+(.+)$`)
	claudeFirst  = regexp.MustCompile(`^\s*(?:❯\s*)?1\.\s`)
	sidebarGap   = regexp.MustCompile(`\s{3,}`)
)

func Supported(kind string) bool {
	_, ok := parsers[kind]
	return ok
}

func Parse(kind, screen string) (Prompt, error) {
	parse, ok := parsers[kind]
	if !ok {
		return Prompt{}, fmt.Errorf("answering prompts isn't supported for %s", kind)
	}
	p, ok := parse(strings.Split(strings.TrimRight(screen, "\n"), "\n"))
	if !ok {
		return Prompt{}, errNoPrompt
	}
	return p, nil
}

func Read(kind, pane string) (Prompt, error) {
	screen, err := tmux.Capture(pane, 0)
	if err != nil {
		return Prompt{}, err
	}
	return Parse(kind, screen)
}

func Answer(kind, pane string, shown Prompt, c Choice) error {
	now, err := Read(kind, pane)
	if err != nil {
		return err
	}
	o, ok := now.Option(c)
	if !ok || !slices.Equal(now.Lines, shown.Lines) {
		return errChanged
	}
	return tmux.SendKeys(pane, o.Keys...)
}

func (p Prompt) Option(c Choice) (Option, bool) {
	i := slices.IndexFunc(p.Options, func(o Option) bool { return o.Choice == c })
	if i < 0 {
		return Option{}, false
	}
	return p.Options[i], true
}

func (p Prompt) complete() bool {
	_, once := p.Option(Once)
	_, deny := p.Option(Deny)
	return once && deny
}

func lastIndex(lines []string, match func(string) bool) int {
	for i, l := range slices.Backward(lines) {
		if match(l) {
			return i
		}
	}
	return -1
}

func parseClaude(lines []string) (Prompt, bool) {
	end := lastIndex(lines, func(l string) bool { return strings.Contains(l, "Esc to cancel") })
	if end < 0 {
		return Prompt{}, false
	}
	start := lastIndex(lines[:end], claudeFirst.MatchString)
	if start < 0 {
		return Prompt{}, false
	}
	top := lastIndex(lines[:start], isRule) + 1
	p := Prompt{Lines: body(lines[top:start]), Options: claudeOptions(lines[start:end])}
	return p, p.complete()
}

func claudeOptions(lines []string) []Option {
	var labels []string
	var keys []string
	for _, l := range lines {
		if m := claudeOption.FindStringSubmatch(l); m != nil {
			labels, keys = append(labels, strings.TrimSpace(m[2])), append(keys, m[1])
		} else if t := strings.TrimSpace(l); t != "" && len(labels) > 0 {
			labels[len(labels)-1] += "\n" + t
		}
	}
	var opts []Option
	for i, label := range labels {
		if c, ok := claudeChoice(label); ok && !slices.ContainsFunc(opts, func(o Option) bool { return o.Choice == c }) {
			opts = append(opts, Option{c, label, []string{keys[i]}})
		}
	}
	return opts
}

func claudeChoice(label string) (Choice, bool) {
	switch {
	case label == "Yes":
		return Once, true
	case strings.HasPrefix(label, "Yes, and"):
		return Always, true
	case label == "No" || strings.HasPrefix(label, "No,"):
		return Deny, true
	}
	return 0, false
}

func isRule(l string) bool {
	t := strings.TrimSpace(l)
	return t != "" && strings.Trim(t, "─") == ""
}

func body(lines []string) []string {
	var out []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" && strings.Trim(t, "╌") != "" {
			out = append(out, t)
		}
	}
	return out
}

var opencodeChoices = []struct {
	choice Choice
	label  string
}{
	{Once, "Allow once"},
	{Always, "Always allow"},
	{Deny, "Reject"},
}

func parseOpencode(lines []string) (Prompt, bool) {
	head := lastIndex(lines, func(l string) bool { return strings.Contains(l, "Permission required") })
	if head < 0 {
		return Prompt{}, false
	}
	rest := lines[head+1:]
	row := slices.IndexFunc(rest, func(l string) bool { return strings.Contains(l, "Allow once") })
	if row < 0 {
		return Prompt{}, false
	}
	var opts []Option
	for _, c := range opencodeChoices {
		if strings.Contains(rest[row], c.label) {
			opts = append(opts, Option{c.choice, c.label, append(slices.Repeat([]string{"Right"}, len(opts)), "Enter")})
		}
	}
	p := Prompt{Lines: body(stripSidebar(rest[:row], indent(lines[head]))), Options: opts}
	return p, p.complete()
}

func indent(l string) int {
	_, after, _ := strings.Cut(l, "┃")
	return len(after) - len(strings.TrimLeft(after, " "))
}

func stripSidebar(lines []string, column int) []string {
	var out []string
	for _, l := range lines {
		if _, after, ok := strings.Cut(l, "┃"); ok && indent(l) <= column {
			out = append(out, sidebarGap.Split(strings.TrimSpace(after), 2)[0])
		}
	}
	return out
}
