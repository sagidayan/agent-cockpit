package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/require"
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
	cases := []struct {
		name    string
		in      spawnArgs
		wantDir string
		wantErr string
	}{
		{"needs dir", spawnArgs{Task: "t"}, "", "exactly one of project or dir"},
		{"with dir", spawnArgs{Dir: dir, Task: "t"}, dir, ""},
		{"both set", spawnArgs{Dir: dir, Project: "p"}, "", "exactly one of project or dir"},
		{"relative dir", spawnArgs{Dir: "rel"}, "", "not an absolute directory"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := spawnSpec(tc.in)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantDir, got.Dir)
		})
	}
}
