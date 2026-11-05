package launch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		kind     string
		resumeID string
		want     string
	}{
		{"claude new", "claude", "", "claude"},
		{"claude resume", "claude", "8f1c-22", "claude --resume '8f1c-22'"},
		{"codex new", "codex", "", "codex"},
		{"codex resume", "codex", "0199a", "codex resume '0199a'"},
		{"opencode new", "opencode", "", "opencode --standalone"},
		{"opencode resume", "opencode", "ses_1", "opencode --standalone --session 'ses_1'"},
		{"quote in id", "claude", "a'b", `claude --resume 'a'\''b'`},
		{"unknown kind", "vim", "x", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Command(tc.kind, tc.resumeID))
		})
	}
}

func TestStart(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		kind   string
		prompt string
		want   string
	}{
		{"no prompt", "claude", "", "claude"},
		{"claude prompt", "claude", "fix the tests", "claude 'fix the tests'"},
		{"codex prompt", "codex", "fix it", "codex 'fix it'"},
		{"opencode prompt", "opencode", "fix it", "opencode --standalone --prompt 'fix it'"},
		{"quote in prompt", "claude", "don't", `claude 'don'\''t'`},
		{"unknown kind", "vim", "x", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Start(tc.kind, tc.prompt))
		})
	}
}
