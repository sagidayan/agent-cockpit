package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePanes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want []string
	}{
		{"empty trailing label kept", "%0\tw\t0\tapi\t/a\tclaude\t\t\t\t12\n%1\tw\t1\tweb\t/b\tclaude\t\t\t\t12\n", []string{"%0", "%1"}},
		{"label with spaces", "%2\tw\t2\tapi\t/a\tclaude\tbilling bug\treview\t%1\t14\n", []string{"%2"}},
		{"linked window listed once", "%4\tmain\t1\tapi\t/a\tclaude\t\t\t\t12\n%4\tcockpit\t3\tapi\t/a\tclaude\t\t\t\t12\n", []string{"%4"}},
		{"malformed line skipped", "garbage\n%3\tw\t3\tapi\t/a\tzsh\t\t\t\t13\n", []string{"%3"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ids []string
			for _, p := range parsePanes(tc.out) {
				ids = append(ids, p.ID)
			}
			require.Equal(t, tc.want, ids)
		})
	}
}

func TestGuard(t *testing.T) {
	t.Parallel()
	panes := map[string]paneInfo{
		"%1":       {"cockpit", "%1", ""},
		"cockpit:": {"cockpit", "%1", ""},
		"%7":       {"0", "%7", ""},
		"@3":       {"work", "%8", ""},
		"%20":      {"0", "%20", "1"},
		"%21":      {"0", "%21", ""},
	}
	Own("%21")
	describe := func(target string) (paneInfo, error) {
		p, ok := panes[target]
		if !ok {
			return paneInfo{}, errors.New("can't find pane")
		}
		return p, nil
	}
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"read-only command anywhere", []string{"capture-pane", "-p", "-t", "%7"}, ""},
		{"listing", []string{"list-panes", "-a"}, ""},
		{"send keys to own pane", []string{"send-keys", "-t", "%1", "Enter"}, ""},
		{"new window in own session", []string{"new-window", "-d", "-t", "cockpit:", "-n", "x"}, ""},
		{"send keys to a user pane", []string{"send-keys", "-t", "%7", "Enter"}, `%7 is in your "0" tmux session`},
		{"kill a user pane", []string{"kill-pane", "-t", "%7"}, "only changes its own"},
		{"swap a user pane in", []string{"swap-pane", "-d", "-s", "%7", "-t", "%1"}, `%7 is in your "0"`},
		{"switch a client to a user window", []string{"switch-client", "-t", "@3"}, `"work"`},
		{"cockpit agent shown in the dashboard window", []string{"send-keys", "-t", "%20", "Enter"}, ""},
		{"dashboard's own pane", []string{"split-window", "-h", "-t", "%21", "x"}, ""},
		{"swap an agent into the viewer slot", []string{"swap-pane", "-d", "-s", "%1", "-t", "%21"}, ""},
		{"link a cockpit window next to the dashboard", []string{"link-window", "-a", "-s", "cockpit:", "-t", "%21"}, ""},
		{"move a user window", []string{"move-window", "-d", "-s", "%7", "-t", "cockpit:"}, `%7 is in your "0"`},
		{"break out a handed-over agent", []string{"break-pane", "-d", "-s", "%20", "-t", "cockpit:"}, ""},
		{"mutating without a target", []string{"select-pane", "-L"}, "explicit target"},
		{"unknown target", []string{"kill-pane", "-t", "%99"}, "no target %99"},
		{"kill server", []string{"kill-server"}, "never runs tmux kill-server"},
		{"kill session", []string{"kill-session", "-t", "%1"}, "never runs tmux kill-session"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := guard(tc.args, describe)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
