package config

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/sagidayan/agent-cockpit/internal/state"
)

type Config struct {
	Notify Notify `toml:"notify"`
	Claude Claude `toml:"claude"`
	UI     UI     `toml:"ui"`
	Editor Editor `toml:"editor"`
	Spawn  Spawn  `toml:"spawn"`
	Modes  []Mode `toml:"mode"`
}

type Spawn struct {
	MaxDepth int `toml:"max_depth"`
}

type Mode struct {
	Name   string `toml:"name"`
	Title  string `toml:"title"`
	Input  string `toml:"input"`
	Agent  string `toml:"agent"`
	Prompt string `toml:"prompt"`
}

type Editor struct {
	Command string `toml:"command"`
	Width   string `toml:"width"`
}

type UI struct {
	Icons bool `toml:"icons"`
}

type Claude struct {
	Statusline string `toml:"statusline"`
}

type Notify struct {
	Events  []state.Status `toml:"events"`
	Desktop bool           `toml:"desktop"`
	Tmux    bool           `toml:"tmux"`
	Command string         `toml:"command"`
}

func Default() Config {
	return Config{
		Notify: Notify{
			Events:  []state.Status{state.NeedsInput, state.Done},
			Desktop: true,
			Tmux:    true,
		},
		UI:     UI{Icons: true},
		Editor: Editor{Command: "nvim", Width: "50%"},
		Spawn:  Spawn{MaxDepth: 3},
	}
}

func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "agent-cockpit", "config.toml")
}

func Load() (Config, error) {
	c := Default()
	_, err := toml.DecodeFile(Path(), &c)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	return c, err
}

func Write(v any) error {
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.Indent = ""
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func SetClaudeStatusline(command string) error {
	values := map[string]any{}
	if _, err := toml.DecodeFile(Path(), &values); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	claude, _ := values["claude"].(map[string]any)
	if claude == nil {
		claude = map[string]any{}
	}
	claude["statusline"] = command
	values["claude"] = claude
	return Write(values)
}
