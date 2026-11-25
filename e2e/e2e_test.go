//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGuard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		session string
		args    []string
		wantErr bool
	}{
		{"send to a user pane", "0", []string{"send", "hello"}, true},
		{"kill a user pane", "0", []string{"kill"}, true},
		{"label a user pane", "0", []string{"label", "mine"}, true},
		{"send to its own pane", "cockpit", []string{"send", "hello"}, false},
		{"kill its own pane", "cockpit", []string{"kill"}, false},
		{"label its own pane", "cockpit", []string{"label", "mine"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			pane := e.agent(tc.session)
			args := append([]string{tc.args[0], pane}, tc.args[1:]...)
			out, err := e.cockpit(args...)
			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, out, `is in your "0" tmux session`)
				require.Equal(t, "0", e.format(pane, "#{session_name}"))
				require.Empty(t, e.format(pane, "#{@cockpit_label}"))
				return
			}
			require.NoError(t, err, out)
		})
	}
}

func TestMoveHandsOverOnlyTheAgentPane(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	agent := e.agent("0")
	other := e.tmux("split-window", "-d", "-P", "-F", "#{pane_id}", "-t", agent, "sleep 100000")
	d := e.dashboard()
	e.showing(d, "claude")
	e.keys(d, "m")
	e.eventually(func() bool { return e.format(agent, "#{session_name}") != "0" })
	require.Equal(t, "1", e.format(agent, "#{@cockpit_agent}"))
	require.Equal(t, "0", e.format(other, "#{session_name}"))
	require.Equal(t, []string{other}, e.panes(other))
}

func TestViewerRestoresOnQuit(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	agent := e.agent("cockpit")
	window := e.format(agent, "#{window_id}")
	d := e.dashboard()
	e.viewing(d, agent)
	e.keys(d, "q")
	e.eventually(func() bool { return e.format(d, "#{pane_dead}") == "1" })
	require.Equal(t, window, e.format(agent, "#{window_id}"))
	require.Equal(t, []string{d}, e.panes(d))
}

func TestGoToLinksAndUnlinks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		nonAgent bool
	}{
		{"agent pane active", false},
		{"non-agent pane active", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			first := e.agent("cockpit")
			if tc.nonAgent {
				e.tmux("split-window", "-t", first, "sleep 100000")
			}
			second := e.agent("cockpit")
			firstWindow, secondWindow := e.format(first, "#{window_id}"), e.format(second, "#{window_id}")
			d := e.dashboard()
			e.viewing(d, first)
			e.keys(d, "g", "t")
			e.eventually(func() bool { return slices.Contains(e.windows("work"), firstWindow) })
			e.keys(d, "j")
			e.viewing(d, second)
			e.keys(d, "g", "t")
			e.eventually(func() bool { return slices.Contains(e.windows("work"), secondWindow) })
			require.NotContains(t, e.windows("work"), firstWindow)
			e.keys(d, "q")
			e.eventually(func() bool { return e.format(d, "#{pane_dead}") == "1" })
			require.NotContains(t, e.windows("work"), secondWindow)
			require.Subset(t, e.windows("cockpit"), []string{firstWindow, secondWindow})
		})
	}
}

func TestPauseAndResume(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	agent := e.agent("cockpit")
	e.hook(agent, "UserPromptSubmit", "prompt", "fix the bug")
	d := e.dashboard()
	e.showing(d, "1 working")
	e.keys(d, "z")
	e.showing(d, "1 paused")
	require.True(t, e.state(agent).Paused)
	e.keys(d, "z")
	e.showing(d, "1 working")
	require.False(t, e.state(agent).Paused)
	e.showing(agent, "You were paused from the cockpit dashboard")
}

func TestChildReportReachesParent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	parent := e.agent("cockpit")
	child := e.agent("cockpit")
	e.tmux("set-option", "-p", "-t", child, "@cockpit_parent", parent)
	e.hook(parent, "SessionStart")
	e.hook(child, "UserPromptSubmit", "prompt", "run the tests")
	e.hook(child, "Stop", "last_assistant_message", "All tests pass")
	e.showing(parent, "[cockpit] child "+child+" is done: All tests pass")
}

func TestNewStartsALead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		mode string
		lead bool
	}{
		{name: "coding is a lead", mode: "code", lead: true},
		{name: "hands-on is not", mode: "hands-on", lead: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			e.tmux("set-environment", "-g", "PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := e.cockpit("new", "--mode", tc.mode, "--dir", e.dir, "--prompt", "plan it")
			require.NoError(t, err, out)
			pane := strings.TrimSpace(out)
			var launched []string
			e.eventually(func() bool {
				data, err := os.ReadFile(filepath.Join(e.dir, ".fakeagent"))
				launched = strings.Split(string(data), "\n")
				return err == nil
			})
			require.Equal(t, tc.lead, launched[0] == "1")
			require.Equal(t, tc.lead, slices.Contains(launched, "--disallowedTools"))
			require.Equal(t, "plan it", launched[len(launched)-1])
			require.Equal(t, tc.lead, e.format(pane, "#{@cockpit_lead}") == "1")
		})
	}
}
