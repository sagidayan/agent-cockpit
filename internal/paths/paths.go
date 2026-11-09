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

const (
	chatRoot   = "/tmp/cockpit-chats"
	ReviewRoot = "/tmp/prs"
)

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

func Disposable(dir string) bool {
	return below(dir, "/tmp") || below(dir, os.TempDir())
}

func below(dir, root string) bool {
	resolved, _ := filepath.EvalSymlinks(root)
	for _, r := range []string{filepath.Clean(root), resolved} {
		rel, err := filepath.Rel(r, dir)
		if r != "" && err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, "../") {
			return true
		}
	}
	return false
}
