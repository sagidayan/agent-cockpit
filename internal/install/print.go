package install

import (
	"encoding/json"
	"fmt"
	"os"
)

func Print(kind string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	switch kind {
	case "claude":
		out, err := json.MarshalIndent(map[string]any{"hooks": claudeHooks(exe)}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println("// Merge into the \"hooks\" key of ~/.claude/settings.json")
		fmt.Println(string(out))
	case "codex":
		fmt.Println("# Add at the top level of ~/.codex/config.toml (before any [table]):")
		fmt.Println(codexNotify(exe))
	case "opencode":
		fmt.Println("// Save as ~/.config/opencode/plugins/agent-cockpit.js and run opencode with --standalone (the shared background service has no TMUX_PANE)")
		fmt.Printf(opencodePlugin, exe)
	default:
		return fmt.Errorf("unknown agent %q (claude|codex|opencode)", kind)
	}
	return nil
}
