package changes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

func repo(t *testing.T, dir, change string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	if change != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte(change+"\n"), 0o644))
	}
	return dir
}

func agent(id, parent, cwd, transcript string) agents.Live {
	return agents.Live{
		Agent: state.Agent{Pane: id, Kind: "claude", Cwd: cwd, Transcript: transcript},
		Pane:  tmux.Pane{ID: id, Parent: parent, Cwd: cwd},
	}
}

func edits(t *testing.T, files ...string) string {
	t.Helper()
	var line string
	for _, f := range files {
		line += fmt.Sprintf(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":%q}}]}}`+"\n", f)
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(line), 0o644))
	return path
}

func TestDiff(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	chat, err := paths.NewChat()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(chat) })
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	own := repo(t, filepath.Join(root, "own"), "own change")
	elsewhere := repo(t, filepath.Join(root, "elsewhere"), "edited elsewhere")
	child := repo(t, filepath.Join(root, "child"), "child change")
	closed := repo(t, filepath.Join(root, "closed"), "closed child change")
	clean := repo(t, filepath.Join(root, "clean"), "")
	cloned := repo(t, filepath.Join(chat, "clone"), "cloned change")
	other := repo(t, filepath.Join(chat, "other"), "other clone change")
	broken := repo(t, filepath.Join(root, "broken"), "x")
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".git", "index"), []byte("garbage"), 0o644))
	loose := filepath.Join(root, "notes.md")
	require.NoError(t, os.WriteFile(filepath.Join(own, ".gitignore"), []byte("secret.env\n"), 0o644))
	ignored := filepath.Join(own, "secret.env")
	require.NoError(t, os.WriteFile(ignored, []byte("hidden change\n"), 0o644))

	parent := agent("%1", "", own, edits(t, filepath.Join(elsewhere, "f.txt"), filepath.Join(clean, "x"), loose, ignored, filepath.Join(home, ".claude", "plans", "p.md")))
	parent.Touched = []string{closed}
	chatAgent := agent("%3", "%2", cloned, "")
	chatAgent.Pane.Cwd = chat
	live := []agents.Live{
		parent,
		agent("%2", "%1", child, ""),
		chatAgent,
		agent("%4", "", repo(t, filepath.Join(root, "unrelated"), "unrelated change"), ""),
	}

	cases := []struct {
		name    string
		target  agents.Live
		want    []string
		wantNot []string
	}{
		{"parent covers its repo, edits elsewhere, closed and live descendants", parent,
			[]string{"# " + own, "# " + elsewhere, "# " + child, "# " + closed, "# " + cloned, "+own change", "+edited elsewhere", "+child change", "+closed child change", "+cloned change", "# " + loose, "# gitignored, no diff available:\n# " + ignored},
			[]string{"unrelated", "# " + clean, ".claude", "hidden change", "outside git, no diff available:\n# " + ignored}},
		{"chat that moved into one clone still finds the other", chatAgent, []string{"# " + cloned, "# " + other, "+other clone change"}, []string{"own change", "child change"}},
		{"a failing repo is reported without hiding the rest", agent("%5", "", own, edits(t, filepath.Join(broken, "f.txt"))), []string{"# " + broken + ": git", "+own change"}, nil},
		{"clean repo has nothing", agent("%9", "", clean, ""), nil, nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Diff(tc.target, live)
			if tc.want == nil {
				require.Empty(t, got)
			}
			for _, w := range tc.want {
				require.Contains(t, got, w)
			}
			for _, w := range tc.wantNot {
				require.NotContains(t, got, w)
			}
		})
	}
}

func TestTouched(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	own := repo(t, filepath.Join(root, "own"), "")
	elsewhere := repo(t, filepath.Join(root, "elsewhere"), "")
	loose := filepath.Join(root, "notes.md")
	require.NoError(t, os.WriteFile(filepath.Join(own, ".gitignore"), []byte("secret.env\n"), 0o644))
	ignored := filepath.Join(own, "secret.env")
	a := agent("%1", "%0", own, edits(t, filepath.Join(elsewhere, "f.txt"), loose, ignored))
	require.Equal(t, []string{own, elsewhere, loose, ignored}, Touched(a))
}
