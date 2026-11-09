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
	sessions := map[string]string{"%1": "cockpit", "cockpit:": "cockpit", "%7": "0", "@3": "work"}
	sessionOf := func(target string) (string, error) {
		s, ok := sessions[target]
		if !ok {
			return "", errors.New("can't find pane")
		}
		return s, nil
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
		{"mutating without a target", []string{"select-pane", "-L"}, "explicit target"},
		{"unknown target", []string{"kill-pane", "-t", "%99"}, "no target %99"},
		{"kill server", []string{"kill-server"}, "never runs tmux kill-server"},
		{"kill session", []string{"kill-session", "-t", "%1"}, "never runs tmux kill-session"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := guard(tc.args, sessionOf)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
