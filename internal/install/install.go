package install

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/config"
)

var agents = []struct {
	name    string
	install func(exe string) error
}{
	{"claude", installClaude},
	{"codex", installCodex},
	{"opencode", installOpencode},
}

func Run(args []string) error {
	yes := len(args) > 0 && (args[0] == "-y" || args[0] == "--yes")
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		fmt.Println("! tmux not found - cockpit needs it")
	}
	in := bufio.NewReader(os.Stdin)
	ask := func(q string) bool {
		if yes {
			return true
		}
		fmt.Printf("%s [Y/n] ", q)
		answer, _ := in.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		return answer == "" || answer == "y" || answer == "yes"
	}
	if err := writeDefaultConfig(); err != nil {
		fmt.Println("  ✗", err)
	}
	for _, a := range agents {
		if _, err := exec.LookPath(a.name); err != nil {
			fmt.Printf("- %s not installed, skipping\n", a.name)
			continue
		}
		if !ask(fmt.Sprintf("Set up %s?", a.name)) {
			continue
		}
		if err := a.install(exe); err != nil {
			fmt.Println("  ✗", err)
		}
	}
	fmt.Println("\nDone. Restart running agents, then run `cockpit` inside tmux.")
	return nil
}

func writeDefaultConfig() error {
	path := config.Path()
	if _, err := os.Stat(path); err == nil {
		done("config exists at " + path)
		return nil
	}
	if err := config.Write(config.Default()); err != nil {
		return err
	}
	done("config written to " + path)
	return nil
}

func done(msg string) {
	fmt.Println("  ✓", msg)
}

func command(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func configHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home(), ".config")
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func writeWithBackup(path string, old, data []byte) error {
	if bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(old) > 0 {
		if err := os.WriteFile(path+".bak", old, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return buf.Bytes(), err
}

func shellQuote(s string) string {
	if !strings.ContainsAny(s, " '\"$`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
