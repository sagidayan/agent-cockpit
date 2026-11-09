package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func repo(t *testing.T) string {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	dir := filepath.Join(root, "app")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	run(t, dir, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "a.txt"), []byte("one\n"), 0o644))
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "init")
	return dir
}

func TestCreate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		setup      func(t *testing.T, repo string) string
		wantPath   string
		wantBranch string
		wantErr    string
	}{
		{"from repo root", func(t *testing.T, repo string) string { return repo }, "app-worktrees/fix-bug", "cockpit/fix-bug", ""},
		{"from a subfolder", func(t *testing.T, repo string) string { return filepath.Join(repo, "sub") }, "app-worktrees/fix-bug", "cockpit/fix-bug", ""},
		{
			"from another worktree lands next to the main repo",
			func(t *testing.T, repo string) string {
				wt := filepath.Join(filepath.Dir(repo), "elsewhere")
				run(t, repo, "worktree", "add", "-q", "-b", "other", wt)
				run(t, wt, "commit", "-q", "--allow-empty", "-m", "other")
				return wt
			},
			"app-worktrees/fix-bug", "cockpit/fix-bug", "",
		},
		{
			"path taken",
			func(t *testing.T, repo string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(filepath.Dir(repo), "app-worktrees", "fix-bug"), 0o755))
				return repo
			},
			"app-worktrees/fix-bug-2", "cockpit/fix-bug-2", "",
		},
		{
			"branch taken",
			func(t *testing.T, repo string) string {
				run(t, repo, "branch", "cockpit/fix-bug")
				run(t, repo, "branch", "cockpit/fix-bug-2")
				return repo
			},
			"app-worktrees/fix-bug-3", "cockpit/fix-bug-3", "",
		},
		{"not a repo", func(t *testing.T, _ string) string { return t.TempDir() }, "", "", "not in a git repository"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := repo(t)
			dir := tc.setup(t, r)
			got, err := Create(dir, "Fix bug!")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, filepath.Join(filepath.Dir(r), tc.wantPath), got)
			require.Equal(t, tc.wantBranch, run(t, got, "symbolic-ref", "--short", "HEAD"))
			require.Equal(t, run(t, dir, "rev-parse", "HEAD"), run(t, got, "rev-parse", "HEAD"))
			require.FileExists(t, filepath.Join(got, "sub", "a.txt"))
		})
	}
}

func TestInRepo(t *testing.T) {
	t.Parallel()
	r := repo(t)
	cases := []struct {
		name string
		dir  string
		want bool
	}{
		{"repo root", r, true},
		{"subfolder", filepath.Join(r, "sub"), true},
		{"plain folder", t.TempDir(), false},
		{"missing folder", filepath.Join(r, "nope"), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, InRepo(tc.dir))
		})
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"payments", "payments"},
		{"Fix the Flaky  refund test!", "fix-the-flaky-refund-test"},
		{"review web#7", "review-web-7"},
		{"--", "agent"},
		{"", "agent"},
		{strings.Repeat("abcd-", 10), "abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, slugify(tc.in), tc.in)
	}
}

func TestOrigin(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"/src/app-worktrees/fix-bug", "/src/app"},
		{"/src/app", "/src/app"},
		{"/src/app-worktrees", "/src/app-worktrees"},
		{"/src/app-worktrees/fix-bug/sub", "/src/app-worktrees/fix-bug/sub"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, Origin(tc.in), tc.in)
	}
}
