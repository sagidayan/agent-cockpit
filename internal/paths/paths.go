package paths

import (
	"os"
	"path/filepath"
	"strings"
)

func Expand(p string) string {
	home, _ := os.UserHomeDir()
	if p == "~" || strings.HasPrefix(p, "~/") {
		return home + p[1:]
	}
	return p
}

func Contract(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + p[len(home):]
	}
	return p
}

const chatRoot = "/tmp/cockpit-chats"

func NewChat() (string, error) {
	if err := os.MkdirAll(chatRoot, 0o755); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(chatRoot, "chat-")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(dir)
}

func IsChat(dir string) bool {
	root, err := filepath.EvalSymlinks(chatRoot)
	return err == nil && filepath.Dir(dir) == root
}
