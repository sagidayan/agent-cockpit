package permission

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		kind    string
		fixture string
		lines   []string
		options []Option
		wantErr string
	}{
		{
			"claude bash", "claude", "claude-bash.txt",
			[]string{"Bash command", "Tip: auto mode handles these prompts for you — choose \"switch to auto mode\" below", "Print 42 using Python", `python3 -c "print(42)"`, "This command requires approval", "Do you want to proceed?"},
			[]Option{
				{Once, "Yes", []string{"1"}},
				{Always, "Yes, and don’t ask again for: python3 *", []string{"2"}},
				{Deny, "No", []string{"4"}},
			},
			"",
		},
		{
			"claude write", "claude", "claude-write.txt",
			[]string{"Create file", "notes.txt", "1 hi", "Do you want to create notes.txt?"},
			[]Option{
				{Once, "Yes", []string{"1"}},
				{Always, "Yes, and switch to accept edits (auto-approve file edits and common file commands) for this session (shift+tab)", []string{"2"}},
				{Deny, "No", []string{"3"}},
			},
			"",
		},
		{
			"claude wrapped", "claude", "claude-wrapped.txt",
			[]string{"Bash command", "Tip: auto mode handles these prompts for you — choose \"switch to", "auto mode\" below", "Create empty file e2e-deny.txt", "touch e2e-deny.txt", "Do you want to proceed?"},
			[]Option{
				{Once, "Yes", []string{"1"}},
				{Always, "Yes, and always allow access to /private/tmp/claude-502/-Users-s\nagidayan-Development-agent-cockpit-wt-answer-prompts/4ec0502d-9a\nf9-4e5a-8225-74adcfe15c50/scratchpad/claude-probe from this", []string{"2"}},
				{Deny, "No", []string{"4"}},
			},
			"",
		},
		{"claude trust dialog", "claude", "claude-trust.txt", nil, nil, "no permission prompt"},
		{"claude without dialog", "claude", "opencode-bash.txt", nil, nil, "no permission prompt"},
		{
			"opencode bash", "opencode", "opencode-bash.txt",
			[]string{"$ touch oc-a.txt && ls -la oc-a.txt"},
			[]Option{
				{Once, "Allow once", []string{"Enter"}},
				{Always, "Always allow", []string{"Right", "Enter"}},
				{Deny, "Reject", []string{"Right", "Right", "Enter"}},
			},
			"",
		},
		{"opencode without dialog", "opencode", "claude-bash.txt", nil, nil, "no permission prompt"},
		{"codex", "codex", "claude-bash.txt", nil, nil, "isn't supported for codex"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			screen, err := os.ReadFile("testdata/" + tc.fixture)
			require.NoError(t, err)
			p, err := Parse(tc.kind, string(screen))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.lines, p.Lines)
			require.Equal(t, tc.options, p.Options)
		})
	}
}
