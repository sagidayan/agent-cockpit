package install

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetCodexNotify(t *testing.T) {
	t.Parallel()
	line := codexNotify("/bin/cockpit")
	cases := []struct {
		name    string
		config  string
		want    string
		wantErr bool
	}{
		{"empty", "", line + "\n", false},
		{"prepends", "model = \"o3\"\n", line + "\nmodel = \"o3\"\n", false},
		{"replaces own", "notify = [\"/old/cockpit\", \"hook\", \"codex\"]\n", line + "\n", false},
		{"keeps foreign", "notify = [\"other\"]\n", "", true},
		{"ignores table keys", "[tui]\nnotify = true\n", line + "\n[tui]\nnotify = true\n", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := setCodexNotify(tc.config, line)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMergeClaudeHooks(t *testing.T) {
	t.Parallel()
	settings := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"theme": "dark",
		"hooks": {
			"Stop": [
				{"hooks": [{"type": "command", "command": "~/mine.sh"}]},
				{"hooks": [{"type": "command", "command": "/old/cockpit hook claude"}]}
			]
		}
	}`), &settings))

	mergeClaudeHooks(settings, "/new path/cockpit")
	mergeClaudeHooks(settings, "/new path/cockpit")

	require.Equal(t, "dark", settings["theme"])
	hooks := settings["hooks"].(map[string]any)
	require.Len(t, hooks, len(claudeEvents))
	stop := hooks["Stop"].([]any)
	require.Len(t, stop, 2)
	require.False(t, isCockpitGroup(stop[0]))
	require.Equal(t, claudeGroup("/new path/cockpit", ""), stop[1])
	require.Equal(t, "'/new path/cockpit' hook claude", stop[1].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"])
}

func TestMergeStatusLine(t *testing.T) {
	t.Parallel()
	own := "'/new path/cockpit' statusline"
	cases := []struct {
		name         string
		settings     string
		wantLine     map[string]any
		wantOriginal string
		wantReplaced bool
	}{
		{"none", `{}`, map[string]any{"type": "command", "command": own}, "", true},
		{"foreign", `{"statusLine": {"type": "command", "command": "~/.claude/statusline.sh", "padding": 0, "refreshInterval": 1}}`,
			map[string]any{"type": "command", "command": own, "padding": float64(0), "refreshInterval": float64(1)}, "~/.claude/statusline.sh", true},
		{"own", `{"statusLine": {"type": "command", "command": "/old/cockpit statusline", "padding": 2}}`,
			map[string]any{"type": "command", "command": own, "padding": float64(2)}, "/old/cockpit statusline", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			settings := map[string]any{}
			require.NoError(t, json.Unmarshal([]byte(tc.settings), &settings))
			original, replaced := mergeStatusLine(settings, "/new path/cockpit")
			require.Equal(t, tc.wantReplaced, replaced)
			require.Equal(t, tc.wantOriginal, original)
			require.Equal(t, tc.wantLine, settings["statusLine"])
		})
	}
}
