package control

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func TestMatch(t *testing.T) {
	t.Parallel()
	live := []agents.Live{
		{Pane: tmux.Pane{ID: "%1", WindowName: "claude", Label: "billing bug"}},
		{Pane: tmux.Pane{ID: "%2", WindowName: "claude"}},
		{Pane: tmux.Pane{ID: "%3", WindowName: "web"}},
	}
	cases := []struct {
		name    string
		target  string
		want    string
		wantErr string
	}{
		{"pane id", "%2", "%2", ""},
		{"label case insensitive", "Billing Bug", "%1", ""},
		{"window name", "web", "%3", ""},
		{"ambiguous window name", "claude", "", "matches 2 agents"},
		{"no match", "nope", "", "no agent matches"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := match(live, tc.target)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, a.ID)
		})
	}
}

func TestHandOver(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMUX", "/tmp/sock,1,0")
	repo := t.TempDir()
	out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput()
	require.NoError(t, err, string(out))
	repo, err = filepath.EvalSymlinks(repo)
	require.NoError(t, err)
	require.NoError(t, state.Save(state.Agent{Pane: "%1", Kind: "claude", Touched: []string{"/z/earlier"}}))
	require.NoError(t, state.Save(state.Agent{Pane: "%6", Kind: "codex", Touched: []string{"/z/grandchild"}}))

	child := func(id, parent string) agents.Live {
		return agents.Live{Agent: state.Agent{Pane: id, Kind: "codex", Cwd: repo}, Pane: tmux.Pane{ID: id, Parent: parent, Cwd: repo}}
	}
	handOver(child("%2", "%1"))
	handOver(child("%3", "%1"))
	handOver(child("%4", "%9"))
	handOver(child("%5", ""))
	handOver(child("%6", "%1"))

	got, err := state.Load("%1")
	require.NoError(t, err)
	require.Equal(t, []string{repo, "/z/earlier", "/z/grandchild"}, got.Touched)
	_, err = state.Load("%9")
	require.Error(t, err)
}
