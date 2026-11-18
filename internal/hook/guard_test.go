package hook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTmuxGuard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		command string
		blocked string
	}{
		{"kill-server", "tmux kill-server", "kill-server"},
		{"kill-session", "tmux kill-session -t main", "kill-session"},
		{"kill-session prefix", "tmux kill-ses -t main", "kill-session"},
		{"kill-pane alias", "tmux killp -t %1", "kill-pane"},
		{"kill-window alias", "tmux killw -t 2", "kill-window"},
		{"unlink-window", "tmux unlink-window -k -t 0:3", "unlink-window"},
		{"respawn-pane", "tmux respawn-pane -k", "respawn-pane"},
		{"send-keys is allowed", "tmux send -t 1 ls Enter", ""},
		{"new-window is allowed", "tmux new-window -n logs", ""},
		{"set-option is allowed", "tmux set -g status off", ""},
		{"full path", "/opt/homebrew/bin/tmux kill-server", "kill-server"},
		{"quoted", `"tmux" 'kill-server'`, "kill-server"},
		{"escaped", `\tmux kill-server`, "kill-server"},
		{"after &&", "make test && tmux kill-server", "kill-server"},
		{"after ;", "echo hi; tmux kill-server", "kill-server"},
		{"after newline", "echo hi\ntmux kill-server", "kill-server"},
		{"in pipeline", "echo y | tmux kill-server", "kill-server"},
		{"or", "tmux has -t x || tmux kill-session -t x", "kill-session"},
		{"subshell", "(cd /tmp && tmux kill-server)", "kill-server"},
		{"command substitution", "echo $(tmux kill-server)", "kill-server"},
		{"substitution in quotes", `echo "out: $(tmux kill-server)"`, "kill-server"},
		{"backticks", "echo `tmux kill-server`", "kill-server"},
		{"env prefix", "env FOO=1 tmux kill-server", "kill-server"},
		{"assignment prefix", "TMUX= tmux kill-server", "kill-server"},
		{"sudo", "sudo -n tmux kill-server", "kill-server"},
		{"bash -c", `bash -c "tmux kill-server"`, "kill-server"},
		{"sh -lc", `sh -lc 'tmux kill-session -t x'`, "kill-session"},
		{"eval", `eval "tmux kill-server"`, "kill-server"},
		{"xargs", "echo x | xargs tmux kill-session -t", "kill-session"},
		{"redirect glued", "tmux kill-server>/dev/null 2>&1", "kill-server"},
		{"global flags", "tmux -u -f /dev/null kill-server", "kill-server"},
		{"default socket name", "tmux -L default kill-server", "kill-server"},
		{"default socket path", "tmux -S /tmp/tmux-501/default kill-server", "kill-server"},
		{"read then kill", `tmux list-sessions \; kill-server`, "kill-server"},
		{"read then kill glued", `tmux ls\; kill-server`, "kill-server"},
		{"dynamic binary", `$TMUX_BIN kill-server`, "kill-server"},
		{"dynamic which", `"$(which tmux)" kill-session -t x`, "kill-session"},
		{"other server", "tmux -L test kill-server", ""},
		{"other server glued", "tmux -Ltest kill-server", ""},
		{"other server clustered", "tmux -uL test kill-server", ""},
		{"other socket", "tmux -S /tmp/test.sock kill-session -t x", ""},
		{"other server variable", `tmux -L "$NAME" send-keys -t x ls Enter`, ""},
		{"other server in chain", "tmux -L t1 new -d && tmux -L t1 kill-server", ""},
		{"list-sessions", "tmux list-sessions", ""},
		{"ls alias", "tmux ls", ""},
		{"display-message", "tmux display-message -p '#S'", ""},
		{"capture-pane", "tmux capture-pane -p -t %1", ""},
		{"has-session", "tmux has-session -t main", ""},
		{"show-options", "tmux show-options -g", ""},
		{"no command", "tmux", ""},
		{"not tmux", "go test ./...", ""},
		{"commit message", `git commit -m "block tmux kill-server"`, ""},
		{"grep", "grep -rn kill-server internal", ""},
		{"grep with variable after", `grep -rn kill-server "$DIR"`, ""},
		{"single quoted substitution", `echo '$(tmux kill-server)'`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tmuxGuard(tc.command)
			if tc.blocked == "" {
				require.Empty(t, got)
				return
			}
			require.Contains(t, got, "(tmux "+tc.blocked+")")
		})
	}
}
