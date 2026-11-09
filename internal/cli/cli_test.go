package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/modes"
	"github.com/sagidayan/agent-cockpit/internal/projects"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestRequest(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	list := []projects.Project{{Name: "api", Dir: "/opt/api", Agent: "codex"}, {Name: "web", Dir: "/opt/web"}}
	all := modes.All([]config.Mode{
		{Name: modes.Review, Prompt: "R {{.URL}}"},
		{Name: modes.Bug, Prompt: "B {{.Ticket}}"},
		{Name: "standup", Input: modes.InputNone, Prompt: "S {{.Task}}"},
		{Name: "docs", Agent: "opencode", Prompt: "D {{.Task}}"},
	})
	cases := []struct {
		name    string
		args    []string
		none    bool
		want    control.Request
		wantErr string
	}{
		{"defaults", nil, false, control.Request{Kind: "claude", Dir: "/work", Mode: "code"}, ""},
		{"relative dir", []string{"--dir", "sub/.."}, false, control.Request{Kind: "claude", Dir: "/work", Mode: "code"}, ""},
		{"absolute dir", []string{"--dir", "/tmp/x/"}, false, control.Request{Kind: "claude", Dir: "/tmp/x", Mode: "code"}, ""},
		{"project fills dir name agent", []string{"--project", "api"}, false, control.Request{Kind: "codex", Dir: "/opt/api", Name: "api", Mode: "code"}, ""},
		{"flags override project", []string{"--project", "api", "--agent", "claude", "--name", "n"}, false, control.Request{Kind: "claude", Dir: "/opt/api", Name: "n", Mode: "code"}, ""},
		{"project without agent", []string{"--project", "web"}, false, control.Request{Kind: "claude", Dir: "/opt/web", Name: "web", Mode: "code"}, ""},
		{"all flags", []string{"--agent", "opencode", "--label", "L", "--prompt", "hi there"}, false, control.Request{Kind: "opencode", Dir: "/work", Label: "L", Prompt: "hi there", Mode: "code"}, ""},
		{"worktree outside a repo", []string{"--worktree"}, false, control.Request{}, "/work is not in a git repository"},
		{"review infers dir and name", []string{"--mode", "review", "--url", "https://github.com/acme/web/pull/7"}, false, control.Request{Kind: "claude", Dir: "/tmp/prs/web-7", Name: "review web#7", Prompt: "R https://github.com/acme/web/pull/7", Mode: "review"}, ""},
		{"review without url", []string{"--mode", "review"}, false, control.Request{}, "PR URL is required"},
		{"bug in cwd named after ticket", []string{"--mode", "bug", "--ticket", "SP-7"}, false, control.Request{Kind: "claude", Dir: "/work", Name: "bug SP-7", Prompt: "B SP-7", Mode: "bug"}, ""},
		{"bug without ticket", []string{"--mode", "bug"}, false, control.Request{}, "a ticket is required"},
		{"no-input mode in home", []string{"--mode", "standup", "--prompt", "ship it"}, false, control.Request{Kind: "claude", Dir: home, Name: "standup", Prompt: "S ship it", Mode: "standup"}, ""},
		{"no-input mode with dir", []string{"--mode", "standup", "--dir", "sub"}, false, control.Request{Kind: "claude", Dir: "/work/sub", Name: "standup", Prompt: "S", Mode: "standup"}, ""},
		{"custom mode agent", []string{"--mode", "docs", "--prompt", "x"}, false, control.Request{Kind: "opencode", Dir: "/work", Prompt: "D x", Mode: "docs"}, ""},
		{"unknown mode", []string{"--mode", "nope"}, false, control.Request{}, `no mode "nope"`},
		{"unknown project", []string{"--project", "nope"}, false, control.Request{}, `no project "nope"`},
		{"project and dir", []string{"--project", "api", "--dir", "/x"}, false, control.Request{}, "not both"},
		{"stray argument", []string{"extra"}, false, control.Request{}, `unexpected argument "extra"`},
		{"unknown flag", []string{"--bogus"}, false, control.Request{}, "bogus"},
		{"no agent installed", nil, true, control.Request{}, "no agent CLI installed"},
		{"explicit agent when none installed", []string{"--agent", "codex"}, true, control.Request{Kind: "codex", Dir: "/work", Mode: "code"}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			installed := []string{"claude", "opencode"}
			if tc.none {
				installed = nil
			}
			got, err := request(tc.args, list, all, "/work", installed)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		words   []string
		stdin   string
		want    string
		wantErr bool
	}{
		{"joins words", []string{"fix", "the", "bug"}, "", "fix the bug", false},
		{"dash reads stdin", []string{"-"}, "line one\nline two\n", "line one\nline two", false},
		{"dash among words is literal", []string{"a", "-"}, "ignored", "a -", false},
		{"empty stdin", []string{"-"}, "\n", "", true},
		{"blank words", []string{" "}, "", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := message(tc.words, strings.NewReader(tc.stdin))
			require.Equal(t, tc.wantErr, err != nil)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestPick(t *testing.T) {
	t.Parallel()
	list := []snapshot.Entry{
		{Kind: "claude", Name: "api", Label: "auth"},
		{Kind: "codex", Name: "web"},
		{Kind: "claude", Name: "web", Label: "api"},
	}
	cases := []struct {
		name    string
		key     string
		want    int
		wantErr string
	}{
		{"number", "2", 1, ""},
		{"label", "AUTH", 0, ""},
		{"label wins over name", "api", 2, ""},
		{"ambiguous name", "web", 0, "matches 2 favorites"},
		{"number out of range", "4", 0, "no favorite #4"},
		{"zero", "0", 0, "no favorite #0"},
		{"unknown", "nope", 0, `no favorite matches "nope"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pick(list, tc.key)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, list[tc.want], got)
		})
	}
}

func live(id, kind, cwd, window, label string, status state.Status, cost float64) agents.Live {
	return agents.Live{
		Agent:    state.Agent{Pane: id, Kind: kind, Cwd: cwd, Status: status},
		Pane:     tmux.Pane{ID: id, WindowName: window, Cwd: cwd, Label: label},
		Stats:    state.Stats{CostUSD: cost},
		HasStats: cost > 0,
	}
}

func child(a agents.Live, parent, mode string) agents.Live {
	a.Parent, a.Mode = parent, mode
	return a
}

func TestTables(t *testing.T) {
	t.Parallel()
	projs := []projects.Project{{Name: "api", Dir: "/opt/api"}}
	running := live("%3", "claude", "/opt/api", "api", "auth fix", state.Working, 1.234)
	idle := live("%12", "codex", "/srv/web", "web", "", "", 0)
	master := child(live("%1", "claude", "/home", "master", "", state.Idle, 0), "", "master")
	sub := child(live("%4", "codex", "/opt/api", "api", "", state.Done, 0), "%1", "code")
	subsub := child(live("%5", "claude", "/srv/web", "web", "", state.Done, 0), "%4", "")
	orphan := child(idle, "%99", "")
	favs := []snapshot.Entry{
		snapshot.Tree(running, nil),
		{Kind: "codex", Dir: "/srv/old", Name: "old"},
		snapshot.Tree(master, []agents.Live{master, sub, subsub}),
	}
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"agents", table([]agents.Live{running, idle}, projs), "" +
			"PANE  STATUS   AGENT   NAME      WHERE     COST\n" +
			"%3    working  claude  auth fix  api       $1.23\n" +
			"%12   unknown  codex   web       /srv/web\n"},
		{"no agents", table(nil, projs), "PANE  STATUS  AGENT  NAME  WHERE  COST\n"},
		{"tree with modes", table([]agents.Live{subsub, orphan, sub, master}, projs), "" +
			"PANE  STATUS   AGENT   MODE    NAME     WHERE     COST\n" +
			"%12   unknown  codex   code    web      /srv/web\n" +
			"%1    idle     claude  master  master   /home\n" +
			"%4    done     codex   code    └ api    api\n" +
			"%5    done     claude  code      └ web  /srv/web\n"},
		{"code-only tree hides mode", table([]agents.Live{running, child(idle, "%3", "code")}, projs), "" +
			"PANE  STATUS   AGENT   NAME      WHERE     COST\n" +
			"%3    working  claude  auth fix  api       $1.23\n" +
			"%12   unknown  codex   └ web     /srv/web\n"},
		{"favorites", favTable(favs, []agents.Live{running}), "" +
			"1  running    claude  auth fix  /opt/api\n" +
			"2  resumable  codex   old       /srv/old\n" +
			"3  resumable  claude  master    /home     +1 children\n"},
		{"no favorites", favTable(nil, nil), ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.got)
		})
	}
}
