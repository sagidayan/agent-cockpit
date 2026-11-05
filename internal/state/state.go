package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sagidayan/agent-cockpit/internal/tmux"
)

type Status string

const (
	Working    Status = "working"
	Background Status = "background"
	NeedsInput Status = "needs_input"
	Done       Status = "done"
	Idle       Status = "idle"
)

type Agent struct {
	Pane       string    `json:"pane"`
	Kind       string    `json:"kind"`
	Cwd        string    `json:"cwd"`
	Status     Status    `json:"status"`
	Message    string    `json:"message"`
	Prompt     string    `json:"prompt,omitempty"`
	Transcript string    `json:"transcript,omitempty"`
	Server     string    `json:"server,omitempty"`
	SessionID  string    `json:"session_id,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Stats struct {
	Model        string    `json:"model"`
	ContextUsed  int       `json:"context_used"`
	ContextMax   int       `json:"context_max"`
	CostUSD      float64   `json:"cost_usd"`
	LinesAdded   int       `json:"lines_added"`
	LinesRemoved int       `json:"lines_removed"`
	Server       string    `json:"server,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func root() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "agent-cockpit")
}

func Dir() string {
	return filepath.Join(root(), "agents")
}

func statsDir() string {
	return filepath.Join(root(), "stats")
}

func file(dir, pane string) string {
	return filepath.Join(dir, strings.TrimPrefix(pane, "%")+".json")
}

func path(pane string) string {
	return file(Dir(), pane)
}

func write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func read(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func Save(a Agent) error {
	a.UpdatedAt = time.Now()
	a.Server = tmux.ServerPID()
	return write(path(a.Pane), a)
}

func Load(pane string) (Agent, error) {
	var a Agent
	return a, read(path(pane), &a)
}

func Update(pane string, change func(prev Agent) (Agent, bool)) (prev, cur Agent, saved bool, err error) {
	unlock, err := lock()
	if err != nil {
		return prev, cur, false, err
	}
	defer unlock()
	prev, _ = Load(pane)
	cur, saved = change(prev)
	if !saved {
		return prev, cur, false, nil
	}
	return prev, cur, true, Save(cur)
}

func lock() (func(), error) {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(Dir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil
}

func SaveStats(pane string, s Stats) error {
	s.UpdatedAt = time.Now()
	s.Server = tmux.ServerPID()
	return write(file(statsDir(), pane), s)
}

func LoadStats(pane string) (Stats, bool) {
	var s Stats
	return s, read(file(statsDir(), pane), &s) == nil
}

func List() ([]Agent, error) {
	files, err := filepath.Glob(filepath.Join(Dir(), "*.json"))
	if err != nil {
		return nil, err
	}
	agents := make([]Agent, 0, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var a Agent
		if json.Unmarshal(data, &a) == nil {
			agents = append(agents, a)
		}
	}
	return agents, nil
}

func Remove(pane string) error {
	_ = os.Remove(file(statsDir(), pane))
	return os.Remove(path(pane))
}
