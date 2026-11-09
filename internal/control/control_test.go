package control

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
	"github.com/sagidayan/agent-cockpit/internal/worktree"
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
	require.NoError(t, state.Save(state.Agent{Pane: "%1", Kind: "claude", Touched: []string{"/z/earlier"}, ClosedCost: 0.5}))
	require.NoError(t, state.Save(state.Agent{Pane: "%6", Kind: "codex", Touched: []string{"/z/grandchild"}, ClosedCost: 0.25}))

	child := func(id, parent string) agents.Live {
		return agents.Live{Agent: state.Agent{Pane: id, Kind: "codex", Cwd: repo}, Pane: tmux.Pane{ID: id, Parent: parent, Cwd: repo}, Stats: state.Stats{CostUSD: 1}, HasStats: true}
	}
	handOver(child("%2", "%1"))
	handOver(child("%3", "%1"))
	handOver(child("%4", "%9"))
	handOver(child("%5", ""))
	handOver(child("%6", "%1"))

	got, err := state.Load("%1")
	require.NoError(t, err)
	require.Equal(t, []string{repo, "/z/earlier", "/z/grandchild"}, got.Touched)
	require.InDelta(t, 3.75, got.ClosedCost, 1e-9)
	_, err = state.Load("%9")
	require.Error(t, err)
}

func TestRecord(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		out, err := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x\n"), 0o644))
	a := agents.Live{
		Agent: state.Agent{Pane: "%1", Kind: "claude", Cwd: repo, SessionID: "s1", Message: "done"},
		Pane:  tmux.Pane{ID: "%1", WindowName: "w", Label: "bug", Cwd: repo},
		Stats: state.Stats{CostUSD: 1.5}, HasStats: true,
	}
	got := record(a)
	require.WithinDuration(t, time.Now(), got.ClosedAt, time.Minute)
	got.ClosedAt = time.Time{}
	require.Equal(t, state.Closed{Name: "w", Label: "bug", Kind: "claude", Dir: repo, SessionID: "s1", Message: "done", Diff: "1 files changed, +1 -0: f.txt", CostUSD: 1.5}, got)
}

func TestRestartRefuses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		agent   state.Agent
		wantErr string
	}{
		{"no session", state.Agent{Kind: "codex"}, "no session recorded"},
		{"unknown kind", state.Agent{Kind: "vim", SessionID: "s1"}, "unknown agent"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorContains(t, Restart(agents.Live{Agent: tc.agent}), tc.wantErr)
		})
	}
}

func TestCleanWorktrees(t *testing.T) {
	t.Parallel()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git := func(dir string, args ...string) {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git(repo, "init", "-q", "-b", "main")
	git(repo, "commit", "-q", "--allow-empty", "-m", "init")
	create := func(name string) string {
		path, err := worktree.Create(repo, name)
		require.NoError(t, err)
		return path
	}
	clean, dirty, busy := create("clean"), create("dirty"), create("busy")
	require.NoError(t, os.WriteFile(filepath.Join(dirty, "new.txt"), nil, 0o644))
	live := []agents.Live{{Pane: tmux.Pane{Cwd: busy}}}

	removed, kept := CleanWorktrees([]string{repo, busy}, live)
	require.Equal(t, 1, removed)
	require.Equal(t, []string{dirty + " (uncommitted changes)"}, kept)
	require.NoDirExists(t, clean)
	require.DirExists(t, busy)
}

func TestReport(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		a     state.Agent
		label string
		want  string
	}{
		{"done with label", state.Agent{Pane: "%12", Status: state.Done, Message: "Fixed the **retry** bug."}, "billing", "[cockpit] child %12 (billing) is done: Fixed the retry bug. Read it with get_output / get_diff."},
		{"needs input without label", state.Agent{Pane: "%3", Status: state.NeedsInput, Message: "Claude needs your permission to use Bash"}, "", "[cockpit] child %3 needs input: Claude needs your permission to use Bash. Read it with get_output and answer with send_task."},
		{"done without message", state.Agent{Pane: "%4", Status: state.Done}, "", "[cockpit] child %4 is done. Read it with get_output / get_diff."},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, report(tc.a, tc.label))
		})
	}
}

func TestPauseUndoesTheMarkWhenEscFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMUX", "/tmp/no-such-cockpit-test-sock,1,0")
	require.NoError(t, state.Save(state.Agent{Pane: "%1", Kind: "claude", Status: state.Working}))

	require.Error(t, Pause(agents.Live{Agent: state.Agent{Pane: "%1", Kind: "claude"}, Pane: tmux.Pane{ID: "%1"}}))

	got, err := state.Load("%1")
	require.NoError(t, err)
	require.False(t, got.Paused)
}

func TestEscapes(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]int{"claude": 1, "codex": 1, "opencode": 2} {
		require.Equal(t, want, escapes(kind), kind)
	}
}
