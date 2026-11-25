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
		lead     bool
		want     string
	}{
		{name: "claude new", kind: "claude", resumeID: "", want: "claude"},
		{name: "claude resume", kind: "claude", resumeID: "8f1c-22", want: "claude --resume '8f1c-22'"},
		{name: "codex new", kind: "codex", resumeID: "", want: "codex"},
		{name: "codex resume", kind: "codex", resumeID: "0199a", want: "codex resume '0199a'"},
		{name: "opencode new", kind: "opencode", resumeID: "", want: "opencode --standalone"},
		{name: "opencode resume", kind: "opencode", resumeID: "ses_1", want: "opencode --standalone --session 'ses_1'"},
		{name: "quote in id", kind: "claude", resumeID: "a'b", want: `claude --resume 'a'\''b'`},
		{name: "unknown kind", kind: "vim", resumeID: "x", want: ""},
		{name: "claude lead resume", kind: "claude", resumeID: "s1", lead: true, want: "COCKPIT_LEAD=1 claude --disallowedTools Edit,Write,NotebookEdit --append-system-prompt " + Quote(LeadPrompt) + " --resume 's1'"},
		{name: "opencode lead resume", kind: "opencode", resumeID: "ses_1", lead: true, want: "COCKPIT_LEAD=1 opencode --standalone --session 'ses_1'"},
		{name: "unknown lead", kind: "vim", lead: true, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Command(tc.kind, tc.resumeID, tc.lead))
		})
	}
}

func TestStart(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		kind   string
		prompt string
		lead   bool
		want   string
	}{
		{name: "no prompt", kind: "claude", prompt: "", want: "claude"},
		{name: "claude prompt", kind: "claude", prompt: "fix the tests", want: "claude 'fix the tests'"},
		{name: "codex prompt", kind: "codex", prompt: "fix it", want: "codex 'fix it'"},
		{name: "opencode prompt", kind: "opencode", prompt: "fix it", want: "opencode --standalone --prompt 'fix it'"},
		{name: "quote in prompt", kind: "claude", prompt: "don't", want: `claude 'don'\''t'`},
		{name: "unknown kind", kind: "vim", prompt: "x", want: ""},
		{name: "claude lead", kind: "claude", prompt: "plan it", lead: true, want: "COCKPIT_LEAD=1 claude --disallowedTools Edit,Write,NotebookEdit --append-system-prompt " + Quote(LeadPrompt) + " 'plan it'"},
		{name: "codex lead gets the role in its prompt", kind: "codex", prompt: "plan it", lead: true, want: "COCKPIT_LEAD=1 codex " + Quote(LeadPrompt+"\n\nplan it")},
		{name: "opencode lead without a task", kind: "opencode", lead: true, want: "COCKPIT_LEAD=1 opencode --standalone --prompt " + Quote(LeadPrompt)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Start(tc.kind, tc.prompt, tc.lead))
		})
	}
}
