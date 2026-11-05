package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/modes"
)

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		in    string
		limit int
		want  string
	}{
		{"short", "abc", 5, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"long", "abcdef", 3, "abc\n[truncated: showing 3 of 6 bytes]"},
		{"multibyte cut", "aé", 2, "a\n[truncated: showing 2 of 3 bytes]"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := truncate(tc.in, tc.limit); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUntracked(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"none", " M a.go\n", "(none)\n"},
		{"mixed", " M a.go\n?? b.go\n?? dir/\n", "b.go\ndir/\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := untracked(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWaitSeconds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want int
	}{
		{0, 300},
		{-5, 300},
		{60, 60},
		{5000, 1800},
	}
	for _, tc := range cases {
		if got := waitSeconds(tc.in); got != tc.want {
			t.Errorf("waitSeconds(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestLevel(t *testing.T) {
	t.Parallel()
	parents := map[string]string{"%1": "", "%2": "%1", "%3": "%2", "%8": "%9", "%9": "%8"}
	cases := []struct {
		name string
		pane string
		want int
	}{
		{"not an agent pane", "", 0},
		{"root", "%1", 1},
		{"child", "%2", 2},
		{"grandchild", "%3", 3},
		{"unknown pane is a root", "%7", 1},
		{"cycle stops", "%8", len(parents) + 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, level(tc.pane, parents))
		})
	}
}

func TestSpawnSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mode := func(name string) config.Mode {
		m, _ := modes.Find(modes.All(nil), name)
		return m
	}
	cases := []struct {
		name    string
		in      spawnArgs
		mode    string
		wantDir string
		wantErr string
	}{
		{"master without dir", spawnArgs{Task: "t"}, modes.Master, "", ""},
		{"master with dir", spawnArgs{Dir: dir, Task: "t"}, modes.Master, dir, ""},
		{"code needs dir", spawnArgs{Task: "t"}, modes.Code, "", "exactly one of project or dir"},
		{"code with dir", spawnArgs{Dir: dir, Task: "t"}, modes.Code, dir, ""},
		{"both set", spawnArgs{Dir: dir, Project: "p"}, modes.Master, "", "exactly one of project or dir"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := spawnSpec(tc.in, mode(tc.mode))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantDir, got.Dir)
		})
	}
}
