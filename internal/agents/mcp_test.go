package agents

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestElapsed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"00:05", 5 * time.Second},
		{"12:34", 12*time.Minute + 34*time.Second},
		{"01:00:00", time.Hour},
		{"2-03:04:05", 2*24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, elapsed(tc.in), tc.in)
	}
}

func TestMCPDown(t *testing.T) {
	t.Parallel()
	procs := parseProcesses(`
  100     1   10:00 -zsh
  101   100   09:59 claude
  102   101   09:58 /Users/me/bin/cockpit mcp
  200     1   10:00 -zsh
  201   200   09:59 claude --resume abc
  300     1   00:10 claude
  400     1   10:00 opencode --standalone
  401   400   09:59 /opt/homebrew/bin/opencode serve --stdio
  402   401   09:58 /Users/me/bin/cockpit mcp
  500     1   10:00 -zsh
  600     1   10:00 codex
  700     1   10:00 claude
  701   700   09:59 /opt/claude daemon run --origin transient
  702   701   09:58 claude bg-pty-host --bg-pty-host /tmp/x.sock
  703   702   09:58 claude bg-spare --bg-spare /tmp/y.sock
  704   703   09:57 /Users/me/bin/cockpit mcp
  800     1   10:00 opencode
`)
	cases := []struct {
		name string
		kind string
		pid  string
		want bool
	}{
		{"claude with its server", "claude", "100", false},
		{"claude without a server", "claude", "200", true},
		{"claude still starting", "claude", "300", false},
		{"opencode server as a grandchild", "opencode", "400", false},
		{"agent exited to the shell", "claude", "500", false},
		{"unchecked kind", "codex", "600", false},
		{"only a spare session of the claude daemon has a server", "claude", "700", true},
		{"opencode without a server", "opencode", "800", true},
		{"unknown pane pid", "claude", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, mcpDown(tc.kind, tc.pid, procs))
		})
	}
}
