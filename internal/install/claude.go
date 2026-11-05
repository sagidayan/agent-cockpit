package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/config"
)

var claudeEvents = []struct{ name, matcher string }{
	{"SessionStart", "startup|resume|clear"},
	{"UserPromptSubmit", ""},
	{"Notification", "permission_prompt|elicitation_dialog"},
	{"Stop", ""},
	{"SessionEnd", ""},
}

const (
	claudeHookArgs       = " hook claude"
	claudeStatuslineArgs = " statusline"
)

func claudeGroup(exe, matcher string) map[string]any {
	g := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": shellQuote(exe) + claudeHookArgs}}}
	if matcher != "" {
		g["matcher"] = matcher
	}
	return g
}

func claudeHooks(exe string) map[string]any {
	hooks := map[string]any{}
	for _, e := range claudeEvents {
		hooks[e.name] = []any{claudeGroup(exe, e.matcher)}
	}
	return hooks
}

func isCockpitGroup(group any) bool {
	g, _ := group.(map[string]any)
	hooks, _ := g["hooks"].([]any)
	for _, h := range hooks {
		cmd, _ := h.(map[string]any)["command"].(string)
		if strings.HasSuffix(cmd, claudeHookArgs) {
			return true
		}
	}
	return false
}

func mergeClaudeHooks(settings map[string]any, exe string) {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, e := range claudeEvents {
		groups, _ := hooks[e.name].([]any)
		var kept []any
		for _, g := range groups {
			if !isCockpitGroup(g) {
				kept = append(kept, g)
			}
		}
		hooks[e.name] = append(kept, claudeGroup(exe, e.matcher))
	}
	settings["hooks"] = hooks
}

func mergeStatusLine(settings map[string]any, exe string) (original string, replaced bool) {
	line, _ := settings["statusLine"].(map[string]any)
	if line == nil {
		line = map[string]any{"type": "command"}
	}
	original, _ = line["command"].(string)
	replaced = !strings.HasSuffix(original, claudeStatuslineArgs)
	line["command"] = shellQuote(exe) + claudeStatuslineArgs
	settings["statusLine"] = line
	return original, replaced
}

func claudeSettingsPath() string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		dir = filepath.Join(home(), ".claude")
	}
	return filepath.Join(dir, "settings.json")
}

func installClaude(exe string) error {
	path := claudeSettingsPath()
	data, err := readOptional(path)
	if err != nil {
		return err
	}
	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	}
	mergeClaudeHooks(settings, exe)
	if original, replaced := mergeStatusLine(settings, exe); replaced {
		if err := config.SetClaudeStatusline(original); err != nil {
			return err
		}
	}
	out, err := marshalJSON(settings)
	if err != nil {
		return err
	}
	if err := writeWithBackup(path, data, out); err != nil {
		return err
	}
	done("hooks and statusLine added to " + path)
	_ = command("claude", "mcp", "remove", "-s", "user", "cockpit")
	if err := command("claude", "mcp", "add", "-s", "user", "cockpit", "--", exe, "mcp"); err != nil {
		return err
	}
	done("MCP server registered (claude mcp, user scope)")
	return nil
}
