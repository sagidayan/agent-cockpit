package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func codexNotify(exe string) string {
	return fmt.Sprintf("notify = [%q, \"hook\", \"codex\"]", exe)
}

func setCodexNotify(config, line string) (string, error) {
	lines := strings.Split(config, "\n")
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		if !strings.HasPrefix(trimmed, "notify") {
			continue
		}
		if !strings.Contains(trimmed, `"hook", "codex"`) {
			return "", errors.New("config already has a notify program; add cockpit manually: " + line)
		}
		lines[i] = line
		return strings.Join(lines, "\n"), nil
	}
	return line + "\n" + config, nil
}

func codexConfigPath() string {
	dir := os.Getenv("CODEX_HOME")
	if dir == "" {
		dir = filepath.Join(home(), ".codex")
	}
	return filepath.Join(dir, "config.toml")
}

func installCodex(exe string) error {
	path := codexConfigPath()
	data, err := readOptional(path)
	if err != nil {
		return err
	}
	updated, err := setCodexNotify(string(data), codexNotify(exe))
	if err != nil {
		return err
	}
	if err := writeWithBackup(path, data, []byte(updated)); err != nil {
		return err
	}
	done("notify added to " + path)
	_ = command("codex", "mcp", "remove", "cockpit")
	if err := command("codex", "mcp", "add", "cockpit", "--", exe, "mcp"); err != nil {
		return err
	}
	done("MCP server registered (codex mcp)")
	return nil
}
