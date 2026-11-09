package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func repo(t *testing.T) string {
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "init")
	return dir
}

func TestCollect(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		setup   func(t *testing.T, dir string) string
		want    []string
		wantNot []string
	}{
		{"clean", func(t *testing.T, dir string) string { return dir }, nil, nil},
		{
			"no commits yet",
			func(t *testing.T, _ string) string {
				dir := t.TempDir()
				run(t, dir, "init", "-q", "-b", "main")
				require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x\n"), 0o644))
				run(t, dir, "add", "x.txt")
				require.NoError(t, os.WriteFile(filepath.Join(dir, "y.txt"), []byte("y\n"), 0o644))
				return dir
			},
			[]string{"branch main compared to nothing", "+x", "+y"},
			nil,
		},
		{
			"uncommitted change",
			func(t *testing.T, dir string) string {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two\n"), 0o644))
				return dir
			},
			[]string{"branch main compared to main", "-one", "+two"},
			nil,
		},
		{
			"untracked file with a space",
			func(t *testing.T, dir string) string {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "new file.txt"), []byte("hi\n"), 0o644))
				return dir
			},
			[]string{"new file.txt", "+hi"},
			nil,
		},
		{
			"worktree branch includes its commits",
			func(t *testing.T, dir string) string {
				wt := filepath.Join(t.TempDir(), "wt")
				run(t, dir, "worktree", "add", "-q", "-b", "feature", wt)
				require.NoError(t, os.WriteFile(filepath.Join(wt, "b.txt"), []byte("committed\n"), 0o644))
				run(t, wt, "add", ".")
				run(t, wt, "commit", "-qm", "feature")
				require.NoError(t, os.WriteFile(filepath.Join(wt, "a.txt"), []byte("dirty\n"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("main checkout\n"), 0o644))
				return wt
			},
			[]string{"branch feature compared to main", "+committed", "+dirty"},
			[]string{"main checkout"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Collect(tc.setup(t, repo(t)))
			require.NoError(t, err)
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

func TestRootAndNested(t *testing.T) {
	t.Parallel()
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	for _, d := range []string{"one", "deep/er/two", "too/deep/for/three", ".hidden/four", "node_modules/five", "plain/dir"} {
		require.NoError(t, os.MkdirAll(filepath.Join(scratch, d), 0o755))
		if filepath.Base(d) != "dir" {
			run(t, filepath.Join(scratch, d), "init", "-q")
		}
	}
	require.NoError(t, os.MkdirAll(filepath.Join(scratch, "one", "sub"), 0o755))

	require.Equal(t, []string{filepath.Join(scratch, "deep/er/two"), filepath.Join(scratch, "one")}, Nested(scratch))

	cases := []struct {
		name   string
		path   string
		want   string
		wantOK bool
	}{
		{"repo root", filepath.Join(scratch, "one"), filepath.Join(scratch, "one"), true},
		{"subfolder", filepath.Join(scratch, "one", "sub"), filepath.Join(scratch, "one"), true},
		{"deleted file", filepath.Join(scratch, "one", "gone", "x.go"), filepath.Join(scratch, "one"), true},
		{"outside git", filepath.Join(scratch, "plain", "dir", "x.go"), "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Root(tc.path)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.want, got)
		})
	}
}
